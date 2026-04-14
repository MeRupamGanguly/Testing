package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ldCache "featuresgflags/LDKillSwitch/cache"
	ldConfig "featuresgflags/LDKillSwitch/config"
	ldService "featuresgflags/LDKillSwitch/service"
	"featuresgflags/SampleApp/client"
	"featuresgflags/SampleApp/config"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	ldclient "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/testhelpers/ldtestdata"
)

// Helper to initialize common dependencies
func setupTestEnv(t *testing.T) (*WebClientService, *ldtestdata.TestDataSource, *httptest.Server) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": "mocked"}`))
	}))

	td := ldtestdata.DataSource()
	ldCfg := ldclient.Config{DataSource: td}
	ldc, _ := ldclient.MakeCustomClient("dummy", ldCfg, 5*time.Second)

	cacheProps := ldConfig.FeatureFlagCacheProperties{CacheEnabled: false}
	ffCache := ldCache.NewFeatureFlagCache(ldc, cacheProps)
	ffService := ldService.NewFeatureFlagService(ffCache)

	baseURL := server.URL
	timeout := 1000
	pathTpl := "/api/locations/%s"

	appProps := config.WebClientProperties{
		BaseURL:       baseURL,
		ReadTimeoutMs: timeout,
		Services: map[string]config.ServiceConfig{
			"locationService": {
				Path: &pathTpl,
			},
		},
	}
	clients := client.NewWebClientManager(appProps)
	svc := NewWebClientService(appProps, clients, ffService)

	return svc, td, server
}

func TestWebClientService_Get_Success(t *testing.T) {
	svc, td, server := setupTestEnv(t)
	defer server.Close()

	ctx := ldcontext.New("test-user")
	flagKey := "location-feature-flag"

	td.Update(td.Flag(flagKey).VariationForAll(true))

	resp, err := svc.Get("locationService", ctx, flagKey, "123")
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if resp != `{"data": "mocked"}` {
		t.Errorf("Unexpected response: %s", resp)
	}
}

func TestWebClientService_Get_FeatureDisabled(t *testing.T) {
	svc, td, server := setupTestEnv(t)
	defer server.Close()

	ctx := ldcontext.New("test-user")
	flagKey := "location-feature-flag"

	td.Update(td.Flag(flagKey).VariationForAll(false))

	_, err := svc.Get("locationService", ctx, flagKey, "123")
	if err == nil {
		t.Fatal("Expected error because feature is disabled, got success")
	}
	if err.Error() != "feature_disabled_by_killswitch" {
		t.Errorf("Expected killswitch error, got: %v", err)
	}
}

func TestWebClientService_Get_ServiceNotFound(t *testing.T) {
	svc, td, server := setupTestEnv(t)
	defer server.Close()

	ctx := ldcontext.New("test-user")
	flagKey := "location-feature-flag"

	td.Update(td.Flag(flagKey).VariationForAll(true))

	_, err := svc.Get("unknownService", ctx, flagKey, "123")
	if err == nil {
		t.Fatal("Expected error for unknown service, got nil")
	}

	expectedErr := "path not found for service: unknownService"
	if err.Error() != expectedErr {
		t.Errorf("Expected '%s', got: %v", expectedErr, err)
	}
}
