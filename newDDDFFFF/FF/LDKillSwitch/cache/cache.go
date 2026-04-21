package cache

import (
	"encoding/json"
	"featuresgflags/LDKillSwitch/config"
	"fmt"
	"log/slog"
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldvalue"
	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/patrickmn/go-cache"
)

type FeatureFlagCache struct {
	ldClient     *ldclient.LDClient
	flagCache    *cache.Cache
	cacheEnabled bool
}

func NewFeatureFlagCache(client *ldclient.LDClient, props config.FeatureFlagCacheProperties) *FeatureFlagCache {
	expirationTime := props.ExpireAfterWriteMinute * time.Minute
	cleanupInterval := expirationTime * 2
	c := cache.New(expirationTime, cleanupInterval)
	slog.Info("Initialized FeatureFlagCache", "ttl", expirationTime)

	return &FeatureFlagCache{
		ldClient:     client,
		flagCache:    c,
		cacheEnabled: props.CacheEnabled,
	}
}

// GetBooleanFlagValue works exactly as before
func (f *FeatureFlagCache) GetBooleanFlagValue(flagKey string, ctx ldcontext.Context, defaultValue bool) bool {
	slog.Debug("Cache lookup", "flagKey", flagKey, "contextKey", ctx.Key())
	cacheKey := fmt.Sprintf("bool:%s:%s", flagKey, ctx.Key())

	if f.cacheEnabled {
		if val, found := f.flagCache.Get(cacheKey); found {
			slog.Debug("Cache hit", "flagKey", flagKey, "value", val)
			return val.(bool)
		}
	}

	slog.Debug("Cache miss", "flagKey", flagKey, "contextKey", ctx.Key())
	freshValue, err := f.ldClient.BoolVariation(flagKey, ctx, defaultValue)
	if err != nil {
		slog.Error("Error evaluating flag", "flagKey", flagKey, "error", err)
		return defaultValue
	}

	slog.Debug("LaunchDarkly evaluated", "flagKey", flagKey, "value", freshValue)
	f.flagCache.Set(cacheKey, freshValue, cache.DefaultExpiration)
	return freshValue
}

// GetStringFlagValue for string flags
func (f *FeatureFlagCache) GetStringFlagValue(flagKey string, ctx ldcontext.Context, defaultValue string) string {
	slog.Debug("Cache lookup (string)", "flagKey", flagKey, "contextKey", ctx.Key())
	cacheKey := fmt.Sprintf("str:%s:%s", flagKey, ctx.Key())

	if f.cacheEnabled {
		if val, found := f.flagCache.Get(cacheKey); found {
			slog.Debug("Cache hit (string)", "flagKey", flagKey, "value", val)
			return val.(string)
		}
	}

	slog.Debug("Cache miss (string)", "flagKey", flagKey, "contextKey", ctx.Key())
	freshValue, err := f.ldClient.StringVariation(flagKey, ctx, defaultValue)
	if err != nil {
		slog.Error("Error evaluating string flag", "flagKey", flagKey, "error", err)
		return defaultValue
	}

	slog.Debug("LaunchDarkly evaluated (string)", "flagKey", flagKey, "value", freshValue)
	f.flagCache.Set(cacheKey, freshValue, cache.DefaultExpiration)
	return freshValue
}

// GetJSONFlagValue for JSON flags – returns raw Go interface{} from ldvalue.Value
func (f *FeatureFlagCache) GetJSONFlagValue(flagKey string, ctx ldcontext.Context, defaultValue interface{}) interface{} {

	cacheKey := fmt.Sprintf("json:%s:%s", flagKey, ctx.Key())

	if f.cacheEnabled {
		if val, found := f.flagCache.Get(cacheKey); found {
			slog.Debug("Cache hit (JSON)", "flagKey", flagKey, "value", val)
			return val
		}
	}

	slog.Debug("Cache miss (JSON)", "flagKey", flagKey, "contextKey", ctx.Key())

	// Convert the Go default value to an ldvalue.Value by marshalling it to JSON
	// and then parsing it. This handles any JSON-serializable type.
	defaultVal, err := f.convertToLDValue(defaultValue)
	if err != nil {
		slog.Error("Failed to convert default value to ldvalue.Value", "error", err)
		return defaultValue
	}

	freshValue, err := f.ldClient.JSONVariation(flagKey, ctx, defaultVal)
	if err != nil {
		slog.Error("Error evaluating JSON flag", "flagKey", flagKey, "error", err)
		return defaultValue
	}

	// Convert ldvalue.Value to raw Go type (slice, map, etc.)
	raw := freshValue.AsRaw()
	slog.Debug("LaunchDarkly evaluated (JSON)", "flagKey", flagKey, "value", raw)
	f.flagCache.Set(cacheKey, raw, cache.DefaultExpiration)
	return raw
}

// Helper method to convert a Go value to an ldvalue.Value by marshalling to JSON.
func (f *FeatureFlagCache) convertToLDValue(v interface{}) (ldvalue.Value, error) {
	// Marshal the Go value to JSON.
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return ldvalue.Null(), err
	}

	var val ldvalue.Value
	if err := val.UnmarshalJSON(jsonBytes); err != nil {
		return ldvalue.Null(), err
	}
	return val, nil
}

// GetJSONFlagNoCache bypasses the cache and evaluates directly from LaunchDarkly.
func (f *FeatureFlagCache) GetJSONFlagNoCache(flagKey string, ctx ldcontext.Context, defaultValue interface{}) interface{} {

	// Convert default value to ldvalue.Value
	defaultVal, err := f.convertToLDValue(defaultValue)
	if err != nil {
		slog.Error("Failed to convert default value to ldvalue.Value", "error", err)
		return defaultValue
	}
	freshValue, err := f.ldClient.JSONVariation(flagKey, ctx, defaultVal)
	if err != nil {
		slog.Error("Error evaluating JSON flag (no cache)", "flagKey", flagKey, "error", err)
		return defaultValue
	}
	return freshValue.AsRaw()
}
