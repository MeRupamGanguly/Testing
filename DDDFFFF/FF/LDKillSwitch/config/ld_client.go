package config

import (
	"errors"
	"log/slog"
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldlog"
	ldredis "github.com/launchdarkly/go-server-sdk-redis-go-redis"

	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/ldcomponents"
	"github.com/launchdarkly/go-server-sdk/v7/ldfiledata"
	"github.com/launchdarkly/go-server-sdk/v7/ldfilewatch"
)

// NewLDClient initializes the LaunchDarkly client and returns an error on failure.
func NewLDClient(props LaunchDarklyProperties, useRedis bool, redisURL string, localTtlSecond int) (*ldclient.LDClient, error) {
	slog.Debug("Creating LaunchDarkly client", "offlineMode", props.OfflineMode, "useRedis", useRedis)
	config := ldclient.Config{}

	if props.OfflineMode || props.SdkKey == "" {
		flagsPath := "../../flags.json"

		slog.Info("Offline Mode: Using Local File Data Source", "path", flagsPath)
		config.DataSource = ldfiledata.DataSource().
			FilePaths(flagsPath). // Use the detected path
			Reloader(ldfilewatch.WatchFiles)
		config.Logging = ldcomponents.Logging().MinLevel(ldlog.Debug)
		config.Events = ldcomponents.NoEvents()
	} else {
		slog.Info("Initializing LaunchDarkly client with live SDK")
		if useRedis {
			if redisURL == "" {
				return nil, errors.New("redis URL is required when useRedis is true")
			}
			redisStore := ldredis.DataStore().URL(redisURL)
			config.DataStore = ldcomponents.PersistentDataStore(redisStore).CacheSeconds(localTtlSecond)
			slog.Info("Redis persistent datastore enabled", "url", redisURL, "cacheSeconds", localTtlSecond)
		}
	}

	client, err := ldclient.MakeCustomClient(props.SdkKey, config, 5*time.Second)
	if err != nil {
		slog.Error("Failed to create LaunchDarkly client", "error", err)
		return nil, err
	}
	slog.Info("LaunchDarkly client ready")
	return client, nil
}
