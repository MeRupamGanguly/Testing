package service

import (
	"errors"

	"featuresgflags/LDKillSwitch/service"
	"featuresgflags/SampleApp/client"
	"featuresgflags/SampleApp/config"
	"fmt"
	"log"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

type WebClientService struct {
	props              config.WebClientProperties
	clients            map[string]*client.WebClient
	featureFlagService *service.FeatureFlagService // Injected from library
}

func NewWebClientService(
	props config.WebClientProperties,
	clients map[string]*client.WebClient,
	ffService *service.FeatureFlagService,
) *WebClientService {
	return &WebClientService{
		props:              props,
		clients:            clients,
		featureFlagService: ffService,
	}
}

func (s *WebClientService) Get(serviceName string, ldCtx ldcontext.Context, flagKey string, uriVars ...interface{}) (string, error) {
	if ldCtx.IsDefined() {
		// Calling the library's implementation
		isEnabled := s.featureFlagService.IsFeatureEnabled(flagKey, ldCtx, false)
		if !isEnabled {
			log.Printf("[WebClientService] Feature '%s' is disabled for context. Blocking call to %s", flagKey, serviceName)
			return "", errors.New("feature_disabled_by_killswitch")
		}
	}
	fmt.Println("This IS FROM SERVICE Line Number 42")
	svcConfig, exists := s.props.Services[serviceName]
	if !exists || svcConfig.Path == nil {
		return "", fmt.Errorf("path not found for service: %s", serviceName)
	}
	path := fmt.Sprintf(*svcConfig.Path, uriVars...)

	webClient, ok := s.clients[serviceName]
	if !ok {
		return "", fmt.Errorf("client not initialized for: %s", serviceName)
	}

	data, err := webClient.DoGet(path)
	fmt.Println("This IS FROM SERVICE Line Number 55")
	if err != nil {
		return `{"status": "error", "msg": "fallback triggered"}`, err
	}

	return string(data), nil
}
