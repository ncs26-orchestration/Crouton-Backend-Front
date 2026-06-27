package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/orgdir"
	"golang.org/x/crypto/bcrypt"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$`)

// OrgsHandler owns all /orgs, /orgs/:orgId/teams, and /orgs/:orgId/members routes.
type OrgsHandler struct {
	logger *slog.Logger
	db     *pgxpool.Pool
}

// NewOrgsHandler constructs an OrgsHandler.
func NewOrgsHandler(logger *slog.Logger, db *pgxpool.Pool) *OrgsHandler {
	return &OrgsHandler{logger: logger, db: db}
}

// requireOrgMember verifies the caller belongs to the org and returns their role.
func requireOrgMember(c echo.Context, db *pgxpool.Pool, orgID string, userID int64) (string, error) {
	var role string
	err := db.QueryRow(c.Request().Context(),
		`SELECT role FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", echo.NewHTTPError(http.StatusForbidden, "not a member of this organization")
	}
	if err != nil {
		return "", err
	}
	return role, nil
}

// ── Org endpoints ─────────────────────────────────────────────────────────────

// CreateOrg handles POST /orgs.
func (h *OrgsHandler) CreateOrg(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var body struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.Name == "" || body.Slug == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name and slug are required"})
	}
	if len(body.Slug) < 2 || !slugRe.MatchString(body.Slug) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "slug must be lowercase alphanumeric and dashes, min 2 chars"})
	}

	orgID := fmt.Sprintf("org_%s", randomHex(8))
	ctx := c.Request().Context()

	tx, err := h.db.Begin(ctx)
	if err != nil {
		h.logger.Error("create org: begin tx", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var createdAt time.Time
	err = tx.QueryRow(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3) RETURNING created_at`,
		orgID, body.Name, body.Slug,
	).Scan(&createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "slug already taken"})
		}
		h.logger.Error("create org: insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, 'admin')`,
		orgID, claims.UserID,
	)
	if err != nil {
		h.logger.Error("create org: add creator as admin", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Seed the standard department directory (F10): one team per department, an
	// agent per team, and the starter policies. The agent/policy content lives
	// in internal/orgdir so this path and the demo seed share one source.
	teamByDept := make(map[string]string, len(orgdir.Agents))
	for _, a := range orgdir.Agents {
		if _, ok := teamByDept[a.Department]; ok {
			continue
		}
		teamID := "team_" + randomHex(8)
		teamByDept[a.Department] = teamID
		if _, err := tx.Exec(ctx, `
			INSERT INTO teams (id, org_id, name, description) VALUES ($1, $2, $3, $4)
		`, teamID, orgID, a.Department, a.Department+" department"); err != nil {
			h.logger.Error("create org: seed team", slog.String("name", a.Department), slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
	}
	for _, a := range orgdir.Agents {
		if _, err := tx.Exec(ctx, `
			INSERT INTO agents (id, org_id, team_id, agent_type, name, capabilities) VALUES ($1, $2, $3, $4, $5, $6)
		`, "agent_"+randomHex(8), orgID, teamByDept[a.Department], a.AgentType, a.Name, a.Capabilities); err != nil {
			h.logger.Error("create org: seed agent", slog.String("type", a.AgentType), slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
	}
	for _, p := range orgdir.Policies {
		if _, err := tx.Exec(ctx, `
			INSERT INTO department_policies (id, org_id, team_id, title, body) VALUES ($1, $2, $3, $4, $5)
		`, "pol_"+randomHex(8), orgID, teamByDept[p.Department], p.Title, p.Body); err != nil {
			h.logger.Error("create org: seed policy", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
	}
	// Seed a Maintenance team so technicians have a team to belong to.
	// Existing orgs are backfilled by the migration; new orgs get it here.
	maintID := fmt.Sprintf("team_%s", randomHex(8))
	_, err = tx.Exec(ctx,
		`INSERT INTO teams (id, org_id, name, description) VALUES ($1, $2, 'Maintenance', 'Equipment maintenance and repair')`,
		maintID, orgID,
	)
	if err != nil {
		h.logger.Error("create org: seed maintenance team", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	// Add the creator as the Maintenance team lead so they can manage
	// technicians (add members, assign machines).
	_, err = tx.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'lead')`,
		maintID, claims.UserID,
	)
	if err != nil {
		h.logger.Error("create org: add creator as maintenance lead", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if err := tx.Commit(ctx); err != nil {
		h.logger.Error("create org: commit", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"id":         orgID,
		"name":       body.Name,
		"slug":       body.Slug,
		"created_at": createdAt,
	})
}

// ListOrgs handles GET /orgs — lists orgs the caller belongs to.
func (h *OrgsHandler) ListOrgs(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	ctx := c.Request().Context()
	rows, err := h.db.Query(ctx,
		`SELECT o.id, o.name, o.slug, om.role, o.created_at
		   FROM organizations o
		   JOIN org_members om ON om.org_id = o.id
		  WHERE om.user_id = $1
		  ORDER BY o.created_at DESC`,
		claims.UserID,
	)
	if err != nil {
		h.logger.Error("list orgs: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer rows.Close()

	type orgItem struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		Slug      string    `json:"slug"`
		Role      string    `json:"role"`
		CreatedAt time.Time `json:"created_at"`
	}
	result := make([]orgItem, 0)
	for rows.Next() {
		var item orgItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Role, &item.CreatedAt); err != nil {
			h.logger.Error("list orgs: scan", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		h.logger.Error("list orgs: rows err", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, result)
}

// GetOrg handles GET /orgs/:orgId.
func (h *OrgsHandler) GetOrg(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	ctx := c.Request().Context()

	var (
		name      string
		slug      string
		role      string
		createdAt time.Time
	)
	err := h.db.QueryRow(ctx,
		`SELECT o.name, o.slug, om.role, o.created_at
		   FROM organizations o
		   JOIN org_members om ON om.org_id = o.id
		  WHERE o.id = $1 AND om.user_id = $2`,
		orgID, claims.UserID,
	).Scan(&name, &slug, &role, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "organization not found or you are not a member"})
	}
	if err != nil {
		h.logger.Error("get org", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":         orgID,
		"name":       name,
		"slug":       slug,
		"role":       role,
		"created_at": createdAt,
	})
}

// DeleteOrg handles DELETE /orgs/:orgId. Admin only.
func (h *OrgsHandler) DeleteOrg(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	role, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins can delete the organization"})
	}

	ctx := c.Request().Context()
	tag, err := h.db.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	if err != nil {
		h.logger.Error("delete org", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "organization not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Team endpoints ─────────────────────────────────────────────────────────────

// CreateTeam handles POST /orgs/:orgId/teams. Admin or executor only.
func (h *OrgsHandler) CreateTeam(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	role, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" && role != "executor" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins and executors can create teams"})
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}

	teamID := fmt.Sprintf("team_%s", randomHex(8))
	ctx := c.Request().Context()

	var createdAt time.Time
	err = h.db.QueryRow(ctx,
		`INSERT INTO teams (id, org_id, name, description) VALUES ($1, $2, $3, $4) RETURNING created_at`,
		teamID, orgID, body.Name, body.Description,
	).Scan(&createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "a team with that name already exists in this organization"})
		}
		h.logger.Error("create team: insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"id":          teamID,
		"org_id":      orgID,
		"name":        body.Name,
		"description": body.Description,
		"created_at":  createdAt,
	})
}

// ListTeams handles GET /orgs/:orgId/teams.
func (h *OrgsHandler) ListTeams(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	if _, err := requireOrgMember(c, h.db, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	ctx := c.Request().Context()
	rows, err := h.db.Query(ctx,
		`SELECT t.id, t.name, t.description, t.created_at, COUNT(tm.user_id) AS member_count
		   FROM teams t
		   LEFT JOIN team_members tm ON tm.team_id = t.id
		  WHERE t.org_id = $1
		  GROUP BY t.id
		  ORDER BY t.created_at ASC`,
		orgID,
	)
	if err != nil {
		h.logger.Error("list teams: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer rows.Close()

	type teamItem struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		CreatedAt   time.Time `json:"created_at"`
		MemberCount int64     `json:"member_count"`
	}
	result := make([]teamItem, 0)
	for rows.Next() {
		var item teamItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.CreatedAt, &item.MemberCount); err != nil {
			h.logger.Error("list teams: scan", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, result)
}

// GetTeam handles GET /orgs/:orgId/teams/:teamId.
func (h *OrgsHandler) GetTeam(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	teamID := c.Param("teamId")

	if _, err := requireOrgMember(c, h.db, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	ctx := c.Request().Context()

	var (
		name        string
		description string
		createdAt   time.Time
	)
	err := h.db.QueryRow(ctx,
		`SELECT name, description, created_at FROM teams WHERE id = $1 AND org_id = $2`,
		teamID, orgID,
	).Scan(&name, &description, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "team not found"})
	}
	if err != nil {
		h.logger.Error("get team", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Fetch team members.
	rows, err := h.db.Query(ctx,
		`SELECT u.id, u.email, u.name, tm.role, tm.joined_at
		   FROM team_members tm
		   JOIN users u ON u.id = tm.user_id
		  WHERE tm.team_id = $1
		  ORDER BY tm.joined_at ASC`,
		teamID,
	)
	if err != nil {
		h.logger.Error("get team: members query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer rows.Close()

	type memberItem struct {
		ID       int64     `json:"id"`
		Email    string    `json:"email"`
		Name     string    `json:"name"`
		Role     string    `json:"role"`
		JoinedAt time.Time `json:"joined_at"`
	}
	members := make([]memberItem, 0)
	for rows.Next() {
		var m memberItem
		if err := rows.Scan(&m.ID, &m.Email, &m.Name, &m.Role, &m.JoinedAt); err != nil {
			h.logger.Error("get team: scan member", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"id":          teamID,
		"org_id":      orgID,
		"name":        name,
		"description": description,
		"created_at":  createdAt,
		"members":     members,
	})
}

// UpdateTeam handles PATCH /orgs/:orgId/teams/:teamId. Admin or executor only.
func (h *OrgsHandler) UpdateTeam(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	teamID := c.Param("teamId")

	role, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" && role != "executor" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins and executors can update teams"})
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.Name == nil && body.Description == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "at least one of name or description must be provided"})
	}

	ctx := c.Request().Context()

	tag, err := h.db.Exec(ctx,
		`UPDATE teams
		    SET name        = COALESCE($1, name),
		        description = COALESCE($2, description)
		  WHERE id = $3 AND org_id = $4`,
		body.Name, body.Description, teamID, orgID,
	)
	if err != nil {
		h.logger.Error("update team", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "team not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteTeam handles DELETE /orgs/:orgId/teams/:teamId. Admin only.
func (h *OrgsHandler) DeleteTeam(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	teamID := c.Param("teamId")

	role, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins can delete teams"})
	}

	ctx := c.Request().Context()
	tag, err := h.db.Exec(ctx, `DELETE FROM teams WHERE id = $1 AND org_id = $2`, teamID, orgID)
	if err != nil {
		h.logger.Error("delete team", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "team not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Org member endpoints ───────────────────────────────────────────────────────

// AddOrgMember handles POST /orgs/:orgId/members. Admin only.
func (h *OrgsHandler) AddOrgMember(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	role, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins can add members"})
	}

	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.Email == "" || body.Role == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email and role are required"})
	}
	if !isValidRole(body.Role) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "role must be admin, executor, employee, or technician"})
	}

	ctx := c.Request().Context()
	targetID, _, err := userByEmail(ctx, h.db, body.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	if err != nil {
		h.logger.Error("add org member: lookup user", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	_, err = h.db.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3)
		     ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, targetID, body.Role,
	)
	if err != nil {
		h.logger.Error("add org member: insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"org_id":  orgID,
		"user_id": targetID,
		"role":    body.Role,
	})
}

// InviteMember handles POST /orgs/:orgId/members/invite.
// Creates a new user + adds them to the org in one call. Admin only.
func (h *OrgsHandler) InviteMember(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	role, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins can invite members"})
	}

	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	body.Email = strings.TrimSpace(body.Email)
	body.Name = strings.TrimSpace(body.Name)

	if body.Email == "" || body.Role == "" || body.Name == "" || body.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name, email, password, and role are required"})
	}
	if !isValidRole(body.Role) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "role must be admin, executor, employee, or technician"})
	}
	if !strings.Contains(body.Email, "@") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid email address"})
	}

	ctx := c.Request().Context()

	// Check if email already exists.
	var existing int64
	err = h.db.QueryRow(ctx,
		`SELECT id FROM users WHERE email = $1 LIMIT 1`, body.Email,
	).Scan(&existing)
	if err == nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "email already registered"})
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		h.logger.Error("invite: check email", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		h.logger.Error("invite: bcrypt", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	var userID int64
	err = h.db.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		body.Email, body.Name, string(hash),
	).Scan(&userID)
	if err != nil {
		h.logger.Error("invite: insert user", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	orgLevelRole := orgRole(body.Role)
	_, err = h.db.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, $3)
		     ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID, orgLevelRole,
	)
	if err != nil {
		h.logger.Error("invite: insert member", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// If the role is "technician", add as org employee + auto-provision
	// into a Maintenance team with the technician role.
	if body.Role == "technician" {
		// Find the first Maintenance team in the org
		var teamID string
		err = h.db.QueryRow(ctx,
			`SELECT id FROM teams WHERE org_id = $1
			   AND (name ILIKE '%maintenance%' OR name ILIKE '%tech%')
			 ORDER BY created_at LIMIT 1`,
			orgID,
		).Scan(&teamID)

		if err == nil {
			_, _ = h.db.Exec(ctx,
				`INSERT INTO team_members (team_id, user_id, role)
				 VALUES ($1, $2, 'technician')
				 ON CONFLICT (team_id, user_id) DO NOTHING`,
				teamID, userID,
			)
		}
		// If no Maintenance team found, the user can be assigned manually later.
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"org_id":  orgID,
		"user_id": userID,
		"name":    body.Name,
		"email":   body.Email,
		"role":    body.Role,
	})
}

func isValidRole(r string) bool {
	return r == "admin" || r == "executor" || r == "employee" || r == "technician"
}

// orgRole maps a team role to the org-level role used for org_members.
// Technicians are stored as employees at the org level.
func orgRole(role string) string {
	if role == "technician" {
		return "employee"
	}
	return role
}

// ListOrgMembers handles GET /orgs/:orgId/members.
func (h *OrgsHandler) ListOrgMembers(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	if _, err := requireOrgMember(c, h.db, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	ctx := c.Request().Context()
	rows, err := h.db.Query(ctx,
		`SELECT u.id, u.email, u.name, om.role, om.joined_at
		   FROM org_members om
		   JOIN users u ON u.id = om.user_id
		  WHERE om.org_id = $1
		  ORDER BY om.joined_at ASC`,
		orgID,
	)
	if err != nil {
		h.logger.Error("list org members: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer rows.Close()

	type memberItem struct {
		ID       int64     `json:"id"`
		Email    string    `json:"email"`
		Name     string    `json:"name"`
		Role     string    `json:"role"`
		JoinedAt time.Time `json:"joined_at"`
	}
	result := make([]memberItem, 0)
	for rows.Next() {
		var m memberItem
		if err := rows.Scan(&m.ID, &m.Email, &m.Name, &m.Role, &m.JoinedAt); err != nil {
			h.logger.Error("list org members: scan", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, result)
}

// UpdateOrgMemberRole handles PATCH /orgs/:orgId/members/:userId. Admin only.
func (h *OrgsHandler) UpdateOrgMemberRole(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	targetUserID := c.Param("userId")

	role, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins can change member roles"})
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.Role != "admin" && body.Role != "executor" && body.Role != "employee" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "role must be admin, executor, or employee"})
	}

	ctx := c.Request().Context()
	tag, err := h.db.Exec(ctx,
		`UPDATE org_members SET role = $1 WHERE org_id = $2 AND user_id = $3`,
		body.Role, orgID, targetUserID,
	)
	if err != nil {
		h.logger.Error("update org member role", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "member not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveOrgMember handles DELETE /orgs/:orgId/members/:userId. Admin only.
// Cannot remove self if last admin.
func (h *OrgsHandler) RemoveOrgMember(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	targetUserIDStr := c.Param("userId")

	role, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins can remove members"})
	}

	ctx := c.Request().Context()

	// Prevent removing self if last admin.
	if fmt.Sprintf("%d", claims.UserID) == targetUserIDStr {
		var adminCount int
		if err := h.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM org_members WHERE org_id = $1 AND role = 'admin'`, orgID,
		).Scan(&adminCount); err != nil {
			h.logger.Error("remove org member: count admins", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		if adminCount <= 1 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot remove the last admin"})
		}
	}

	tag, err := h.db.Exec(ctx,
		`DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`,
		orgID, targetUserIDStr,
	)
	if err != nil {
		h.logger.Error("remove org member: delete", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "member not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Team member endpoints ──────────────────────────────────────────────────────

// AddTeamMember handles POST /orgs/:orgId/teams/:teamId/members.
// The user must already be an org member.
func (h *OrgsHandler) AddTeamMember(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	teamID := c.Param("teamId")

	callerRole, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if callerRole != "admin" && callerRole != "executor" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins and executors can add team members"})
	}

	var body struct {
		UserID int64  `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if body.UserID == 0 || body.Role == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "user_id and role are required"})
	}
	if body.Role != "lead" && body.Role != "member" && body.Role != "technician" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "role must be lead, member, or technician"})
	}

	ctx := c.Request().Context()

	// Verify target is org member.
	var targetRole string
	err = h.db.QueryRow(ctx,
		`SELECT role FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, body.UserID,
	).Scan(&targetRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "user is not a member of this organization"})
	}
	if err != nil {
		h.logger.Error("add team member: check org membership", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	// Verify team belongs to org.
	var teamOrgID string
	err = h.db.QueryRow(ctx, `SELECT org_id FROM teams WHERE id = $1`, teamID).Scan(&teamOrgID)
	if errors.Is(err, pgx.ErrNoRows) || teamOrgID != orgID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "team not found"})
	}
	if err != nil {
		h.logger.Error("add team member: verify team", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	_, err = h.db.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)
		     ON CONFLICT (team_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		teamID, body.UserID, body.Role,
	)
	if err != nil {
		h.logger.Error("add team member: insert", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"team_id": teamID,
		"user_id": body.UserID,
		"role":    body.Role,
	})
}

// RemoveTeamMember handles DELETE /orgs/:orgId/teams/:teamId/members/:userId.
func (h *OrgsHandler) RemoveTeamMember(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")
	teamID := c.Param("teamId")
	targetUserID := c.Param("userId")

	callerRole, err := requireOrgMember(c, h.db, orgID, claims.UserID)
	if err != nil {
		return handleOrgMemberErr(c, err)
	}
	if callerRole != "admin" && callerRole != "executor" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "only admins and executors can remove team members"})
	}

	ctx := c.Request().Context()

	// Verify team belongs to org.
	var teamOrgID string
	err = h.db.QueryRow(ctx, `SELECT org_id FROM teams WHERE id = $1`, teamID).Scan(&teamOrgID)
	if errors.Is(err, pgx.ErrNoRows) || teamOrgID != orgID {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "team not found"})
	}
	if err != nil {
		h.logger.Error("remove team member: verify team", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	tag, err := h.db.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, targetUserID,
	)
	if err != nil {
		h.logger.Error("remove team member: delete", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	if tag.RowsAffected() == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "member not found in team"})
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Agent endpoints (F10 seeds the roster; F9 adds live status) ─────────────────

// ListAgents handles GET /orgs/:orgId/agents. The roster is seeded by F10; F9
// adds a live status derived from the org's workflow nodes — each agent's nodes
// across every request in the org are aggregated into completed/active/blocked
// counts, and the status reduces to "blocked" (a node is waiting on another
// department), "busy" (a node is in_progress), or "idle". The aggregate is a
// LEFT JOIN LATERAL so agents with no activity still appear (all zero).
func (h *OrgsHandler) ListAgents(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	if _, err := requireOrgMember(c, h.db, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	ctx := c.Request().Context()
	rows, err := h.db.Query(ctx, `
		SELECT a.id, a.org_id, a.team_id, a.agent_type, a.name, a.avatar, a.capabilities, a.created_at,
		       COALESCE(t.name, '') AS team_name,
		       COALESCE(agg.completed, 0), COALESCE(agg.active, 0), COALESCE(agg.blocked, 0),
		       COALESCE(agg.total, 0), COALESCE(agg.request_count, 0), COALESCE(agg.latest_status, '')
		FROM agents a
		LEFT JOIN teams t ON t.id = a.team_id
		LEFT JOIN LATERAL (
			SELECT
				count(*)                                          AS total,
				count(*) FILTER (WHERE wn.status = 'completed')   AS completed,
				count(*) FILTER (WHERE wn.status = 'in_progress') AS active,
				count(*) FILTER (WHERE wn.status = 'blocked')     AS blocked,
				count(DISTINCT wn.request_id)                     AS request_count,
				(
					SELECT wn2.status_text
					FROM workflow_nodes wn2
					JOIN requests rq2 ON rq2.id = wn2.request_id
					WHERE rq2.org_id = a.org_id
					  AND wn2.agent_type = a.agent_type
					  AND wn2.status_text <> ''
					ORDER BY COALESCE(wn2.completed_at, wn2.started_at, wn2.created_at) DESC
					LIMIT 1
				) AS latest_status
			FROM workflow_nodes wn
			JOIN requests rq ON rq.id = wn.request_id
			WHERE rq.org_id = a.org_id AND wn.agent_type = a.agent_type
		) agg ON true
		WHERE a.org_id = $1
		ORDER BY a.created_at ASC
	`, orgID)
	if err != nil {
		h.logger.Error("list agents: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer rows.Close()

	type agentItem struct {
		ID           string    `json:"id"`
		OrgID        string    `json:"org_id"`
		TeamID       string    `json:"team_id"`
		TeamName     string    `json:"team_name"`
		AgentType    string    `json:"agent_type"`
		Name         string    `json:"name"`
		Avatar       string    `json:"avatar"`
		Capabilities string    `json:"capabilities"`
		CreatedAt    time.Time `json:"created_at"`
		Status       string    `json:"status"`
		Completed    int       `json:"completed"`
		Active       int       `json:"active"`
		Blocked      int       `json:"blocked"`
		Total        int       `json:"total"`
		RequestCount int       `json:"request_count"`
		LatestStatus string    `json:"latest_status"`
	}
	result := make([]agentItem, 0)
	for rows.Next() {
		var item agentItem
		if err := rows.Scan(
			&item.ID, &item.OrgID, &item.TeamID, &item.AgentType, &item.Name, &item.Avatar, &item.Capabilities, &item.CreatedAt, &item.TeamName,
			&item.Completed, &item.Active, &item.Blocked, &item.Total, &item.RequestCount, &item.LatestStatus,
		); err != nil {
			h.logger.Error("list agents: scan", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		item.Status = agentLiveStatus(item.Active, item.Blocked)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, map[string]any{"agents": result})
}

// agentLiveStatus reduces an agent's live node counts to a single label. A
// blocked node takes precedence (the agent is waiting on another department);
// otherwise an in_progress node means busy; everything else is idle.
func agentLiveStatus(active, blocked int) string {
	switch {
	case blocked > 0:
		return "blocked"
	case active > 0:
		return "busy"
	default:
		return "idle"
	}
}

// ── Policy endpoints (F10) ─────────────────────────────────────────────────────

// ListPolicies handles GET /orgs/:orgId/policies.
func (h *OrgsHandler) ListPolicies(c echo.Context) error {
	claims := middleware.UserFromCtx(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	orgID := c.Param("orgId")

	if _, err := requireOrgMember(c, h.db, orgID, claims.UserID); err != nil {
		return handleOrgMemberErr(c, err)
	}

	ctx := c.Request().Context()
	rows, err := h.db.Query(ctx, `
		SELECT dp.id, dp.org_id, dp.team_id, dp.title, dp.body, dp.created_at,
		       COALESCE(t.name, '') AS team_name
		FROM department_policies dp
		LEFT JOIN teams t ON t.id = dp.team_id
		WHERE dp.org_id = $1
		ORDER BY dp.created_at ASC
	`, orgID)
	if err != nil {
		h.logger.Error("list policies: query", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	defer rows.Close()

	type policyItem struct {
		ID        string    `json:"id"`
		OrgID     string    `json:"org_id"`
		TeamID    string    `json:"team_id"`
		TeamName  string    `json:"team_name"`
		Title     string    `json:"title"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"created_at"`
	}
	result := make([]policyItem, 0)
	for rows.Next() {
		var item policyItem
		if err := rows.Scan(&item.ID, &item.OrgID, &item.TeamID, &item.Title, &item.Body, &item.CreatedAt, &item.TeamName); err != nil {
			h.logger.Error("list policies: scan", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, map[string]any{"policies": result})
}

// ── helpers ────────────────────────────────────────────────────────────────────

// handleOrgMemberErr converts requireOrgMember errors to appropriate HTTP responses.
func handleOrgMemberErr(c echo.Context, err error) error {
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return c.JSON(he.Code, map[string]string{"error": fmt.Sprintf("%v", he.Message)})
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && (containsCode(err, "23505"))
}

func containsCode(err error, code string) bool {
	type pgErr interface {
		SQLState() string
	}
	var pe pgErr
	if errors.As(err, &pe) {
		return pe.SQLState() == code
	}
	return false
}
