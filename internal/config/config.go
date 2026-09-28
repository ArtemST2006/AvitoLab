// Package config загружает настройки сервиса из .env и переменных окружения.
package config

import (
	"log"
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

// MustLoad читает YAML-файл CONFIG_PATH, если он задан, и затем переменные окружения.
// Переменные окружения перекрывают значения из файла.
func MustLoad() *Config {
	var cfg Config

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Print("CONFIG_PATH is not set, loading config from environment only")

		if err := cleanenv.ReadEnv(&cfg); err != nil {
			log.Fatalf("failed to load config from environment: %v", err)
		}

		return &cfg
	}

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("failed to load config from %q and environment: %v", configPath, err)
	}

	return &cfg
}
