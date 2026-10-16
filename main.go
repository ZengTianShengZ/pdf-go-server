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
	srv := &service.Server{
		TemplatePath: cfg.TemplatePath,
		Converter:    client,
	}

	http.HandleFunc("/health", srv.Health)
	http.HandleFunc("/api/pdf", srv.PDF)

	log.Printf("listening on :%s (gotenberg=%s template=%s)", cfg.Port, cfg.GotenbergURL, cfg.TemplatePath)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
