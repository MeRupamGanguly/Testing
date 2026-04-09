package config

type WebClientProperties struct {
	BaseURL        string
	DefaultHeaders map[string]string
	ConnectTimeout int
	ReadTimeout    int
	Retry          RetryConfig
	CircuitBreaker CircuitBreakerConfig
	Services       map[string]ServiceConfig
}

type ServiceConfig struct {
	BaseURL        string
	Path           string
	ConnectTimeout int
	ReadTimeout    int
	Headers        map[string]string
	Retry          *RetryConfig
	CircuitBreaker *CircuitBreakerConfig
}

type RetryConfig struct {
	Enabled     bool
	MaxAttempts int
	Backoff     BackoffConfig
}

type BackoffConfig struct {
	Delay      int
	MaxDelay   int
	Multiplier float64
}

type CircuitBreakerConfig struct {
	Enabled              bool
	FailureRateThreshold float64
	MinimumNumberOfCalls uint32
	TimeoutOpenStateMs   int
}
