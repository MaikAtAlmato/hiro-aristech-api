package config

import (
	"time"

	bardiocconfig "bitbucket.org/almatoag/bardioc-go/config"
	"bitbucket.org/almatoag/env"
	"github.com/rs/zerolog/log"
)

// HTTP holds the public REST API server configuration.
type HTTP struct {
	Port int `env:"HTTP_PORT" default:"8080"`
}

// Monitoring holds the health-check server configuration.
type Monitoring struct {
	Port int `env:"MONITORING_PORT" default:"8081"`
}

// JWT holds the token signing configuration.
type JWT struct {
	Secret string        `env:"JWT_SECRET" required:"true"`
	TTL    time.Duration `env:"JWT_TTL" default:"15m"`
}

// Config is the complete environment-driven configuration for the service.
type Config struct {
	LogLevel   string `env:"LOG_LEVEL" default:"info"`
	Bardioc    bardiocconfig.Bardioc
	HTTP       HTTP
	Monitoring Monitoring
	JWT        JWT
	APIKey     string `env:"API_KEY" required:"true"`
}

// NewConfig parses configuration from environment variables, exiting the
// process if a required value is missing.
func NewConfig() Config {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal().Msgf("configuration parsing failed: %v", err)
	}
	return cfg
}
