package cache

import (
	"testing"
	"time"

	"featuresgflags/LDKillSwitch/config"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/testhelpers/ldtestdata"
)

// setupMockClient initializes a mock LaunchDarkly client with test data
func setupMockClient(t *testing.T) (*ldclient.LDClient, *ldtestdata.TestDataSource) {
	td := ldtestdata.DataSource()
	config := ldclient.Config{
		DataSource: td,
	}
	client, err := ldclient.MakeCustomClient("dummy-sdk-key", config, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to create mock client: %v", err)
	}
	return client, td
}

// 1. Test Cache Miss
func TestGetBooleanFlagValue_CacheMiss(t *testing.T) {
	client, td := setupMockClient(t)
	defer client.Close()

	flagKey := "test-flag"
	td.Update(td.Flag(flagKey).VariationForAll(true))

	ctx := ldcontext.New("user-123")
	props := config.FeatureFlagCacheProperties{
		CacheEnabled:           true,
		ExpireAfterWriteMinute: 1,
	}
	ffCache := NewFeatureFlagCache(client, props)
	val := ffCache.GetBooleanFlagValue(flagKey, ctx, false)
	if val != true {
		t.Errorf("Expected true on cache miss, got %v", val)
	}
}

// 2. Test Cache Hit
func TestGetBooleanFlagValue_CacheHit(t *testing.T) {
	client, td := setupMockClient(t)
	defer client.Close()

	flagKey := "test-flag"
	td.Update(td.Flag(flagKey).VariationForAll(true))

	ctx := ldcontext.New("user-123")
	props := config.FeatureFlagCacheProperties{
		CacheEnabled:           true,
		ExpireAfterWriteMinute: 1,
	}

	ffCache := NewFeatureFlagCache(client, props)
	ffCache.GetBooleanFlagValue(flagKey, ctx, false)
	td.Update(td.Flag(flagKey).VariationForAll(false))
	val := ffCache.GetBooleanFlagValue(flagKey, ctx, false)
	if val != true {
		t.Errorf("Expected true from cache hit despite LD change, got %v", val)
	}
}

// 3. Test Cache Disabled
func TestGetBooleanFlagValue_CacheDisabled(t *testing.T) {
	client, td := setupMockClient(t)
	defer client.Close()

	flagKey := "test-flag"
	td.Update(td.Flag(flagKey).VariationForAll(false))

	ctx := ldcontext.New("user-123")
	props := config.FeatureFlagCacheProperties{
		CacheEnabled:           false, // CACHE DISABLED
		ExpireAfterWriteMinute: 1,
	}

	ffCache := NewFeatureFlagCache(client, props)
	val := ffCache.GetBooleanFlagValue(flagKey, ctx, true)
	if val != false {
		t.Errorf("Expected false directly from LD because cache is disabled, got %v", val)
	}
}

// 4. Test Fallback to Default
func TestGetBooleanFlagValue_Fallback(t *testing.T) {
	client, _ := setupMockClient(t)
	defer client.Close()

	ctx := ldcontext.New("user-123")
	props := config.FeatureFlagCacheProperties{
		CacheEnabled:           true,
		ExpireAfterWriteMinute: 1,
	}

	ffCache := NewFeatureFlagCache(client, props)
	val := ffCache.GetBooleanFlagValue("unknown-flag", ctx, true)
	if val != true {
		t.Errorf("Expected default value (true) for unknown flag, got %v", val)
	}
}
