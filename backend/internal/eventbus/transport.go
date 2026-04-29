package eventbus

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"game-admin/backend/internal/config"
)

var ErrUnsupportedMQDriver = errors.New("unsupported mq driver")

type Transport interface {
	Publish(context.Context, string, Delivery) error
	Consume(context.Context, string, int) ([]Delivery, error)
	Close() error
}

type TransportOptions struct {
	HTTPClient *http.Client
}

func NewTransport(cfg config.MQConfig, options TransportOptions) (Transport, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	switch driver {
	case "", "rabbitmq":
		return NewRabbitMQAMQPTransport(cfg)
	case "rabbitmq-management":
		return NewRabbitMQManagementTransport(cfg, options)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedMQDriver, cfg.Driver)
	}
}

func defaultTransportHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 5 * time.Second}
}
