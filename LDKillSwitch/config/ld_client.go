package config

import (
	"fmt"
	"log"
	"time"

	ldredis "github.com/launchdarkly/go-server-sdk-redis-go-redis"
	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/ldcomponents"
	"github.com/launchdarkly/go-server-sdk/v7/ldfiledata"
)

// NewLDClient initializes the LaunchDarkly client.
func NewLDClient(props LaunchDarklyProperties, useRedis bool, redisURL string, localTtlSecond int) *ldclient.LDClient {
	fmt.Println("This IS FROM CORE KILLSWITCH CLIENT CONFIG Line Number 16")
	config := ldclient.Config{}

	if props.OfflineMode || props.SdkKey == "" {
		log.Println("Offline Mode: Using Local File Data Source.")

		// OPTION A: Using flags.json (preferred for your earlier setup)
		config.DataSource = ldfiledata.DataSource().FilePaths("flags.json")

		// OPTION B: If you prefer programmatic (uncomment to use)
		// td := ldtestdata.DataSource()
		// td.Flag("location-feature-flag").VariationForAll(true).On(true)
		// config.DataSource = td

		config.Events = ldcomponents.NoEvents()
	} else {
		log.Println("Initializing LaunchDarkly client.")
		if useRedis {
			redisStore := ldredis.DataStore().URL(redisURL)
			config.DataStore = ldcomponents.PersistentDataStore(redisStore).CacheSeconds(localTtlSecond)
		}
	}

	client, err := ldclient.MakeCustomClient(props.SdkKey, config, 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to create LaunchDarkly client: %v", err)
	}

	return client
}
