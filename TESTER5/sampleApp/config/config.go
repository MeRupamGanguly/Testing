package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server          ServerConfig   `yaml:"server"`
	DefaultCurrency string         `yaml:"default_currency"`
	Database        DatabaseConfig `yaml:"database"`
}

type ServerConfig struct {
	Port                   int `yaml:"port"`
	ShutdownTimeoutSeconds int `yaml:"shutdown_timeout_seconds"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ShutdownTimeoutSeconds == 0 {
		cfg.Server.ShutdownTimeoutSeconds = 10
	}
	if cfg.DefaultCurrency == "" {
		cfg.DefaultCurrency = "USD"
	}
	return &cfg, nil
}

func (c *Config) ServerAddr() string {
	return ":" + strconv.Itoa(c.Server.Port)
}

func (c *Config) ShutdownTimeout() time.Duration {
	return time.Duration(c.Server.ShutdownTimeoutSeconds) * time.Second
}
