package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/ncs26-orchestration/solution/apps/api/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles /auth/register and /auth/login.
type AuthHandler struct {
	logger    *slog.Logger
	db        *pgxpool.Pool
	jwtSecret string
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(logger *slog.Logger, db *pgxpool.Pool, jwtSecret string) *AuthHandler {
	return &AuthHandler{logger: logger, db: db, jwtSecret: jwtSecret}
}

// randomHex returns a hex-encoded string of n random bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// Register handles POST /auth/register.
// Body: { email, password, name }
// Returns: { token, user: { id, email, name } }
func (h *AuthHandler) Register(c echo.Context) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	body.Email = strings.TrimSpace(body.Email)
	body.Name = strings.TrimSpace(body.Name)

	if body.Email == "" || body.Password == "" || body.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email, password and name are required"})
	}
	if !strings.Contains(body.Email, "@") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid email address"})
	}

	ctx := c.Request().Context()

	// Check if email is already taken.
	var existing int64
	err := h.db.QueryRow(ctx,
		`SELECT id FROM users WHERE email = $1 LIMIT 1`, body.Email,
	).Scan(&existing)
	if err == nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "email already registered"})
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		h.logger.Error("register: check email", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		h.logger.Error("register: bcrypt", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	var userID int64
	err = h.db.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		body.Email, body.Name, string(hash),
	).Scan(&userID)
	if err != nil {
		h.logger.Error("register: insert user", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	token, err := auth.IssueToken(h.jwtSecret, userID, body.Email, body.Name)
	if err != nil {
		h.logger.Error("register: issue token", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"token": token,
		"user": map[string]any{
			"id":    userID,
			"email": body.Email,
			"name":  body.Name,
		},
	})
}

// Login handles POST /auth/login.
// Body: { email, password }
// Returns: { token, user: { id, email, name } }
func (h *AuthHandler) Login(c echo.Context) error {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	body.Email = strings.TrimSpace(body.Email)

	if body.Email == "" || body.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email and password are required"})
	}

	ctx := c.Request().Context()

	var (
		userID  int64
		name    string
		hashStr string
	)
	err := h.db.QueryRow(ctx,
		`SELECT id, name, password_hash FROM users WHERE email = $1 LIMIT 1`, body.Email,
	).Scan(&userID, &name, &hashStr)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}
	if err != nil {
		h.logger.Error("login: fetch user", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashStr), []byte(body.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	token, err := auth.IssueToken(h.jwtSecret, userID, body.Email, name)
	if err != nil {
		h.logger.Error("login: issue token", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id":    userID,
			"email": body.Email,
			"name":  name,
		},
	})
}

// LookupUser handles GET /users/lookup?email=xxx.
// Returns { id, name, email } for the user with the given email address.
// No auth middleware — used in invite/add-member flows.
func (h *AuthHandler) LookupUser(c echo.Context) error {
	email := c.QueryParam("email")
	if email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email required"})
	}
	var id int64
	var name string
	err := h.db.QueryRow(c.Request().Context(),
		`SELECT id, name FROM users WHERE email = $1`, email,
	).Scan(&id, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	return c.JSON(http.StatusOK, map[string]any{"id": id, "name": name, "email": email})
}

// userExists is a helper used by other handlers to look up a user by email.
func userByEmail(ctx context.Context, db *pgxpool.Pool, email string) (id int64, name string, err error) {
	err = db.QueryRow(ctx,
		`SELECT id, name FROM users WHERE email = $1 LIMIT 1`, email,
	).Scan(&id, &name)
	return
}
