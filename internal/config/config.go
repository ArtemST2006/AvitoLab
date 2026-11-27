// Package config загружает настройки сервиса из .env и переменных окружения.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config — настройки Trip Service.
type Config struct {
	Env      string `yaml:"env" env:"ENV" env-default:"local"`
	LogLevel string `yaml:"log_level" env:"LOG_LEVEL" env-required:"true"`

	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT" env-required:"true"`

	HTTPServer HTTPServer `yaml:"http-server"`
	Database   Database   `yaml:"database"`
}

// HTTPServer — настройки HTTP-сервера.
type HTTPServer struct {
	Address string `yaml:"address" env:"HTTP_ADDR" env-required:"true"`

	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout" env:"HTTP_READ_HEADER_TIMEOUT" env-default:"5s"`
	ReadTimeout       time.Duration `yaml:"read_timeout" env:"HTTP_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout      time.Duration `yaml:"write_timeout" env:"HTTP_WRITE_TIMEOUT" env-default:"10s"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" env:"HTTP_IDLE_TIMEOUT" env-default:"60s"`
}

// Database — настройки подключения к PostgreSQL.
type Database struct {
	URL             string        `yaml:"url" env:"DATABASE_URL" env-required:"true"`
	MaxConns        int32         `yaml:"max_conns" env:"DATABASE_MAX_CONNS" env-required:"true"`
	MinConns        int32         `yaml:"min_conns" env:"DATABASE_MIN_CONNS" env-required:"true"`
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime" env:"DATABASE_MAX_CONN_LIFETIME" env-required:"true"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout" env:"DATABASE_CONNECT_TIMEOUT" env-required:"true"`
	QueryTimeout    time.Duration `yaml:"query_timeout" env:"DATABASE_QUERY_TIMEOUT" env-required:"true"`
}

// Load читает YAML-файл CONFIG_PATH, если он задан, и затем переменные окружения.
// Переменные окружения перекрывают значения из файла.
func Load() (*Config, error) {
	var cfg Config

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("load config from environment: %w", err)
		}
		return validate(&cfg)
	}

	// #nosec G703 -- CONFIG_PATH is an operator-provided path to the service configuration.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if readErr := cleanenv.ReadEnv(&cfg); readErr != nil {
			return nil, fmt.Errorf("load config from environment: %w", readErr)
		}
		return validate(&cfg)
	} else if err != nil {
		return nil, fmt.Errorf("inspect config %q: %w", configPath, err)
	}
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("load config from %q and environment: %w", configPath, err)
	}

	return validate(&cfg)
}

func validate(cfg *Config) (*Config, error) {
	for _, value := range []struct {
		name     string
		duration time.Duration
	}{
		{"SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout},
		{"HTTP_READ_HEADER_TIMEOUT", cfg.HTTPServer.ReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", cfg.HTTPServer.ReadTimeout},
		{"HTTP_WRITE_TIMEOUT", cfg.HTTPServer.WriteTimeout},
		{"HTTP_IDLE_TIMEOUT", cfg.HTTPServer.IdleTimeout},
		{"DATABASE_MAX_CONN_LIFETIME", cfg.Database.MaxConnLifetime},
		{"DATABASE_CONNECT_TIMEOUT", cfg.Database.ConnectTimeout},
		{"DATABASE_QUERY_TIMEOUT", cfg.Database.QueryTimeout},
	} {
		if value.duration <= 0 {
			return nil, fmt.Errorf("%s must be positive", value.name)
		}
	}
	if cfg.Database.MaxConns <= 0 {
		return nil, fmt.Errorf("DATABASE_MAX_CONNS must be positive")
	}
	if cfg.Database.MinConns < 0 || cfg.Database.MinConns > cfg.Database.MaxConns {
		return nil, fmt.Errorf("DATABASE_MIN_CONNS must be between 0 and DATABASE_MAX_CONNS")
	}
	return cfg, nil
}
