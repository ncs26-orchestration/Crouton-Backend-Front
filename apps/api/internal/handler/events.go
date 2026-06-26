package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/ncs26-orchestration/solution/apps/api/internal/auth"
	"github.com/ncs26-orchestration/solution/apps/api/internal/orchestrator"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
)

// EventsHandler streams SSE events for a request.
type EventsHandler struct {
	logger    *slog.Logger
	pg        *pgxpool.Pool
	jwtSecret string
	bus       *orchestrator.Bus
	requests  *repo.RequestRepo
}

func NewEventsHandler(logger *slog.Logger, pg *pgxpool.Pool, jwtSecret string, bus *orchestrator.Bus) *EventsHandler {
	return &EventsHandler{
		logger:    logger,
		pg:        pg,
		jwtSecret: jwtSecret,
		bus:       bus,
		requests:  repo.NewRequestRepo(pg),
	}
}

// Stream handles GET /requests/:id/events. It authenticates via ?token=
// (because EventSource cannot set custom headers), sets SSE headers, and
// streams bus events until the client disconnects.
func (h *EventsHandler) Stream(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing token query parameter"})
	}
	claims, err := auth.ParseToken(h.jwtSecret, token)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
	}

	requestID := c.Param("id")

	ctx := c.Request().Context()
	req, err := h.requests.GetByID(ctx, requestID)
	if errors.Is(err, repo.ErrNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
	}
	if err != nil {
		h.logger.Error("events: get request", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if _, err := requireOrgMember(c, h.pg, req.OrgID, claims.UserID); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request not found"})
		}
		return handleOrgMemberErr(c, err)
	}

	ch, cleanup := h.bus.Subscribe(requestID)
	defer cleanup()

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			data, err := json.Marshal(ev)
			if err != nil {
				h.logger.Error("sse marshal", slog.String("err", err.Error()))
				continue
			}
			_, err = fmt.Fprintf(c.Response().Writer, "event: %s\ndata: %s\n\n", ev.Type, data)
			if err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}
