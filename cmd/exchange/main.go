package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"stock-exchange-ws/internal/kafkabus"
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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the HUB
	hub := ws.NewHub()
	go hub.Run()
	log.Println("✓ Hub started")

	kafkaConfig := kafkabus.ConfigFromEnv()
	log.Printf("Kafka brokers: %v\n", kafkaConfig.Brokers)

	kafkaConsumers := kafkabus.NewConsumers(kafkaConfig, hub)
	kafkaConsumers.Start(ctx)
	defer func() {
		if err := kafkaConsumers.Close(); err != nil {
			log.Printf("Kafka consumers close error: %v\n", err)
		}
	}()
	log.Println("✓ Kafka consumers started")

	// Initialize mock services for testing
	orderService := kafkabus.NewOrderProducer(kafkaConfig)
	defer func() {
		if err := orderService.Close(); err != nil {
			log.Printf("Kafka order producer close error: %v\n", err)
		}
	}()
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

	// Start HTTP server
	listenAddr := ":8080"
	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting WebSocket server on ws://localhost:8080/ws\n\n")
		log.Printf("Kafka topics:\n")
		log.Printf("  %s\n", kafkaConfig.Topics.PriceUpdates)
		log.Printf("  %s\n", kafkaConfig.Topics.OrderUpdates)
		log.Printf("  %s\n", kafkaConfig.Topics.OrderBookUpdates)
		log.Printf("  %s\n", kafkaConfig.Topics.MarketEvents)
		log.Printf("  %s\n", kafkaConfig.Topics.OrderCommands)
		log.Printf("  %s\n\n", kafkaConfig.Topics.OrderCommandResults)
		log.Printf("Test credentials: api_key=test-api-key, api_secret=test-api-secret\n\n")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	// shutdown
	<-ctx.Done()

	log.Println("\n\n Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v\n", err)
	}
	log.Println("✓ Server stopped")
}
