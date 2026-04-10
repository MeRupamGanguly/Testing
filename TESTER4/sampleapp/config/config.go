package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WebClient WebClientProperties `yaml:"webclient"`
}

type WebClientProperties struct {
	BaseURL          string                   `yaml:"baseUrl"`
	ConnectTimeoutMs int                      `yaml:"connectTimeoutMs"`
	ReadTimeoutMs    int                      `yaml:"readTimeoutMs"`
	Retry            RetryConfig              `yaml:"retry"`
	CircuitBreaker   CircuitBreakerConfig     `yaml:"circuitBreaker"`
	Services         map[string]ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	BaseURL          *string               `yaml:"baseUrl"`
	Path             *string               `yaml:"path"`
	ConnectTimeoutMs *int                  `yaml:"connectTimeoutMs"`
	ReadTimeoutMs    *int                  `yaml:"readTimeoutMs"`
	Retry            *RetryConfig          `yaml:"retry"`
	CircuitBreaker   *CircuitBreakerConfig `yaml:"circuitBreaker"`
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
	Enabled                   bool    `yaml:"enabled"`
	FailureRateThreshold      float64 `yaml:"failureRateThreshold"`
	MinimumNumberOfCalls      uint32  `yaml:"minimumNumberOfCalls"`
	WaitDurationInOpenStateMs int     `yaml:"waitDurationInOpenStateMs"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
