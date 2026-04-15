package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rainhu/ado/internal/config"
	"github.com/rainhu/ado/internal/logging"
)

type Client struct {
	cfg    *config.Config
	http   *http.Client
}

func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{}}
}

func (c *Client) do(req *http.Request, result any) error {
	req.SetBasicAuth("", c.cfg.PAT)

	start := time.Now()
	log := logging.L()
	resp, err := c.http.Do(req)
	if err != nil {
		log.Error("http request error",
			"method", req.Method,
			"url", req.URL.String(),
			"elapsed_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
		)
		return err
	}
	defer resp.Body.Close()

	elapsedMS := time.Since(start).Milliseconds()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		log.Error("http non-2xx",
			"method", req.Method,
			"url", req.URL.String(),
			"status", resp.StatusCode,
			"elapsed_ms", elapsedMS,
		)
		return fmt.Errorf("API %d: %s", resp.StatusCode, string(body))
	}
	log.Info("http ok",
		"method", req.Method,
		"url", req.URL.String(),
		"status", resp.StatusCode,
		"elapsed_ms", elapsedMS,
	)

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

func (c *Client) get(url string, result any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	return c.do(req, result)
}

func (c *Client) patch(url string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json-patch+json")
	return c.do(req, result)
}

func (c *Client) post(url string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, result)
}

func (c *Client) put(url string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, result)
}

func (c *Client) patchJSON(url string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, result)
}

func (c *Client) BaseURL() string {
	return c.cfg.Org
}

func (c *Client) Project() string {
	return c.cfg.Project
}

func (c *Client) Config() *config.Config {
	return c.cfg
}
