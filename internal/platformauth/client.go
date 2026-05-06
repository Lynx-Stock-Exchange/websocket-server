package platformauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stock-exchange-ws/internal/ws"
)

const defaultTimeout = 3 * time.Second

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Config struct {
	BaseURL    string
	HTTPClient *http.Client
}

type verifyRequest struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

type verifyResponse struct {
	Valid        bool   `json:"valid"`
	PlatformID   string `json:"platform_id"`
	PlatformName string `json:"platform_name"`
}

func New(config Config) *Client {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *Client) AuthenticatePlatform(ctx context.Context, credentials ws.PlatformCredentials) (ws.AuthenticatedPlatform, error) {
	if c.baseURL == "" {
		return ws.AuthenticatedPlatform{}, fmt.Errorf("platform auth base URL is not configured")
	}

	requestBody, err := json.Marshal(verifyRequest{
		APIKey:    credentials.APIKey,
		APISecret: credentials.APISecret,
	})
	if err != nil {
		return ws.AuthenticatedPlatform{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/platforms/verify", bytes.NewReader(requestBody))
	if err != nil {
		return ws.AuthenticatedPlatform{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ws.AuthenticatedPlatform{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ws.AuthenticatedPlatform{}, ws.ErrUnauthorized
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ws.AuthenticatedPlatform{}, fmt.Errorf("platform verify failed with status %d", resp.StatusCode)
	}

	var verify verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verify); err != nil {
		return ws.AuthenticatedPlatform{}, err
	}
	if !verify.Valid || verify.PlatformID == "" {
		return ws.AuthenticatedPlatform{}, ws.ErrUnauthorized
	}

	return ws.AuthenticatedPlatform{ID: verify.PlatformID}, nil
}
