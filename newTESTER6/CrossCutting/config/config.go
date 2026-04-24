package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	JWT struct {
		Secret string `yaml:"secret"`
	} `yaml:"jwt"`
	Redis struct {
		Addr string `yaml:"addr"`
	} `yaml:"redis"`
	RateLimit struct {
		Limit  int    `yaml:"limit"`
		Window string `yaml:"window"`
	} `yaml:"rate_limit"`
	Limits struct {
		MaxHeaderBytes  int   `yaml:"max_header_bytes"`
		MaxRequestBody  int64 `yaml:"max_request_body"`
		MaxResponseBody int64 `yaml:"max_response_body"`
	} `yaml:"limits"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if s := os.Getenv("JWT_SECRET"); s != "" {
		cfg.JWT.Secret = s
	}
	if s := os.Getenv("REDIS_ADDR"); s != "" {
		cfg.Redis.Addr = s
	}
	return &cfg, nil
}
