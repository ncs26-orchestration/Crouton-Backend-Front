package camunda7

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

// DeploymentResult is what Camunda's REST returns on a successful
// deployment — the subset AUP cares about.
type DeploymentResult struct {
	ID                         string                               `json:"id"`
	Name                       string                               `json:"name"`
	DeployedProcessDefinitions map[string]DeployedProcessDefinition `json:"deployedProcessDefinitions"`
}

type DeployedProcessDefinition struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// Deploy pushes a BPMN artifact to Camunda as a new deployment. The
// deployment is idempotent on content: enable-duplicate-filtering is
// set so redeploying the same XML under the same name is a no-op.
//
// Camunda's /deployment/create expects multipart/form-data with the
// BPMN bytes as a file field. The filename must end in .bpmn or .xml
// for the engine to classify it correctly.
func (c *Client) Deploy(ctx context.Context, deploymentName, fileName string, xml []byte) (*DeploymentResult, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	for _, f := range [][2]string{
		{"deployment-name", deploymentName},
		{"tenant-id", "aup"},
		{"deployment-source", "aup"},
		{"enable-duplicate-filtering", "true"},
	} {
		if err := mw.WriteField(f[0], f[1]); err != nil {
			return nil, err
		}
	}

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="data"; filename=%q`, fileName))
	h.Set("Content-Type", "application/xml")
	part, err := mw.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(xml); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/deployment/create", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.Username != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deploy POST: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("camunda %d: %s", resp.StatusCode, string(raw))
	}
	var out DeploymentResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode deployment: %w (raw: %s)", err, string(raw[:min(len(raw), 400)]))
	}
	return &out, nil
}

// StartInstance starts a new process instance by process-definition
// key. Variables are Camunda's typed-value envelope; callers pass a
// map like { "amount": { "value": 50000, "type": "Long" } }.
type StartResult struct {
	ID           string `json:"id"`
	DefinitionID string `json:"definitionId"`
	Ended        bool   `json:"ended"`
}

func (c *Client) StartInstance(
	ctx context.Context,
	processKey string,
	variables map[string]any,
) (*StartResult, error) {
	body, _ := json.Marshal(map[string]any{"variables": variables})
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/process-definition/key/%s/start", c.Endpoint, processKey),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Username != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("start POST: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("camunda %d: %s", resp.StatusCode, string(raw))
	}
	var out StartResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode start: %w", err)
	}
	return &out, nil
}
