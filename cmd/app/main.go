package main

import (
	"log"

	"media-transcriber/internal/bootstrap"
)

func main() {
	app, err := bootstrap.New()
	if err != nil {
		log.Fatalf("bootstrap app: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
