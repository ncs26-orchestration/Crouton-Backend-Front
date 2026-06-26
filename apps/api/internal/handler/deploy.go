package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/compiler"
	"github.com/ncs26-orchestration/solution/apps/api/internal/compiler/bpmn"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/camunda7"
	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// DeployHandler compiles a Workflow IR for the engine of the requested
// connection, pushes it via the engine's own REST API, and returns
// references the UI can link to (Cockpit / Tasklist URLs).
//
// AUP writes exactly one thing back to the engine: the BPMN artifact.
// Identities, task routing, running-state: all owned by Camunda.
type DeployHandler struct {
	logger    *slog.Logger
	engines   *repo.EngineRepo
	validator *ir.Validator
}

func NewDeployHandler(logger *slog.Logger, pg *pgxpool.Pool) (*DeployHandler, error) {
	v, err := ir.NewValidator()
	if err != nil {
		return nil, err
	}
	return &DeployHandler{
		logger:    logger,
		engines:   repo.NewEngineRepo(pg),
		validator: v,
	}, nil
}

type deployRequest struct {
	TenantID       string          `json:"tenant_id,omitempty"`
	EngineID       string          `json:"engine_id,omitempty"`
	IR             json.RawMessage `json:"ir"`
	Start          bool            `json:"start,omitempty"`
	StartVariables map[string]any  `json:"start_variables,omitempty"`
}

type deployResponse struct {
	EngineID            string          `json:"engine_id"`
	DeploymentID        string          `json:"deployment_id"`
	ProcessDefinitionID string          `json:"process_definition_id,omitempty"`
	ProcessKey          string          `json:"process_key,omitempty"`
	CockpitURL          string          `json:"cockpit_url,omitempty"`
	TasklistURL         string          `json:"tasklist_url,omitempty"`
	InstanceID          string          `json:"instance_id,omitempty"`
	ArtifactBytes       int             `json:"artifact_bytes"`
	Diagnostics         []ir.Diagnostic `json:"diagnostics,omitempty"`
}

var slugPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// DeployBPMN handles POST /deploy/camunda.
func (h *DeployHandler) DeployBPMN(c echo.Context) error {
	var req deployRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	tenantID := orDefault(strings.TrimSpace(req.TenantID), "demo")
	engineID := strings.TrimSpace(req.EngineID)
	if len(req.IR) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ir_missing"})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()

	// If the caller did not name an engine, pick the tenant's first
	// camunda7 connection. Keeps the happy path one click in the UI.
	ec, err := h.pickEngine(ctx, tenantID, engineID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if ec.Kind != "camunda7" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("engine %q is kind %q; only camunda7 deploy is wired", ec.ExternalID, ec.Kind),
		})
	}

	// Validate IR + cross-ref against the projected IS before we touch
	// the engine. A cross-ref error is *not* fatal here — unresolved
	// ids are surfaced as warnings and the deploy proceeds. Camunda
	// itself accepts any string for candidateGroups/assignee; tasks
	// pointing at unknown users simply never land in an inbox. We'd
	// rather tell the user than block them.
	wf, schemaDiags, err := h.validator.ValidateWorkflowJSON(req.IR)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ir_unparseable: " + err.Error()})
	}
	if len(schemaDiags) > 0 {
		return c.JSON(http.StatusUnprocessableEntity, deployResponse{Diagnostics: schemaDiags})
	}
	proj, err := h.engines.ReadTenantProjection(ctx, tenantID)
	if err != nil {
		h.logger.Error("read projection", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	registry := buildRegistry(proj)
	crossDiags := h.validator.CrossRef(wf, registry)

	// Lower then compile — same pipeline the /compile/bpmn endpoint
	// runs, so deploy sees the same lowering fixups (synthesized
	// default branches, resolved actor ids) and surfaces the same
	// suggestion diagnostics.
	exe, loweringDiags, err := compiler.Lower(wf, registry)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "lowering_failed", "details": err.Error()})
	}
	artifact, rawCompDiags, err := bpmn.Compile(exe, bpmn.DefaultCamunda7Profile())
	compDiags := toIRDiagnostics(rawCompDiags)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, deployResponse{Diagnostics: append(append(crossDiags, loweringDiags...), compDiags...)})
	}

	// Deploy.
	cli := camunda7.New(ec.Endpoint, ec.AuthUsername, ec.AuthSecret)
	deploymentName := slugify(wf.Metadata.Name)
	depl, err := cli.Deploy(ctx, deploymentName, deploymentName+".bpmn", artifact)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "camunda_deploy_failed", "details": err.Error()})
	}

	resp := deployResponse{
		EngineID:      ec.ExternalID,
		DeploymentID:  depl.ID,
		ArtifactBytes: len(artifact),
		Diagnostics:   append(crossDiags, compDiags...),
	}
	// Pick the first deployed process definition — BPMN files with one
	// process (our only shape in v0.1) always have exactly one.
	for _, pd := range depl.DeployedProcessDefinitions {
		resp.ProcessDefinitionID = pd.ID
		resp.ProcessKey = pd.Key
		break
	}
	resp.CockpitURL, resp.TasklistURL = camundaWebAppURLs(ec.Endpoint, resp.ProcessDefinitionID)

	if req.Start && resp.ProcessKey != "" {
		inst, err := cli.StartInstance(ctx, resp.ProcessKey, normalizeCamundaVars(req.StartVariables))
		if err != nil {
			resp.Diagnostics = append(resp.Diagnostics, ir.Diagnostic{
				Severity: "warning",
				Message:  "deploy ok but start failed: " + err.Error(),
			})
		} else {
			resp.InstanceID = inst.ID
		}
	}

	return c.JSON(http.StatusOK, resp)
}

// DeleteEngine — POST was not rest-y, DELETE is. Removes an engine
// connection and cascades through projected_* via FK ON DELETE CASCADE.
func (h *DeployHandler) DeleteEngine(c echo.Context) error {
	tenantID := tenantFromRequest(c)
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	if err := h.engines.DeleteEngineConnection(ctx, tenantID, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "engine_not_found"})
		}
		h.logger.Error("delete engine", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *DeployHandler) pickEngine(ctx context.Context, tenantID, engineID string) (*repo.EngineConnection, error) {
	if engineID != "" {
		ec, err := h.engines.GetEngineConnection(ctx, tenantID, engineID)
		if err != nil {
			return nil, fmt.Errorf("engine_not_found: %s", engineID)
		}
		return ec, nil
	}
	list, err := h.engines.ListEngineConnections(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("db_error")
	}
	for i := range list {
		if list[i].Kind == "camunda7" && list[i].LastSyncedAt != nil {
			return &list[i], nil
		}
	}
	// Fall back to the first camunda7 even if it never synced (user
	// may have just connected it).
	for i := range list {
		if list[i].Kind == "camunda7" {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("no_camunda7_engine_registered")
}

// camundaWebAppURLs derives Cockpit + Tasklist URLs from the REST
// endpoint. Camunda's Run distribution serves the web apps at the
// same host, under /camunda/app/{cockpit,tasklist}. The compose
// service publishes 8080 → host 8180; we swap that in so the links
// open in the user's browser, not inside the docker network.
func camundaWebAppURLs(restEndpoint, processDefinitionID string) (cockpit, tasklist string) {
	host := hostFromRESTEndpoint(restEndpoint)
	base := fmt.Sprintf("http://%s/camunda/app", host)
	cockpit = base + "/cockpit/default/#/processes"
	if processDefinitionID != "" {
		cockpit = fmt.Sprintf("%s/cockpit/default/#/process-definition/%s", base, processDefinitionID)
	}
	tasklist = base + "/tasklist/default/"
	return
}

// hostFromRESTEndpoint extracts the host:port from a URL like
// "http://camunda7:8080/engine-rest" and rewrites the docker-internal
// service name to the host-published one we know about (camunda7:8080
// → localhost:8180) so the browser can reach it.
func hostFromRESTEndpoint(endpoint string) string {
	e := strings.TrimPrefix(endpoint, "http://")
	e = strings.TrimPrefix(e, "https://")
	if i := strings.Index(e, "/"); i > 0 {
		e = e[:i]
	}
	if strings.HasPrefix(e, "camunda7:") {
		return "localhost:8180"
	}
	return e
}

func normalizeCamundaVars(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		// Callers can pre-wrap as { "value":..., "type":... } or pass
		// scalars and let us guess the Camunda type.
		if m, ok := v.(map[string]any); ok {
			if _, has := m["value"]; has {
				out[k] = m
				continue
			}
		}
		out[k] = map[string]any{"value": v, "type": camundaTypeOf(v)}
	}
	return out
}

// toIRDiagnostics converts []bpmn.Diagnostic to []ir.Diagnostic.
// Even though bpmn.Diagnostic is a Go type-alias of ir.Diagnostic,
// go/types in some toolchain configurations surfaces a spurious
// incompatibility on append; this helper keeps both sides happy.
func toIRDiagnostics(in []bpmn.Diagnostic) []ir.Diagnostic {
	if len(in) == 0 {
		return nil
	}
	out := make([]ir.Diagnostic, len(in))
	for i, d := range in {
		out[i] = ir.Diagnostic(d)
	}
	return out
}

func camundaTypeOf(v any) string {
	switch v.(type) {
	case bool:
		return "Boolean"
	case float64:
		return "Double"
	case int, int32, int64:
		return "Long"
	default:
		return "String"
	}
}

func slugify(s string) string {
	out := slugPattern.ReplaceAllString(strings.ToLower(s), "_")
	out = strings.Trim(out, "_")
	if out == "" {
		return "process"
	}
	return out
}

func buildRegistry(proj *repo.TenantProjection) *ir.ISRegistry {
	// Avoid an import cycle with service package — duplicate the
	// minimal projection conversion here. Kept intentionally small.
	reg := &ir.ISRegistry{TenantID: proj.Tenant}
	engineRef := map[int64]string{}
	for _, ec := range proj.EngineConnections {
		engineRef[ec.ID] = ec.ExternalID
		reg.EngineConnections = append(reg.EngineConnections, ir.EngineConnection{
			ID: ec.ExternalID, Kind: ec.Kind, Endpoint: ec.Endpoint,
		})
	}
	for _, g := range proj.Groups {
		reg.Groups = append(reg.Groups, ir.ISGroup{ID: g.ExternalID, Name: g.DisplayName, EngineRef: engineRef[g.EngineConnectionID]})
	}
	mems := map[string][]string{}
	for _, m := range proj.Memberships {
		mems[m.UserExternalID] = append(mems[m.UserExternalID], m.GroupExternalID)
	}
	for _, u := range proj.Users {
		reg.Users = append(reg.Users, ir.ISUser{
			ID: u.ExternalID, Name: u.DisplayName, Email: u.Email,
			GroupIDs: mems[u.ExternalID], EngineRef: engineRef[u.EngineConnectionID],
		})
	}
	for _, f := range proj.DeployedForms {
		reg.DeployedForms = append(reg.DeployedForms, ir.DeployedForm{FormKey: f.FormKey, EngineRef: engineRef[f.EngineConnectionID]})
	}
	for _, d := range proj.DeclaredSystems {
		reg.Systems = append(reg.Systems, ir.ISSystem{
			ID: d.ExternalID, Name: d.Name, Kind: d.Kind, Endpoint: d.Endpoint, Capabilities: d.Capabilities,
		})
	}
	return reg
}
