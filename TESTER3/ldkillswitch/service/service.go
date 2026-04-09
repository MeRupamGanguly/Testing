package service

import (
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/rupam/ldkillswitch/cache"
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
