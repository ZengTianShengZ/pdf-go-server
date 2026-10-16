package gotenberg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"path"
	"time"
)

var (
	ErrUnreachable = errors.New("gotenberg unreachable")
	ErrTimeout     = errors.New("gotenberg timeout")
	ErrConvert     = errors.New("gotenberg convert failed")
)

// Asset is an extra file uploaded alongside index.html (same relative path in Chromium).
type Asset struct {
	Name string
	Data []byte
}

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
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return mapDoError(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUnreachable, res.StatusCode)
	}
	return nil
}

func (c *Client) ConvertHTML(ctx context.Context, html []byte, assets ...Asset) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(html); err != nil {
		return nil, err
	}
	for _, asset := range assets {
		// mime/multipart FileName() uses filepath.Base — keep names flat.
		name := path.Base(path.Clean("/" + asset.Name))
		if name == "" || name == "." || name == "/" {
			return nil, fmt.Errorf("%w: invalid asset name", ErrConvert)
		}
		ap, err := w.CreateFormFile("files", name)
		if err != nil {
			return nil, err
		}
		if _, err := ap.Write(asset.Data); err != nil {
			return nil, err
		}
	}
	// Wait for template JS + chart init before Chromium captures PDF.
	if err := w.WriteField("waitDelay", "2s"); err != nil {
		return nil, err
	}
	// A4. Form margins override @page; print backgrounds so tags and cards keep color.
	fields := [][2]string{
		{"paperWidth", "8.27"},
		{"paperHeight", "11.69"},
		{"preferCssPageSize", "true"},
		{"printBackground", "true"},
		{"marginTop", "0.47"},
		{"marginBottom", "0.55"},
		{"marginLeft", "0.47"},
		{"marginRight", "0.47"},
	}
	for _, field := range fields {
		if err := w.WriteField(field[0], field[1]); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/forms/chromium/convert/html", &buf)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, mapDoError(err)
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

func mapDoError(err error) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("%w: %v", ErrTimeout, err)
	}
	return fmt.Errorf("%w: %v", ErrUnreachable, err)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}
