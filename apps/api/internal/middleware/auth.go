package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/ncs26-orchestration/solution/apps/api/internal/auth"
)

const claimsKey = "user"

// NewAuthMiddleware returns an Echo middleware that validates a Bearer JWT.
// It stores *auth.Claims in the context under the key "user".
func NewAuthMiddleware(jwtSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid authorization header format"})
			}

			claims, err := auth.ParseToken(jwtSecret, parts[1])
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			}

			c.Set(claimsKey, claims)
			return next(c)
		}
	}
}

// UserFromCtx extracts *auth.Claims from the echo context.
// Returns nil if no claims are present (e.g. unauthenticated route).
func UserFromCtx(c echo.Context) *auth.Claims {
	v := c.Get(claimsKey)
	if v == nil {
		return nil
	}
	claims, _ := v.(*auth.Claims)
	return claims
}
