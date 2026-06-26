package is

import (
	"context"
	"fmt"

	"github.com/ncs26-orchestration/solution/apps/api/internal/engine"
)

// LDAPSource is the typed stub for an LDAP / Active Directory
// enterprise adapter. The configuration shape is real — it
// documents what the implementation will need — but Discover
// returns ErrNotImplemented until we ship the LDAP search code.
//
// Rationale: the typed stub locks the API surface. Any UI work to
// register LDAP sources can land against this stable interface, and
// the one engineer wiring go-ldap later doesn't have to rename
// fields downstream.
type LDAPSource struct{}

func NewLDAPSource() *LDAPSource { return &LDAPSource{} }

func (s *LDAPSource) Kind() string { return "ldap" }
func (s *LDAPSource) Name() string { return "LDAP / Active Directory" }

// LDAPConfig is the shape Discover expects in Config.Extra. We
// document it here so the Settings UI can build the right form
// fields without hardcoding keys in two places.
type LDAPConfig struct {
	// BaseDN is the tree root to search under, e.g.
	// "dc=example,dc=com".
	BaseDN string

	// UserFilter is the LDAP filter for user lookups, defaulting to
	// "(objectClass=person)". The implementation substitutes
	// {username} with the actual search value.
	UserFilter string

	// GroupFilter, usually "(objectClass=groupOfNames)".
	GroupFilter string

	// MemberAttr identifies which attribute on a group entry lists
	// its members; "member" (groupOfNames) vs "memberUid" (posix).
	MemberAttr string
}

// Parse picks the LDAPConfig out of a generic Source Config. Any
// missing keys fall back to conventional defaults.
func ParseLDAPConfig(cfg Config) LDAPConfig {
	pick := func(k, def string) string {
		if v, ok := cfg.Extra[k]; ok && v != "" {
			return v
		}
		return def
	}
	return LDAPConfig{
		BaseDN:      pick("base_dn", ""),
		UserFilter:  pick("user_filter", "(objectClass=person)"),
		GroupFilter: pick("group_filter", "(objectClass=groupOfNames)"),
		MemberAttr:  pick("member_attr", "member"),
	}
}

func (s *LDAPSource) Discover(ctx context.Context, cfg Config) (*engine.Projection, error) {
	// Validate config up-front so the eventual implementation gets
	// a ready-to-use shape.
	lcfg := ParseLDAPConfig(cfg)
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("ldap: endpoint required")
	}
	if lcfg.BaseDN == "" {
		return nil, fmt.Errorf("ldap: base_dn required")
	}
	// Suppress the unused-variable warnings that this stub otherwise
	// accumulates; real impl consumes these.
	_ = ctx
	_ = lcfg
	return nil, ErrNotImplemented
}
