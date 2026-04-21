package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	WebClient WebClientProperties `yaml:"webclient"`
}

type WebClientProperties struct {
	BaseURL          string                   `yaml:"baseUrl"`
	ConnectTimeoutMs int                      `yaml:"connectTimeoutMs"`
	ReadTimeoutMs    int                      `yaml:"readTimeoutMs"`
	Retry            RetryConfig              `yaml:"retry"`
	CircuitBreaker   CircuitBreakerConfig     `yaml:"circuitBreaker"`
	Services         map[string]ServiceConfig `yaml:"services"`
	LDKillSwitch     LDKillSwitchConfig       `yaml:"ldkillswitch"`
	RedisConfig      RedisConfig              `yaml:"redis"`
	FlagPath         string                   `yaml:"flagsPath"`
}

type RetryConfig struct {
	Enabled     bool          `yaml:"enabled"`
	MaxAttempts int           `yaml:"maxAttempts"`
	Backoff     BackoffConfig `yaml:"backoff"`
}

type BackoffConfig struct {
	DelayMs    int     `yaml:"delayMs"`
	MaxDelayMs int     `yaml:"maxDelayMs"`
	Multiplier float64 `yaml:"multiplier"`
}

type CircuitBreakerConfig struct {
	Enabled                   bool   `yaml:"enabled"`
	FailureRateThreshold      int    `yaml:"failureRateThreshold"`
	MinimumNumberOfCalls      uint32 `yaml:"minimumNumberOfCalls"`
	WaitDurationInOpenStateMs int    `yaml:"waitDurationInOpenStateMs"`
	HalfOpenMaxRequests       uint32 `yaml:"halfOpenMaxRequests"` // NEW: controls requests in half-open state
}

type ServiceConfig struct {
	Path           *string               `yaml:"path"`
	Retry          *RetryConfig          `yaml:"retry"`
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuitBreaker"`
	ReadTimeoutMs  *int                  `yaml:"readTimeoutMs"`
	BaseURL        *string               `yaml:"baseUrl"`
}

type LDKillSwitchConfig struct {
	SdkKey                 string `yaml:"sdkKey"`
	Offline                bool   `yaml:"offline"`
	CacheEnabled           bool   `yaml:"cacheEnabled"`
	ExpireAfterWriteMinute int    `yaml:"expireAfterWriteMinute"`
	// MaximumSize removed
}

type RedisConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Url             string `yaml:"url"`
	LocalTtlSeconds int    `yaml:"local-ttl-seconds"`
}

func LoadConfig(filename string) (*AppConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	// Set default for HalfOpenMaxRequests if missing
	for _, svc := range config.WebClient.Services {
		if svc.CircuitBreaker != nil && svc.CircuitBreaker.HalfOpenMaxRequests == 0 {
			svc.CircuitBreaker.HalfOpenMaxRequests = 3
		}
	}
	if config.WebClient.CircuitBreaker.HalfOpenMaxRequests == 0 {
		config.WebClient.CircuitBreaker.HalfOpenMaxRequests = 3
	}
	return &config, nil
}
