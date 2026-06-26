package elsa3

import (
	"context"
	"fmt"

	"github.com/ncs26-orchestration/solution/apps/api/internal/engine"
	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// Adapter is the engine.Adapter for Elsa 3. Compile + Deploy are
// wired; Discover returns ErrNotSupported because Elsa's identity
// plane is pluggable (OIDC/JWT etc.) and a generic discovery pass
// would mismatch most deployments. The AIOS operator tool grounds in
// chat context, not the engine's identity store, so skipping it
// doesn't hurt the user flow.
type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) Kind() string { return "elsa3" }

func (a *Adapter) Name() string { return "Elsa 3" }

func (a *Adapter) Capabilities() engine.Capabilities {
	return engine.Capabilities{
		CanDiscover:  false,
		CanDeploy:    true,
		ArtifactMime: "application/json",
		ArtifactExt:  "json",
	}
}

func (a *Adapter) Compile(exe *ir.ExecutableIR) ([]byte, string, []ir.Diagnostic, error) {
	artifact, diags, err := Compile(exe)
	return artifact, "application/json", diags, err
}

func (a *Adapter) Discover(ctx context.Context, endpoint, authUser, authSecret string) (*engine.Projection, error) {
	return nil, engine.ErrNotSupported
}

// Deploy pushes the compiled WorkflowDefinition JSON to an Elsa Server
// via POST /elsa/api/workflow-definitions. The adapter's generic
// contract only carries one auth credential per field — authUser maps
// to HTTP Basic user, authSecret maps to API key / bearer / basic pw.
// The DeployTarget's AuthKind selects which mode the Client uses; we
// surface it here via the authUser field as a "<kind>:<user>" prefix
// so the adapter interface doesn't need to change. When authUser is
// plain (no colon), we default to ApiKey mode, which matches Elsa
// Server's out-of-box auth scheme.
func (a *Adapter) Deploy(ctx context.Context, endpoint, authUser, authSecret, deploymentName string, artifact []byte) (engine.DeploymentResult, error) {
	authKind, user := splitAuthSpec(authUser)
	client := NewClient(endpoint, authKind, user, authSecret)
	r, err := client.Deploy(ctx, artifact)
	if err != nil {
		return engine.DeploymentResult{}, fmt.Errorf("elsa deploy: %w", err)
	}
	return engine.DeploymentResult{
		DeploymentID:        r.ID,
		ProcessDefinitionID: r.ID,
		ProcessKey:          r.DefinitionID,
	}, nil
}

// splitAuthSpec peels a "kind:user" prefix off the authUser field.
// An unqualified value is interpreted as ApiKey mode (user ignored).
func splitAuthSpec(spec string) (kind, user string) {
	for i := 0; i < len(spec); i++ {
		if spec[i] == ':' {
			return spec[:i], spec[i+1:]
		}
	}
	if spec == "" {
		return "apikey", ""
	}
	// Plain value — treat the whole thing as the username under basic
	// auth only if it looks like "user@...". Otherwise assume apikey.
	return "apikey", spec
}
