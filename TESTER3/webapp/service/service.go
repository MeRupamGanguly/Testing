package service

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	ldService "github.com/rupam/ldkillswitch/service"
	webclient "github.com/rupam/ldwebapp/client"
	"github.com/rupam/ldwebapp/config"
)

type Service struct {
	properties config.WebClientProperties
	clients    map[string]*webclient.ResilientClient
	ffService  *ldService.FeatureFlagService
}

func NewWebClientService(props config.WebClientProperties, ffService *ldService.FeatureFlagService) *Service {
	clients := make(map[string]*webclient.ResilientClient)
	for name, srvProps := range props.Services {
		clients[name] = webclient.NewResilientClient(name, props, srvProps)
	}
	return &Service{
		properties: props,
		clients:    clients,
		ffService:  ffService,
	}
}

// Get executes a GET request using Generics to deserialize the response, mirroring the Java Class<T> parameter
func Get[T any](s *Service, serviceName string, ctx *ldcontext.Context, flagKey string, fallback func(error) (T, error)) (T, error) {
	var result T

	if ctx != nil {
		isValid := s.ffService.IsFeatureEnabled(flagKey, *ctx, false)
		if !isValid {
			log.Printf("Request context is invalid. Skipping service call to '%s'.", serviceName)
			return fallback(fmt.Errorf("invalid request context: 400 Bad Request"))
		}
	}

	client, ok := s.clients[serviceName]
	if !ok {
		return result, fmt.Errorf("no WebClient configured for service: %s", serviceName)
	}

	path := s.properties.Services[serviceName].Path
	respBytes, err := client.DoGet(path)
	if err != nil {
		log.Printf("GET request to service failed: %v", err)
		return fallback(err)
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return fallback(err)
	}

	return result, nil
}
