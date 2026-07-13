package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BARDIOC_ENDPOINT", "hiro.example.com")
	t.Setenv("BARDIOC_USERNAME", "svc-aristech")
	t.Setenv("BARDIOC_PASSWORD", "secret")
	t.Setenv("BARDIOC_CLIENT_ID", "client-id")
	t.Setenv("BARDIOC_CLIENT_SECRET", "client-secret")
	t.Setenv("BARDIOC_SCOPE", "scope-123")
	t.Setenv("JWT_SECRET", "jwt-secret")
	t.Setenv("API_KEY", "test-api-key")
}

func TestNewConfig_Defaults(t *testing.T) {
	setRequiredEnv(t)

	cfg := NewConfig()

	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, 8080, cfg.HTTP.Port)
	require.Equal(t, 8081, cfg.Monitoring.Port)
	require.Equal(t, 15*time.Minute, cfg.JWT.TTL)
	require.Equal(t, "jwt-secret", cfg.JWT.Secret)
	require.Equal(t, "hiro.example.com", cfg.Bardioc.Endpoint)
}

func TestNewConfig_Overrides(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("JWT_TTL", "30m")

	cfg := NewConfig()

	require.Equal(t, "debug", cfg.LogLevel)
	require.Equal(t, 9090, cfg.HTTP.Port)
	require.Equal(t, 30*time.Minute, cfg.JWT.TTL)
}
