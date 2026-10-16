# pdf-go-server

Single-container HTML → PDF service. Go API injects request JSON into a local HTML template and converts it with [Gotenberg](https://github.com/gotenberg/gotenberg) (Chromium) running on localhost inside the same container.

## Quick start

```bash
docker build -t pdf-go-server .
docker run --rm -p 8080:80 pdf-go-server
```

Generate a PDF:

```bash
curl -X POST http://localhost:8080/api/pdf \
  -H 'content-type: application/json' \
  -d '{
    "name": "John Doe",
    "email": "john.doe@example.com",
    "phone": "1234567890",
    "address": "123 Main St, Anytown, USA",
    "city": "Anytown",
    "state": "CA",
    "zip": "12345",
    "country": "USA"
  }' \
  -o report.pdf
```

Health check:

```bash
curl http://localhost:8080/health
```

## API

### `POST /api/pdf`

- Body: JSON object (template data)
- Success: `200` `application/pdf`
- Errors: JSON `{ "code": <http status>, "message": "..." }`

### `GET /health`

- Gotenberg reachable: `200` `{ "status": "ok" }`
- Otherwise: `503` `{ "status": "unavailable", "message": "gotenberg unreachable" }`

## Template

Uses `template/v1-home-inspection.html`. The service replaces the object literal assigned to `window.__REPORT_DATA__` with the request JSON; Chromium runs the page JS before PDF capture.

## Environment

| Variable | Default | Description |
|---|---|---|
| `PORT` | `80` | Go listen port |
| `GOTENBERG_URL` | `http://127.0.0.1:3000` | In-container Gotenberg |
| `TEMPLATE_PATH` | `template/v1-home-inspection.html` | HTML template path |
| `PDF_TIMEOUT_SEC` | `60` | Conversion timeout |

## Local unit tests

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.22-alpine go test ./...
```
