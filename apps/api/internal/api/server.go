package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/ncs26-orchestration/solution/apps/api/internal/db"
)

type Server struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewServer(pool *pgxpool.Pool) *Server {
	return &Server{pool: pool, queries: db.New(pool)}
}

// Router builds the Echo instance with all routes and middleware.
func (s *Server) Router() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.RequestID())

	g := e.Group("/api")
	g.GET("/health", s.health)
	g.GET("/notes", s.listNotes)
	g.POST("/notes", s.createNote)
	g.GET("/notes/:id", s.getNote)

	return e
}

func (s *Server) health(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		return c.JSON(http.StatusServiceUnavailable, echo.Map{"status": "unhealthy", "db": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
}

func (s *Server) listNotes(c echo.Context) error {
	notes, err := s.queries.ListNotes(c.Request().Context())
	if err != nil {
		return err
	}
	if notes == nil {
		notes = []db.Note{}
	}
	return c.JSON(http.StatusOK, notes)
}

type createNoteRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (s *Server) createNote(c echo.Context) error {
	var req createNoteRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid JSON body")
	}
	if req.Title == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title is required")
	}
	note, err := s.queries.CreateNote(c.Request().Context(), db.CreateNoteParams{
		Title: req.Title,
		Body:  req.Body,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, note)
}

func (s *Server) getNote(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	note, err := s.queries.GetNote(c.Request().Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return echo.NewHTTPError(http.StatusNotFound, "note not found")
		}
		return err
	}
	return c.JSON(http.StatusOK, note)
}
