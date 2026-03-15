package main

import (
	"context"
	"flag"
	"log"

	"inkmuse/backend/internal/infra/storage"
	"inkmuse/backend/pkg/config"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to runtime config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := storage.RunMigrations(context.Background(), cfg.Storage); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
}
