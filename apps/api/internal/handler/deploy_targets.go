package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/compiler"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/elsa3"
	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// DeployTargetsHandler owns the per-project /deploy-targets CRUD and
// the chat-scoped /chats/:id/deploy dispatcher. It compiles the
// chat's latest IR and pushes the artifact through the adapter
// registry — one handler, both engines.
type DeployTargetsHandler struct {
	logger    *slog.Logger
	pg        *pgxpool.Pool
	targets   *repo.DeployTargetRepo
	chats     *repo.ChatRepo
	versions  *repo.WorkflowVersionRepo
	adapters  *engine.Registry
	validator *ir.Validator
}

func NewDeployTargetsHandler(logger *slog.Logger, pg *pgxpool.Pool, adapters *engine.Registry) *DeployTargetsHandler {
	v, err := ir.NewValidator()
	if err != nil {
		// Same posture as the compile handler — a busted embedded schema
		// is a build-time bug, not something to recover from.
		panic(err)
	}
	return &DeployTargetsHandler{
		logger:    logger,
		pg:        pg,
		targets:   repo.NewDeployTargetRepo(pg),
		chats:     repo.NewChatRepo(pg),
		versions:  repo.NewWorkflowVersionRepo(pg),
		adapters:  adapters,
		validator: v,
	}
}

type deployTargetRequest struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	AuthKind   string `json:"auth_kind"`
	AuthUser   string `json:"auth_user"`
	AuthSecret string `json:"auth_secret"`
}

type deployTargetResponse struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	AuthKind  string    `json:"auth_kind"`
	AuthUser  string    `json:"auth_user"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *DeployTargetsHandler) Create(c echo.Context) error {
	projectID := c.Param("id")
	var req deployTargetRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	kind := strings.TrimSpace(req.Kind)
	if kind != "camunda7" && kind != "elsa3" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported_kind"})
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = kind
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "endpoint_required"})
	}
	authKind := strings.TrimSpace(req.AuthKind)
	if authKind == "" {
		authKind = "none"
	}

	t := repo.DeployTarget{
		ID:         makeID("dt", name),
		ProjectID:  projectID,
		Kind:       kind,
		Name:       name,
		Endpoint:   endpoint,
		AuthKind:   authKind,
		AuthUser:   strings.TrimSpace(req.AuthUser),
		AuthSecret: req.AuthSecret,
	}
	if err := h.targets.Create(c.Request().Context(), t); err != nil {
		h.logger.Error("create deploy target", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.JSON(http.StatusCreated, deployTargetResponse{
		ID: t.ID, ProjectID: t.ProjectID, Kind: t.Kind, Name: t.Name,
		Endpoint: t.Endpoint, AuthKind: t.AuthKind, AuthUser: t.AuthUser,
		CreatedAt: time.Now(),
	})
}

func (h *DeployTargetsHandler) List(c echo.Context) error {
	ts, err := h.targets.ListByProject(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	out := make([]deployTargetResponse, 0, len(ts))
	for _, t := range ts {
		out = append(out, deployTargetResponse{
			ID: t.ID, ProjectID: t.ProjectID, Kind: t.Kind, Name: t.Name,
			Endpoint: t.Endpoint, AuthKind: t.AuthKind, AuthUser: t.AuthUser,
			CreatedAt: t.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"deploy_targets": out})
}

func (h *DeployTargetsHandler) Delete(c echo.Context) error {
	if err := h.targets.Delete(c.Request().Context(), c.Param("id")); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.NoContent(http.StatusNoContent)
}

// --- /chats/:id/deploy ---

type deployChatRequest struct {
	TargetID string `json:"target_id"`
}

type deployChatResponse struct {
	TargetID            string          `json:"target_id"`
	Kind                string          `json:"kind"`
	DeploymentID        string          `json:"deployment_id"`
	ProcessDefinitionID string          `json:"process_definition_id,omitempty"`
	ProcessKey          string          `json:"process_key,omitempty"`
	ArtifactBytes       int             `json:"artifact_bytes"`
	Diagnostics         []ir.Diagnostic `json:"diagnostics,omitempty"`
	// StudioURL is a browser-openable link to the engine's UI for
	// this just-deployed definition — Elsa Studio's detail page
	// today; Camunda Cockpit can join here in a follow-up. Omitted
	// when the adapter can't construct a meaningful URL.
	StudioURL string `json:"studio_url,omitempty"`
}

// DeployChat compiles the chat's latest workflow and pushes it to the
// named deploy target. Target + chat must belong to the same project
// (otherwise a user could cross-deploy between client companies).
//
// The adapter's Compile + Deploy do all the engine-specific work; this
// handler is just the glue: load target, load IR, lower, compile,
// deploy, return.
func (h *DeployTargetsHandler) DeployChat(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 60*time.Second)
	defer cancel()
	chatID := c.Param("id")

	var req deployChatRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	if strings.TrimSpace(req.TargetID) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "target_id_required"})
	}

	chat, err := h.chats.Get(ctx, chatID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "chat_not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	if chat.LatestWorkflowVersionID == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no_workflow_yet"})
	}
	target, err := h.targets.Get(ctx, req.TargetID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "target_not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	if target.ProjectID != chat.ProjectID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "target_project_mismatch"})
	}
	adapter, err := h.adapters.Get(target.Kind)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unknown_adapter_kind: " + target.Kind})
	}
	if !adapter.Capabilities().CanDeploy {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "adapter_cannot_deploy: " + target.Kind})
	}

	version, err := h.versions.Get(ctx, *chat.LatestWorkflowVersionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}

	// Validate + lower + compile. Operator-tool mode has no grounding
	// registry, so we pass an empty ISRegistry through — the lowering
	// pass degrades gracefully (no cross-ref fixups, but still emits
	// default-branch synthesis and condition normalization).
	wf, schemaDiags, err := h.validator.ValidateWorkflowJSON(version.IRJSON)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{
			"error":       "ir_unparseable",
			"details":     err.Error(),
			"diagnostics": schemaDiags,
		})
	}
	if len(schemaDiags) > 0 {
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{
			"error":       "ir_schema_errors",
			"diagnostics": schemaDiags,
		})
	}
	exe, loweringDiags, err := compiler.Lower(wf, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":   "lowering_failed",
			"details": err.Error(),
		})
	}
	artifact, _, compDiags, err := adapter.Compile(exe)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]any{
			"error":       "compile_failed",
			"details":     err.Error(),
			"diagnostics": append(loweringDiags, compDiags...),
		})
	}

	// For Elsa, thread the target's AuthKind through the authUser slot
	// (see elsa3.splitAuthSpec). Camunda only uses Basic, so we pass
	// user/secret as-is and let the adapter's client compose it.
	authKind := target.AuthKind
	authUser := target.AuthUser
	authSecret := target.AuthSecret
	if target.Kind == "elsa3" {
		authKind, authUser, authSecret = normalizeElsaDeployAuth(target.Endpoint, authKind, authUser, authSecret)
		if authKind != "" && authKind != "none" {
			authUser = authKind + ":" + authUser
		}
	}
	result, err := adapter.Deploy(ctx, target.Endpoint, authUser, authSecret, slugify(chat.Title), artifact)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{
			"error":   fmt.Sprintf("%s_deploy_failed", target.Kind),
			"details": err.Error(),
		})
	}

	// Engine-specific browser link. Elsa Studio shows the definition
	// detail view at /workflow-definitions/<id>; Camunda's Cockpit
	// URL computation already lives in deploy.go (camundaWebAppURLs)
	// and will join here when we unify the two deploy paths.
	studioURL := ""
	if target.Kind == "elsa3" {
		studioURL = elsa3.StudioURL(target.Endpoint, result.ProcessKey)
	} else if target.Kind == "camunda7" {
		studioURL, _ = camundaWebAppURLs(target.Endpoint, result.ProcessDefinitionID)
	}

	return c.JSON(http.StatusOK, deployChatResponse{
		TargetID:            target.ID,
		Kind:                target.Kind,
		DeploymentID:        result.DeploymentID,
		ProcessDefinitionID: result.ProcessDefinitionID,
		ProcessKey:          result.ProcessKey,
		ArtifactBytes:       len(artifact),
		Diagnostics:         append(loweringDiags, compDiags...),
		StudioURL:           studioURL,
	})
}

// --- helpers ---

// silence unused import warnings on platforms where json/silencing
// different slices. Kept trivial — the package uses json via
// encoding/json elsewhere.
var _ = json.RawMessage(nil)

func normalizeElsaDeployAuth(endpoint, authKind, authUser, authSecret string) (string, string, string) {
	if authKind != "" && authKind != "none" {
		return authKind, authUser, authSecret
	}
	if strings.Contains(endpoint, "://elsa3:8080") || strings.Contains(endpoint, "://localhost:8280") {
		return "credentials", "admin", "password"
	}
	return authKind, authUser, authSecret
}
