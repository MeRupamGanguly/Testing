package service

import (
	"featuresgflags/LDKillSwitch/cache"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

type FeatureFlagService struct {
	cache *cache.FeatureFlagCache
}

func NewFeatureFlagService(cache *cache.FeatureFlagCache) *FeatureFlagService {
	return &FeatureFlagService{cache: cache}
}

func (s *FeatureFlagService) IsFeatureEnabled(flagKey string, ctx ldcontext.Context, defaultValue bool) bool {

	return s.cache.GetBooleanFlagValue(flagKey, ctx, defaultValue)
}

func (s *FeatureFlagService) GetStringFlag(flagKey string, ctx ldcontext.Context, defaultValue string) string {

	return s.cache.GetStringFlagValue(flagKey, ctx, defaultValue)
}

// GetJSONFlag returns a JSON value (slice, map, etc.) from a JSON flag
func (s *FeatureFlagService) GetJSONFlag(flagKey string, ctx ldcontext.Context, defaultValue interface{}) interface{} {

	return s.cache.GetJSONFlagValue(flagKey, ctx, defaultValue)
}

func (s *FeatureFlagService) GetJSONFlagNoCache(flagKey string, ctx ldcontext.Context, defaultValue interface{}) interface{} {

	return s.cache.GetJSONFlagNoCache(flagKey, ctx, defaultValue)
}
