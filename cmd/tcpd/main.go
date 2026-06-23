package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"go-entry-task/internal/config"
	"go-entry-task/internal/service"
	"go-entry-task/internal/store/mysqlstore"
	"go-entry-task/internal/store/redisstore"
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
	db, err := mysqlstore.Open(cfg.Database.DSN, cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime.Duration)
	if err != nil {
		log.Error("open database failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Error("ping database failed", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var sessions service.SessionStore
	var pictureCache service.PictureCache
	if cfg.Redis.Addr != "" {
		redisSessions := redisstore.New(cfg.Redis)
		if err := redisSessions.Ping(ctx); err != nil {
			log.Warn("redis unavailable; continuing without redis-backed sessions or picture cache", "addr", cfg.Redis.Addr, "error", err)
		} else {
			sessions = redisSessions
			pictureCache = redisSessions
			log.Info("redis sessions and picture cache enabled", "addr", cfg.Redis.Addr)
		}
	}
	svc, err := service.New(mysqlstore.New(db), cfg, service.Options{
		Sessions:     sessions,
		PictureCache: pictureCache,
	})
	if err != nil {
		log.Error("create service failed", "error", err)
		os.Exit(1)
	}
	if err := service.NewTCPServer(cfg.TCP, svc, log).ListenAndServe(ctx); err != nil {
		log.Error("tcp server failed", "error", err)
		os.Exit(1)
	}
}
