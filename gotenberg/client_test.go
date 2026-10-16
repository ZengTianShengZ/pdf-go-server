package gotenberg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"up"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	if err := c.Healthy(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHealthy_Down(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	if err := c.Healthy(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestConvertHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/chromium/convert/html" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("waitDelay"); got != "500ms" {
			t.Fatalf("waitDelay=%q", got)
		}
		f, _, err := r.FormFile("files")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	pdf, err := c.ConvertHTML(context.Background(), []byte("<html></html>"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("got %q", pdf)
	}
}
