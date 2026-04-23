package tracing

import (
	"crosscutting/utils"

	"github.com/gin-gonic/gin"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Tracing returns OpenTelemetry tracing middleware with custom attributes.
func Tracing(serviceName string) gin.HandlerFunc {
	// Use standard otelgin middleware but we wrap to add request ID and user info as span attributes.
	otelMiddleware := otelgin.Middleware(serviceName, otelgin.WithPropagators(propagation.TraceContext{}))

	return func(c *gin.Context) {
		// Run standard OpenTelemetry instrumentation
		otelMiddleware(c)

		// After otelgin has started the span, we can add custom attributes.
		span := trace.SpanFromContext(c.Request.Context())
		if span.IsRecording() {
			// Add request ID if present
			if reqID, exists := c.Get(string(utils.RequestIDKey)); exists {
				span.SetAttributes(attribute.String("request.id", reqID.(string)))
			}
			// Add user ID if authenticated later; we can also defer a function to add after auth middleware.
			// We'll rely on auth middleware to add attributes using the same span.
		}

		// Store trace ID in context for logging correlation
		if span.SpanContext().HasTraceID() {
			c.Set(string(utils.TraceIDKey), span.SpanContext().TraceID().String())
		}
		c.Next()
	}
}

// AddSpanUserID can be called from auth middleware to enrich the span with user ID.
func AddSpanUserID(c *gin.Context, userID string) {
	span := trace.SpanFromContext(c.Request.Context())
	if span.IsRecording() {
		span.SetAttributes(attribute.String("user.id", userID))
	}
}
