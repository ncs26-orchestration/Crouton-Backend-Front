package is_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Noussour/aup/apps/api/internal/is"
)

// TestRegistry verifies the lookup + listing behaviour. This is the
// minimum contract downstream code relies on (server.go wiring, UI
// /is/sources route in a future slice).
func TestRegistry(t *testing.T) {
	r := is.NewRegistry()
	r.Register(is.NewCamunda7Source())
	r.Register(is.NewLDAPSource())

	got, err := r.Get("ldap")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Kind() != "ldap" {
		t.Errorf("kind: want ldap, got %q", got.Kind())
	}

	if _, err := r.Get("never"); !errors.Is(err, is.ErrUnknownKind) {
		t.Errorf("expected ErrUnknownKind, got %v", err)
	}

	listing := r.List()
	if len(listing) != 2 {
		t.Fatalf("want 2 entries, got %d", len(listing))
	}
	// Sorted by kind: camunda7 < ldap.
	if listing[0].Kind != "camunda7" || listing[1].Kind != "ldap" {
		t.Errorf("listing order: %+v", listing)
	}
}

func TestLDAP_DiscoverReturnsNotImplemented(t *testing.T) {
	s := is.NewLDAPSource()
	cfg := is.Config{
		Endpoint: "ldap://example.com",
		Extra:    map[string]string{"base_dn": "dc=example,dc=com"},
	}
	_, err := s.Discover(context.Background(), cfg)
	if !errors.Is(err, is.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
}

func TestLDAP_ValidatesConfigBeforeStubbing(t *testing.T) {
	s := is.NewLDAPSource()
	// Missing endpoint — the stub should reject even before falling
	// through to ErrNotImplemented, so the UI gets a useful error.
	_, err := s.Discover(context.Background(), is.Config{})
	if err == nil {
		t.Fatal("expected error on missing endpoint")
	}
	if errors.Is(err, is.ErrNotImplemented) {
		t.Errorf("config validation should happen before ErrNotImplemented")
	}
}

func TestParseLDAPConfig_Defaults(t *testing.T) {
	cfg := is.ParseLDAPConfig(is.Config{})
	if cfg.UserFilter != "(objectClass=person)" {
		t.Errorf("UserFilter default: %q", cfg.UserFilter)
	}
	if cfg.MemberAttr != "member" {
		t.Errorf("MemberAttr default: %q", cfg.MemberAttr)
	}
}
