package is

import (
	"context"

	"github.com/ncs26-orchestration/solution/apps/api/internal/engine"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/camunda7"
)

// Camunda7Source exposes a running Camunda 7 engine as an identity
// source. This is the cheapest source to ship because the engine
// adapter already knows how to pull users, groups, and
// memberships — we just wrap it to satisfy the Source interface.
//
// When a tenant registers a Camunda engine today, AIOS gets both
// "where to deploy" and "where identities live" in one connection.
// Future sources (LDAP, SCIM) decouple those.
type Camunda7Source struct {
	adapter *camunda7.Adapter
}

func NewCamunda7Source() *Camunda7Source {
	return &Camunda7Source{adapter: camunda7.NewAdapter()}
}

func (s *Camunda7Source) Kind() string { return "camunda7" }
func (s *Camunda7Source) Name() string { return "Camunda 7 identity" }

func (s *Camunda7Source) Discover(ctx context.Context, cfg Config) (*engine.Projection, error) {
	return s.adapter.Discover(ctx, cfg.Endpoint, cfg.AuthUser, cfg.AuthSecret)
}
