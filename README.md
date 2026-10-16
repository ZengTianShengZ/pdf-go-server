# pdf-go-server

Single-container HTML → PDF service. Go API injects request JSON into a local HTML template and converts it with [Gotenberg](https://github.com/gotenberg/gotenberg) (Chromium) running on localhost inside the same container.

## Quick start

```bash
docker build -t pdf-go-server .
docker run --rm -p 8080:80 pdf-go-server
```

Generate a PDF (binary):

```bash
curl -X POST http://localhost:8080/api/pdf \
  -H 'content-type: application/json' \
  -d @template/mock-inspection.json \
  -o report.pdf
```

Generate a PDF as base64 JSON (recommended for 微信云托管 / `callContainer`):

```bash
curl -X POST 'http://localhost:8080/api/pdf?format=base64' \
  -H 'content-type: application/json' \
  -d @template/mock-inspection.json
```

Health check:

```bash
curl http://localhost:8080/health
```

## API

### `POST /api/pdf`

- Body: JSON object (template data)
- Success (default): `200` `application/pdf` binary
- Success (`?format=base64` or header `X-PDF-Encoding: base64` or `Accept: application/json`):

```json
{
  "filename": "report.pdf",
  "mimeType": "application/pdf",
  "size": 123456,
  "pdfBase64": "<base64>"
}
```

- Errors: JSON `{ "code": <http status>, "message": "..." }`

> If you save an error JSON body as `.pdf`, the file is only ~50 bytes and readers show “未能加载 PDF 文档”. Always check HTTP status / `Content-Type` before writing a file.

### `GET /health`

- Gotenberg reachable: `200` `{ "status": "ok" }`
- Otherwise: `503` `{ "status": "unavailable", "message": "gotenberg unreachable" }`

## 微信云托管注意

云托管 / 小程序 `callContainer` 默认按 **text + json** 解析响应，直接收 `application/pdf` 二进制会被截断或变成几字节垃圾文件。

请用 base64 接口，并在客户端解码后写入文件：

```js
const res = await cloud.callContainer({
  path: '/api/pdf?format=base64',
  method: 'POST',
  header: {
    'X-WX-SERVICE': '你的服务名',
    'content-type': 'application/json',
  },
  data: reportData,
  timeout: 60000, // 转换常需数秒～十几秒
})

if (res.statusCode !== 200 || !res.data?.pdfBase64) {
  throw new Error(res.data?.message || `pdf failed: ${res.statusCode}`)
}

const fs = wx.getFileSystemManager()
const filePath = `${wx.env.USER_DATA_PATH}/report.pdf`
fs.writeFileSync(filePath, res.data.pdfBase64, 'base64')
```

若必须收二进制，需显式设置 `responseType: 'arraybuffer'`，且 **不要** 用默认 `dataType: 'json'`。

容器规格建议 ≥ **1GB 内存**（Chromium）。转换耗时常超过 5s，网关/客户端超时请调大。

## Template

Uses `template/v1-home-inspection.html` plus local `template/echarts.min.js` (uploaded to Gotenberg with the HTML). The service replaces the object literal assigned to `window.__REPORT_DATA__` with the request JSON; Chromium runs the page JS before PDF capture.

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
