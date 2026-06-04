package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
	"time"
)

type Client struct {
	BaseURL    string
	HostHeader string
	HTTP       *http.Client
}

func NewClient(baseURL string) Client {
	return Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func NewHostClient(baseURL, hostHeader string) Client {
	client := NewClient(baseURL)
	client.HostHeader = hostHeader
	return client
}

func (c Client) GetJSON(ctx context.Context, path string, bearer string, out any) (*http.Response, string, error) {
	return c.do(ctx, http.MethodGet, path, bearer, "", nil, out)
}

func (c Client) PostForm(ctx context.Context, path string, bearer string, form url.Values, out any) (*http.Response, string, error) {
	return c.do(ctx, http.MethodPost, path, bearer, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), out)
}

func (c Client) PatchForm(ctx context.Context, path string, bearer string, form url.Values, out any) (*http.Response, string, error) {
	return c.do(ctx, http.MethodPatch, path, bearer, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), out)
}

func (c Client) PostJSON(ctx context.Context, path string, bearer string, payload any, out any) (*http.Response, string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	return c.do(ctx, http.MethodPost, path, bearer, "application/json", bytes.NewReader(raw), out)
}

func (c Client) PostMultipart(ctx context.Context, path string, bearer string, fields map[string]string, fileField, fileName, contentType string, data []byte, out any) (*http.Response, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", err
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, fileField, fileName))
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(data); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return c.do(ctx, http.MethodPost, path, bearer, writer.FormDataContentType(), &body, out)
}

func (c Client) Delete(ctx context.Context, path string, bearer string, out any) (*http.Response, string, error) {
	return c.do(ctx, http.MethodDelete, path, bearer, "", nil, out)
}

func (c Client) do(ctx context.Context, method, path, bearer, contentType string, body io.Reader, out any) (*http.Response, string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, "", err
	}
	if c.HostHeader != "" {
		req.Host = c.HostHeader
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, "", err
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp, string(raw), err
		}
	}
	return resp, string(raw), nil
}

func Require2xx(t testing.TB, resp *http.Response, body string, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("expected 2xx, got %d: %s", resp.StatusCode, body)
	}
}

func WaitFor[T any](ctx context.Context, description string, interval time.Duration, fn func(context.Context) (T, bool, error)) T {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var zero T
	var lastErr error
	for {
		value, ok, err := fn(ctx)
		if err != nil {
			lastErr = err
		} else if ok {
			return value
		}
		select {
		case <-ctx.Done():
			panic(fmt.Sprintf("timed out waiting for %s: %v", description, lastErr))
		case <-ticker.C:
		}
		_ = zero
	}
}
