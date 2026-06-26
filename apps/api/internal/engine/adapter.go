// Package engine defines the Adapter contract every workflow-engine
// backend implements, and the registry that the HTTP layer queries
// when it needs to route a compile/discover/deploy call.
//
// An adapter encapsulates three things per engine kind:
//
//  1. how to turn an ExecutableIR into the engine's native artifact
//     (Compile — XML for Camunda, JSON for Elsa, ...);
//
//  2. how to discover identities and existing deployments on a
//     running instance (Discover);
//
//  3. how to hand that artifact to the live engine (Deploy).
//
// Discover and Deploy are optional per adapter — some engines (Elsa
// in the 48h scope) ship with Compile only; the registry exposes
// capability flags so the UI can disable buttons accordingly.
package engine

import (
	"context"

	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
)

// Adapter is the one shape every engine backend implements.
type Adapter interface {
	// Kind is a stable short identifier ("camunda7", "elsa3", ...).
	// Also the path segment at /compile/:target and /deploy/:target.
	Kind() string

	// Name is a human-readable label shown in the UI target selector.
	Name() string

	// Capabilities describes what this adapter can do, so the UI can
	// enable/disable Deploy buttons without calling the adapter.
	Capabilities() Capabilities

	// Compile turns a lowered ExecutableIR into the engine's native
	// deployable artifact. Diagnostics may be emitted even on
	// success (for lossy lowerings); a non-nil err means compile
	// was refused.
	Compile(exe *ir.ExecutableIR) (artifact []byte, mime string, diags []ir.Diagnostic, err error)

	// Discover fetches users/groups/deployed-forms from a live engine
	// endpoint. Adapters without identity discovery return
	// (nil, ErrNotSupported).
	Discover(ctx context.Context, endpoint, authUser, authSecret string) (*Projection, error)

	// Deploy uploads the compiled artifact to the engine. Adapters
	// without deploy support return (DeploymentResult{}, ErrNotSupported).
	Deploy(ctx context.Context, endpoint, authUser, authSecret, deploymentName string, artifact []byte) (DeploymentResult, error)
}

// Capabilities flags which optional Adapter methods are supported.
// Keep this a struct (not a []string) so the JSON is stable.
type Capabilities struct {
	CanDiscover bool `json:"can_discover"`
	CanDeploy   bool `json:"can_deploy"`
	// ArtifactMime is the Content-Type the adapter's Compile returns,
	// handed to the UI's download-file logic.
	ArtifactMime string `json:"artifact_mime"`
	// ArtifactExt is the file extension used when offering a download
	// ("bpmn", "json").
	ArtifactExt string `json:"artifact_ext"`
}

// Projection is the subset of the IS Registry that a discovery pass
// can populate. Returned in an engine-neutral shape so the service
// layer can merge into the projection tables without knowing which
// adapter produced the rows.
type Projection struct {
	Users         []ir.ISUser       `json:"users"`
	Groups        []ir.ISGroup      `json:"groups"`
	DeployedForms []ir.DeployedForm `json:"deployed_forms,omitempty"`
}

// DeploymentResult is returned by Deploy. The IDs come from the
// engine's response — AIOS records them in its Deployment table.
type DeploymentResult struct {
	DeploymentID        string `json:"deployment_id"`
	ProcessDefinitionID string `json:"process_definition_id,omitempty"`
	ProcessKey          string `json:"process_key,omitempty"`
}
