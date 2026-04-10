package main

import (
	ldCache "featuresgflags/LDKillSwitch/cache"
	ldConfig "featuresgflags/LDKillSwitch/config"
	ldService "featuresgflags/LDKillSwitch/service"
	"featuresgflags/SampleApp/client"
	"featuresgflags/SampleApp/config"
	"featuresgflags/SampleApp/controller"
	"featuresgflags/SampleApp/service"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}
	ldPropClient := ldConfig.LaunchDarklyProperties{
		SdkKey:      cfg.WebClient.LDKillSwitch.SdkKey,
		OfflineMode: cfg.WebClient.LDKillSwitch.Offline,
	}

	isUseRedis := cfg.WebClient.RedisConfig.Enabled
	redisUrl := cfg.WebClient.RedisConfig.Url
	localTtlScecond := cfg.WebClient.RedisConfig.LocalTtlSeconds

	ldClient := ldConfig.NewLDClient(ldPropClient, isUseRedis, redisUrl, localTtlScecond)

	ldPropCache := ldConfig.FeatureFlagCacheProperties{
		CacheEnabled:           cfg.WebClient.LDKillSwitch.CacheEnabled,
		ExpireAfterWriteMinute: time.Duration(cfg.WebClient.LDKillSwitch.ExpireAfterWriteMinute),
		MaximumSize:            cfg.WebClient.LDKillSwitch.MaximumSize,
	}
	cache := ldCache.NewFeatureFlagCache(ldClient, ldPropCache)
	featureFlagService := ldService.NewFeatureFlagService(cache)

	clients := client.NewWebClientManager(cfg.WebClient)

	webClientService := service.NewWebClientService(cfg.WebClient, clients, featureFlagService)

	locController := controller.NewLocationController(webClientService)

	router := gin.Default()
	locController.RegisterRoutes(router)

	log.Println("Server running on :8080")
	router.Run(":8080")
}
