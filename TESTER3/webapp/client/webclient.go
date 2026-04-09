package webclient

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/rupam/ldwebapp/config"

	"github.com/sony/gobreaker"
)

// ResilientClient wraps standard http.Client with Circuit Breaker and Retry logic
type ResilientClient struct {
	HTTPClient     *http.Client
	CircuitBreaker *gobreaker.CircuitBreaker
	RetryConfig    config.RetryConfig
	BaseURL        string
	Headers        map[string]string
}

func NewResilientClient(name string, props config.WebClientProperties, service config.ServiceConfig) *ResilientClient {
	// Fallbacks
	baseURL := props.BaseURL
	if service.BaseURL != "" {
		baseURL = service.BaseURL
	}

	timeoutMs := props.ReadTimeout
	if service.ReadTimeout != 0 {
		timeoutMs = service.ReadTimeout
	}

	client := &http.Client{
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
	}

	rc := &ResilientClient{
		HTTPClient:  client,
		BaseURL:     baseURL,
		Headers:     mergeHeaders(props.DefaultHeaders, service.Headers),
		RetryConfig: props.Retry,
	}
	if service.Retry != nil {
		rc.RetryConfig = *service.Retry
	}

	cbProps := props.CircuitBreaker
	if service.CircuitBreaker != nil {
		cbProps = *service.CircuitBreaker
	}

	if cbProps.Enabled {
		st := gobreaker.Settings{
			Name:        name,
			MaxRequests: cbProps.MinimumNumberOfCalls,
			Interval:    time.Duration(cbProps.TimeoutOpenStateMs) * time.Millisecond,
			Timeout:     time.Duration(cbProps.TimeoutOpenStateMs) * time.Millisecond,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= cbProps.MinimumNumberOfCalls && failureRatio >= (cbProps.FailureRateThreshold/100.0)
			},
		}
		rc.CircuitBreaker = gobreaker.NewCircuitBreaker(st)
	}

	return rc
}

func mergeHeaders(defaultH, serviceH map[string]string) map[string]string {
	res := make(map[string]string)
	for k, v := range defaultH {
		res[k] = v
	}
	for k, v := range serviceH {
		res[k] = v
	}
	return res
}

func (rc *ResilientClient) DoGet(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", rc.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range rc.Headers {
		req.Header.Set(k, v)
	}

	operation := func() (interface{}, error) {
		return rc.executeWithRetry(req)
	}

	var respBytes interface{}
	if rc.CircuitBreaker != nil {
		respBytes, err = rc.CircuitBreaker.Execute(operation)
	} else {
		respBytes, err = operation()
	}

	if err != nil {
		return nil, err
	}
	return respBytes.([]byte), nil
}

func (rc *ResilientClient) executeWithRetry(req *http.Request) ([]byte, error) {
	attempts := rc.RetryConfig.MaxAttempts
	if !rc.RetryConfig.Enabled || attempts < 1 {
		attempts = 1
	}

	delay := time.Duration(rc.RetryConfig.Backoff.Delay) * time.Millisecond

	var lastErr error
	for i := 0; i < attempts; i++ {
		log.Printf("Request: %s %s", req.Method, req.URL.String())
		resp, err := rc.HTTPClient.Do(req)

		if err == nil {
			defer resp.Body.Close()
			log.Printf("Response Status: %d", resp.StatusCode)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return io.ReadAll(resp.Body)
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		time.Sleep(delay)
		delay = time.Duration(float64(delay) * rc.RetryConfig.Backoff.Multiplier)
	}
	return nil, lastErr
}
