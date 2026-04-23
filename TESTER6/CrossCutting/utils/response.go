// Package response provides a standardized API response envelope and header policies.
package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope is the standard JSON response structure.
type Envelope struct {
	Success bool                   `json:"success"`
	Data    any                    `json:"data,omitempty"`
	Error   *ErrorDetail           `json:"error,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// ErrorDetail contains structured error information.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// HeaderPolicy adds standard security and metadata headers to responses.
func HeaderPolicy(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("X-XSS-Protection", "1; mode=block")
	// Add request ID from context if available
	if reqID, exists := c.Get("request_id"); exists {
		c.Header("X-Request-ID", reqID.(string))
	}
}

// OK sends a successful response with data.
func OK(c *gin.Context, data any, meta ...map[string]interface{}) {
	HeaderPolicy(c)
	resp := Envelope{
		Success: true,
		Data:    data,
	}
	if len(meta) > 0 {
		resp.Meta = meta[0]
	}
	c.JSON(http.StatusOK, resp)
}

// Created sends a 201 Created response.
func Created(c *gin.Context, data any) {
	HeaderPolicy(c)
	c.JSON(http.StatusCreated, Envelope{
		Success: true,
		Data:    data,
	})
}

// NoContent sends a 204 No Content response.
func NoContent(c *gin.Context) {
	HeaderPolicy(c)
	c.Status(http.StatusNoContent)
}

// Error sends an error response with appropriate HTTP status.
func Error(c *gin.Context, status int, code, message string, field ...string) {
	HeaderPolicy(c)
	errDetail := ErrorDetail{
		Code:    code,
		Message: message,
	}
	if len(field) > 0 {
		errDetail.Field = field[0]
	}
	c.AbortWithStatusJSON(status, Envelope{
		Success: false,
		Error:   &errDetail,
	})
}

// ValidationError sends a 422 Unprocessable Entity for validation failures.
func ValidationError(c *gin.Context, field, message string) {
	Error(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", message, field)
}

// Unauthorized sends a 401 Unauthorized response.
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "authentication required"
	}
	Error(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden sends a 403 Forbidden response.
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "access denied"
	}
	Error(c, http.StatusForbidden, "FORBIDDEN", message)
}

// NotFound sends a 404 Not Found response.
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "resource not found"
	}
	Error(c, http.StatusNotFound, "NOT_FOUND", message)
}

// InternalServerError sends a 500 Internal Server Error response.
func InternalServerError(c *gin.Context, message string) {
	if message == "" {
		message = "an internal error occurred"
	}
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}
