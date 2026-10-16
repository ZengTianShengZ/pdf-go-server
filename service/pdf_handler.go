package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"pdf-go-server/gotenberg"
)

const maxBodyBytes = 2 << 20 // 2MB

// PDFConverter abstracts Gotenberg so handlers can be unit-tested.
type PDFConverter interface {
	Healthy(ctx context.Context) error
	ConvertHTML(ctx context.Context, html []byte) ([]byte, error)
}

type Server struct {
	TemplatePath string
	Converter    PDFConverter
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

	pdf, err := s.Converter.ConvertHTML(r.Context(), html)
	if err != nil {
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

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}
