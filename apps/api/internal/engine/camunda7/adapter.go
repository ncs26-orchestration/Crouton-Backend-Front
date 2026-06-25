package camunda7

import (
	"context"
	"fmt"
	"strings"

	"github.com/Noussour/aup/apps/api/internal/compiler/bpmn"
	"github.com/Noussour/aup/apps/api/internal/engine"
	"github.com/Noussour/aup/apps/api/internal/ir"
)

// Adapter is the engine.Adapter implementation for Camunda 7. It
// glues the existing Client (identity discovery + deployment over
// the engine-rest API) to the generic adapter contract, so the HTTP
// layer never imports this package directly.
type Adapter struct{}

// NewAdapter returns a ready-to-register Camunda 7 adapter.
// Stateless; safe to register once at boot. The name is
// deliberately different from this package's existing `New` (which
// constructs a Client) to avoid an identifier collision.
func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) Kind() string { return "camunda7" }

func (a *Adapter) Name() string { return "Camunda 7" }

func (a *Adapter) Capabilities() engine.Capabilities {
	return engine.Capabilities{
		CanDiscover:  true,
		CanDeploy:    true,
		ArtifactMime: "application/xml",
		ArtifactExt:  "bpmn",
	}
}

// Compile uses the existing bpmn package to emit Camunda 7 XML. The
// ExecutableIR is already lowered, so the compiler doesn't need to
// do actor resolution or default-branch synthesis itself.
func (a *Adapter) Compile(exe *ir.ExecutableIR) ([]byte, string, []ir.Diagnostic, error) {
	artifact, diags, err := bpmn.Compile(exe, bpmn.DefaultCamunda7Profile())
	return artifact, "application/xml", diags, err
}

// Discover pulls users, groups, and memberships from the engine-rest
// API. Uses the existing Client, so this method is a thin mapping
// between shapes — no HTTP logic lives here.
func (a *Adapter) Discover(ctx context.Context, endpoint, authUser, authSecret string) (*engine.Projection, error) {
	c := New(endpoint, authUser, authSecret)

	users, err := c.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	groups, err := c.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	// Camunda's identity service doesn't return memberships on the
	// user record; query per-group and build the reverse map.
	userGroups := map[string][]string{}
	for _, g := range groups {
		memberIDs, err := c.ListGroupMemberIDs(ctx, g.ID)
		if err != nil {
			return nil, fmt.Errorf("list members of %s: %w", g.ID, err)
		}
		for _, uid := range memberIDs {
			userGroups[uid] = append(userGroups[uid], g.ID)
		}
	}

	proj := &engine.Projection{
		Users:  make([]ir.ISUser, 0, len(users)),
		Groups: make([]ir.ISGroup, 0, len(groups)),
	}
	for _, u := range users {
		proj.Users = append(proj.Users, ir.ISUser{
			ID:       u.ID,
			Name:     strings.TrimSpace(u.FirstName + " " + u.LastName),
			Email:    u.Email,
			GroupIDs: userGroups[u.ID],
		})
	}
	for _, g := range groups {
		proj.Groups = append(proj.Groups, ir.ISGroup{
			ID:   g.ID,
			Name: g.Name,
		})
	}
	return proj, nil
}

// Deploy pushes the compiled BPMN XML via the deployment REST API and
// maps the response to engine.DeploymentResult. The first entry of
// DeployedProcessDefinitions is the "primary" process definition —
// Camunda returns a single map entry for a single-process deployment,
// which is the only shape AUP produces in v0.1.
func (a *Adapter) Deploy(ctx context.Context, endpoint, authUser, authSecret, deploymentName string, artifact []byte) (engine.DeploymentResult, error) {
	c := New(endpoint, authUser, authSecret)
	fileName := deploymentName + ".bpmn"
	depl, err := c.Deploy(ctx, deploymentName, fileName, artifact)
	if err != nil {
		return engine.DeploymentResult{}, err
	}
	out := engine.DeploymentResult{DeploymentID: depl.ID}
	for _, pd := range depl.DeployedProcessDefinitions {
		out.ProcessDefinitionID = pd.ID
		out.ProcessKey = pd.Key
		break
	}
	return out, nil
}
