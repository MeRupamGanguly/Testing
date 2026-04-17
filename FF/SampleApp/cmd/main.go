package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ldCache "featuresgflags/LDKillSwitch/cache"
	ldConfig "featuresgflags/LDKillSwitch/config"
	ldLogging "featuresgflags/LDKillSwitch/logger"
	ldService "featuresgflags/LDKillSwitch/service"
	"featuresgflags/SampleApp/client"
	appConfig "featuresgflags/SampleApp/config"
	"featuresgflags/SampleApp/controller"
	"featuresgflags/SampleApp/service"

	"github.com/gin-gonic/gin"
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

func main() {
	// 1. Load application config
	cfg, err := appConfig.LoadConfig("config.yaml")
	if err != nil {
		slog.Error("Config load failed", "error", err)
		os.Exit(1)
	}

	// 2. LaunchDarkly client setup
	ldPropClient := ldConfig.LaunchDarklyProperties{
		SdkKey:      cfg.WebClient.LDKillSwitch.SdkKey,
		OfflineMode: cfg.WebClient.LDKillSwitch.Offline,
	}
	isUseRedis := cfg.WebClient.RedisConfig.Enabled
	redisUrl := cfg.WebClient.RedisConfig.Url
	localTtlSecond := cfg.WebClient.RedisConfig.LocalTtlSeconds

	ldClient, err := ldConfig.NewLDClient(ldPropClient, isUseRedis, redisUrl, localTtlSecond)
	if err != nil {
		slog.Error("Failed to initialize LaunchDarkly client", "error", err)
		os.Exit(1)
	}
	defer ldClient.Close()

	// 3. Cache and FeatureFlagService (without MaximumSize)
	ldPropCache := ldConfig.FeatureFlagCacheProperties{
		CacheEnabled:           cfg.WebClient.LDKillSwitch.CacheEnabled,
		ExpireAfterWriteMinute: time.Duration(cfg.WebClient.LDKillSwitch.ExpireAfterWriteMinute),
	}
	cache := ldCache.NewFeatureFlagCache(ldClient, ldPropCache)
	featureFlagService := ldService.NewFeatureFlagService(cache)

	// ========== DYNAMIC LOG LEVEL INTEGRATION ==========
	logCtx := ldcontext.New("sample-app-instance")
	logMgr := ldLogging.NewLogLevelManager(
		featureFlagService,
		"log-levels",
		logCtx,
		30*time.Second,
	)
	logMgr.Start()
	defer logMgr.Stop()

	baseHandler := slog.NewJSONHandler(os.Stdout, nil)
	dynamicHandler := logMgr.SetLogLevelHandler(baseHandler)
	slog.SetDefault(slog.New(dynamicHandler))

	slog.Info("🚀 Application starting with dynamic log level", "initial_level", logMgr.Level())

	// 4. Web clients and service layer
	clients := client.NewWebClientManager(cfg.WebClient)
	webClientService := service.NewWebClientService(cfg.WebClient, clients, featureFlagService)

	// 5. Controller & routes
	locController := controller.NewLocationController(webClientService)

	router := gin.Default()
	locController.RegisterRoutes(router)

	// Optional: health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	slog.Debug("Debug log example – only visible when log-level is DEBUG")

	// 6. Graceful shutdown
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		slog.Info("Server listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced shutdown", "error", err)
	}
	slog.Info("Server exited")
}
