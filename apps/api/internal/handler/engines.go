package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Noussour/aup/apps/api/internal/ir"
	"github.com/Noussour/aup/apps/api/internal/repo"
	"github.com/Noussour/aup/apps/api/internal/service"
)

// EnginesHandler groups the endpoints that manage engine connections
// and expose the IS Registry projection. Kept on its own struct so
// wiring in http/server.go can inject the discovery service cleanly.
type EnginesHandler struct {
	logger    *slog.Logger
	engines   *repo.EngineRepo
	discovery *service.Discovery
}

func NewEnginesHandler(logger *slog.Logger, pg *pgxpool.Pool) *EnginesHandler {
	repoE := repo.NewEngineRepo(pg)
	return &EnginesHandler{
		logger:    logger,
		engines:   repoE,
		discovery: service.NewDiscovery(repoE),
	}
}

// --- POST /engines ------------------------------------------------

type createEngineRequest struct {
	TenantID   string `json:"tenant_id"`
	ExternalID string `json:"id"`       // user-facing id, e.g. "local-camunda"
	Kind       string `json:"kind"`     // camunda7 | camunda8 | elsa3
	Endpoint   string `json:"endpoint"` // e.g. http://camunda7:8080/engine-rest
	AuthKind   string `json:"auth_kind,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	AutoSync   *bool  `json:"auto_sync,omitempty"` // default true
}

type engineResponse struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Endpoint     string `json:"endpoint"`
	LastSyncedAt string `json:"last_synced_at,omitempty"`
	SyncedUsers  *int   `json:"synced_users,omitempty"`
	SyncedGroups *int   `json:"synced_groups,omitempty"`
	SyncedForms  *int   `json:"synced_forms,omitempty"`
}

// CreateEngine registers (or updates) an engine connection and — by
// default — immediately runs a discovery sync so the IS Registry is
// populated before the user navigates to the workspace.
func (h *EnginesHandler) CreateEngine(c echo.Context) error {
	var req createEngineRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	req.TenantID = orDefault(strings.TrimSpace(req.TenantID), "demo")
	if req.ExternalID == "" || req.Kind == "" || req.Endpoint == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_fields: id, kind, endpoint required"})
	}
	switch req.Kind {
	case "camunda7", "camunda8", "elsa3":
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported_kind"})
	}
	if req.AuthKind == "" {
		if req.Username != "" {
			req.AuthKind = "basic"
		} else {
			req.AuthKind = "none"
		}
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 20*time.Second)
	defer cancel()

	ec, err := h.engines.UpsertEngineConnection(ctx, &repo.EngineConnection{
		TenantID:     req.TenantID,
		ExternalID:   req.ExternalID,
		Kind:         req.Kind,
		Endpoint:     req.Endpoint,
		AuthKind:     req.AuthKind,
		AuthUsername: req.Username,
		AuthSecret:   req.Password, // in-memory only — not persisted in v0.1
	})
	if err != nil {
		h.logger.Error("upsert engine_connection", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}

	resp := engineResponse{ID: ec.ExternalID, Kind: ec.Kind, Endpoint: ec.Endpoint}

	if req.AutoSync == nil || *req.AutoSync {
		users, groups, forms, err := h.discovery.Sync(ctx, ec)
		if err != nil {
			h.logger.Warn("initial sync failed", "engine", ec.ExternalID, "err", err)
			return c.JSON(http.StatusBadGateway, map[string]string{
				"error":   "initial_sync_failed",
				"details": err.Error(),
			})
		}
		resp.SyncedUsers = &users
		resp.SyncedGroups = &groups
		resp.SyncedForms = &forms
		// Refresh to capture last_synced_at.
		updated, _ := h.engines.GetEngineConnection(ctx, ec.TenantID, ec.ExternalID)
		if updated != nil && updated.LastSyncedAt != nil {
			resp.LastSyncedAt = updated.LastSyncedAt.Format(time.RFC3339)
		}
	}
	return c.JSON(http.StatusOK, resp)
}

// --- POST /engines/:id/sync --------------------------------------

func (h *EnginesHandler) SyncEngine(c echo.Context) error {
	tenantID := tenantFromRequest(c)
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
	defer cancel()

	ec, err := h.engines.GetEngineConnection(ctx, tenantID, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "engine_not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	users, groups, forms, err := h.discovery.Sync(ctx, ec)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "sync_failed", "details": err.Error()})
	}
	refreshed, _ := h.engines.GetEngineConnection(ctx, tenantID, id)
	var lastSynced string
	if refreshed != nil && refreshed.LastSyncedAt != nil {
		lastSynced = refreshed.LastSyncedAt.Format(time.RFC3339)
	}
	return c.JSON(http.StatusOK, engineResponse{
		ID:           ec.ExternalID,
		Kind:         ec.Kind,
		Endpoint:     ec.Endpoint,
		LastSyncedAt: lastSynced,
		SyncedUsers:  &users,
		SyncedGroups: &groups,
		SyncedForms:  &forms,
	})
}

// --- GET /is ------------------------------------------------------

// ListIS returns the full IS Registry projection for a tenant in the
// shape that matches packages/ir/is-registry.schema.json — ready to be
// handed to the extractor or used as cross-ref input for /compile/bpmn.
func (h *EnginesHandler) ListIS(c echo.Context) error {
	tenantID := tenantFromRequest(c)
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	proj, err := h.engines.ReadTenantProjection(ctx, tenantID)
	if err != nil {
		h.logger.Error("read tenant projection", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	reg := service.BuildISRegistry(proj)
	// Normalize nil slices to [] so JSON clients never see null for
	// collection fields — friendlier for TS consumers.
	if reg.Users == nil {
		reg.Users = []ir.ISUser{}
	}
	if reg.Groups == nil {
		reg.Groups = []ir.ISGroup{}
	}
	if reg.Systems == nil {
		reg.Systems = []ir.ISSystem{}
	}
	return c.JSON(http.StatusOK, reg)
}

// --- POST /systems ------------------------------------------------

type declareSystemRequest struct {
	TenantID     string   `json:"tenant_id,omitempty"`
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Kind         string   `json:"kind"`
	Endpoint     string   `json:"endpoint,omitempty"`
	Capabilities []string `json:"capabilities"`
}

// DeclareSystem creates or updates a tenant-declared system (OpenBee,
// Odoo, M365, etc.) with its capability catalog. No identities live
// here — identities are projected from engines/IdPs.
func (h *EnginesHandler) DeclareSystem(c echo.Context) error {
	var req declareSystemRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	req.TenantID = orDefault(strings.TrimSpace(req.TenantID), "demo")
	if req.ID == "" || req.Kind == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_fields: id, kind required"})
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	ds, err := h.engines.UpsertDeclaredSystem(ctx, &repo.DeclaredSystem{
		TenantID:     req.TenantID,
		ExternalID:   req.ID,
		Name:         req.Name,
		Kind:         req.Kind,
		Endpoint:     req.Endpoint,
		Capabilities: req.Capabilities,
	})
	if err != nil {
		h.logger.Error("upsert declared_system", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"id":           ds.ExternalID,
		"kind":         ds.Kind,
		"capabilities": ds.Capabilities,
	})
}

// tenantFromRequest keeps v0.1 simple: accept a ?tenant= query arg or
// X-Tenant-Id header; default to "demo" if both are absent. A real
// auth layer is future work.
func tenantFromRequest(c echo.Context) string {
	if v := c.QueryParam("tenant"); v != "" {
		return v
	}
	if v := c.Request().Header.Get("X-Tenant-Id"); v != "" {
		return v
	}
	return "demo"
}

// orDefault returns s if non-empty, else def.
func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
