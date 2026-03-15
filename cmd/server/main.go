package main

import (
	"flag"
	"log"

	"inkmuse/backend/internal/app"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to runtime config")
	flag.Parse()

	bootstrap, err := app.LoadBootstrap(*configPath)
	if err != nil {
		log.Fatalf("bootstrap service: %v", err)
	}
	defer func() {
		if closeErr := bootstrap.Close(); closeErr != nil {
			log.Printf("close bootstrap resources: %v", closeErr)
		}
	}()

	bootstrap.HTTP.Spin()
}
