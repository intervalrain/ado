package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rainhu/ado/internal/config"
)

type Client struct {
	cfg    *config.Config
	http   *http.Client
}

func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{}}
}

func (c *Client) get(url string, result any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("", c.cfg.PAT)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
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
