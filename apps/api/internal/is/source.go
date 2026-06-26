// Package is defines the Enterprise Adapter contract — AUP's
// mechanism for projecting identity, organizational structure, and
// document metadata out of existing enterprise systems (LDAP,
// Active Directory, SCIM, an OIDC IdP, an ECM like OpenBee, …).
//
// This package is a peer to internal/engine/. Engines ship workflow
// execution; enterprise sources ship the IS data AUP grounds its
// extractions in. They share the same shape of output
// (engine.Projection) so the caching layer doesn't care who
// produced a row.
//
// v0.1 ships two implementations:
//
//	camunda7_source.go — wraps the Camunda 7 engine adapter's
//	                     existing Discover method, so a Camunda
//	                     engine doubles as an identity source.
//	ldap_source.go     — a typed stub. The config is real; the
//	                     Discover call returns ErrNotImplemented
//	                     until we ship an LDAP search client.
//
// Future implementations drop in against this same interface — an
// OIDC/OpenID Connect source, a SCIM source, a SharePoint /
// OpenBee catalog source.
package is

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/ncs26-orchestration/solution/apps/api/internal/engine"
)

// ErrNotImplemented is returned by Source adapters whose Discover
// implementation isn't wired yet. Callers use errors.Is to
// distinguish "source doesn't support discovery" from a transient
// network failure.
var ErrNotImplemented = errors.New("is: source not implemented yet")

// ErrUnknownKind is returned by Registry.Get when the requested
// source kind isn't registered.
var ErrUnknownKind = errors.New("is: unknown source kind")

// Source is the contract every enterprise identity/org source
// implements. Discover returns an engine.Projection so the repo
// layer can upsert rows without branching on source type.
type Source interface {
	// Kind is a stable short identifier ("camunda7", "ldap",
	// "scim", "oidc", ...).
	Kind() string

	// Name is a human-readable label for the UI source picker.
	Name() string

	// Discover fetches users, groups, memberships, and optionally
	// deployed forms. Config is opaque here — each source package
	// defines the concrete shape via its own constructor.
	Discover(ctx context.Context, cfg Config) (*engine.Projection, error)
}

// Config is a loose envelope for source configuration. Fields
// mirror what the repo's engine_connections table holds today so
// non-engine sources can reuse the same storage without a
// migration. Unused fields stay zero.
type Config struct {
	Endpoint   string            // URL for REST/LDAP
	AuthUser   string            // bind DN / API user
	AuthSecret string            // password / token
	Extra      map[string]string // per-source custom knobs (BaseDN, UserFilter, ...)
}

// Registry is the thread-safe lookup for registered sources. Mirrors
// engine.Registry so wiring code looks identical across the two
// plugin dimensions.
type Registry struct {
	mu      sync.RWMutex
	sources map[string]Source
}

func NewRegistry() *Registry {
	return &Registry{sources: map[string]Source{}}
}

func (r *Registry) Register(s Source) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sources[s.Kind()]; exists {
		panic("is: duplicate source registration for kind " + s.Kind())
	}
	r.sources[s.Kind()] = s
}

func (r *Registry) Get(kind string) (Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sources[kind]
	if !ok {
		return nil, ErrUnknownKind
	}
	return s, nil
}

type Listing struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (r *Registry) List() []Listing {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Listing, 0, len(r.sources))
	for _, s := range r.sources {
		out = append(out, Listing{Kind: s.Kind(), Name: s.Name()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}
