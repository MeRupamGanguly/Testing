package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	// Import your internal packages (adjust module paths to match your go.mod)
	ldCaching "github.com/rupam/ldkillswitch/cache"
	ldConfig "github.com/rupam/ldkillswitch/config"
	ldService "github.com/rupam/ldkillswitch/service"

	appConfig "github.com/rupam/ldwebapp/config"
	"github.com/rupam/ldwebapp/controller"
	"github.com/rupam/ldwebapp/service"
)

// MockLocationService implements the interface required by the LocationController.
// In a real application, you would put this in parentapp/service/location_service.go
type MockLocationService struct {
	webClient *service.Service
}

func (m *MockLocationService) GetLocationByIDAsString(locationID, sourceSystem, sourceChannel string, store int) (string, error) {
	// Use fmt.Sprintf for clean string building instead of rune casting
	return fmt.Sprintf(`{"id":"%s", "source":"%s", "store":%d, "status":"active"}`,
		locationID, sourceSystem, store), nil
}

func (m *MockLocationService) GetLocationByID(locationID string) (string, error) {
	return `{"id":"` + locationID + `", "name":"Mock Location"}`, nil
}

func main() {
	// =========================================================================
	// 1. Initialize LaunchDarkly Core Library (ldkillswitch)
	// =========================================================================

	// These properties would typically be loaded from env vars or a YAML file using Viper
	ldProps := ldConfig.LaunchDarklyProperties{
		SdkKey:      "your-ld-sdk-key-here",
		OfflineMode: true, // Set to true for local testing without a key
	}

	// Initialize the LD Client (set useRedis to false for this local example)
	ldClient := ldConfig.NewLDClient(ldProps, false, "")

	cacheProps := ldConfig.FeatureFlagCacheProperties{
		CacheEnabled:           true,
		ExpireAfterWriteMinute: 5 * time.Minute,
		MaximumSize:            1000,
	}

	ffCache := ldCaching.NewFeatureFlagCache(ldClient, cacheProps)
	ffService := ldService.NewFeatureFlagService(ffCache)

	// =========================================================================
	// 2. Initialize Parent App Services & WebClients
	// =========================================================================

	webClientProps := appConfig.WebClientProperties{
		BaseURL:        "http://localhost:8080",
		ConnectTimeout: 5000,
		ReadTimeout:    5000,
		Retry: appConfig.RetryConfig{
			Enabled:     true,
			MaxAttempts: 3,
			Backoff: appConfig.BackoffConfig{
				Delay:      1000,
				MaxDelay:   5000,
				Multiplier: 2.0,
			},
		},
		CircuitBreaker: appConfig.CircuitBreakerConfig{
			Enabled:              true,
			FailureRateThreshold: 50.0,
			MinimumNumberOfCalls: 10,
			TimeoutOpenStateMs:   30000,
		},
		Services: map[string]appConfig.ServiceConfig{
			"inventoryService": {
				Path: "/api/inventory",
			},
		},
	}

	// Initialize the resilient web client service
	webClientService := service.NewWebClientService(webClientProps, ffService)

	// Initialize domain services
	locationService := &MockLocationService{
		webClient: webClientService,
	}

	// =========================================================================
	// 3. Initialize Controllers & Web Server
	// =========================================================================

	locationController := controller.NewLocationController(locationService)

	// Initialize Gin Engine
	router := gin.Default()

	// Register the endpoints defined in the controller
	locationController.RegisterRoutes(router)

	log.Println("Wiring complete. Starting server on :8080...")

	// Start the server
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
