package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"go-entry-task/internal/config"
	"go-entry-task/internal/httpapi"
)

func main() {
	configPath := flag.String("config", "config/config.json", "path to config JSON")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("load config failed", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := httpapi.NewTCPClient(cfg)
	if err := httpapi.NewServer(cfg, client, log).ListenAndServe(ctx); err != nil {
		log.Error("http server failed", "error", err)
		os.Exit(1)
	}
}
