package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stock-exchange-ws/internal/services"
	"stock-exchange-ws/internal/ws"

	"github.com/gorilla/websocket"
)

// Authenticator - accepts fake credentials
type Authenticator struct{}

// Accept fake credentials for testing
// will uptade one we have real authentication logic in place from admin panel
func (a *Authenticator) AuthenticatePlatform(ctx context.Context, creds ws.PlatformCredentials) (ws.AuthenticatedPlatform, error) {
	if creds.APIKey == "test-api-key" && creds.APISecret == "test-api-secret" {
		return ws.AuthenticatedPlatform{ID: "platform-xyz"}, nil
	}
	return ws.AuthenticatedPlatform{}, ws.ErrUnauthorized
}

// Market time provider
type MarketTimeProvider struct{}

func (m *MarketTimeProvider) ServerMarketTime() string {
	// this will change with further implementation of real market time logic
	return time.Now().Format(time.RFC3339)
}

// Order service fake data for testing
type OrderService struct{}

func (m *OrderService) PlaceOrder(ctx context.Context, req services.PlaceOrderRequest) (services.PlaceOrderResponse, error) {
	return services.PlaceOrderResponse{
		OrderID: "order-number",
		Status:  "PENDING",
	}, nil
}

func main() {
	// Start the HUB
	hub := ws.NewHub()
	go hub.Run()
	log.Println("✓ Hub started")

	// Initialize mock services for testing
	orderService := &OrderService{}
	authenticator := &Authenticator{}
	marketTimeProvider := &MarketTimeProvider{}

	// Create WebSocket handler
	handler := ws.NewHandler(ws.HandlerConfig{
		Hub:                hub,
		Authenticator:      authenticator,
		OrderService:       orderService,
		MarketTimeProvider: marketTimeProvider,
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for testing
			},
		},
		SendBufferSize: 256,
	})

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.Handle("/ws", handler)

	// Start price feed simulator - only test purpose, will be replaced with real data feed logic
	// go simulatePriceFeed(hub)

	// Start HTTP server
	listenAddr := ":8080"
	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting WebSocket server on ws://localhost:8080/ws\n\n")
		log.Printf("Test credentials: api_key=test-api-key, api_secret=test-api-secret\n\n")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	// shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n\n Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v\n", err)
	}
	log.Println("✓ Server stopped")
}
