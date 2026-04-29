package eventbus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"game-admin/backend/internal/config"
	"game-admin/backend/internal/domain/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRabbitMQTransportPublishAndConsume(t *testing.T) {
	t.Parallel()

	type queueState struct {
		items []string
	}
	queues := map[string]*queueState{}
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodPut && strings.Contains(req.URL.Path, "/api/queues/"):
				queue := pathQueueName(req.URL.Path)
				if _, ok := queues[queue]; !ok {
					queues[queue] = &queueState{}
				}
				return jsonResponse(http.StatusCreated, map[string]any{})
			case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/publish"):
				var payload struct {
					RoutingKey string `json:"routing_key"`
					Payload    string `json:"payload"`
				}
				require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
				if _, ok := queues[payload.RoutingKey]; !ok {
					queues[payload.RoutingKey] = &queueState{}
				}
				queues[payload.RoutingKey].items = append(queues[payload.RoutingKey].items, payload.Payload)
				return jsonResponse(http.StatusOK, map[string]any{"routed": true})
			case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/get"):
				queue := pathQueueName(req.URL.Path)
				state := queues[queue]
				if state == nil || len(state.items) == 0 {
					return jsonResponse(http.StatusOK, []map[string]any{})
				}
				items := state.items
				state.items = nil
				response := make([]map[string]any, 0, len(items))
				for _, item := range items {
					response = append(response, map[string]any{"payload": item})
				}
				return jsonResponse(http.StatusOK, response)
			default:
				return jsonResponse(http.StatusNotFound, map[string]any{"error": "not found"})
			}
		}),
	}

	db := openRabbitMQTestDB(t)
	event, err := Publish(db, PublishInput{
		EventType:      EventRechargeOrderPaid,
		AggregateType:  "recharge_order",
		AggregateID:    "1001",
		OccurredAt:     time.Now().UTC(),
		IdempotencyKey: "mq-evt-1001",
		Consumers:      []string{ConsumerNotification},
		Payload:        map[string]any{"orderNo": "R-1001"},
	})
	require.NoError(t, err)

	deliveries, err := ClaimPendingDeliveries(db, ConsumerNotification, "publisher", 1, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, deliveries, 1)

	cfg := config.MQConfig{Enabled: true, Driver: "rabbitmq-management", URL: "amqp://guest:guest@localhost:5672/", Topic: "game-admin.events"}
	transport, err := NewTransport(cfg, TransportOptions{HTTPClient: client})
	require.NoError(t, err)
	rmqTransport, ok := transport.(*RabbitMQManagementTransport)
	require.True(t, ok)
	rmqTransport.cfg.managementURL = "http://rabbitmq.local"
	ctx := context.Background()
	require.NoError(t, transport.Publish(ctx, ConsumerNotification, deliveries[0]))
	require.NoError(t, MarkQueued(db, deliveries[0].ID, map[string]any{"stage": "queued"}, time.Now().UTC()))

	queued, err := transport.Consume(ctx, ConsumerNotification, 10)
	require.NoError(t, err)
	require.Len(t, queued, 1)
	require.Equal(t, event.ID, queued[0].Event.ID)

	var stored model.DomainEventDelivery
	require.NoError(t, db.First(&stored, deliveries[0].ID).Error)
	require.Equal(t, model.DomainEventDeliveryStatusQueued, stored.Status)
}

func TestNewTransportRejectsUnsupportedDriver(t *testing.T) {
	t.Parallel()

	transport, err := NewTransport(config.MQConfig{
		Enabled: true,
		Driver:  "kafka",
		URL:     "kafka://broker:9092",
	}, TransportOptions{})
	require.ErrorIs(t, err, ErrUnsupportedMQDriver)
	require.Nil(t, transport)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, payload any) (*http.Response, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(raw)),
	}, nil
}

func openRabbitMQTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rabbitmq.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}

func pathQueueName(rawPath string) string {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	index := len(parts) - 1
	if parts[index] == "get" || parts[index] == "publish" {
		index--
	}
	if index < 0 {
		return ""
	}
	value, _ := url.PathUnescape(parts[index])
	return value
}
