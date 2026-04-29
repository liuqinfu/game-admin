package eventbus

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"game-admin/backend/internal/config"
	"game-admin/backend/internal/domain/model"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRabbitMQAMQPTransportPublishAndConsume(t *testing.T) {
	t.Parallel()

	state := newFakeAMQPState()
	transport, err := newRabbitMQAMQPTransportWithDialer(config.MQConfig{
		Enabled: true,
		Driver:  "rabbitmq",
		URL:     "amqp://guest:guest@localhost:5672/",
		Topic:   "game-admin.events",
	}, state.dialer())
	require.NoError(t, err)
	defer func() { require.NoError(t, transport.Close()) }()

	db := openRabbitMQAMQPTestDB(t)
	_, err = Publish(db, PublishInput{
		EventType:      EventRechargeOrderPaid,
		AggregateType:  "recharge_order",
		AggregateID:    "1001",
		OccurredAt:     time.Now().UTC(),
		IdempotencyKey: "amqp-evt-1001",
		Consumers:      []string{ConsumerNotification},
		Payload:        map[string]any{"orderNo": "R-1001"},
	})
	require.NoError(t, err)

	deliveries, err := ClaimPendingDeliveries(db, ConsumerNotification, "publisher", 1, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, deliveries, 1)

	require.NoError(t, transport.Publish(context.Background(), ConsumerNotification, deliveries[0]))
	queued, err := transport.Consume(context.Background(), ConsumerNotification, 10)
	require.NoError(t, err)
	require.Len(t, queued, 1)
	require.Equal(t, deliveries[0].Event.ID, queued[0].Event.ID)
	require.Equal(t, 1, state.declareCount(queueName(transport.cfg, ConsumerNotification)))
}

func TestRabbitMQAMQPTransportReconnectsWhenChannelClosed(t *testing.T) {
	t.Parallel()

	state := newFakeAMQPState()
	transport, err := newRabbitMQAMQPTransportWithDialer(config.MQConfig{
		Enabled: true,
		Driver:  "rabbitmq",
		URL:     "amqp://guest:guest@localhost:5672/",
		Topic:   "game-admin.events",
	}, state.dialer())
	require.NoError(t, err)
	defer func() { require.NoError(t, transport.Close()) }()

	firstConn := state.lastConn()
	require.NotNil(t, firstConn)
	firstConn.channel.closed = true

	err = transport.Publish(context.Background(), ConsumerNotification, Delivery{
		DomainEventDelivery: model.DomainEventDelivery{BaseModel: model.BaseModel{ID: 1}},
		Event:               model.DomainEvent{BaseModel: model.BaseModel{ID: 11}, EventType: EventRechargeOrderPaid},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, state.dialCount(), 2)
}

type fakeAMQPState struct {
	connections []*fakeAMQPConnection
	queues      map[string][][]byte
	declares    map[string]int
}

type fakeAMQPConnection struct {
	state   *fakeAMQPState
	channel *fakeAMQPChannel
	closed  bool
}

type fakeAMQPChannel struct {
	state  *fakeAMQPState
	closed bool
}

func newFakeAMQPState() *fakeAMQPState {
	return &fakeAMQPState{
		queues:   map[string][][]byte{},
		declares: map[string]int{},
	}
}

func (s *fakeAMQPState) dialer() rabbitMQAMQPDialer {
	return func(_ string) (amqpConnection, error) {
		conn := &fakeAMQPConnection{state: s}
		conn.channel = &fakeAMQPChannel{state: s}
		s.connections = append(s.connections, conn)
		return conn, nil
	}
}

func (s *fakeAMQPState) dialCount() int {
	return len(s.connections)
}

func (s *fakeAMQPState) lastConn() *fakeAMQPConnection {
	if len(s.connections) == 0 {
		return nil
	}
	return s.connections[len(s.connections)-1]
}

func (s *fakeAMQPState) declareCount(queue string) int {
	return s.declares[queue]
}

func (c *fakeAMQPConnection) Channel() (amqpChannel, error) {
	return c.channel, nil
}

func (c *fakeAMQPConnection) Close() error {
	c.closed = true
	return nil
}

func (c *fakeAMQPConnection) IsClosed() bool {
	return c.closed
}

func (c *fakeAMQPChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	c.state.declares[name]++
	if _, ok := c.state.queues[name]; !ok {
		c.state.queues[name] = nil
	}
	return amqp.Queue{Name: name}, nil
}

func (c *fakeAMQPChannel) PublishWithContext(_ context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	c.state.queues[key] = append(c.state.queues[key], append([]byte(nil), msg.Body...))
	return nil
}

func (c *fakeAMQPChannel) Get(queue string, autoAck bool) (amqp.Delivery, bool, error) {
	items := c.state.queues[queue]
	if len(items) == 0 {
		return amqp.Delivery{}, false, nil
	}
	body := items[0]
	c.state.queues[queue] = items[1:]
	return amqp.Delivery{Body: body}, true, nil
}

func (c *fakeAMQPChannel) Close() error {
	c.closed = true
	return nil
}

func (c *fakeAMQPChannel) IsClosed() bool {
	return c.closed
}

func openRabbitMQAMQPTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rabbitmq-amqp.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
