package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
)

type MeHandler struct {
	logger *slog.Logger
	db     *pgxpool.Pool
}

func NewMeHandler(logger *slog.Logger, db *pgxpool.Pool) *MeHandler {
	return &MeHandler{logger: logger, db: db}
}

// GetMeProfile handles GET /me — returns the authenticated user's profile
// including org role and team roles.
func (h *MeHandler) GetMeProfile(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	ctx := c.Request().Context()

	type teamRole struct {
		Team string `json:"team"`
		Role string `json:"role"`
	}

	profile := struct {
		ID        int64      `json:"id"`
		Email     string     `json:"email"`
		Name      string     `json:"name"`
		OrgRole   string     `json:"org_role"`
		OrgID     string     `json:"org_id"`
		OrgName   string     `json:"org_name"`
		TeamRoles []teamRole `json:"team_roles"`
	}{
		ID:    claims.UserID,
		Email: claims.Email,
		Name:  claims.Name,
	}

	// Fetch org membership + team roles.
	err := h.db.QueryRow(ctx, `
		SELECT om.org_id, o.name, om.role,
		       COALESCE(
		         (SELECT jsonb_agg(jsonb_build_object('team', t.name, 'role', tm.role))
		            FROM team_members tm
		            JOIN teams t ON t.id = tm.team_id AND t.org_id = om.org_id
		           WHERE tm.user_id = $1),
		         '[]'::jsonb
		       ) AS team_roles
		  FROM org_members om
		  JOIN organizations o ON o.id = om.org_id
		 WHERE om.user_id = $1
		 LIMIT 1
	`, claims.UserID).Scan(&profile.OrgID, &profile.OrgName, &profile.OrgRole, &profile.TeamRoles)
	if err != nil {
		h.logger.Error("get me profile: query", slog.String("err", err.Error()))
		profile.OrgRole = "unknown"
	}

	return c.JSON(http.StatusOK, profile)
}

type workItemResponse struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	OrgName         string    `json:"org_name"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	RequesterUserID int64     `json:"requester_user_id"`
	RequesterName   string    `json:"requester_name"`
	Priority        string    `json:"priority"`
	Status          string    `json:"status"`
	Progress        int       `json:"progress"`
	CreatedAt       time.Time `json:"created_at"`
	IsRequester     bool      `json:"is_requester"`
	StageName       string    `json:"stage_name,omitempty"`
	StageStatus     string    `json:"stage_status,omitempty"`
}

func (h *MeHandler) GetMyWork(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	ctx := c.Request().Context()
	userID := claims.UserID

	rows, err := h.db.Query(ctx, `
		SELECT
			r.id,
			r.org_id,
			o.name,
			r.title,
			r.description,
			r.requester_user_id,
			u.name,
			r.priority,
			r.status,
			r.progress,
			r.created_at,
			COALESCE(wn.name, ''),
			COALESCE(wn.status, '')
		FROM requests r
		JOIN organizations o ON o.id = r.org_id
		JOIN users u ON u.id = r.requester_user_id
		LEFT JOIN LATERAL (
			SELECT w.name, w.status
			FROM workflow_nodes w
			WHERE w.request_id = r.id
			ORDER BY w.created_at DESC
			LIMIT 1
		) wn ON true
		WHERE r.org_id IN (
			SELECT om.org_id FROM org_members om WHERE om.user_id = $1
		)
		ORDER BY r.created_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		h.logger.Error("get my work: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer rows.Close()

	items := make([]workItemResponse, 0)
	for rows.Next() {
		var item workItemResponse
		if err := rows.Scan(
			&item.ID,
			&item.OrgID,
			&item.OrgName,
			&item.Title,
			&item.Description,
			&item.RequesterUserID,
			&item.RequesterName,
			&item.Priority,
			&item.Status,
			&item.Progress,
			&item.CreatedAt,
			&item.StageName,
			&item.StageStatus,
		); err != nil {
			h.logger.Error("get my work: scan", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		item.IsRequester = item.RequesterUserID == userID
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		h.logger.Error("get my work: rows err", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{"work_items": items})
}
