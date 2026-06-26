package camunda7_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/camunda7"
)

// newMockEngine spins up an httptest server that responds with canned
// payloads for Camunda 7's REST endpoints. Tests register handlers
// per path.
func newMockEngine(t *testing.T, routes map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range routes {
		mux.HandleFunc(path, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_ListUsers(t *testing.T) {
	// Camunda's GET /user returns a JSON array of UserProfileDto.
	srv := newMockEngine(t, map[string]http.HandlerFunc{
		"/engine-rest/user": func(w http.ResponseWriter, r *http.Request) {
			// verify query params set by the client
			q := r.URL.Query()
			if q.Get("firstResult") == "" || q.Get("maxResults") == "" {
				t.Errorf("expected pagination query params, got %v", q)
			}
			_ = json.NewEncoder(w).Encode([]camunda7.User{
				{ID: "demo", FirstName: "Demo", LastName: "User", Email: "demo@example.com"},
				{ID: "john", FirstName: "John", LastName: "Doe"},
				{ID: "mary", FirstName: "Mary", LastName: "Anne"},
				{ID: "peter"},
			})
		},
	})

	c := camunda7.New(srv.URL+"/engine-rest", "", "")
	users, err := c.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 4 {
		t.Fatalf("want 4 users, got %d", len(users))
	}
	if users[1].DisplayName() != "John Doe" {
		t.Errorf("DisplayName for john: want 'John Doe', got %q", users[1].DisplayName())
	}
	if users[3].DisplayName() != "peter" {
		t.Errorf("DisplayName fallback should be the id when no name fields are set; got %q", users[3].DisplayName())
	}
}

func TestClient_ListGroups(t *testing.T) {
	srv := newMockEngine(t, map[string]http.HandlerFunc{
		"/engine-rest/group": func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode([]camunda7.Group{
				{ID: "sales", Name: "Sales", Type: "WORKFLOW"},
				{ID: "accounting", Name: "Accounting", Type: "WORKFLOW"},
				{ID: "management", Name: "Management", Type: "WORKFLOW"},
				{ID: "camunda-admin", Name: "Camunda Admin", Type: "SYSTEM"},
			})
		},
	})

	c := camunda7.New(srv.URL+"/engine-rest", "", "")
	groups, err := c.ListGroups(context.Background())
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 4 {
		t.Fatalf("want 4 groups, got %d", len(groups))
	}
}

func TestClient_ListGroupMemberIDs(t *testing.T) {
	srv := newMockEngine(t, map[string]http.HandlerFunc{
		"/engine-rest/user": func(w http.ResponseWriter, r *http.Request) {
			// The client hits /user?memberOfGroup=<group>
			q := r.URL.Query()
			want := q.Get("memberOfGroup")
			if want == "" {
				t.Errorf("expected memberOfGroup query param")
			}
			// Return Camunda's canonical memberships: john -> sales,
			// mary -> accounting, peter -> management, demo in all.
			members := map[string][]camunda7.User{
				"sales":         {{ID: "demo"}, {ID: "john"}},
				"accounting":    {{ID: "demo"}, {ID: "mary"}},
				"management":    {{ID: "demo"}, {ID: "peter"}},
				"camunda-admin": {{ID: "demo"}},
			}
			_ = json.NewEncoder(w).Encode(members[want])
		},
	})

	c := camunda7.New(srv.URL+"/engine-rest", "", "")
	ids, err := c.ListGroupMemberIDs(context.Background(), "accounting")
	if err != nil {
		t.Fatalf("ListGroupMemberIDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != "demo" || ids[1] != "mary" {
		t.Errorf("accounting members: want [demo mary], got %v", ids)
	}
}

func TestClient_BasicAuth(t *testing.T) {
	var sawAuth string
	srv := newMockEngine(t, map[string]http.HandlerFunc{
		"/engine-rest/user": func(w http.ResponseWriter, r *http.Request) {
			sawAuth = r.Header.Get("Authorization")
			_ = json.NewEncoder(w).Encode([]camunda7.User{})
		},
	})

	c := camunda7.New(srv.URL+"/engine-rest", "admin", "s3cret")
	if _, err := c.ListUsers(context.Background()); err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if sawAuth == "" {
		t.Errorf("expected Authorization header, got none")
	}
}

func TestClient_EndpointEscaping(t *testing.T) {
	// Regression: a deployment id that contains reserved characters
	// must be percent-escaped on the wire. Assert on r.RequestURI
	// because r.URL.Path is always percent-decoded by the stdlib.
	var seen string
	srv := newMockEngine(t, map[string]http.HandlerFunc{
		"/engine-rest/deployment/": func(w http.ResponseWriter, r *http.Request) {
			seen = r.RequestURI
			_ = json.NewEncoder(w).Encode([]camunda7.DeploymentResource{})
		},
	})

	c := camunda7.New(srv.URL+"/engine-rest", "", "")
	id := "id:with:colons"
	_, err := c.ListDeploymentResources(context.Background(), id)
	if err != nil {
		t.Fatalf("ListDeploymentResources: %v", err)
	}
	wantSuffix := "/engine-rest/deployment/" + url.PathEscape(id) + "/resources"
	if seen != wantSuffix {
		t.Errorf("path escape: want %q, got %q", wantSuffix, seen)
	}
}
