package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MrKubis/snake-online/internal/matchmaker"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		slog.Error("env loading failed")
		os.Exit(1)
	}
	cfg := matchmaker.ConfigFromEnv()

	matchMaker, err := matchmaker.NewMatchMaker(cfg)
	if err != nil {
		slog.Error("startup failed", "error", err)
	}
	defer matchMaker.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	matchMaker.RunMatchmaking(ctx)

}
