package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.com/devtrack3_cloud/devtrack_contract"
)

// CLIClient sends HTTP requests to devtrack-server.
type CLIClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewCLIClient(baseURL, token string) *CLIClient {
	return &CLIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CLIClient) Health() (*contract.HealthResponse, error) {
	var resp contract.HealthResponse
	if err := c.get(contract.RouteHealth, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CLIClient) Status() (*contract.StatusResponse, error) {
	var resp contract.StatusResponse
	if err := c.get(contract.RouteStatus, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CLIClient) Logs(tail int) (*contract.LogsResponse, error) {
	url := c.baseURL + contract.RouteLogs
	if tail > 0 {
		url += fmt.Sprintf("?tail=%d", tail)
	}
	var resp contract.LogsResponse
	if err := c.getURL(url, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CLIClient) Start() (*contract.CommandResponse, error) {
	return c.post(contract.RouteStart)
}

func (c *CLIClient) Stop() (*contract.CommandResponse, error) {
	return c.post(contract.RouteStop)
}

func (c *CLIClient) Pause() (*contract.CommandResponse, error) {
	return c.post(contract.RoutePause)
}

func (c *CLIClient) Resume() (*contract.CommandResponse, error) {
	return c.post(contract.RouteResume)
}

func (c *CLIClient) ForceTrigger() (*contract.CommandResponse, error) {
	return c.post(contract.RouteForceTrigger)
}

func (c *CLIClient) Version() (*contract.VersionResponse, error) {
	var resp contract.VersionResponse
	if err := c.get(contract.RouteVersion, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// get performs GET <baseURL><route> and JSON-decodes the response into out.
func (c *CLIClient) get(route string, out any) error {
	return c.getURL(c.baseURL+route, out)
}

func (c *CLIClient) getURL(url string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set(contract.AuthHeader, c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var e contract.ErrorResponse
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return json.Unmarshal(body, out)
}

// post performs POST <baseURL><route> and JSON-decodes a CommandResponse.
func (c *CLIClient) post(route string) (*contract.CommandResponse, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+route, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set(contract.AuthHeader, c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var e contract.ErrorResponse
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return nil, fmt.Errorf("%s", e.Error)
		}
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var out contract.CommandResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
