package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

const (
	DefaultOrderRequestsTopic  = "orders.requests"
	DefaultOrderResponsesTopic = "orders.responses"
	DefaultOrderReplyTimeout   = 5 * time.Second
	defaultOrderReplyGroup     = "websocket-order-replies"
)

type kafkaMessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type kafkaMessageReader interface {
	ReadMessage(ctx context.Context) (kafka.Message, error)
	Close() error
}

type KafkaOrderConfig struct {
	Brokers        []string
	RequestsTopic  string
	ResponsesTopic string
	ResponseGroup  string
	ReplyTimeout   time.Duration
}

type KafkaOrderService struct {
	writer        kafkaMessageWriter
	reader        kafkaMessageReader
	requestsTopic string
	replyTimeout  time.Duration

	mu      sync.Mutex
	pending map[string]chan orderResponse
}

type orderRequestMessage struct {
	PlatformID     string   `json:"platform_id"`
	PlatformUserID string   `json:"platform_user_id"`
	InstrumentType string   `json:"instrument_type"`
	InstrumentID   string   `json:"instrument_id"`
	OrderType      string   `json:"order_type"`
	Side           string   `json:"side"`
	Quantity       int64    `json:"quantity"`
	LimitPrice     *float64 `json:"limit_price,omitempty"`
	ExpiresAt      *string  `json:"expires_at,omitempty"`
}

type orderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type OrderPlacementError struct {
	Code    string
	Message string
	Cause   error
}

func (e OrderPlacementError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "order placement failed"
}

func (e OrderPlacementError) Unwrap() error {
	return e.Cause
}

func (e OrderPlacementError) RejectionCode() string {
	if e.Code != "" {
		return e.Code
	}
	return "ORDER_REJECTED"
}

func NewKafkaOrderService(config KafkaOrderConfig) *KafkaOrderService {
	requestsTopic := strings.TrimSpace(config.RequestsTopic)
	if requestsTopic == "" {
		requestsTopic = DefaultOrderRequestsTopic
	}
	responsesTopic := strings.TrimSpace(config.ResponsesTopic)
	if responsesTopic == "" {
		responsesTopic = DefaultOrderResponsesTopic
	}
	responseGroup := strings.TrimSpace(config.ResponseGroup)
	if responseGroup == "" {
		responseGroup = defaultOrderReplyGroup
	}
	replyTimeout := config.ReplyTimeout
	if replyTimeout <= 0 {
		replyTimeout = DefaultOrderReplyTimeout
	}

	return newKafkaOrderService(
		&kafka.Writer{
			Addr:     kafka.TCP(config.Brokers...),
			Topic:    requestsTopic,
			Balancer: &kafka.Hash{},
		},
		kafka.NewReader(kafka.ReaderConfig{
			Brokers: config.Brokers,
			Topic:   responsesTopic,
			GroupID: responseGroup,
		}),
		requestsTopic,
		replyTimeout,
	)
}

func newKafkaOrderService(writer kafkaMessageWriter, reader kafkaMessageReader, requestsTopic string, replyTimeout time.Duration) *KafkaOrderService {
	if strings.TrimSpace(requestsTopic) == "" {
		requestsTopic = DefaultOrderRequestsTopic
	}
	if replyTimeout <= 0 {
		replyTimeout = DefaultOrderReplyTimeout
	}

	return &KafkaOrderService{
		writer:        writer,
		reader:        reader,
		requestsTopic: requestsTopic,
		replyTimeout:  replyTimeout,
		pending:       make(map[string]chan orderResponse),
	}
}

func (s *KafkaOrderService) Start(ctx context.Context) {
	if s.reader == nil {
		return
	}

	go s.consumeResponses(ctx)
}

func (s *KafkaOrderService) Close() error {
	var closeErr error
	if s.reader != nil {
		closeErr = s.reader.Close()
	}
	if s.writer != nil {
		if err := s.writer.Close(); closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (s *KafkaOrderService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (PlaceOrderResponse, error) {
	if s.writer == nil {
		return PlaceOrderResponse{}, serviceUnavailable("order request producer is not configured", nil)
	}

	correlationID, err := newCorrelationID()
	if err != nil {
		return PlaceOrderResponse{}, serviceUnavailable("failed to generate order correlation id", err)
	}

	responseCh := make(chan orderResponse, 1)
	s.registerPending(correlationID, responseCh)
	defer s.unregisterPending(correlationID)

	messageBody, err := json.Marshal(orderRequestMessage{
		PlatformID:     req.PlatformID,
		PlatformUserID: req.PlatformUserID,
		InstrumentType: req.InstrumentType,
		InstrumentID:   req.InstrumentID,
		OrderType:      req.OrderType,
		Side:           req.Side,
		Quantity:       req.Quantity,
		LimitPrice:     req.LimitPrice,
		ExpiresAt:      req.ExpiresAt,
	})
	if err != nil {
		return PlaceOrderResponse{}, serviceUnavailable("failed to encode order request", err)
	}

	if err := s.writer.WriteMessages(ctx, kafka.Message{
		Topic: s.requestsTopic,
		Key:   []byte(correlationID),
		Value: messageBody,
	}); err != nil {
		return PlaceOrderResponse{}, serviceUnavailable("failed to publish order request", err)
	}

	timer := time.NewTimer(s.replyTimeout)
	defer timer.Stop()

	select {
	case response := <-responseCh:
		return placeOrderResponseFromKafka(response)
	case <-timer.C:
		return PlaceOrderResponse{}, serviceUnavailable("order engine did not reply before timeout", nil)
	case <-ctx.Done():
		return PlaceOrderResponse{}, serviceUnavailable("order placement cancelled", ctx.Err())
	}
}

func (s *KafkaOrderService) consumeResponses(ctx context.Context) {
	for {
		msg, err := s.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("orders.responses: read error: %v", err)
			continue
		}

		s.handleOrderResponse(msg.Key, msg.Value)
	}
}

func (s *KafkaOrderService) handleOrderResponse(key []byte, data []byte) {
	correlationID := strings.TrimSpace(string(key))
	if correlationID == "" {
		log.Printf("orders.responses: missing Kafka key, skipping")
		return
	}

	var response orderResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("orders.responses: invalid message: %v", err)
		return
	}

	responseCh := s.takePending(correlationID)
	if responseCh == nil {
		log.Printf("orders.responses: no pending websocket request for correlation id %s", correlationID)
		return
	}

	responseCh <- response
}

func (s *KafkaOrderService) registerPending(correlationID string, responseCh chan orderResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[correlationID] = responseCh
}

func (s *KafkaOrderService) unregisterPending(correlationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, correlationID)
}

func (s *KafkaOrderService) takePending(correlationID string) chan orderResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	responseCh := s.pending[correlationID]
	delete(s.pending, correlationID)
	return responseCh
}

func placeOrderResponseFromKafka(response orderResponse) (PlaceOrderResponse, error) {
	if response.Code != "" {
		message := response.Message
		if message == "" {
			message = "order rejected"
		}
		return PlaceOrderResponse{}, OrderPlacementError{
			Code:    response.Code,
			Message: message,
		}
	}

	status := strings.TrimSpace(response.Status)
	if status == "" {
		status = "PENDING"
	}

	return PlaceOrderResponse{
		OrderID: response.OrderID,
		Status:  status,
	}, nil
}

func serviceUnavailable(message string, cause error) OrderPlacementError {
	if cause != nil {
		message = fmt.Sprintf("%s: %v", message, cause)
	}
	return OrderPlacementError{
		Code:    "ORDER_SERVICE_UNAVAILABLE",
		Message: message,
		Cause:   cause,
	}
}

func newCorrelationID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}

	return "ws-order-" + hex.EncodeToString(bytes[:]), nil
}
