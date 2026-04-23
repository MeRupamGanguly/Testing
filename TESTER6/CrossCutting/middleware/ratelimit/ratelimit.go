// File: pkg/middleware/ratelimit.go
package ratelimit

import (
	"crosscutting/utils"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// RateLimitConfig configures the rate limiter.
type RateLimitConfig struct {
	// For in-memory token bucket
	Rate  rate.Limit // e.g., rate.Limit(10)
	Burst int

	// For Redis leaky bucket
	RedisClient *redis.Client
	KeyPrefix   string
	Limit       int           // requests per window
	Window      time.Duration // time window
}

// TokenBucketRateLimit returns an in-memory token bucket rate limiter per IP.
// Use this for single-instance deployments.
func TokenBucketRateLimit(r rate.Limit, burst int) gin.HandlerFunc {
	limiters := make(map[string]*rate.Limiter)
	mu := &sync.RWMutex{}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.RLock()
		limiter, exists := limiters[ip]
		mu.RUnlock()
		if !exists {
			limiter = rate.NewLimiter(r, burst)
			mu.Lock()
			limiters[ip] = limiter
			mu.Unlock()
		}

		if !limiter.Allow() {
			c.Header("X-RateLimit-Limit", strconv.Itoa(burst))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", "1")
			utils.Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "rate limit exceeded")
			return
		}

		c.Next()
	}
}

// LeakyBucketRateLimit implements a distributed leaky bucket using Redis.
// Each request consumes a token from a bucket that refills at a constant rate.
// This ensures a smooth request rate even with bursts.
func LeakyBucketRateLimit(config RateLimitConfig) gin.HandlerFunc {
	if config.RedisClient == nil {
		panic("redis client required for leaky bucket rate limiter")
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = "rate_limit:"
	}
	// Refill rate per second
	refillRate := float64(config.Limit) / config.Window.Seconds()

	return func(c *gin.Context) {
		key := config.KeyPrefix + c.ClientIP()
		now := time.Now().UnixNano()

		// Lua script for leaky bucket algorithm
		script := redis.NewScript(`
			local key = KEYS[1]
			local now = tonumber(ARGV[1])
			local refill_rate = tonumber(ARGV[2])
			local capacity = tonumber(ARGV[3])

			local bucket = redis.call('HMGET', key, 'tokens', 'last_update')
			local tokens = tonumber(bucket[1]) or capacity
			local last_update = tonumber(bucket[2]) or now

			local elapsed = (now - last_update) / 1e9 -- seconds
			local new_tokens = math.min(capacity, tokens + elapsed * refill_rate)

			if new_tokens >= 1 then
				redis.call('HMSET', key, 'tokens', new_tokens - 1, 'last_update', now)
				redis.call('EXPIRE', key, 60)
				return 1
			else
				return 0
			end
		`)

		allowed, err := script.Run(c.Request.Context(), config.RedisClient, []string{key}, now, refillRate, config.Limit).Int()
		if err != nil {
			// Fail open or closed? We'll fail open with logging.
			c.Next()
			return
		}

		if allowed == 0 {
			c.Header("X-RateLimit-Limit", strconv.Itoa(config.Limit))
			c.Header("Retry-After", "1")
			utils.Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "rate limit exceeded")
			return
		}
		c.Next()
	}
}
