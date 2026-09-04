package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"agent-events/server/internal/adapters/logger"
	"agent-events/server/internal/adapters/memory"
	"agent-events/server/internal/adapters/oidc"
	"agent-events/server/internal/adapters/postgres"
	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/controller"
	"agent-events/server/internal/transport/http/handler"
	"agent-events/server/internal/transport/http/middleware"
	"agent-events/server/pkg/config"
)

const (
	shutdownTimeout = 10 * time.Second
	providerTimeout = 15 * time.Second
)

func main() {
	fx.New(
		fx.WithLogger(func(z *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: z}
		}),
		fx.Provide(
			config.Load,
			newZapLogger,
			provideLogger,
			provideIdentityVerifier,
			fx.Annotate(provideRateLimiter, fx.As(new(port.RateLimiter))),
			provideAuthConfig,
			provideRepositories,
			usecase.NewAuthService,
			usecase.NewEventService,
			handler.NewEventHandler,
			handler.NewAuthHandler,
			handler.NewAgentHandler,
			middleware.NewAuth,
			controller.NewRouter,
			newHTTPServer,
		),
		fx.Invoke(func(*http.Server) {}),
	).Run()
}

func provideLogger(z *zap.Logger) port.Logger {
	return logger.New(z)
}

func provideIdentityVerifier(cfg config.Config) (port.IdentityVerifier, error) {
	ctx, cancel := context.WithTimeout(context.Background(), providerTimeout)
	defer cancel()

	return oidc.New(ctx, cfg.GoogleClientID, cfg.AppleClientID, cfg.MicrosoftClientID, cfg.MicrosoftTenant)
}

func provideRateLimiter(cfg config.Config) *memory.RateLimiter {
	return memory.NewRateLimiter(map[string]memory.Limit{
		usecase.ActionCreateEvent:  {Max: cfg.RateLimitEventsPerDay, Window: 24 * time.Hour},
		usecase.ActionAuthExchange: {Max: cfg.RateLimitExchangePerHour, Window: time.Hour},
	})
}

func provideAuthConfig(cfg config.Config) usecase.AuthConfig {
	return usecase.AuthConfig{
		OwnerTokenTTL:     cfg.OwnerTokenTTL,
		MaxAgentsPerOwner: cfg.MaxAgentsPerOwner,
	}
}

func provideRepositories(lc fx.Lifecycle, cfg config.Config, log port.Logger) (
	port.EventRepository,
	port.OwnerRepository,
	port.OwnerTokenRepository,
	port.AgentRepository,
	error,
) {
	if cfg.DatabaseURL == "" {
		log.Info("using in-memory storage, set DATABASE_URL for persistence (data is lost on restart)")

		return memory.NewEventRepository(),
			memory.NewOwnerRepository(),
			memory.NewOwnerTokenRepository(),
			memory.NewAgentRepository(),
			nil
	}

	db, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return db.Close()
		},
	})

	if err := postgres.Migrate(db); err != nil {
		return nil, nil, nil, nil, err
	}

	log.Info("postgres storage ready")

	return postgres.NewEventRepository(db),
		postgres.NewOwnerRepository(db),
		postgres.NewOwnerTokenRepository(db),
		postgres.NewAgentRepository(db),
		nil
}

func newZapLogger(lc fx.Lifecycle, cfg config.Config) (*zap.Logger, error) {
	z, err := logger.NewZapLogger(cfg)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			_ = z.Sync()
			return nil
		},
	})

	return z, nil
}

func newHTTPServer(lc fx.Lifecycle, cfg config.Config, router *chi.Mux, log port.Logger) *http.Server {
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", srv.Addr, err)
			}

			log.Info("http server listening", port.Str("address", srv.Addr), port.Str("env", cfg.Env))

			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Error("http server failed", port.Err(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("shutting down http server")

			ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()

			if err := srv.Shutdown(ctx); err != nil {
				return fmt.Errorf("shutdown http server: %w", err)
			}

			return nil
		},
	})

	return srv
}
