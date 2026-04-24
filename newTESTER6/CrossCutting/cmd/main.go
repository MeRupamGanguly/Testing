package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"crosscutting/config"
	"crosscutting/middleware/auth"
	"crosscutting/middleware/logging"
	"crosscutting/middleware/ratelimit"
	"crosscutting/middleware/recovery"
	"crosscutting/middleware/sizelimit"
	"crosscutting/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Override with environment variables
	if s := os.Getenv("PORT"); s != "" {
		if p, err := strconv.Atoi(s); err == nil {
			cfg.Server.Port = p
		}
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	utils.RegisterCustomValidations()
	utils.SetMaxResponseSize(cfg.Limits.MaxResponseBody)

	// Parse the rate limit window duration
	windowDuration, err := time.ParseDuration(cfg.RateLimit.Window)
	if err != nil {
		logger.Error("invalid rate limit window", "error", err)
		os.Exit(1)
	}

	// Redis client (optional)
	var redisClient *redis.Client
	if cfg.Redis.Addr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			logger.Warn("Redis ping failed, falling back to in-memory limiter", "error", err)
			redisClient = nil
		} else {
			defer redisClient.Close()
		}
	}

	router := gin.New()
	router.Use(recovery.Recovery(logger))
	router.Use(logging.Logging(logger))

	// Apply size limits
	if cfg.Limits.MaxHeaderBytes > 0 {
		router.Use(sizelimit.HeaderSize(cfg.Limits.MaxHeaderBytes))
	}
	if cfg.Limits.MaxRequestBody > 0 {
		router.Use(sizelimit.RequestBody(cfg.Limits.MaxRequestBody))
	}

	// Rate limiting
	if redisClient != nil {
		logger.Info("using Redis leaky bucket rate limiter")
		router.Use(ratelimit.LeakyBucketRateLimit(ratelimit.RateLimitConfig{
			RedisClient: redisClient,
			KeyPrefix:   "rl:api:",
			Limit:       cfg.RateLimit.Limit,
			Window:      windowDuration,
		}))
	} else {
		logger.Warn("Redis not available, falling back to in-memory token bucket")
		router.Use(ratelimit.TokenBucketRateLimit(rate.Limit(10), 20))
	}

	authMiddleware := auth.Auth(auth.AuthConfig{
		JWTSecret:      cfg.JWT.Secret,
		TokenLookup:    "header:Authorization",
		AuthScheme:     "Bearer",
		ExcludePaths:   []string{"/health"},
		RoleRequired:   false,
		AdminRoleValue: "admin",
	})

	router.GET("/health", healthHandler)

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

	addr := ":" + strconv.Itoa(cfg.Server.Port)
	logger.Info("starting server", "port", cfg.Server.Port)
	if err := router.Run(addr); err != nil {
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
