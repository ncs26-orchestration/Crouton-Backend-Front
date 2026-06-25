package elsa3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a thin HTTP client for the Elsa Server 3 REST API. We
// only need workflow-definition push for v0.1; identity discovery
// and run-log fetching live in later slices.
//
// Auth modes supported:
//   - "" or "none"     — no auth header
//   - "apikey"         — header `Authorization: ApiKey <secret>`
//   - "bearer"         — header `Authorization: Bearer <secret>`
//   - "basic"          — HTTP Basic with authUser + authSecret
//   - "credentials"    — POST /elsa/api/identity/login with
//     authUser+authPass, cache the JWT, use
//     it as Bearer. Matches the default Elsa
//     Server admin/password flow out of the box.
//
// Elsa Server's default admin login is "admin/password" in dev; in
// prod you'd configure an ApiKey or your own Identity provider.
type Client struct {
	endpoint  string
	authKind  string
	authUser  string
	authPass  string
	cachedJWT string
	http      *http.Client
}

func NewClient(endpoint, authKind, authUser, authSecret string) *Client {
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		authKind: strings.ToLower(strings.TrimSpace(authKind)),
		authUser: authUser,
		authPass: authSecret,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// DeployResult mirrors the subset of Elsa's response we surface back
// to the caller. Elsa returns the full updated WorkflowDefinition on
// success; we extract the definition id (the human-readable one) and
// the auto-generated storage id.
type DeployResult struct {
	DefinitionID string
	ID           string
	Version      int
	IsPublished  bool
}

// Deploy POSTs the compiled workflow-definition JSON to the Elsa Server
// and returns the persisted identifiers. Elsa's SaveWorkflowDefinition
// endpoint expects a `{model, publish}` envelope — the raw compiler
// output (a bare WorkflowDefinition) is wrapped here so callers can
// still think of the artifact as "the workflow" and have the transport
// wrapper be a deploy-time concern.
//
// On the first push for a given definitionId Elsa creates the row; on
// subsequent pushes it updates. Both return 200 with the saved model.
func (c *Client) Deploy(ctx context.Context, artifact []byte) (*DeployResult, error) {
	if c.endpoint == "" {
		return nil, fmt.Errorf("elsa endpoint not configured")
	}
	// Parse the artifact as a generic JSON object so we can nest it
	// under `model`. The raw bytes are well-formed because the
	// compiler json.MarshalIndent'd them; an unmarshal here is cheap
	// and gives us a typed failure if someone ever swaps the output.
	var model map[string]any
	if err := json.Unmarshal(artifact, &model); err != nil {
		return nil, fmt.Errorf("artifact is not valid JSON: %w", err)
	}
	envelope, err := json.Marshal(map[string]any{
		"model":   model,
		"publish": true,
	})
	if err != nil {
		return nil, err
	}

	url := c.endpoint + "/elsa/api/workflow-definitions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(envelope))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	if err := c.applyAuth(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("elsa unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("elsa %d: %s", resp.StatusCode, truncate(string(body), 400))
	}

	// Elsa returns either { workflowDefinition: { id, definitionId, ... } }
	// (the newer shape) or the WorkflowDefinition flat (older). Handle both.
	var wrapped struct {
		WorkflowDefinition *struct {
			ID           string `json:"id"`
			DefinitionID string `json:"definitionId"`
			Version      int    `json:"version"`
			IsPublished  bool   `json:"isPublished"`
		} `json:"workflowDefinition"`
		// Fallback shape
		ID           string `json:"id"`
		DefinitionID string `json:"definitionId"`
		Version      int    `json:"version"`
		IsPublished  bool   `json:"isPublished"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		// Not fatal — return a best-effort empty result.
		return &DeployResult{}, nil
	}
	if wrapped.WorkflowDefinition != nil {
		return &DeployResult{
			ID:           wrapped.WorkflowDefinition.ID,
			DefinitionID: wrapped.WorkflowDefinition.DefinitionID,
			Version:      wrapped.WorkflowDefinition.Version,
			IsPublished:  wrapped.WorkflowDefinition.IsPublished,
		}, nil
	}
	return &DeployResult{
		ID:           wrapped.ID,
		DefinitionID: wrapped.DefinitionID,
		Version:      wrapped.Version,
		IsPublished:  wrapped.IsPublished,
	}, nil
}

// login exchanges the configured user/password for a JWT via
// /elsa/api/identity/login. The token is cached on the client so
// successive Deploy calls within one process don't re-authenticate.
// Tokens are long-lived (24h by default) which is plenty for a
// burst of deploys; a TTL-aware refresh would be a later slice.
func (c *Client) login(ctx context.Context) (string, error) {
	if c.cachedJWT != "" {
		return c.cachedJWT, nil
	}
	body, _ := json.Marshal(map[string]string{
		"username": c.authUser,
		"password": c.authPass,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/elsa/api/identity/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("elsa login unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<19))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("elsa login %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed struct {
		IsAuthenticated bool   `json:"isAuthenticated"`
		AccessToken     string `json:"accessToken"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("elsa login decode: %w", err)
	}
	if !parsed.IsAuthenticated || parsed.AccessToken == "" {
		return "", fmt.Errorf("elsa login rejected (check username + password)")
	}
	c.cachedJWT = parsed.AccessToken
	return parsed.AccessToken, nil
}

func (c *Client) applyAuth(ctx context.Context, req *http.Request) error {
	switch c.authKind {
	case "apikey":
		if c.authPass != "" {
			req.Header.Set("Authorization", "ApiKey "+c.authPass)
		}
	case "bearer":
		if c.authPass != "" {
			req.Header.Set("Authorization", "Bearer "+c.authPass)
		}
	case "basic":
		if c.authUser != "" || c.authPass != "" {
			req.SetBasicAuth(c.authUser, c.authPass)
		}
	case "credentials":
		token, err := c.login(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

// StudioURL returns a best-effort browser-openable link to the
// Elsa Studio edit page for a just-deployed workflow definition.
// The docker-compose dev setup routes container-internal
// http://elsa3:8080 to host-published http://localhost:8280, so we
// swap that mapping and build the /workflow-definitions/<id> path
// Studio uses.
//
// For arbitrary endpoints (prod Elsa behind a domain) we assume the
// Studio and API share the same base — that's true for the
// elsa-server-and-studio-v3 image. Callers that point api-only
// deployments at this helper will get a URL that 404s in a browser;
// acceptable because the fallback is "no link shown" which the UI
// handles by keeping the toast link-less.
func StudioURL(endpoint, definitionID string) string {
	if definitionID == "" {
		return ""
	}
	base := strings.TrimRight(endpoint, "/")
	// Strip any /elsa/api suffix — both Studio and the REST API live
	// at the same host, but Studio's routes are not under /elsa/api.
	base = strings.TrimSuffix(base, "/elsa/api")
	// Dev-mode rewrite: container-internal name → host-published port.
	// Matches the compose.override mapping elsa3:8080 -> host:8280.
	base = strings.Replace(base, "://elsa3:8080", "://localhost:8280", 1)
	return base + "/workflows/definitions/" + definitionID + "/edit"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
