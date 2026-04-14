package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"featuresgflags/SampleApp/config"
)

func TestWebClient_DoGet_Success(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test-path" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	baseURL := server.URL
	timeout := 1000

	props := config.WebClientProperties{
		BaseURL:       baseURL,
		ReadTimeoutMs: timeout,
		Services: map[string]config.ServiceConfig{
			"testService": {},
		},
	}

	clients := NewWebClientManager(props)
	wc := clients["testService"]

	result, err := wc.DoGet("/test-path")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if string(result) != `{"status": "success"}` {
		t.Errorf("Unexpected response: %s", string(result))
	}
}

func TestWebClient_DoGet_RetryExceeded(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	baseURL := server.URL
	timeout := 1000

	props := config.WebClientProperties{
		BaseURL:       baseURL,
		ReadTimeoutMs: timeout,
		Retry: config.RetryConfig{
			Enabled:     true,
			MaxAttempts: 3,
			Backoff: config.BackoffConfig{
				DelayMs:    1,
				MaxDelayMs: 5,
				Multiplier: 2.0,
			},
		},
		Services: map[string]config.ServiceConfig{
			"testService": {},
		},
	}

	clients := NewWebClientManager(props)
	wc := clients["testService"]

	_, err := wc.DoGet("/fail-path")

	if err == nil {
		t.Fatal("Expected an error due to retries failing, got nil")
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("Expected max retries error, got: %v", err)
	}
	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}
