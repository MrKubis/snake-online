package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/MrKubis/snake-online/internal/api"
)

func main() {
	cfg := api.ConfigFromEnv()
	srv, err := api.NewServer(cfg)
	if err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}

	slog.Info("api listening", "addr", cfg.Addr)

	var handler http.Handler
	switch cfg.ServerMode {
	case api.ApiMode:
		handler = srv.ApiRoutes()
	case api.WebSocketMode:
		handler = srv.WSRoutes()
	case api.AllMode:
		handler = srv.Routes()
	default:
		slog.Error("unknown server mode", "mode", cfg.ServerMode)
		os.Exit(1)
	}
	slog.Info("server listening", "mode", cfg.ServerMode, "addr", cfg.Addr)

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
