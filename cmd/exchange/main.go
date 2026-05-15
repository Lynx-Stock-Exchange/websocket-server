package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	kafka "github.com/segmentio/kafka-go"
)

const activePlatformsPollInterval = 3 * time.Second

// Market time provider
type MarketTimeProvider struct{}

func (m *MarketTimeProvider) ServerMarketTime() string {
	return time.Now().Format(time.RFC3339)
}

// kafkaRejection implements the rejectionCoder interface used by orders.go.
type kafkaRejection struct {
	code    string
	message string
}

func (e *kafkaRejection) Error() string         { return e.message }
func (e *kafkaRejection) RejectionCode() string { return e.code }

// OrderService publishes incoming PLACE_ORDER requests to the orders.requests
// Kafka topic so the order-book-engine can consume and persist them.
// It returns an ORDER_ACK immediately with the broker's own order_id; the
// actual fill/partial-fill updates arrive later via the orders.updates topic.
type OrderService struct {
	writer *kafka.Writer
}

func newOrderService(brokers []string) *OrderService {
	return &OrderService{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    "orders.requests",
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (s *OrderService) PlaceOrder(ctx context.Context, req services.PlaceOrderRequest) (services.PlaceOrderResponse, error) {
	payload := map[string]any{
		"order_id":         req.OrderID,
		"platform_id":      req.PlatformID,
		"platform_user_id": req.PlatformUserID,
		"instrument_type":  req.InstrumentType,
		"instrument_id":    req.InstrumentID,
		"order_type":       req.OrderType,
		"side":             req.Side,
		"quantity":         req.Quantity,
	}
	if req.LimitPrice != nil {
		payload["limit_price"] = *req.LimitPrice
	}
	if req.ExpiresAt != nil {
		payload["expires_at"] = *req.ExpiresAt
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return services.PlaceOrderResponse{}, fmt.Errorf("failed to marshal order: %w", err)
	}

	if err := s.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(req.OrderID),
		Value: body,
	}); err != nil {
		return services.PlaceOrderResponse{}, &kafkaRejection{
			code:    "KAFKA_UNAVAILABLE",
			message: fmt.Sprintf("failed to publish order: %v", err),
		}
	}

	log.Printf("Order %s published to orders.requests (platform=%s instrument=%s side=%s qty=%d)",
		req.OrderID, req.PlatformID, req.InstrumentID, req.Side, req.Quantity)

	return services.PlaceOrderResponse{
		OrderID: req.OrderID,
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

	brokers := kafkaBrokers()

	// Start the HUB
	hub := ws.NewHub()
	go hub.Run()
	log.Println("✓ Hub started")

	// Start Kafka consumers
	consumer := kafkaconsumer.New(hub, kafkaconsumer.Config{
		Brokers: brokers,
	})
	consumer.Start(ctx)
	log.Printf("✓ Kafka consumers started (brokers: %s)\n", strings.Join(brokers, ","))

	// Initialize services
	orderService := newOrderService(brokers)
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
				return true
			},
		},
		SendBufferSize: 256,
	})

	// Setup HTTP routes (WebSocket only)
	mux := http.NewServeMux()
	mux.Handle("/ws", handler)

	listenAddr := ":8080"
	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting WebSocket server on ws://localhost:8080/ws\n\n")
		log.Printf("Kafka topics:\n")
		log.Printf("  orders.requests  (producer)\n")
		log.Printf("  orders.updates   (consumer)\n")
		log.Printf("  stock.prices     (consumer)\n")
		log.Printf("  orders.volumes   (consumer)\n")
		log.Printf("  market.events    (consumer)\n")
		log.Printf("  market.ticks     (consumer)\n\n")
		log.Printf("Platform auth verify endpoint: %s/internal/platforms/verify\n\n", platformAPIURL())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n\n Shutting down server...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v\n", err)
	}
	_ = orderService.writer.Close()
	log.Println("✓ Server stopped")
}
