package sizelimit

import (
	"fmt"
	"io"
	"net/http"

	"crosscutting/utils"

	"github.com/gin-gonic/gin"
)

// Config holds all limits. Zero or negative value disables the check.
type Config struct {
	MaxHeaderBytes  int // total size of all request header lines
	MaxRequestBody  int64
	MaxResponseBody int64
}

// HeaderSize returns middleware that aborts with 431 if the total
// header size exceeds maxBytes.
func HeaderSize(maxBytes int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		var total int
		for key, values := range c.Request.Header {
			for _, v := range values {
				total += len(key) + len(v) + 2 // "key: value\r\n" approximately
			}
		}
		if total > maxBytes {
			utils.Error(c, http.StatusRequestHeaderFieldsTooLarge,
				"HEADER_TOO_LARGE",
				fmt.Sprintf("total header size exceeds %d bytes", maxBytes))
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequestBody returns middleware that limits the raw request body to maxBytes.
// Exceeding the limit causes a 413 Payload Too Large error.
func RequestBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()

		// MaxBytesReader sets an error on the Reader when limit exceeded.
		// Gin does not always record it as a c.Error, but the binding will fail.
		// We check for the typical error string.
		for _, e := range c.Errors {
			if e.Err != nil && e.Err.Error() == "http: request body too large" {
				utils.Error(c, http.StatusRequestEntityTooLarge,
					"PAYLOAD_TOO_LARGE",
					fmt.Sprintf("request body exceeds %d bytes", maxBytes))
				c.Abort()
				return
			}
		}
	}
}

// responseWriter wraps gin.ResponseWriter to count bytes written.
type sizeTrackingWriter struct {
	gin.ResponseWriter
	maxBytes int64
	written  int64
}

func (w *sizeTrackingWriter) Write(data []byte) (n int, err error) {
	if w.written+int64(len(data)) > w.maxBytes {
		// Stop writing and return an error
		return 0, io.ErrShortWrite
	}
	n, err = w.ResponseWriter.Write(data)
	w.written += int64(n)
	return
}

func (w *sizeTrackingWriter) WriteString(s string) (n int, err error) {
	if w.written+int64(len(s)) > w.maxBytes {
		return 0, io.ErrShortWrite
	}
	n, err = w.ResponseWriter.WriteString(s)
	w.written += int64(n)
	return
}

// ResponseBody returns middleware that limits the total response body size.
// If the limit is exceeded during writing, the connection is truncated and
// a 500 error has already been partially sent – for a clean JSON error you
// should enforce limits in the response helpers instead. This middleware is
// best‑effort; for strict control combine it with response‑helper checks.
func ResponseBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		// Replace writer with tracking wrapper.
		originalWriter := c.Writer
		tracker := &sizeTrackingWriter{
			ResponseWriter: originalWriter,
			maxBytes:       maxBytes,
		}
		c.Writer = tracker
		c.Next()
		// After handlers have written, we cannot send a different response,
		// so truncation has already occurred. For a proper 500 JSON error,
		// use the response helper limit approach instead.
	}
}
