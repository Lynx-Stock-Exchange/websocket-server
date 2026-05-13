package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"stock-exchange-ws/internal/ws"
)

func TestAuthenticatePlatformCallsVerifyEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/internal/platforms/verify" {
			t.Fatalf("path = %s, want /internal/platforms/verify", r.URL.Path)
		}

		var body verifyRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode request returned %v", err)
		}
		if body.APIKey != "key-1" || body.APISecret != "secret-1" {
			t.Fatalf("request body = %#v", body)
		}

		_ = json.NewEncoder(w).Encode(verifyResponse{
			Valid:      true,
			PlatformID: "platform-1",
		})
	}))
	defer server.Close()

	authenticator := New(server.URL)
	platform, err := authenticator.AuthenticatePlatform(context.Background(), ws.PlatformCredentials{
		APIKey:    "key-1",
		APISecret: "secret-1",
	})
	if err != nil {
		t.Fatalf("AuthenticatePlatform returned %v", err)
	}
	if platform.ID != "platform-1" {
		t.Fatalf("platform ID = %q, want platform-1", platform.ID)
	}
}

func TestAuthenticatePlatformRejectsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(verifyResponse{Valid: false})
	}))
	defer server.Close()

	authenticator := New(server.URL)
	_, err := authenticator.AuthenticatePlatform(context.Background(), ws.PlatformCredentials{
		APIKey:    "key-1",
		APISecret: "wrong-secret",
	})
	if !errors.Is(err, ws.ErrUnauthorized) {
		t.Fatalf("AuthenticatePlatform error = %v, want ErrUnauthorized", err)
	}
}

func TestAuthenticatePlatformRejectsUnauthorizedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	authenticator := New(server.URL)
	_, err := authenticator.AuthenticatePlatform(context.Background(), ws.PlatformCredentials{
		APIKey:    "key-1",
		APISecret: "wrong-secret",
	})
	if !errors.Is(err, ws.ErrUnauthorized) {
		t.Fatalf("AuthenticatePlatform error = %v, want ErrUnauthorized", err)
	}
}

func TestActivePlatformIDsReturnsOnlyActivePlatforms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/internal/platforms/active" {
			t.Fatalf("path = %s, want /internal/platforms/active", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(activePlatformsResponse{
			Platforms: []activePlatform{
				{ID: 101, IsActive: true},
				{ID: "platform-202", IsActive: true},
				{ID: 303, IsActive: false},
			},
		})
	}))
	defer server.Close()

	authenticator := New(server.URL)
	platformIDs, err := authenticator.ActivePlatformIDs(context.Background())
	if err != nil {
		t.Fatalf("ActivePlatformIDs returned %v", err)
	}

	if _, ok := platformIDs["101"]; !ok {
		t.Fatalf("expected numeric platform ID to be present")
	}
	if _, ok := platformIDs["platform-202"]; !ok {
		t.Fatalf("expected string platform ID to be present")
	}
	if _, ok := platformIDs["303"]; ok {
		t.Fatalf("expected inactive platform ID to be excluded")
	}
}
