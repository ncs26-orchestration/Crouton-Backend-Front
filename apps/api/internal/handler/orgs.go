package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
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

	// Seed standard department teams, agents, and starter policies (F10).
	type deptSeed struct {
		teamID       string
		name         string
		agentType    string
		agentName    string
		capabilities string
	}
	depts := []deptSeed{
		{"team_" + randomHex(8), "Finance", "finance", "Finance Agent", "Budget analysis, spend approval, financial risk assessment, ROI calculation"},
		{"team_" + randomHex(8), "Legal", "legal", "Legal Agent", "Contract review, regulatory compliance, risk flagging, policy advisory"},
		{"team_" + randomHex(8), "IT", "it", "IT Agent", "Technical feasibility, security assessment, infrastructure planning, systems integration"},
		{"team_" + randomHex(8), "HR", "hr", "HR Agent", "Staffing assessment, hiring plan, onboarding logistics, people ops"},
		{"team_" + randomHex(8), "Operations", "ops", "Operations Agent", "Logistics planning, facilities, timeline management, execution coordination"},
		{"team_" + randomHex(8), "Planning", "planning", "Planning Agent", "Workflow planning, dependency mapping, timeline estimation"},
		{"team_" + randomHex(8), "Executive", "approval", "Executive Approver", "Strategic decision-making, cross-functional review, approval authority"},
	}
	for _, d := range depts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO teams (id, org_id, name, description) VALUES ($1, $2, $3, $4)
		`, d.teamID, orgID, d.name, d.name+" department"); err != nil {
			h.logger.Error("create org: seed team", slog.String("name", d.name), slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO agents (id, org_id, team_id, agent_type, name, capabilities) VALUES ($1, $2, $3, $4, $5, $6)
		`, "agent_"+randomHex(8), orgID, d.teamID, d.agentType, d.agentName, d.capabilities); err != nil {
			h.logger.Error("create org: seed agent", slog.String("name", d.name), slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
	}

	// Seed starter department policies (one per dept).
	type policySeed struct {
		teamID string
		title  string
		body   string
	}
	policies := []policySeed{
		{depts[0].teamID, "Finance Policy", "All expenditures over $10k require executive approval. Budget allocations must align with quarterly planning. Vendor contracts must include payment terms and cancellation clauses."},
		{depts[1].teamID, "Legal Policy", "All contracts must be reviewed for regulatory compliance. Non-disclosure agreements follow the standard template. Data privacy laws (GDPR, CCPA) apply to any cross-border data handling."},
		{depts[2].teamID, "IT Policy", "New systems must pass a security assessment. Software procurement follows the approved vendor list. Infrastructure changes require change management approval."},
		{depts[3].teamID, "HR Policy", "New headcount requires approved job descriptions and budget allocation. Onboarding includes equipment provisioning, system access, and compliance training."},
		{depts[4].teamID, "Operations Policy", "Project timelines must account for dependencies and buffer time. Vendor onboarding follows the standard integration checklist."},
	}
	for _, p := range policies {
		if _, err := tx.Exec(ctx, `
			INSERT INTO department_policies (id, org_id, team_id, title, body) VALUES ($1, $2, $3, $4, $5)
		`, "pol_"+randomHex(8), orgID, p.teamID, p.title, p.body); err != nil {
			h.logger.Error("create org: seed policy", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
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
	if body.Role != "admin" && body.Role != "executor" && body.Role != "employee" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "role must be admin, executor, or employee"})
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
	if body.Role != "lead" && body.Role != "member" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "role must be lead or member"})
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

// ── Agent endpoints (F10) ──────────────────────────────────────────────────────

// ListAgents handles GET /orgs/:orgId/agents.
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
		       COALESCE(t.name, '') AS team_name
		FROM agents a
		LEFT JOIN teams t ON t.id = a.team_id
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
	}
	result := make([]agentItem, 0)
	for rows.Next() {
		var item agentItem
		if err := rows.Scan(&item.ID, &item.OrgID, &item.TeamID, &item.AgentType, &item.Name, &item.Avatar, &item.Capabilities, &item.CreatedAt, &item.TeamName); err != nil {
			h.logger.Error("list agents: scan", slog.String("err", err.Error()))
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
	return c.JSON(http.StatusOK, result)
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
	return c.JSON(http.StatusOK, result)
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
