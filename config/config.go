package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port         string
	GotenbergURL string
	TemplatePath string
	PDFTimeout   time.Duration
}

func Load() Config {
	timeoutSec := 60
	if v := os.Getenv("PDF_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	url := os.Getenv("GOTENBERG_URL")
	if url == "" {
		url = "http://127.0.0.1:3000"
	}

	tpl := os.Getenv("TEMPLATE_PATH")
	if tpl == "" {
		tpl = "template/v1-home-inspection.html"
	}

	return Config{
		Port:         port,
		GotenbergURL: url,
		TemplatePath: tpl,
		PDFTimeout:   time.Duration(timeoutSec) * time.Second,
	}
}
