package client

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"yourmodule/config"

	"github.com/sony/gobreaker"
)

type WebClient struct {
	httpClient *http.Client
	cb         *gobreaker.CircuitBreaker
	retryCfg   config.RetryConfig
	baseURL    string
}

func NewWebClientManager(props config.WebClientProperties) map[string]*WebClient {
	clients := make(map[string]*WebClient)

	for name, svcProps := range props.Services {
		// Resolve defaults vs overrides
		retryCfg := props.Retry
		if svcProps.Retry != nil {
			retryCfg = *svcProps.Retry
		}

		cbCfg := props.CircuitBreaker
		if svcProps.CircuitBreaker != nil {
			cbCfg = *svcProps.CircuitBreaker
		}

		timeoutMs := props.ReadTimeoutMs
		if svcProps.ReadTimeoutMs != nil {
			timeoutMs = *svcProps.ReadTimeoutMs
		}

		baseURL := props.BaseURL
		if svcProps.BaseURL != nil {
			baseURL = *svcProps.BaseURL
		}

		httpClient := &http.Client{
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
		}

		var cb *gobreaker.CircuitBreaker
		if cbCfg.Enabled {
			st := gobreaker.Settings{
				Name:        name,
				MaxRequests: cbCfg.MinimumNumberOfCalls,
				Timeout:     time.Duration(cbCfg.WaitDurationInOpenStateMs) * time.Millisecond,
				ReadyToTrip: func(counts gobreaker.Counts) bool {
					failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
					return counts.Requests >= cbCfg.MinimumNumberOfCalls && failureRatio >= (cbCfg.FailureRateThreshold/100.0)
				},
			}
			cb = gobreaker.NewCircuitBreaker(st)
		}

		clients[name] = &WebClient{
			httpClient: httpClient,
			cb:         cb,
			retryCfg:   retryCfg,
			baseURL:    baseURL,
		}
	}

	return clients
}

func (wc *WebClient) DoGet(path string) ([]byte, error) {
	reqFunc := func() (interface{}, error) {
		req, err := http.NewRequest(http.MethodGet, wc.baseURL+path, nil)
		if err != nil {
			return nil, err
		}

		resp, err := wc.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		return io.ReadAll(resp.Body)
	}

	// 1. Wrap with Retry
	retriableFunc := func() (interface{}, error) {
		if !wc.retryCfg.Enabled {
			return reqFunc()
		}

		var lastErr error
		delay := float64(wc.retryCfg.Backoff.DelayMs)

		for attempt := 1; attempt <= wc.retryCfg.MaxAttempts; attempt++ {
			res, err := reqFunc()
			if err == nil {
				return res, nil
			}
			lastErr = err

			if attempt < wc.retryCfg.MaxAttempts {
				time.Sleep(time.Duration(delay) * time.Millisecond)
				delay = math.Min(delay*wc.retryCfg.Backoff.Multiplier, float64(wc.retryCfg.Backoff.MaxDelayMs))
			}
		}
		return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
	}

	// 2. Wrap with Circuit Breaker
	var result interface{}
	var err error

	if wc.cb != nil {
		result, err = wc.cb.Execute(retriableFunc)
	} else {
		result, err = retriableFunc()
	}

	if err != nil {
		return nil, err
	}

	return result.([]byte), nil
}
