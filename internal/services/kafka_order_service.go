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
	DefaultOrderResponsesTopic = "orders.updates"
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

	mu            sync.Mutex
	pending       map[string]pendingOrder
	pendingByUser map[string]string
}

type pendingOrder struct {
	responseCh chan orderResponse
	userKey    string
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
	PlatformID      string `json:"platform_id,omitempty"`
	PlatformUserID  string `json:"platform_user_id,omitempty"`
	OrderID         string `json:"order_id"`
	Status          string `json:"status,omitempty"`
	Code            string `json:"code,omitempty"`
	Message         string `json:"message,omitempty"`
	RejectionReason string `json:"rejection_reason,omitempty"`
}

type orderResponseEnvelope struct {
	Type    string        `json:"type"`
	Payload orderResponse `json:"payload"`
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
		pending:       make(map[string]pendingOrder),
		pendingByUser: make(map[string]string),
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
	if err := s.registerPending(correlationID, orderUserKey(req.PlatformID, req.PlatformUserID), responseCh); err != nil {
		return PlaceOrderResponse{}, err
	}
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
			log.Printf("%s: read error: %v", DefaultOrderResponsesTopic, err)
			continue
		}

		s.handleOrderResponse(msg.Key, msg.Value)
	}
}

func (s *KafkaOrderService) handleOrderResponse(key []byte, data []byte) {
	correlationID := strings.TrimSpace(string(key))
	response, err := parseOrderResponse(data)
	if err != nil {
		log.Printf("%s: invalid message for correlation id %s: %v", DefaultOrderResponsesTopic, correlationID, err)
		return
	}

	responseCh := s.takePending(correlationID)
	if responseCh == nil {
		responseCh = s.takePendingByUserKey(orderUserKey(response.PlatformID, response.PlatformUserID))
	}
	if responseCh == nil {
		log.Printf("%s: no pending websocket request for correlation id %s", DefaultOrderResponsesTopic, correlationID)
		return
	}

	responseCh <- response
}

func parseOrderResponse(data []byte) (orderResponse, error) {
	var envelope orderResponseEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && strings.TrimSpace(envelope.Type) != "" {
		return envelope.Payload, nil
	}

	var response orderResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return orderResponse{}, err
	}
	return response, nil
}

func (s *KafkaOrderService) registerPending(correlationID, userKey string, responseCh chan orderResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userKey != "" {
		if _, exists := s.pendingByUser[userKey]; exists {
			return OrderPlacementError{
				Code:    "ORDER_ALREADY_PENDING",
				Message: "another order is already pending for this platform user",
			}
		}
		s.pendingByUser[userKey] = correlationID
	}

	s.pending[correlationID] = pendingOrder{
		responseCh: responseCh,
		userKey:    userKey,
	}
	return nil
}

func (s *KafkaOrderService) unregisterPending(correlationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.takePendingLocked(correlationID)
}

func (s *KafkaOrderService) takePending(correlationID string) chan orderResponse {
	if strings.TrimSpace(correlationID) == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.takePendingLocked(correlationID)
}

func (s *KafkaOrderService) takePendingByUserKey(userKey string) chan orderResponse {
	if strings.TrimSpace(userKey) == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	correlationID := s.pendingByUser[userKey]
	return s.takePendingLocked(correlationID)
}

func (s *KafkaOrderService) takePendingLocked(correlationID string) chan orderResponse {
	pending, ok := s.pending[correlationID]
	if !ok {
		return nil
	}

	delete(s.pending, correlationID)
	if pending.userKey != "" {
		delete(s.pendingByUser, pending.userKey)
	}
	return pending.responseCh
}

func placeOrderResponseFromKafka(response orderResponse) (PlaceOrderResponse, error) {
	if response.Code != "" {
		message := response.Message
		if message == "" {
			message = rejectionMessage(response.Code)
		}
		return PlaceOrderResponse{}, OrderPlacementError{
			Code:    response.Code,
			Message: message,
		}
	}

	status := strings.TrimSpace(response.Status)
	if strings.EqualFold(status, "REJECTED") {
		code := strings.TrimSpace(response.RejectionReason)
		if code == "" {
			code = "ORDER_REJECTED"
		}
		message := response.Message
		if message == "" {
			message = rejectionMessage(code)
		}
		return PlaceOrderResponse{}, OrderPlacementError{
			Code:    code,
			Message: message,
		}
	}
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

func orderUserKey(platformID, platformUserID string) string {
	platformID = strings.TrimSpace(platformID)
	platformUserID = strings.TrimSpace(platformUserID)
	if platformID == "" || platformUserID == "" {
		return ""
	}
	return platformID + "\x00" + platformUserID
}

func rejectionMessage(code string) string {
	switch strings.TrimSpace(code) {
	case "MARKET_CLOSED":
		return "Market is currently closed."
	case "INVALID_SIDE":
		return "Order side is invalid."
	case "INVALID_QUANTITY":
		return "Order quantity is invalid."
	case "ORDER_SIZE_EXCEEDED":
		return "Order size exceeds the allowed limit."
	case "INVALID_TICKER":
		return "Instrument is not available."
	case "INVALID_ORDER_TYPE":
		return "Order type is invalid."
	case "INVALID_LIMIT_PRICE":
		return "Limit price is invalid."
	case "":
		return "Order rejected."
	default:
		return "Order rejected: " + code
	}
}
