package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"game-admin/backend/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitMQAMQPDialer func(string) (amqpConnection, error)

type amqpConnection interface {
	Channel() (amqpChannel, error)
	Close() error
	IsClosed() bool
}

type amqpChannel interface {
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Get(queue string, autoAck bool) (amqp.Delivery, bool, error)
	Close() error
	IsClosed() bool
}

type RabbitMQAMQPTransport struct {
	mu             sync.Mutex
	cfg            rabbitMQConfig
	dial           rabbitMQAMQPDialer
	conn           amqpConnection
	channel        amqpChannel
	declaredQueues map[string]struct{}
}

type realAMQPConnection struct {
	conn *amqp.Connection
}

type realAMQPChannel struct {
	channel *amqp.Channel
}

func NewRabbitMQAMQPTransport(cfg config.MQConfig) (*RabbitMQAMQPTransport, error) {
	return newRabbitMQAMQPTransportWithDialer(cfg, dialRabbitMQAMQP)
}

func newRabbitMQAMQPTransportWithDialer(cfg config.MQConfig, dial rabbitMQAMQPDialer) (*RabbitMQAMQPTransport, error) {
	rmq, err := newRabbitMQConfig(cfg)
	if err != nil {
		return nil, err
	}
	if dial == nil {
		dial = dialRabbitMQAMQP
	}
	transport := &RabbitMQAMQPTransport{
		cfg:            rmq,
		dial:           dial,
		declaredQueues: map[string]struct{}{},
	}
	if err := transport.ensureConnectedLocked(); err != nil {
		return nil, err
	}
	return transport, nil
}

func (t *RabbitMQAMQPTransport) Publish(ctx context.Context, consumer string, delivery Delivery) error {
	if t == nil {
		return errors.New("rabbitmq amqp transport is nil")
	}
	queue := queueName(t.cfg, consumer)
	body, err := json.Marshal(delivery)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.ensureConnectedLocked(); err != nil {
		return err
	}
	if err := t.ensureQueueLocked(queue); err != nil {
		return err
	}
	return t.channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (t *RabbitMQAMQPTransport) Consume(_ context.Context, consumer string, count int) ([]Delivery, error) {
	if t == nil {
		return nil, errors.New("rabbitmq amqp transport is nil")
	}
	if count <= 0 {
		count = 10
	}
	queue := queueName(t.cfg, consumer)

	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.ensureConnectedLocked(); err != nil {
		return nil, err
	}
	if err := t.ensureQueueLocked(queue); err != nil {
		return nil, err
	}

	deliveries := make([]Delivery, 0, count)
	for i := 0; i < count; i++ {
		msg, ok, err := t.channel.Get(queue, true)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		var delivery Delivery
		if err := json.Unmarshal(msg.Body, &delivery); err != nil {
			return nil, fmt.Errorf("decode rabbitmq delivery: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, nil
}

func (t *RabbitMQAMQPTransport) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	var closeErr error
	if t.channel != nil && !t.channel.IsClosed() {
		if err := t.channel.Close(); err != nil {
			closeErr = err
		}
	}
	if t.conn != nil && !t.conn.IsClosed() {
		if err := t.conn.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	t.channel = nil
	t.conn = nil
	t.declaredQueues = map[string]struct{}{}
	return closeErr
}

func (t *RabbitMQAMQPTransport) ensureConnectedLocked() error {
	if t.conn != nil && !t.conn.IsClosed() && t.channel != nil && !t.channel.IsClosed() {
		return nil
	}
	if t.channel != nil && !t.channel.IsClosed() {
		_ = t.channel.Close()
	}
	if t.conn != nil && !t.conn.IsClosed() {
		_ = t.conn.Close()
	}
	conn, err := t.dial(t.cfg.url)
	if err != nil {
		return err
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}
	t.conn = conn
	t.channel = channel
	t.declaredQueues = map[string]struct{}{}
	return nil
}

func (t *RabbitMQAMQPTransport) ensureQueueLocked(queue string) error {
	if _, ok := t.declaredQueues[queue]; ok {
		return nil
	}
	if _, err := t.channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}
	t.declaredQueues[queue] = struct{}{}
	return nil
}

func dialRabbitMQAMQP(rawURL string) (amqpConnection, error) {
	conn, err := amqp.Dial(rawURL)
	if err != nil {
		return nil, err
	}
	return realAMQPConnection{conn: conn}, nil
}

func (c realAMQPConnection) Channel() (amqpChannel, error) {
	channel, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}
	return realAMQPChannel{channel: channel}, nil
}

func (c realAMQPConnection) Close() error {
	return c.conn.Close()
}

func (c realAMQPConnection) IsClosed() bool {
	return c.conn.IsClosed()
}

func (c realAMQPChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	return c.channel.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
}

func (c realAMQPChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	return c.channel.PublishWithContext(ctx, exchange, key, mandatory, immediate, msg)
}

func (c realAMQPChannel) Get(queue string, autoAck bool) (amqp.Delivery, bool, error) {
	return c.channel.Get(queue, autoAck)
}

func (c realAMQPChannel) Close() error {
	return c.channel.Close()
}

func (c realAMQPChannel) IsClosed() bool {
	return c.channel.IsClosed()
}
