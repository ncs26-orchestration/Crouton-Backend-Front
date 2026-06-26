package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/camunda7"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// DeploymentsHandler proxies the subset of Camunda 7 REST that the
// Runs view needs: deployed process definitions + live instance counts.
// AIOS never mutates the engine through these endpoints; strictly reads.
type DeploymentsHandler struct {
	logger  *slog.Logger
	engines *repo.EngineRepo
	http    *http.Client
}

func NewDeploymentsHandler(logger *slog.Logger, pg *pgxpool.Pool) *DeploymentsHandler {
	return &DeploymentsHandler{
		logger:  logger,
		engines: repo.NewEngineRepo(pg),
		http:    &http.Client{Timeout: 8 * time.Second},
	}
}

type processDefinition struct {
	ID                string `json:"id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	Version           int    `json:"version"`
	DeploymentID      string `json:"deployment_id"`
	TenantID          string `json:"tenant_id,omitempty"`
	RunningInstances  int    `json:"running_instances"`
	HistoryTimeToLive int    `json:"history_time_to_live"`
}

type runsResponse struct {
	EngineID           string              `json:"engine_id"`
	Endpoint           string              `json:"endpoint"`
	CockpitURL         string              `json:"cockpit_url,omitempty"`
	TasklistURL        string              `json:"tasklist_url,omitempty"`
	LastSyncedAt       string              `json:"last_synced_at,omitempty"`
	ProcessDefinitions []processDefinition `json:"process_definitions"`
}

// ListRuns handles GET /engines/:id/runs — returns the deployed
// process definitions + each definition's running instance count for
// the UI's Runs view.
func (h *DeploymentsHandler) ListRuns(c echo.Context) error {
	tenantID := tenantFromRequest(c)
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request().Context(), 20*time.Second)
	defer cancel()

	ec, err := h.engines.GetEngineConnection(ctx, tenantID, id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "engine_not_found"})
	}
	if ec.Kind != "camunda7" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "only_camunda7_supported"})
	}

	defs, err := fetchJSON[[]rawProcessDefinition](ctx, h.http, ec, "/process-definition?latestVersion=true&sortBy=name&sortOrder=asc&maxResults=200")
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "fetch_definitions_failed", "details": err.Error()})
	}

	out := make([]processDefinition, 0, len(defs))
	for _, d := range defs {
		count := 0
		q := url.Values{"processDefinitionId": []string{d.ID}, "active": []string{"true"}}
		if cnt, err := fetchJSON[countDto](ctx, h.http, ec, "/process-instance/count?"+q.Encode()); err == nil {
			count = cnt.Count
		}
		out = append(out, processDefinition{
			ID:                d.ID,
			Key:               d.Key,
			Name:              firstNonEmpty(d.Name, d.Key),
			Version:           d.Version,
			DeploymentID:      d.DeploymentID,
			TenantID:          d.TenantID,
			RunningInstances:  count,
			HistoryTimeToLive: d.HistoryTimeToLive,
		})
	}

	cockpit, tasklist := camundaWebAppURLs(ec.Endpoint, "")
	var lastSynced string
	if ec.LastSyncedAt != nil {
		lastSynced = ec.LastSyncedAt.Format(time.RFC3339)
	}
	return c.JSON(http.StatusOK, runsResponse{
		EngineID:           ec.ExternalID,
		Endpoint:           ec.Endpoint,
		CockpitURL:         cockpit,
		TasklistURL:        tasklist,
		LastSyncedAt:       lastSynced,
		ProcessDefinitions: out,
	})
}

// StartInstance handles POST /engines/:id/processes/:key/start so the
// UI can kick off an instance from the Runs view.
func (h *DeploymentsHandler) StartInstance(c echo.Context) error {
	tenantID := tenantFromRequest(c)
	id := c.Param("id")
	key := c.Param("key")
	ctx, cancel := context.WithTimeout(c.Request().Context(), 15*time.Second)
	defer cancel()

	ec, err := h.engines.GetEngineConnection(ctx, tenantID, id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "engine_not_found"})
	}
	var body map[string]any
	_ = c.Bind(&body)
	vars, _ := body["variables"].(map[string]any)

	cli := camunda7.New(ec.Endpoint, ec.AuthUsername, ec.AuthSecret)
	inst, err := cli.StartInstance(ctx, key, normalizeCamundaVars(vars))
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "start_failed", "details": err.Error()})
	}
	return c.JSON(http.StatusOK, inst)
}

// --- internal -----------------------------------------------------

type rawProcessDefinition struct {
	ID                string `json:"id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	Version           int    `json:"version"`
	DeploymentID      string `json:"deploymentId"`
	TenantID          string `json:"tenantId"`
	HistoryTimeToLive int    `json:"historyTimeToLive"`
}

type countDto struct {
	Count int `json:"count"`
}

func fetchJSON[T any](ctx context.Context, h *http.Client, ec *repo.EngineConnection, path string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ec.Endpoint+path, nil)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Accept", "application/json")
	if ec.AuthUsername != "" {
		req.SetBasicAuth(ec.AuthUsername, ec.AuthSecret)
	}
	resp, err := h.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("%d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, err
	}
	return out, nil
}
