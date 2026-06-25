package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	logger *slog.Logger
	pg     *pgxpool.Pool
	rdb    *redis.Client
}

func New(logger *slog.Logger, pg *pgxpool.Pool, rdb *redis.Client) *Handler {
	return &Handler{logger: logger, pg: pg, rdb: rdb}
}

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
	Redis  string `json:"redis"`
}

func (h *Handler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, healthResponse{Status: "ok", DB: "skipped", Redis: "skipped"})
}

func (h *Handler) Ready(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()

	resp := healthResponse{Status: "ok", DB: "up", Redis: "up"}
	status := http.StatusOK

	if err := h.pg.Ping(ctx); err != nil {
		resp.DB = "down"
		resp.Status = "degraded"
		status = http.StatusServiceUnavailable
	}
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		resp.Redis = "down"
		resp.Status = "degraded"
		status = http.StatusServiceUnavailable
	}
	return c.JSON(status, resp)
}
