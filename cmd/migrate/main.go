package main

import (
	"context"
	"flag"
	"log"
	"time"

	"go-entry-task/internal/config"
	"go-entry-task/internal/store/mysqlstore"
)

func main() {
	configPath := flag.String("config", "config/config.json", "path to config JSON")
	schemaPath := flag.String("schema", "db/schema.sql", "path to schema SQL")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	db, err := mysqlstore.Open(cfg.Database.DSN, cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime.Duration)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	if err := mysqlstore.Migrate(ctx, db, *schemaPath); err != nil {
		log.Fatal(err)
	}
	log.Println("migration complete")
}
