package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	shutdownTimeoutEnv = "SHUTDOWN_TIMEOUT"
	readTimeoutEnv     = "HTTP_READ_TIMEOUT"
	queryTimeoutEnv    = "DATABASE_QUERY_TIMEOUT"
	maxConnsEnv        = "DATABASE_MAX_CONNS"
	minConnsEnv        = "DATABASE_MIN_CONNS"
	defaultTimeout     = "10s"
)

func TestLoadRejectsInvalidSettings(t *testing.T) {
	for _, setting := range []struct {
		name  string
		value string
		want  string
	}{
		{shutdownTimeoutEnv, "0s", shutdownTimeoutEnv},
		{readTimeoutEnv, "0s", readTimeoutEnv},
		{queryTimeoutEnv, "-1s", queryTimeoutEnv},
		{maxConnsEnv, "0", maxConnsEnv},
		{minConnsEnv, "11", minConnsEnv},
	} {
		t.Run(setting.name, func(t *testing.T) {
			setValidEnvironment(t)
			t.Setenv(setting.name, setting.value)
			_, err := Load()
			require.ErrorContains(t, err, setting.want)
		})
	}
}

func TestLoadValidSettings(t *testing.T) {
	setValidEnvironment(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.HTTPServer.Address)
	require.EqualValues(t, 10, cfg.Database.MaxConns)
}

func setValidEnvironment(t *testing.T) {
	t.Helper()
	for name, value := range map[string]string{
		"CONFIG_PATH":                "",
		"HTTP_ADDR":                  ":8080",
		"LOG_LEVEL":                  "info",
		shutdownTimeoutEnv:           defaultTimeout,
		"HTTP_READ_HEADER_TIMEOUT":   "5s",
		"HTTP_READ_TIMEOUT":          "10s",
		"HTTP_WRITE_TIMEOUT":         "10s",
		"HTTP_IDLE_TIMEOUT":          "60s",
		"DATABASE_URL":               "postgres://localhost:5432/example",
		"DATABASE_MAX_CONNS":         "10",
		"DATABASE_MIN_CONNS":         "2",
		"DATABASE_MAX_CONN_LIFETIME": "30m",
		"DATABASE_CONNECT_TIMEOUT":   "5s",
		"DATABASE_QUERY_TIMEOUT":     "3s",
	} {
		t.Setenv(name, value)
	}
}
