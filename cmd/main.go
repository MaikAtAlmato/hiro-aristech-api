// cmd/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	bardiocauth "bitbucket.org/almatoag/bardioc-go/auth"
	"bitbucket.org/almatoag/bardioc-go/graph/ws"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/api"
	appauth "bitbucket.org/almatoag/hiro-aristech-api/internal/auth"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/config"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"bitbucket.org/almatoag/monitoring"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Configure logging format first (before loading config to catch any config errors)
	ConfigureLoggerFormat()

	cfg := config.NewConfig()

	// Configure logging level after config is loaded
	ConfigureLoggerLevel(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	h := monitoring.GetHealthCheck()
	h.Register()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	authorizer := bardiocauth.NewAuthorizer(ctx, bardiocauth.NewTokenProvider(ctx, httpClient, cfg.Bardioc))
	client := ws.NewClient(cfg.Bardioc, authorizer)

	msgraphRepo := bardioc.NewMsgraphPersonRepository(client)
	valuemationRepo := bardioc.NewValuemationPersonRepository(client)
	ticketRepo := bardioc.NewTicketRepository(client)
	accountRepo := bardioc.NewMsgraphAccountRepository(client)
	issueRepo := bardioc.NewAutomationIssueRepository(client)
	intentRepo := bardioc.NewIntentRepository(client)

	resolver := &identity.Resolver{Msgraph: msgraphRepo, Valuemation: valuemationRepo}
	matcher := &identity.Matcher{Msgraph: msgraphRepo, Valuemation: valuemationRepo}
	tokens := appauth.NewTokenService(cfg.JWT.Secret, cfg.JWT.TTL)
	server := api.NewServer(resolver, matcher, tokens, ticketRepo, accountRepo, issueRepo, intentRepo, cfg.APIKey)

	mux := http.NewServeMux()
	humaAPI := humago.New(mux, huma.DefaultConfig("Aristech Voicebot API", "1.0.0"))
	server.Register(humaAPI)
	mux.HandleFunc("/health", h.Handler)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler: mux,
	}

	go func() {
		log.Info().Str("addr", httpServer.Addr).Msg("starting HTTP server")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failed")
		}
	}()

	go func() {
		log.Info().Int("port", cfg.Monitoring.Port).Msg("starting health check server")
		h.Start(cfg.Monitoring.Port)
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed")
	}
}

// ConfigureLoggerFormat configures the logger output format (stdout/stderr)
// This should be called first, before loading configuration
func ConfigureLoggerFormat() {
	// Configure output: stdout for all logs, stderr for fatal/panic only
	stdoutWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	stderrWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		NoColor:    false,
	}

	// Use MultiLevelWriter with a filtered writer for stderr
	multiWriter := zerolog.MultiLevelWriter(
		stdoutWriter,
		&zerolog.FilteredLevelWriter{
			Writer: zerolog.LevelWriterAdapter{Writer: stderrWriter},
			Level:  zerolog.FatalLevel,
		},
	)

	log.Logger = zerolog.New(multiWriter).With().Timestamp().Logger()
}

// ConfigureLoggerLevel sets the log level from configuration
// This should be called after loading configuration
func ConfigureLoggerLevel(level string) {
	logLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil || logLevel == zerolog.NoLevel {
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)
}
