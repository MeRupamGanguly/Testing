// File: pkg/middleware/auth.go
package auth

import (
	"crosscutting/middleware/tracing"
	"crosscutting/utils"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig holds JWT configuration.
type AuthConfig struct {
	JWTSecret      string
	TokenLookup    string // e.g., "header:Authorization"
	AuthScheme     string // e.g., "Bearer"
	ExcludePaths   []string
	RoleRequired   bool   // if true, role claim is required
	AdminRoleValue string // e.g., "admin"
}

// Auth returns a JWT authentication middleware.
func Auth(config AuthConfig) gin.HandlerFunc {
	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization"
	}
	if config.AuthScheme == "" {
		config.AuthScheme = "Bearer"
	}

	return func(c *gin.Context) {
		// Skip excluded paths
		for _, path := range config.ExcludePaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				c.Next()
				return
			}
		}

		var token string
		parts := strings.Split(config.TokenLookup, ":")
		if len(parts) != 2 {
			utils.InternalServerError(c, "invalid auth configuration")
			return
		}
		switch parts[0] {
		case "header":
			authHeader := c.GetHeader(parts[1])
			token = strings.TrimPrefix(authHeader, config.AuthScheme+" ")
		case "query":
			token = c.Query(parts[1])
		case "cookie":
			var err error
			token, err = c.Cookie(parts[1])
			if err != nil {
				utils.Unauthorized(c, "missing authentication token")
				return
			}
		default:
			utils.InternalServerError(c, "unsupported token lookup method")
			return
		}

		if token == "" {
			utils.Unauthorized(c, "missing authentication token")
			return
		}

		// Parse and validate JWT
		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(config.JWTSecret), nil
		})

		if err != nil || !parsedToken.Valid {
			utils.Unauthorized(c, "invalid or expired token")
			return
		}

		claims, ok := parsedToken.Claims.(jwt.MapClaims)
		if !ok {
			utils.Unauthorized(c, "invalid token claims")
			return
		}

		// Extract user ID (sub claim) and role
		userID, ok := claims["sub"].(string)
		if !ok {
			utils.Unauthorized(c, "invalid user identifier")
			return
		}

		// Store in context
		c.Set(string(utils.UserIDKey), userID)

		if role, ok := claims["role"].(string); ok {
			c.Set(string(utils.UserRoleKey), role)
		}

		// Add user ID to tracing span
		tracing.AddSpanUserID(c, userID)

		// Role-based access control check
		if config.RoleRequired {
			role, _ := c.Get(string(utils.UserRoleKey))
			if role != config.AdminRoleValue {
				utils.Forbidden(c, "insufficient permissions")
				return
			}
		}

		c.Next()
	}
}

// RequireRole is a convenience middleware to check role.
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(string(utils.UserRoleKey))
		if !exists || role != requiredRole {
			utils.Forbidden(c, "insufficient permissions")
			return
		}
		c.Next()
	}
}
