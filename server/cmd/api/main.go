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
	"agent-events/server/internal/core/port"
	"agent-events/server/internal/core/usecase"
	"agent-events/server/internal/transport/http/controller"
	"agent-events/server/internal/transport/http/handler"
	"agent-events/server/pkg/config"
)

const shutdownTimeout = 10 * time.Second

func main() {
	fx.New(
		fx.WithLogger(func(z *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: z}
		}),
		fx.Provide(
			config.Load,
			newZapLogger,
			provideLogger,
			fx.Annotate(memory.NewEventRepository, fx.As(new(port.EventRepository))),
			usecase.NewEventService,
			handler.NewEventHandler,
			controller.NewRouter,
			newHTTPServer,
		),
		fx.Invoke(func(*http.Server) {}),
	).Run()
}

func provideLogger(z *zap.Logger) port.Logger {
	return logger.New(z)
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
