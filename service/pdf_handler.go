package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"pdf-go-server/gotenberg"
)

const maxBodyBytes = 2 << 20 // 2MB

// PDFConverter abstracts Gotenberg so handlers can be unit-tested.
type PDFConverter interface {
	Healthy(ctx context.Context) error
	ConvertHTML(ctx context.Context, html []byte, assets ...gotenberg.Asset) ([]byte, error)
}

type Server struct {
	TemplatePath string
	Converter    PDFConverter
	// Assets uploaded with the HTML (e.g. local echarts). Optional.
	Assets []gotenberg.Asset
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := s.Converter.Healthy(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":  "unavailable",
			"message": "gotenberg unreachable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) PDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "failed to read body")
		return
	}
	if len(body) > maxBodyBytes {
		writeErr(w, http.StatusBadRequest, "body too large")
		return
	}

	if err := EnsureJSONObjectRoot(body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	marshaled, err := json.Marshal(obj)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	html, err := InjectReportData(s.TemplatePath, marshaled)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "template inject failed")
		return
	}

	pdf, err := s.Converter.ConvertHTML(r.Context(), html, s.Assets...)
	if err != nil {
		log.Printf("pdf convert error: %v", err)
		switch {
		case errors.Is(err, gotenberg.ErrTimeout):
			writeErr(w, http.StatusGatewayTimeout, "pdf conversion timed out")
		case errors.Is(err, gotenberg.ErrUnreachable):
			writeErr(w, http.StatusBadGateway, "gotenberg unreachable")
		default:
			writeErr(w, http.StatusInternalServerError, "pdf conversion failed")
		}
		return
	}

	if !strings.HasPrefix(string(pdf), "%PDF") {
		log.Printf("pdf convert returned non-pdf payload (%d bytes)", len(pdf))
		writeErr(w, http.StatusBadGateway, "pdf conversion returned invalid document")
		return
	}

	// WeChat Cloud / callContainer defaults to JSON+text and mangles binary PDF.
	// Prefer base64 JSON when clients ask for it (or Accept: application/json).
	if wantBase64PDF(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"filename":  "report.pdf",
			"mimeType":  "application/pdf",
			"size":      len(pdf),
			"pdfBase64": base64.StdEncoding.EncodeToString(pdf),
		})
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

func wantBase64PDF(r *http.Request) bool {
	if strings.EqualFold(r.URL.Query().Get("format"), "base64") {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-PDF-Encoding"), "base64") {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") &&
		!strings.Contains(accept, "application/pdf")
}

// LoadTemplateAssets loads local assets referenced by the HTML template
// (echarts.min.js next to the template; multipart filenames are basenamed).
func LoadTemplateAssets(templatePath string) ([]gotenberg.Asset, error) {
	dir := filepath.Dir(templatePath)
	echartsPath := filepath.Join(dir, "echarts.min.js")
	data, err := os.ReadFile(echartsPath)
	if err != nil {
		return nil, err
	}
	return []gotenberg.Asset{{
		Name: "echarts.min.js",
		Data: data,
	}}, nil
}
