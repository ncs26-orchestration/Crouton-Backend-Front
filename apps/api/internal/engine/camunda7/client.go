// Package camunda7 is a minimal REST client for Camunda 7's engine-rest
// API. AUP uses it read-only to project identity (users, groups,
// memberships) and existing deployments; AUP never writes to the engine
// through this client.
package camunda7

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is safe to share across goroutines. Endpoint should be the
// root of the Camunda REST API — e.g. "http://camunda7:8080/engine-rest".
type Client struct {
	Endpoint  string
	Username  string
	Password  string
	HTTP      *http.Client
}

// New builds a client with a sane default HTTP timeout. Pass your own
// *http.Client via the struct field if you need to customize further.
func New(endpoint, username, password string) *Client {
	return &Client{
		Endpoint: endpoint,
		Username: username,
		Password: password,
		HTTP:     &http.Client{Timeout: 10 * time.Second},
	}
}

// User mirrors Camunda's UserProfileDto. Only the fields AUP cares
// about are decoded.
type User struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Email     string `json:"email,omitempty"`
}

// DisplayName returns a best-effort label for UIs.
func (u User) DisplayName() string {
	switch {
	case u.FirstName != "" && u.LastName != "":
		return u.FirstName + " " + u.LastName
	case u.FirstName != "":
		return u.FirstName
	case u.LastName != "":
		return u.LastName
	default:
		return u.ID
	}
}

// Group mirrors Camunda's GroupDto.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// ListUsers returns all users the engine's identity service exposes.
// Camunda's /user endpoint is paginated via firstResult/maxResults;
// the demo/free platform usually has <100 users, so we ask for up to
// 1000 in one shot and page only if the response is full.
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	var all []User
	first := 0
	const pageSize = 1000
	for {
		q := url.Values{
			"firstResult": []string{fmt.Sprintf("%d", first)},
			"maxResults":  []string{fmt.Sprintf("%d", pageSize)},
		}
		var page []User
		if err := c.getJSON(ctx, "/user?"+q.Encode(), &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			break
		}
		first += pageSize
	}
	return all, nil
}

// ListGroups returns all groups from the engine's identity service.
func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	var all []Group
	first := 0
	const pageSize = 1000
	for {
		q := url.Values{
			"firstResult": []string{fmt.Sprintf("%d", first)},
			"maxResults":  []string{fmt.Sprintf("%d", pageSize)},
		}
		var page []Group
		if err := c.getJSON(ctx, "/group?"+q.Encode(), &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			break
		}
		first += pageSize
	}
	return all, nil
}

// ListGroupMemberIDs returns the user ids that belong to a given
// group. Camunda does not expose a direct "members of group" endpoint;
// the recommended path is to filter /user by memberOfGroup.
func (c *Client) ListGroupMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	q := url.Values{
		"memberOfGroup": []string{groupID},
		"maxResults":    []string{"1000"},
	}
	var users []User
	if err := c.getJSON(ctx, "/user?"+q.Encode(), &users); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids, nil
}

// Deployment mirrors the minimal fields of Camunda's DeploymentDto we
// need to enumerate deployed resources.
type Deployment struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source,omitempty"`
}

// DeploymentResource mirrors DeploymentResourceDto. AUP looks at the
// name to identify deployed forms (resources ending in .form).
type DeploymentResource struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	DeploymentID   string `json:"deploymentId"`
}

// ListDeployments returns all deployments visible to the engine.
func (c *Client) ListDeployments(ctx context.Context) ([]Deployment, error) {
	var out []Deployment
	if err := c.getJSON(ctx, "/deployment?maxResults=1000", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListDeploymentResources returns the resources attached to a given
// deployment. Used to enumerate form files.
func (c *Client) ListDeploymentResources(ctx context.Context, deploymentID string) ([]DeploymentResource, error) {
	var out []DeploymentResource
	path := fmt.Sprintf("/deployment/%s/resources", url.PathEscape(deploymentID))
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dst any) error {
	full := c.Endpoint + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.Username != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", full, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("GET %s: status %d: %s", full, resp.StatusCode, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
