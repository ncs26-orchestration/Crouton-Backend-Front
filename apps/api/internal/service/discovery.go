// Package service wires engine-client + repo into the discovery flow.
// Nothing in this package writes to an engine — AIOS's stance is
// "read-only against the engine, authoritative only over its own
// owned entities".
package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/camunda7"
	"github.com/ncs26-orchestration/solution/apps/api/internal/ir"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// Discovery handles the sync flow: given an engine connection, pull
// users / groups / memberships / deployed forms from the engine and
// write the whole snapshot into the projection.
type Discovery struct {
	Engines *repo.EngineRepo
}

func NewDiscovery(engines *repo.EngineRepo) *Discovery {
	return &Discovery{Engines: engines}
}

// Sync runs a full discovery cycle for one engine connection. It
// returns the freshly-written projection so the caller (HTTP handler)
// can confirm what landed in AIOS's cache.
//
// This is the only writer against projected_* tables. Every call
// replaces the entire projection for that engine connection: the
// snapshot semantics are simple and keep AIOS's view tightly aligned
// with the engine's current state.
func (d *Discovery) Sync(ctx context.Context, ec *repo.EngineConnection) (int /*users*/, int /*groups*/, int /*forms*/, error) {
	switch ec.Kind {
	case "camunda7":
		return d.syncCamunda7(ctx, ec)
	default:
		return 0, 0, 0, fmt.Errorf("discovery not implemented for engine kind %q", ec.Kind)
	}
}

func (d *Discovery) syncCamunda7(ctx context.Context, ec *repo.EngineConnection) (int, int, int, error) {
	// Secrets are session-scoped in v0.1 and may be blank after a
	// restart. Callers typically pass secrets directly via the request
	// shape; when missing, the client runs anonymously which works
	// fine for Camunda distributions that have auth disabled.
	cli := camunda7.New(ec.Endpoint, ec.AuthUsername, ec.AuthSecret)

	users, err := cli.ListUsers(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("list users: %w", err)
	}
	groups, err := cli.ListGroups(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("list groups: %w", err)
	}

	var members []repo.ProjectedGroupMember
	for _, g := range groups {
		ids, err := cli.ListGroupMemberIDs(ctx, g.ID)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("list members for group %s: %w", g.ID, err)
		}
		for _, uid := range ids {
			members = append(members, repo.ProjectedGroupMember{
				EngineConnectionID: ec.ID,
				GroupExternalID:    g.ID,
				UserExternalID:     uid,
			})
		}
	}

	// Forms: scan deployments, keep resources that look like forms.
	// Camunda's built-in concept of a form is either (a) an embedded
	// BPMN formKey, which AIOS emits directly from the IR and does not
	// need to project back, or (b) a deployed .form file. For v0.1 we
	// pick up (b) only; (a) is derived from IR at compile time.
	deployments, err := cli.ListDeployments(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("list deployments: %w", err)
	}
	var forms []repo.ProjectedForm
	for _, dep := range deployments {
		resources, err := cli.ListDeploymentResources(ctx, dep.ID)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("list resources for deployment %s: %w", dep.ID, err)
		}
		for _, r := range resources {
			if !strings.HasSuffix(strings.ToLower(r.Name), ".form") {
				continue
			}
			// Form key convention in Camunda 7: "camunda-forms:deployment:<name>".
			forms = append(forms, repo.ProjectedForm{
				EngineConnectionID: ec.ID,
				FormKey:            "camunda-forms:deployment:" + r.Name,
				SourceResource:     dep.Name,
			})
		}
	}

	// Translate to projection rows.
	pu := make([]repo.ProjectedUser, 0, len(users))
	for _, u := range users {
		pu = append(pu, repo.ProjectedUser{
			EngineConnectionID: ec.ID,
			ExternalID:         u.ID,
			DisplayName:        u.DisplayName(),
			Email:              u.Email,
		})
	}
	pg := make([]repo.ProjectedGroup, 0, len(groups))
	for _, g := range groups {
		pg = append(pg, repo.ProjectedGroup{
			EngineConnectionID: ec.ID,
			ExternalID:         g.ID,
			DisplayName:        g.Name,
		})
	}

	if err := d.Engines.ReplaceProjection(ctx, ec.ID, pu, pg, members, forms); err != nil {
		return 0, 0, 0, fmt.Errorf("replace projection: %w", err)
	}
	return len(pu), len(pg), len(forms), nil
}

// BuildISRegistry reads the full tenant projection and converts it into
// the ir.ISRegistry shape the validator and extractor consume. User
// group_ids are aggregated across engine connections so the extractor
// can answer "which groups does john belong to" in one step.
func BuildISRegistry(proj *repo.TenantProjection) *ir.ISRegistry {
	reg := &ir.ISRegistry{TenantID: proj.Tenant}

	// Engine connections.
	for _, ec := range proj.EngineConnections {
		var last string
		if ec.LastSyncedAt != nil {
			last = ec.LastSyncedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		reg.EngineConnections = append(reg.EngineConnections, ir.EngineConnection{
			ID:           ec.ExternalID,
			Kind:         ec.Kind,
			Endpoint:     ec.Endpoint,
			LastSyncedAt: last,
		})
	}

	// Groups first so we can attach group_ids to users.
	engineRefByID := map[int64]string{}
	for _, ec := range proj.EngineConnections {
		engineRefByID[ec.ID] = ec.ExternalID
	}
	for _, g := range proj.Groups {
		reg.Groups = append(reg.Groups, ir.ISGroup{
			ID:        g.ExternalID,
			Name:      g.DisplayName,
			EngineRef: engineRefByID[g.EngineConnectionID],
		})
	}

	// Users with membership arrays.
	memberships := map[string][]string{} // user_external_id -> [group_external_id]
	for _, m := range proj.Memberships {
		memberships[m.UserExternalID] = append(memberships[m.UserExternalID], m.GroupExternalID)
	}
	for _, u := range proj.Users {
		reg.Users = append(reg.Users, ir.ISUser{
			ID:        u.ExternalID,
			Name:      u.DisplayName,
			Email:     u.Email,
			GroupIDs:  memberships[u.ExternalID],
			EngineRef: engineRefByID[u.EngineConnectionID],
		})
	}

	// Deployed forms.
	for _, f := range proj.DeployedForms {
		reg.DeployedForms = append(reg.DeployedForms, ir.DeployedForm{
			FormKey:   f.FormKey,
			EngineRef: engineRefByID[f.EngineConnectionID],
		})
	}

	// Declared systems.
	for _, ds := range proj.DeclaredSystems {
		reg.Systems = append(reg.Systems, ir.ISSystem{
			ID:           ds.ExternalID,
			Name:         ds.Name,
			Kind:         ds.Kind,
			Endpoint:     ds.Endpoint,
			Capabilities: ds.Capabilities,
		})
	}

	return reg
}
