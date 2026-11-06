package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Context keys populated by AuthMiddleware.
const (
	CallerIDKey = "callerID"
	TenantIDKey = "tenantID"
	RoleKey     = "role"
	RolesKey    = "roles"
)

// AuthMiddleware validates the bearer JWT and stores the caller identity in the
// Gin context. It implements the authentication errors documented in authDoc.md.
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortWithAuthError(c, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			abortWithAuthError(c, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(jwtSecret), nil
		}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))

		if err != nil || !token.Valid {
			abortWithAuthError(c, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		if claims.UserID == "" {
			abortWithAuthError(c, http.StatusUnauthorized, "invalid token claims: missing user_id")
			return
		}

		c.Set(CallerIDKey, claims.UserID)
		c.Set(RoleKey, claims.Role)
		c.Set(RolesKey, claims.Roles)
		if claims.Tenant != "" {
			c.Set(TenantIDKey, claims.Tenant)
		}

		c.Next()
	}
}

// AuthzMiddleware restricts a route to any of the given roles. With no roles it
// simply requires an authenticated caller.
func AuthzMiddleware(roles ...Role) gin.HandlerFunc {
	allowed := make([]string, 0, len(roles))
	for _, r := range roles {
		allowed = append(allowed, string(r))
	}

	return func(c *gin.Context) {
		if len(allowed) == 0 {
			c.Next()
			return
		}

		if !callerHasAnyRole(c, allowed) {
			abortWithAuthError(c, http.StatusForbidden, "insufficient permissions")
			return
		}

		c.Next()
	}
}

// callerHasAnyRole reports whether the authenticated caller holds one of roles.
func callerHasAnyRole(c *gin.Context, roles []string) bool {
	role, _ := c.Get(RoleKey)
	if rs, ok := role.(string); ok && rs != "" {
		for _, allowed := range roles {
			if rs == allowed {
				return true
			}
		}
	}

	rolesValue, _ := c.Get(RolesKey)
	candidates, _ := rolesValue.([]string)
	for _, candidate := range candidates {
		for _, allowed := range roles {
			if candidate == allowed {
				return true
			}
		}
	}

	return false
}

// abortWithAuthError writes the {"error": ...} envelope documented for auth failures.
func abortWithAuthError(c *gin.Context, status int, message string) {
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.AbortWithStatusJSON(status, ErrorResponse{Error: message})
}
