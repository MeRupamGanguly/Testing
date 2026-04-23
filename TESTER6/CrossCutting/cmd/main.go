package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"crosscutting/middleware/auth"
	"crosscutting/middleware/logging"
	"crosscutting/middleware/metrics"
	"crosscutting/middleware/ratelimit"
	"crosscutting/middleware/recovery"
	"crosscutting/middleware/tracing"
	"crosscutting/utils"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"golang.org/x/time/rate"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	tp := initTracer(logger)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Register custom validations with Gin's default validator.
	utils.RegisterCustomValidations()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer redisClient.Close()

	router := gin.New()

	router.Use(recovery.Recovery(logger))
	router.Use(logging.Logging(logger))
	router.Use(tracing.Tracing("ecommerce-api"))
	router.Use(metrics.Metrics())

	if err := redisClient.Ping(context.Background()).Err(); err == nil {
		logger.Info("using Redis leaky bucket rate limiter")
		router.Use(ratelimit.LeakyBucketRateLimit(ratelimit.RateLimitConfig{
			RedisClient: redisClient,
			KeyPrefix:   "rl:api:",
			Limit:       100,
			Window:      time.Minute,
		}))
	} else {
		logger.Warn("Redis not available, falling back to in-memory token bucket", "error", err)
		router.Use(ratelimit.TokenBucketRateLimit(rate.Limit(10), 20))
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		logger.Warn("JWT_SECRET not set, using default (insecure)")
		secret = "change-me-in-production"
	}
	authMiddleware := auth.Auth(auth.AuthConfig{
		JWTSecret:      secret,
		TokenLookup:    "header:Authorization",
		AuthScheme:     "Bearer",
		ExcludePaths:   []string{"/health", "/metrics"},
		RoleRequired:   false,
		AdminRoleValue: "admin",
	})

	router.GET("/health", healthHandler)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := router.Group("/api/v1")
	api.Use(authMiddleware)
	{
		api.GET("/profile", getProfile)
		api.POST("/orders", createOrder)

		admin := api.Group("/admin")
		admin.Use(auth.RequireRole("admin"))
		{
			admin.GET("/users", listUsers)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	logger.Info("starting server", "port", port)
	if err := router.Run(":" + port); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func healthHandler(c *gin.Context) {
	utils.OK(c, gin.H{"status": "ok"})
}

type CreateOrderRequest struct {
	ProductSKU string  `json:"product_sku" binding:"required,sku"`
	Quantity   int     `json:"quantity" binding:"required,min=1"`
	Price      float64 `json:"price" binding:"required,price"`
}

func createOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	userID, _ := c.Get(string(utils.UserIDKey))
	utils.Created(c, gin.H{
		"order_id": "ord_123",
		"user_id":  userID,
	})
}

func getProfile(c *gin.Context) {
	userID, _ := c.Get(string(utils.UserIDKey))
	utils.OK(c, gin.H{
		"user_id": userID,
		"email":   "user@example.com",
	})
}

func listUsers(c *gin.Context) {
	utils.OK(c, []string{"user1", "user2"})
}

func initTracer(logger *slog.Logger) *sdktrace.TracerProvider {
	exporter, err := otlptracegrpc.New(context.Background())
	if err != nil {
		logger.Error("failed to create OTLP trace exporter", "error", err)
		return sdktrace.NewTracerProvider()
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("ecommerce-api"),
		),
	)
	if err != nil {
		logger.Error("failed to create resource", "error", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	return tp
}
