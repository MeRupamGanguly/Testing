package cache

import (
	"fmt"
	"log"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/patrickmn/go-cache"

	"github.com/rupam/ldkillswitch/config"
)

type FeatureFlagCache struct {
	ldClient         *ldclient.LDClient
	booleanFlagCache *cache.Cache
	cacheEnabled     bool
}

func NewFeatureFlagCache(client *ldclient.LDClient, props config.FeatureFlagCacheProperties) *FeatureFlagCache {
	// go-cache acts as the Guava CacheBuilder equivalent
	c := cache.New(props.ExpireAfterWriteMinute, props.ExpireAfterWriteMinute*2)

	log.Printf("Initialized FeatureFlagCache with %v TTL.", props.ExpireAfterWriteMinute)

	return &FeatureFlagCache{
		ldClient:         client,
		booleanFlagCache: c,
		cacheEnabled:     props.CacheEnabled,
	}
}

func (f *FeatureFlagCache) GetBooleanFlagValue(flagKey string, ctx ldcontext.Context, defaultValue bool) bool {
	cacheKey := fmt.Sprintf("%s:%s", flagKey, ctx.Key())

	if f.cacheEnabled {
		if val, found := f.booleanFlagCache.Get(cacheKey); found {
			return val.(bool)
		}
	}

	log.Printf("Cache miss for flag '%s' and context '%s'. Fetching from LaunchDarkly.", flagKey, ctx.Key())
	freshValue, err := f.ldClient.BoolVariation(flagKey, ctx, defaultValue)
	if err != nil {
		log.Printf("Error evaluating flag: %v", err)
		return defaultValue
	}

	f.booleanFlagCache.Set(cacheKey, freshValue, cache.DefaultExpiration)
	return freshValue
}
