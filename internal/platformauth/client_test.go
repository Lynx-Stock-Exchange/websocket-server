package platformauth

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
			Valid:        true,
			PlatformID:   "platform-1",
			PlatformName: "Broker One",
		})
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	platform, err := client.AuthenticatePlatform(context.Background(), ws.PlatformCredentials{
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

	client := New(Config{BaseURL: server.URL})
	_, err := client.AuthenticatePlatform(context.Background(), ws.PlatformCredentials{
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

	client := New(Config{BaseURL: server.URL})
	_, err := client.AuthenticatePlatform(context.Background(), ws.PlatformCredentials{
		APIKey:    "key-1",
		APISecret: "wrong-secret",
	})
	if !errors.Is(err, ws.ErrUnauthorized) {
		t.Fatalf("AuthenticatePlatform error = %v, want ErrUnauthorized", err)
	}
}
