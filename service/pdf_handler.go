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
	"time"

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
	start := time.Now()
	reqID := r.Header.Get("X-Request-Id")
	if reqID == "" {
		reqID = r.Header.Get("X-Cloudbase-Request-Id")
	}
	if reqID == "" {
		reqID = "-"
	}
	logf := func(format string, args ...any) {
		log.Printf("[pdf req=%s] "+format, append([]any{reqID}, args...)...)
	}

	if r.Method != http.MethodPost {
		logf("reject method=%s", r.Method)
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		logf("read body failed: %v", err)
		writeErr(w, http.StatusBadRequest, "failed to read body")
		return
	}
	if len(body) > maxBodyBytes {
		logf("body too large: %d bytes", len(body))
		writeErr(w, http.StatusBadRequest, "body too large")
		return
	}

	if err := EnsureJSONObjectRoot(body); err != nil {
		logf("json root invalid: %v bodyBytes=%d", err, len(body))
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		logf("json unmarshal failed: %v bodyBytes=%d", err, len(body))
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	reportID, _ := obj["id"].(string)
	outMode := "binary"
	if wantBase64PDF(r) {
		outMode = "base64"
	}
	logf("start reportId=%q bodyBytes=%d outMode=%s accept=%q xPdfEnc=%q format=%q assets=%d",
		reportID, len(body), outMode, r.Header.Get("Accept"),
		r.Header.Get("X-PDF-Encoding"), r.URL.Query().Get("format"), len(s.Assets))

	marshaled, err := json.Marshal(obj)
	if err != nil {
		logf("json remarshal failed: %v", err)
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	html, err := InjectReportData(s.TemplatePath, marshaled)
	if err != nil {
		logf("template inject failed: %v", err)
		writeErr(w, http.StatusInternalServerError, "template inject failed")
		return
	}
	logf("template injected htmlBytes=%d", len(html))

	convertStart := time.Now()
	pdf, err := s.Converter.ConvertHTML(r.Context(), html, s.Assets...)
	convertMs := time.Since(convertStart).Milliseconds()
	if err != nil {
		logf("convert error after %dms: %v", convertMs, err)
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
		preview := string(pdf)
		if len(preview) > 80 {
			preview = preview[:80]
		}
		logf("convert returned non-pdf after %dms pdfBytes=%d preview=%q", convertMs, len(pdf), preview)
		writeErr(w, http.StatusBadGateway, "pdf conversion returned invalid document")
		return
	}

	// WeChat Cloud / callContainer defaults to JSON+text and mangles binary PDF.
	// Prefer base64 JSON when clients ask for it (or Accept: application/json).
	if outMode == "base64" {
		b64 := base64.StdEncoding.EncodeToString(pdf)
		logf("ok outMode=base64 pdfBytes=%d b64Chars=%d convertMs=%d totalMs=%d",
			len(pdf), len(b64), convertMs, time.Since(start).Milliseconds())
		writeJSON(w, http.StatusOK, map[string]any{
			"filename":  "report.pdf",
			"mimeType":  "application/pdf",
			"size":      len(pdf),
			"pdfBase64": b64,
		})
		return
	}

	logf("ok outMode=binary pdfBytes=%d convertMs=%d totalMs=%d",
		len(pdf), convertMs, time.Since(start).Milliseconds())
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
