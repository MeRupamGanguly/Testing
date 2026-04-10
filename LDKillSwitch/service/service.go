package service

import (
	"featuresgflags/LDKillSwitch/cache"
	"fmt"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

type FeatureFlagService struct {
	cache *cache.FeatureFlagCache
}

func NewFeatureFlagService(cache *cache.FeatureFlagCache) *FeatureFlagService {
	return &FeatureFlagService{cache: cache}
}

func (s *FeatureFlagService) IsFeatureEnabled(flagKey string, ctx ldcontext.Context, defaultValue bool) bool {
	fmt.Println("This IS FROM CORE KILLSWITCH SERVICE Line Number 19")
	return s.cache.GetBooleanFlagValue(flagKey, ctx, defaultValue)
}
