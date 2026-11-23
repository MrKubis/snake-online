package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MrKubis/snake-online/internal/api"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Error("env loading failed")
		os.Exit(1)
	}
	cfg := api.ConfigFromEnv()
	srv, err := api.NewServer(cfg)
	if err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var handler http.Handler
	var runHub bool

	switch cfg.ServerMode {
	case api.ApiMode:
		handler = srv.ApiRoutes()
	case api.WebSocketMode:
		handler = srv.WSRoutes()
		runHub = true
	case api.AllMode:
		handler = srv.Routes()
		runHub = true
	default:
		slog.Error("unknown server mode", "mode", cfg.ServerMode)
		os.Exit(1)
	}
	slog.Info("server listening", "mode", cfg.ServerMode, "addr", cfg.Addr)

	hubDone := make(chan struct{})
	if runHub {
		go func() {
			defer close(hubDone)
			srv.RunHub(ctx)
		}()
		srv.StartQueueReaper(ctx, 10*time.Second, api.QueueHeartbeatMaxAge)
	} else {
		close(hubDone)
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		stop()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
			_ = httpSrv.Close()
		}

		select {
		case <-hubDone:
		case <-shutdownCtx.Done():
			slog.Warn("hub did not stop in time")
		}
	}

	slog.Info("server stopped")
}
