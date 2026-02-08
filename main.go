package main

import (
	"embed"
	"log"

	"media-transcriber/internal/bootstrap"
)

//go:embed frontend/index.html frontend/wailsjs
var appAssets embed.FS

func main() {
	app, err := bootstrap.NewWithAssets(appAssets)
	if err != nil {
		log.Fatalf("bootstrap app: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
