package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	auth "stock-exchange-ws/internal"
	"stock-exchange-ws/internal/kafkaconsumer"
	"stock-exchange-ws/internal/services"
	"stock-exchange-ws/internal/ws"

	"github.com/gorilla/websocket"
)

const activePlatformsPollInterval = 3 * time.Second

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

func kafkaBrokers() []string {
	env := os.Getenv("KAFKA_BROKERS")
	if env == "" {
		env = "localhost:9092"
	}
	return strings.Split(env, ",")
}

func platformAPIURL() string {
	env := os.Getenv("PLATFORM_API_URL")
	if env == "" {
		env = "http://localhost:8000"
	}
	return env
}

func syncActivePlatforms(ctx context.Context, authenticator *auth.Authenticator, hub *ws.Hub) {
	ticker := time.NewTicker(activePlatformsPollInterval)
	defer ticker.Stop()

	for {
		activePlatformIDs, err := authenticator.ActivePlatformIDs(ctx)
		if err != nil {
			log.Printf("active platforms sync failed: %v", err)
		} else {
			hub.SyncActivePlatforms(activePlatformIDs)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the HUB
	hub := ws.NewHub()
	go hub.Run()
	log.Println("✓ Hub started")

	// Start Kafka consumers
	consumer := kafkaconsumer.New(hub, kafkaconsumer.Config{
		Brokers: kafkaBrokers(),
	})
	consumer.Start(ctx)
	log.Printf("✓ Kafka consumers started (brokers: %s)\n", strings.Join(kafkaBrokers(), ","))

	// Initialize services
	orderService := &OrderService{}
	authenticator := auth.New(platformAPIURL())
	marketTimeProvider := &MarketTimeProvider{}
	go syncActivePlatforms(ctx, authenticator, hub)

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

	// Setup HTTP routes (WebSocket only)
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
		log.Printf("  stock.prices\n")
		log.Printf("  orders.updates\n")
		log.Printf("  orders.volumes\n")
		log.Printf("  market.events\n")
		log.Printf("  market.ticks\n\n")
		log.Printf("Platform auth verify endpoint: %s/internal/platforms/verify\n\n", platformAPIURL())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	// shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n\n Shutting down server...")

	cancel() // stops Kafka consumers

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v\n", err)
	}
	log.Println("✓ Server stopped")
}
