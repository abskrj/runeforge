package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Get(ctx context.Context, authHeader string, path string, out any) error {
	return c.do(ctx, http.MethodGet, authHeader, path, nil, out)
}

func (c *Client) Post(ctx context.Context, authHeader string, path string, body any, out any) error {
	return c.do(ctx, http.MethodPost, authHeader, path, body, out)
}

func (c *Client) PostWithHeaders(ctx context.Context, authHeader string, headers map[string]string, path string, body any, out any) error {
	if authHeader == "" {
		return fmt.Errorf("authorization header is required")
	}

	fullURL := c.baseURL + path
	var payload io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		payload = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, payload)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		if v == "" {
			continue
		}
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("control-plane error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}
	if out == nil || len(respBytes) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, authHeader, path string, body any, out any) error {
	if authHeader == "" {
		return fmt.Errorf("authorization header is required")
	}

	fullURL := c.baseURL + path
	var payload io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		payload = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, payload)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("control-plane error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	if out == nil || len(respBytes) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBytes, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func Query(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	q := url.Values{}
	for k, v := range values {
		if v == "" {
			continue
		}
		q.Set(k, v)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}
