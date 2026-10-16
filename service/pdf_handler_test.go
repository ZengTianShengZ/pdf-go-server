package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pdf-go-server/gotenberg"
)

type stubConverter struct {
	healthyErr error
	convertPDF []byte
	convertErr error
	lastHTML   []byte
}

func (s *stubConverter) Healthy(ctx context.Context) error {
	return s.healthyErr
}

func (s *stubConverter) ConvertHTML(ctx context.Context, html []byte) ([]byte, error) {
	s.lastHTML = append([]byte(nil), html...)
	return s.convertPDF, s.convertErr
}

func writeTempTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "t.html")
	src := `<script>window.__REPORT_DATA__ = {"name":"old"};</script><body></body>`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHealth_OK(t *testing.T) {
	srv := &Server{Converter: &stubConverter{}}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	srv.Health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestHealth_Unavailable(t *testing.T) {
	srv := &Server{Converter: &stubConverter{healthyErr: errors.New("down")}}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	srv.Health(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestPDF_InvalidJSON(t *testing.T) {
	srv := &Server{TemplatePath: writeTempTemplate(t), Converter: &stubConverter{}}
	req := httptest.NewRequest(http.MethodPost, "/api/pdf", strings.NewReader(`{`))
	rr := httptest.NewRecorder()
	srv.PDF(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestPDF_NonObject(t *testing.T) {
	srv := &Server{TemplatePath: writeTempTemplate(t), Converter: &stubConverter{}}
	req := httptest.NewRequest(http.MethodPost, "/api/pdf", strings.NewReader(`[]`))
	rr := httptest.NewRecorder()
	srv.PDF(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestPDF_Success(t *testing.T) {
	stub := &stubConverter{convertPDF: []byte("%PDF-ok")}
	srv := &Server{TemplatePath: writeTempTemplate(t), Converter: stub}
	req := httptest.NewRequest(http.MethodPost, "/api/pdf", strings.NewReader(`{"name":"Jane"}`))
	rr := httptest.NewRecorder()
	srv.PDF(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("content-type %s", ct)
	}
	if !strings.Contains(string(stub.lastHTML), `"name":"Jane"`) {
		t.Fatalf("html not injected: %s", stub.lastHTML)
	}
	if rr.Body.String() != "%PDF-ok" {
		t.Fatalf("body %q", rr.Body.String())
	}
}

func TestPDF_Unreachable(t *testing.T) {
	srv := &Server{
		TemplatePath: writeTempTemplate(t),
		Converter:    &stubConverter{convertErr: gotenberg.ErrUnreachable},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/pdf", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	srv.PDF(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status %d", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if int(body["code"].(float64)) != http.StatusBadGateway {
		t.Fatalf("code field %+v", body)
	}
}

func TestPDF_Timeout(t *testing.T) {
	srv := &Server{
		TemplatePath: writeTempTemplate(t),
		Converter:    &stubConverter{convertErr: gotenberg.ErrTimeout},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/pdf", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	srv.PDF(rr, req)
	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status %d", rr.Code)
	}
}
