package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/rupam/ldkillswitch/cache"
	"github.com/rupam/ldkillswitch/config"
	"github.com/rupam/ldkillswitch/service"
	"github.com/rupam/ldwebapp/client"

	appConfig "yourmodule/config"
	"yourmodule/controller"
	appService "yourmodule/service"
)

func main() {
	// 1. Load Application Configuration
	cfg, err := appConfig.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}

	// 2. Initialize your ldkillswitch library components
	// Mapping from app yaml to library config
	libConfig := config.LDConfig{
		SDKKey:  "your-key",
		Offline: true,
	}

	// Optional: Initialize your caching layer if library requires it
	ldCache := cache.NewInMemoryCache()

	// Initialize the library service
	featureFlagService, err := service.NewFeatureFlagService(libConfig, ldCache)
	if err != nil {
		log.Fatalf("Failed to init FeatureFlagService: %v", err)
	}

	// 3. Setup WebClients (Circuit Breakers/Retries)
	// Passing the webclient properties from YAML
	clients := client.NewWebClientManager(cfg.WebClient)

	// 4. Setup the App Service (Injecting the library service)
	webClientService := appService.NewWebClientService(cfg.WebClient, clients, featureFlagService)

	// 5. Setup Controller & Gin
	locController := controller.NewLocationController(webClientService)

	router := gin.Default()
	locController.RegisterRoutes(router)

	log.Println("Server running on :8080")
	router.Run(":8080")
}
