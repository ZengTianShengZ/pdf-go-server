# Gotenberg HTML→PDF Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform this repo into a single-container HTML→PDF service: Go API injects JSON into `template/v1-home-inspection.html` and converts via localhost Gotenberg.

**Architecture:** Multi-stage Docker image based on `gotenberg/gotenberg:8`. Entrypoint starts Gotenberg on `127.0.0.1:3000`, waits for health, then runs Go on `:80`. Go exposes `POST /api/pdf` and `GET /health`; template injection replaces only the `{...}` after `window.__REPORT_DATA__ =`.

**Tech Stack:** Go 1.22+ (stdlib `net/http`), Gotenberg 8 Chromium convert/html, Docker multi-stage build.

**Spec:** `docs/superpowers/specs/2026-10-16-gotenberg-html-pdf-design.md`

---

## File Structure

| Path | Responsibility |
|---|---|
| `main.go` | Config from env, wire handlers, listen |
| `config/config.go` | Parse `PORT`, `GOTENBERG_URL`, `TEMPLATE_PATH`, `PDF_TIMEOUT_SEC` |
| `service/response.go` | JSON error writer `{code, message}` |
| `service/template.go` | Load template + inject `__REPORT_DATA__` |
| `service/template_test.go` | Unit tests for injection |
| `gotenberg/client.go` | Health check + ConvertHTML |
| `gotenberg/client_test.go` | httptest tests |
| `service/pdf_handler.go` | `POST /api/pdf`, `GET /health` |
| `service/pdf_handler_test.go` | Handler tests with fake client |
| `docker-entrypoint.sh` | Start Gotenberg → wait health → exec Go |
| `Dockerfile` | builder + gotenberg runtime |
| `template/v1-home-inspection.html` | Fixed HTML template (already present) |
| `README.md` | Replace with PDF service docs |

**Delete:** `db/`, `service/counter_service.go`, `index.html`, MySQL bits in `container.config.json`, old gorm deps in `go.mod`.

**Module rename:** `wxcloudrun-golang` → `pdf-go-server`.

---

### Task 1: Scaffold module + config + wipe counter stack

**Files:**
- Create: `config/config.go`
- Modify: `go.mod`, `main.go` (stub)
- Delete: `db/`, `service/counter_service.go`, `index.html`
- Modify: `container.config.json` (remove DB fields)

- [ ] **Step 1: Rename module and drop MySQL deps**

```bash
cd /Users/tiansheng/tiansheng/pdf-go-server
# rewrite go.mod to:
# module pdf-go-server
# go 1.22
rm -rf db
rm -f service/counter_service.go index.html go.sum
```

Write `go.mod`:

```go
module pdf-go-server

go 1.22
```

Run: `go mod tidy`  
Expected: empty require block (stdlib only), new `go.sum` optional/empty.

- [ ] **Step 2: Add config package**

Create `config/config.go`:

```go
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port          string
	GotenbergURL  string
	TemplatePath  string
	PDFTimeout    time.Duration
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
```

- [ ] **Step 3: Stub main that compiles**

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"pdf-go-server/config"
)

func main() {
	cfg := config.Load()
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	log.Printf("listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
```

Run: `go build -o /tmp/pdf-go-server .`  
Expected: success.

- [ ] **Step 4: Slim `container.config.json`**

Keep containerPort 80; remove `dataBaseName`, `executeSQLs`, and MySQL-related comments. Leave empty `envParams` or document PDF env vars as comments if JSON allows (JSON has no comments — just empty envParams).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: scaffold PDF service module and remove counter/MySQL stack"
```

---

### Task 2: Template injection (TDD)

**Files:**
- Create: `service/template.go`
- Test: `service/template_test.go`

- [ ] **Step 1: Write failing tests**

Create `service/template_test.go`:

```go
package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectReportData_ReplacesObjectOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.html")
	src := `<script>window.__REPORT_DATA__ = {"name":"old"};</script>`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := InjectReportData(path, []byte(`{"name":"Jane","city":"SF"}`))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `window.__REPORT_DATA__ = `) {
		t.Fatalf("assignment missing: %s", s)
	}
	if !strings.Contains(s, `"name":"Jane"`) || !strings.Contains(s, `"city":"SF"`) {
		t.Fatalf("new data missing: %s", s)
	}
	if strings.Contains(s, `"name":"old"`) {
		t.Fatalf("old data still present: %s", s)
	}
}

func TestInjectReportData_MissingMarker(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.html")
	if err := os.WriteFile(path, []byte(`<html></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := InjectReportData(path, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run: `go test ./service/ -run InjectReportData -v`  
Expected: FAIL (`InjectReportData` undefined).

- [ ] **Step 3: Implement `service/template.go`**

```go
package service

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
)

// Matches: window.__REPORT_DATA__ = { ... };
// Non-greedy object body; assumes single top-level assignment in template.
var reportDataRE = regexp.MustCompile(`window\.__REPORT_DATA__\s*=\s*\{[\s\S]*?\};`)

func InjectReportData(templatePath string, jsonObject []byte) ([]byte, error) {
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}
	if !reportDataRE.Match(raw) {
		return nil, fmt.Errorf("template missing window.__REPORT_DATA__ assignment")
	}
	replacement := append([]byte("window.__REPORT_DATA__ = "), jsonObject...)
	replacement = append(replacement, ';')
	return reportDataRE.ReplaceAll(raw, replacement), nil
}

// EnsureJSONObject returns an error if data is not a JSON object.
func EnsureJSONObject(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return fmt.Errorf("body must be a JSON object")
	}
	return nil
}
```

Note: callers must pass `json.Marshal` output of a decoded `map[string]any` (or raw body after object check). Prefer: decode with `json.Decoder.UseNumber()`, verify `map[string]json.RawMessage` / `map[string]any`, re-marshal for stable injection.

Refine handler later to: `json.Unmarshal` into `map[string]any`; if fail or not object → 400; `json.Marshal(obj)` → inject.

Update `InjectReportData` usage: pass marshaled object bytes only.

- [ ] **Step 4: Run tests — expect PASS**

Run: `go test ./service/ -run InjectReportData -v`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/template.go service/template_test.go
git commit -m "feat: inject JSON into window.__REPORT_DATA__ template marker"
```

---

### Task 3: Gotenberg client (TDD)

**Files:**
- Create: `gotenberg/client.go`
- Test: `gotenberg/client_test.go`

- [ ] **Step 1: Write failing tests**

```go
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
		w.Write([]byte(`{"status":"up"}`))
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
		if r.FormValue("waitDelay") != "500ms" {
			t.Fatalf("waitDelay=%q", r.FormValue("waitDelay"))
		}
		f, _, err := r.FormFile("files")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF-1.4 fake"))
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
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./gotenberg/ -v`  
Expected: FAIL (package undefined).

- [ ] **Step 3: Implement `gotenberg/client.go`**

```go
package gotenberg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Healthy(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("gotenberg unhealthy: status %d", res.StatusCode)
	}
	return nil
}

func (c *Client) ConvertHTML(ctx context.Context, html []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(html); err != nil {
		return nil, err
	}
	if err := w.WriteField("waitDelay", "500ms"); err != nil { // spec: 0.5s
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/forms/chromium/convert/html", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := c.httpClient.Do(req)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, fmt.Errorf("%w", ErrTimeout)
		}
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", ErrConvert, res.StatusCode, truncate(body, 200))
	}
	return body, nil
}

var (
	ErrUnreachable = fmt.Errorf("gotenberg unreachable")
	ErrTimeout     = fmt.Errorf("gotenberg timeout")
	ErrConvert     = fmt.Errorf("gotenberg convert failed")
)

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}
```

Use `errors.Is` / wrapped sentinel errors so handlers map to 502 / 504 / 500.

Prefer `errors.New` + `fmt.Errorf("...: %w", ErrX)` — fix compile if `var ErrX = fmt.Errorf(...)` is awkward; use `var ErrX = errors.New(...)`.

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./gotenberg/ -v`  
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add gotenberg/
git commit -m "feat: add Gotenberg HTTP client for health and HTML convert"
```

---

### Task 4: HTTP handlers (TDD)

**Files:**
- Create: `service/response.go`, `service/pdf_handler.go`
- Test: `service/pdf_handler_test.go`
- Modify: `main.go`

- [ ] **Step 1: Define converter interface + failing handler tests**

In `service/pdf_handler.go` (interface first):

```go
type PDFConverter interface {
	Healthy(ctx context.Context) error
	ConvertHTML(ctx context.Context, html []byte) ([]byte, error)
}
```

Tests with stub converter:
- invalid JSON → 400
- JSON array → 400
- healthy ok → 200 `{"status":"ok"}`
- healthy fail → 503
- success path → 200 `application/pdf`
- unreachable → 502
- timeout → 504
- convert error → 500

Use temp template file with `__REPORT_DATA__` marker.

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./service/ -v`  
Expected: FAIL on missing handlers.

- [ ] **Step 3: Implement handlers**

`service/response.go`:

```go
func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    status,
		"message": msg,
	})
}
```

`PDFHandler` / `HealthHandler` as methods on `struct Server { TemplatePath string; Converter PDFConverter }`.

`POST /api/pdf` flow:
1. Method check
2. `io.ReadAll` body (limit e.g. 2MB)
3. `json.Unmarshal` → `map[string]any`; error → 400
4. `json.Marshal` map → inject
5. `ConvertHTML`; map errors: unreachable 502, timeout 504, else 500
6. Write PDF bytes

`GET /health`: call `Healthy`; ok → 200; else 503 with spec body.

- [ ] **Step 4: Wire `main.go`**

```go
cfg := config.Load()
client := gotenberg.New(cfg.GotenbergURL, cfg.PDFTimeout)
srv := &service.Server{TemplatePath: cfg.TemplatePath, Converter: client}
http.HandleFunc("/health", srv.Health)
http.HandleFunc("/api/pdf", srv.PDF)
log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
```

- [ ] **Step 5: Run all unit tests**

Run: `go test ./...`  
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add main.go service/ config/
git commit -m "feat: add /api/pdf and /health HTTP handlers"
```

---

### Task 5: Docker one-container runtime

**Files:**
- Create: `docker-entrypoint.sh`
- Modify: `Dockerfile`
- Modify: `README.md`

- [ ] **Step 1: Write `docker-entrypoint.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

gotenberg \
  --api-port=3000 \
  --api-bind-ip=127.0.0.1 \
  &

# Wait until Gotenberg /health returns 200 (max ~60s)
for i in $(seq 1 60); do
  if curl -sf http://127.0.0.1:3000/health >/dev/null; then
    break
  fi
  sleep 1
done

if ! curl -sf http://127.0.0.1:3000/health >/dev/null; then
  echo "gotenberg failed to become healthy" >&2
  exit 1
fi

exec /app/pdf-go-server
```

Make executable: `chmod +x docker-entrypoint.sh`

Note: official `gotenberg/gotenberg` image may not include `curl`/`bash`. Prefer `wget` if present, or a tiny Go wait, or use the image's shell. **During implementation:** inspect image (`docker run --rm --entrypoint sh gotenberg/gotenberg:8 -c 'which curl wget bash sh'`) and adapt. If no curl, use:

```bash
gotenberg --api-port=3000 --api-bind-ip=127.0.0.1 &
# busy-wait with /dev/tcp if bash available:
for i in $(seq 1 60); do
  (echo > /dev/tcp/127.0.0.1/3000) >/dev/null 2>&1 && break
  sleep 1
done
```

Or install nothing and let Go retry on first request while entrypoint just backgrounds gotenberg and `exec`s Go immediately — **spec prefers wait**; implement wait with whatever the base image provides.

Gotenberg image user is typically non-root `gotenberg`. Run entrypoint accordingly; binary and templates must be readable/executable by that user (or run as root if image allows override).

- [ ] **Step 2: Write Dockerfile**

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/pdf-go-server .

FROM gotenberg/gotenberg:8
USER root
WORKDIR /app
COPY --from=builder /out/pdf-go-server /app/pdf-go-server
COPY template /app/template
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/pdf-go-server /app/docker-entrypoint.sh
ENV PORT=80 \
    GOTENBERG_URL=http://127.0.0.1:3000 \
    TEMPLATE_PATH=/app/template/v1-home-inspection.html \
    PDF_TIMEOUT_SEC=60
EXPOSE 80
ENTRYPOINT ["/app/docker-entrypoint.sh"]
```

Verify whether base image entrypoint/cmd must be cleared; `ENTRYPOINT` override is enough.

- [ ] **Step 3: Build image**

Run: `docker build -t pdf-go-server .`  
Expected: success.

- [ ] **Step 4: Run and smoke-test**

```bash
docker run --rm -d -p 8080:80 --name pdf-go-server pdf-go-server
# wait a few seconds for Chromium
curl -s http://localhost:8080/health
curl -X POST http://localhost:8080/api/pdf \
  -H 'content-type: application/json' \
  -d '{"name":"John Doe","email":"john.doe@example.com","phone":"1234567890","address":"123 Main St","city":"Anytown","state":"CA","zip":"12345","country":"USA"}' \
  -o /tmp/out.pdf
file /tmp/out.pdf
docker stop pdf-go-server
```

Expected: health `{"status":"ok"}`; `out.pdf` is PDF (`file` reports PDF).

Negative checks:
```bash
curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/api/pdf -H 'content-type: application/json' -d '[]'
# expect 400
```

- [ ] **Step 5: Update README**

Replace counter docs with: what the service does, env vars, `docker build/run`, curl examples for `/api/pdf` and `/health`.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile docker-entrypoint.sh README.md template/
git commit -m "feat: single-container Docker image with Gotenberg and Go API"
```

---

### Task 6: Final verification

- [ ] **Step 1: Unit tests**

Run: `go test ./...`  
Expected: PASS.

- [ ] **Step 2: Docker acceptance (from spec)**

Rebuild and re-run smoke tests from Task 5 Step 4.

- [ ] **Step 3: Commit any leftover fixes**

```bash
git status
# if dirty: commit fix
```

---

## Notes for implementers

- **waitDelay:** Spec says 0.5 seconds; Gotenberg form value must be a duration string → use `500ms` ([docs](https://gotenberg.dev/docs/convert-with-chromium/convert-html-to-pdf)).
- **Health:** Gotenberg `GET /health` ([docs](https://gotenberg.dev/docs/system/get-health-check)).
- **Bind:** `--api-bind-ip=127.0.0.1` ([configuration](https://gotenberg.dev/docs/configuration)).
- Do not reintroduce MySQL/gorm.
- Keep template client-side JS fill; do not switch to Go `html/template` for DOM.
