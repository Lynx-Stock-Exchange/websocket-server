package auth

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

type Authenticator struct {
	baseURL    string
	httpClient *http.Client
}

type verifyRequest struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

type verifyResponse struct {
	Valid      bool   `json:"valid"`
	PlatformID string `json:"platform_id"`
}

type activePlatformsResponse struct {
	Platforms []activePlatform `json:"platforms"`
}

type activePlatform struct {
	ID       any  `json:"id"`
	IsActive bool `json:"is_active"`
}

func New(baseURL string) *Authenticator {
	return &Authenticator{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (a *Authenticator) AuthenticatePlatform(ctx context.Context, credentials ws.PlatformCredentials) (ws.AuthenticatedPlatform, error) {
	if a.baseURL == "" {
		return ws.AuthenticatedPlatform{}, fmt.Errorf("platform auth base URL is not configured")
	}

	requestBody, err := json.Marshal(verifyRequest{
		APIKey:    credentials.APIKey,
		APISecret: credentials.APISecret,
	})
	if err != nil {
		return ws.AuthenticatedPlatform{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/internal/platforms/verify", bytes.NewReader(requestBody))
	if err != nil {
		return ws.AuthenticatedPlatform{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
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

func (a *Authenticator) ActivePlatformIDs(ctx context.Context) (map[string]struct{}, error) {
	if a.baseURL == "" {
		return nil, fmt.Errorf("platform auth base URL is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/internal/platforms/active", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("active platforms request failed with status %d", resp.StatusCode)
	}

	var active activePlatformsResponse
	if err := json.NewDecoder(resp.Body).Decode(&active); err != nil {
		return nil, err
	}

	platformIDs := make(map[string]struct{}, len(active.Platforms))
	for _, platform := range active.Platforms {
		if !platform.IsActive {
			continue
		}

		id := strings.TrimSpace(fmt.Sprint(platform.ID))
		if id == "" || id == "<nil>" {
			continue
		}
		platformIDs[id] = struct{}{}
	}

	return platformIDs, nil
}
