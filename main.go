package main

import (
	"log"
	"net/http"

	"pdf-go-server/config"
	"pdf-go-server/gotenberg"
	"pdf-go-server/service"
)

func main() {
	cfg := config.Load()
	client := gotenberg.New(cfg.GotenbergURL, cfg.PDFTimeout)

	assets, err := service.LoadTemplateAssets(cfg.TemplatePath)
	if err != nil {
		log.Fatalf("load template assets: %v", err)
	}

	srv := &service.Server{
		TemplatePath: cfg.TemplatePath,
		Converter:    client,
		Assets:       assets,
	}

	http.HandleFunc("/health", srv.Health)
	http.HandleFunc("/api/pdf", srv.PDF)

	log.Printf("listening on :%s (gotenberg=%s template=%s assets=%d)", cfg.Port, cfg.GotenbergURL, cfg.TemplatePath, len(assets))
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
