package ws

import (
	"context"
	"errors"
	"net/http"
)

var (
	ErrMissingCredentials = errors.New("missing websocket credentials")
	ErrUnauthorized       = errors.New("unauthorized websocket credentials")
)

type PlatformCredentials struct {
	APIKey    string
	APISecret string
}

type AuthenticatedPlatform struct {
	ID string
}

type Authenticator interface {
	AuthenticatePlatform(ctx context.Context, credentials PlatformCredentials) (AuthenticatedPlatform, error)
}

func credentialsFromRequest(r *http.Request) (PlatformCredentials, error) {
	query := r.URL.Query()
	credentials := PlatformCredentials{
		APIKey:    query.Get("api_key"),
		APISecret: query.Get("api_secret"),
	}
	if credentials.APIKey == "" || credentials.APISecret == "" {
		return PlatformCredentials{}, ErrMissingCredentials
	}

	return credentials, nil
}
