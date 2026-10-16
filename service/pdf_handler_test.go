package service

import (
	"context"
	"encoding/base64"
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
	lastAssets []gotenberg.Asset
}

func (s *stubConverter) Healthy(ctx context.Context) error {
	return s.healthyErr
}

func (s *stubConverter) ConvertHTML(ctx context.Context, html []byte, assets ...gotenberg.Asset) ([]byte, error) {
	s.lastHTML = append([]byte(nil), html...)
	s.lastAssets = append([]gotenberg.Asset(nil), assets...)
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
	assets := []gotenberg.Asset{{Name: "echarts.min.js", Data: []byte("/*echarts*/")}}
	srv := &Server{TemplatePath: writeTempTemplate(t), Converter: stub, Assets: assets}
	req := httptest.NewRequest(http.MethodPost, "/api/pdf", strings.NewReader(`{"name":"Jane"}`))
	rr := httptest.NewRecorder()
	srv.PDF(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("content-type %s", ct)
	}
	if rr.Header().Get("Content-Length") != "7" {
		t.Fatalf("content-length %s", rr.Header().Get("Content-Length"))
	}
	if !strings.Contains(string(stub.lastHTML), `"name":"Jane"`) {
		t.Fatalf("html not injected: %s", stub.lastHTML)
	}
	if len(stub.lastAssets) != 1 || stub.lastAssets[0].Name != "echarts.min.js" {
		t.Fatalf("assets %+v", stub.lastAssets)
	}
	if rr.Body.String() != "%PDF-ok" {
		t.Fatalf("body %q", rr.Body.String())
	}
}

func TestPDF_Base64Format(t *testing.T) {
	stub := &stubConverter{convertPDF: []byte("%PDF-ok")}
	srv := &Server{TemplatePath: writeTempTemplate(t), Converter: stub}
	req := httptest.NewRequest(http.MethodPost, "/api/pdf?format=base64", strings.NewReader(`{"name":"Jane"}`))
	rr := httptest.NewRecorder()
	srv.PDF(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type %s", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(body["pdfBase64"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "%PDF-ok" {
		t.Fatalf("decoded %q", raw)
	}
	if int(body["size"].(float64)) != 7 {
		t.Fatalf("size %+v", body["size"])
	}
}

func TestPDF_InvalidPayload(t *testing.T) {
	srv := &Server{
		TemplatePath: writeTempTemplate(t),
		Converter:    &stubConverter{convertPDF: []byte("not-a-pdf")},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/pdf", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	srv.PDF(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
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

func TestLoadTemplateAssets(t *testing.T) {
	dir := t.TempDir()
	tpl := filepath.Join(dir, "t.html")
	if err := os.WriteFile(tpl, []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "echarts.min.js"), []byte("js"), 0o644); err != nil {
		t.Fatal(err)
	}
	assets, err := LoadTemplateAssets(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Name != "echarts.min.js" || string(assets[0].Data) != "js" {
		t.Fatalf("%+v", assets)
	}
}
