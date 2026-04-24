package logging

import (
	"crosscutting/utils"
	"log/slog"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
)

// Logging returns a structured logging middleware using slog.
func Logging(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Add request ID to context and response header
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		c.Set(string(utils.RequestIDKey), reqID)
		c.Header("X-Request-ID", reqID)

		// Process request
		c.Next()

		// Log after request
		end := time.Now()
		latency := end.Sub(start)

		attrs := []slog.Attr{
			slog.String("request_id", reqID),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.Int("status", c.Writer.Status()),
			slog.String("client_ip", c.ClientIP()),
			slog.Duration("latency", latency),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Int("body_size", c.Writer.Size()),
		}

		if userID, exists := c.Get(string(utils.UserIDKey)); exists {
			attrs = append(attrs, slog.String("user_id", userID.(string)))
		}

		if len(c.Errors) > 0 {
			errs := make([]string, len(c.Errors))
			for i, e := range c.Errors {
				errs[i] = e.Error()
			}
			attrs = append(attrs, slog.Any("errors", errs))
		}

		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		} else if c.Writer.Status() >= 400 {
			level = slog.LevelWarn
		}

		logger.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

func generateRequestID() string {
	// Use a simple UUID-like generation for demonstration; in production use a proper UUID library.
	return "req_" + time.Now().Format("20060102150405") + "_" + randomString(6)
}

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// randomString generates a random alphanumeric string of given length.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}
