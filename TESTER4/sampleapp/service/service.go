package service

import (
	"errors"
	"fmt"
	"log"

	"yourmodule/config"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/rupam/ldkillswitch/service" // Your library
	"github.com/rupam/ldwebapp/client"      // Your client
)

type WebClientService struct {
	props              config.WebClientProperties
	clients            map[string]*client.WebClient
	featureFlagService service.FeatureFlagService // Injected from library
}

func NewWebClientService(
	props config.WebClientProperties,
	clients map[string]*client.WebClient,
	ffService service.FeatureFlagService,
) *WebClientService {
	return &WebClientService{
		props:              props,
		clients:            clients,
		featureFlagService: ffService,
	}
}

func (s *WebClientService) Get(serviceName string, ldCtx ldcontext.Context, flagKey string, uriVars ...interface{}) (string, error) {
	// 1. Killswitch / Feature Flag check using your library
	if ldCtx.IsDefined() && s.featureFlagService != nil {
		// Calling the library's implementation
		isEnabled := s.featureFlagService.IsFeatureEnabled(flagKey, ldCtx, false)
		if !isEnabled {
			log.Printf("[WebClientService] Feature '%s' is disabled for context. Blocking call to %s", flagKey, serviceName)
			return "", errors.New("feature_disabled_by_killswitch")
		}
	}

	// 2. Resolve Service Path
	svcConfig, exists := s.props.Services[serviceName]
	if !exists || svcConfig.Path == nil {
		return "", fmt.Errorf("path not found for service: %s", serviceName)
	}
	path := fmt.Sprintf(*svcConfig.Path, uriVars...)

	// 3. Execute via WebClient (Retry + Circuit Breaker)
	webClient, ok := s.clients[serviceName]
	if !ok {
		return "", fmt.Errorf("client not initialized for: %s", serviceName)
	}

	data, err := webClient.DoGet(path)
	if err != nil {
		// Error handling matches your Java onErrorResume/Fallback
		return `{"status": "error", "msg": "fallback triggered"}`, err
	}

	return string(data), nil
}
