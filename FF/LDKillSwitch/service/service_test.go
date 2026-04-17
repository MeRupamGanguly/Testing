package service

import (
	"testing"
	"time"

	"featuresgflags/LDKillSwitch/cache"
	"featuresgflags/LDKillSwitch/config"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/testhelpers/ldtestdata"
)

func TestFeatureFlagService_IsFeatureEnabled(t *testing.T) {
	td := ldtestdata.DataSource()
	ldConfig := ldclient.Config{DataSource: td}
	client, _ := ldclient.MakeCustomClient("dummy-sdk-key", ldConfig, 5*time.Second)
	defer client.Close()

	td.Update(td.Flag("service-test-flag").VariationForAll(true))
	props := config.FeatureFlagCacheProperties{
		CacheEnabled:           true,
		ExpireAfterWriteMinute: 1,
	}
	ffCache := cache.NewFeatureFlagCache(client, props)
	svc := NewFeatureFlagService(ffCache)

	ctx := ldcontext.New("test-user")

	// Test A: Feature is enabled
	if !svc.IsFeatureEnabled("service-test-flag", ctx, false) {
		t.Errorf("Expected feature 'service-test-flag' to be enabled")
	}

	// Test B: Feature is missing, fallback default kicks in
	if !svc.IsFeatureEnabled("missing-flag", ctx, true) {
		t.Errorf("Expected fallback default value to be true for missing flag")
	}

	if svc.IsFeatureEnabled("missing-flag-false", ctx, false) {
		t.Errorf("Expected fallback default value to be false for missing flag")
	}
}
