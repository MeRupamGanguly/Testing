// Package middleware provides HTTP middleware components.
package recovery

import (
	"crosscutting/utils"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery returns a middleware that recovers from panics and logs the error.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				logger.ErrorContext(c.Request.Context(), "panic recovered",
					slog.Any("error", err),
					slog.String("stack", string(stack)),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
				)

				// Check if headers already sent
				if !c.Writer.Written() {
					utils.InternalServerError(c, "an unexpected error occurred")
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}
