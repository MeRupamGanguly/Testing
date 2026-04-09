package config

import (
	"log"
	"time"

	ldredis "github.com/launchdarkly/go-server-sdk-redis-go-redis"
	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/ldcomponents"
)

// NewLDClient initializes the LaunchDarkly client.
// Includes the requested Redis configuration as an option for the persistent data store.
func NewLDClient(props LaunchDarklyProperties, useRedis bool, redisURL string) *ldclient.LDClient {
	config := ldclient.Config{}

	if props.OfflineMode || props.SdkKey == "" {
		log.Println("Initializing LaunchDarkly client in Offline mode.")
		config.Offline = true
	} else {
		log.Println("Initializing LaunchDarkly client.")
		if useRedis {
			// Using the requested Redis package for LD's internal data store
			redisStore := ldredis.DataStore().URL(redisURL)
			config.DataStore = ldcomponents.PersistentDataStore(redisStore).CacheSeconds(30)
		}
	}

	client, err := ldclient.MakeCustomClient(props.SdkKey, config, 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to create LaunchDarkly client: %v", err)
	}

	return client
}
