package cache

import (
	"featuresgflags/LDKillSwitch/config"
	"fmt"
	"log"
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/patrickmn/go-cache"
)

type FeatureFlagCache struct {
	ldClient         *ldclient.LDClient
	booleanFlagCache *cache.Cache
	cacheEnabled     bool
}

func NewFeatureFlagCache(client *ldclient.LDClient, props config.FeatureFlagCacheProperties) *FeatureFlagCache {
	// FIX: Explicitly cast to time.Minute to prevent nanosecond expiration
	expirationTime := props.ExpireAfterWriteMinute * time.Minute
	cleanupInterval := expirationTime * 2

	c := cache.New(expirationTime, cleanupInterval)

	log.Printf("Initialized FeatureFlagCache with %v TTL.", expirationTime)

	return &FeatureFlagCache{
		ldClient:         client,
		booleanFlagCache: c,
		cacheEnabled:     props.CacheEnabled,
	}
}

func (f *FeatureFlagCache) GetBooleanFlagValue(flagKey string, ctx ldcontext.Context, defaultValue bool) bool {
	fmt.Println("This IS FROM CORE KILLSWITCH CACHE Line Number 34")
	cacheKey := fmt.Sprintf("%s:%s", flagKey, ctx.Key())

	if f.cacheEnabled {
		if val, found := f.booleanFlagCache.Get(cacheKey); found {
			return val.(bool)
		}
	}

	log.Printf("Cache miss for flag '%s' and context '%s'. Fetching from LaunchDarkly.", flagKey, ctx.Key())
	freshValue, err := f.ldClient.BoolVariation(flagKey, ctx, defaultValue)

	log.Printf("[DEBUG] LaunchDarkly returned %v for flag %s (Error: %v)", freshValue, flagKey, err)
	if err != nil {
		log.Printf("Error evaluating flag: %v", err)
		return defaultValue
	}

	f.booleanFlagCache.Set(cacheKey, freshValue, cache.DefaultExpiration)
	return freshValue
}
