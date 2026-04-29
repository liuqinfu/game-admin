package eventbus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"game-admin/backend/internal/config"
)

type RabbitMQManagementTransport struct {
	client *http.Client
	cfg    rabbitMQConfig
}

type rabbitMQConfig struct {
	url           string
	managementURL string
	username      string
	password      string
	vhost         string
	exchange      string
	queuePrefix   string
}

type rabbitMQMessage struct {
	Payload string `json:"payload"`
}

type rabbitMQGetResponse struct {
	PayloadBytes int    `json:"payload_bytes"`
	Redelivered  bool   `json:"redelivered"`
	RoutingKey   string `json:"routing_key"`
	Payload      string `json:"payload"`
}

func newRabbitMQConfig(cfg config.MQConfig) (rabbitMQConfig, error) {
	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		return rabbitMQConfig{}, errors.New("rabbitmq url is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rabbitMQConfig{}, fmt.Errorf("parse rabbitmq url: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return rabbitMQConfig{}, errors.New("rabbitmq host is required")
	}
	username := "guest"
	password := "guest"
	if parsed.User != nil {
		if value := parsed.User.Username(); strings.TrimSpace(value) != "" {
			username = value
		}
		if value, ok := parsed.User.Password(); ok && strings.TrimSpace(value) != "" && value != "***" {
			password = value
		}
	}
	vhost := strings.Trim(path.Clean(parsed.Path), "/")
	if vhost == "." || vhost == "" {
		vhost = "/"
	}
	managementURL := fmt.Sprintf("http://%s:15672", host)
	return rabbitMQConfig{
		url:           rawURL,
		managementURL: managementURL,
		username:      username,
		password:      password,
		vhost:         vhost,
		exchange:      "amq.default",
		queuePrefix:   firstNonEmpty(strings.TrimSpace(cfg.Topic), TopicDomain),
	}, nil
}

func queueName(rmq rabbitMQConfig, consumer string) string {
	return fmt.Sprintf("%s.%s", rmq.queuePrefix, strings.TrimSpace(consumer))
}

func NewRabbitMQManagementTransport(cfg config.MQConfig, options TransportOptions) (*RabbitMQManagementTransport, error) {
	rmq, err := newRabbitMQConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &RabbitMQManagementTransport{
		client: defaultTransportHTTPClient(options.HTTPClient),
		cfg:    rmq,
	}, nil
}

func (t *RabbitMQManagementTransport) Publish(ctx context.Context, consumer string, delivery Delivery) error {
	if t == nil {
		return errors.New("rabbitmq transport is nil")
	}
	return publishToRabbitMQWithConfig(ctx, t.client, t.cfg, consumer, delivery)
}

func (t *RabbitMQManagementTransport) Consume(ctx context.Context, consumer string, count int) ([]Delivery, error) {
	if t == nil {
		return nil, errors.New("rabbitmq transport is nil")
	}
	return consumeFromRabbitMQWithConfig(ctx, t.client, t.cfg, consumer, count)
}

func (t *RabbitMQManagementTransport) Close() error {
	return nil
}

func PublishToRabbitMQ(ctx context.Context, client *http.Client, cfg config.MQConfig, consumer string, delivery Delivery) error {
	transport, err := NewRabbitMQManagementTransport(cfg, TransportOptions{HTTPClient: client})
	if err != nil {
		return err
	}
	return transport.Publish(ctx, consumer, delivery)
}

func publishToRabbitMQWithConfig(ctx context.Context, client *http.Client, rmq rabbitMQConfig, consumer string, delivery Delivery) error {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	queue := queueName(rmq, consumer)
	if err := ensureRabbitMQQueue(ctx, client, rmq, queue); err != nil {
		return err
	}
	body, err := json.Marshal(delivery)
	if err != nil {
		return err
	}
	requestPayload := map[string]any{
		"properties":       map[string]any{"content_type": "application/json"},
		"routing_key":      queue,
		"payload":          string(body),
		"payload_encoding": "string",
	}
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return err
	}
	endpoint := rmq.managementURL + "/api/exchanges/" + url.PathEscape(rmq.vhost) + "/" + url.PathEscape(rmq.exchange) + "/publish"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	req.SetBasicAuth(rmq.username, rmq.password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rabbitmq publish failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

func ConsumeFromRabbitMQ(ctx context.Context, client *http.Client, cfg config.MQConfig, consumer string, count int) ([]Delivery, error) {
	transport, err := NewRabbitMQManagementTransport(cfg, TransportOptions{HTTPClient: client})
	if err != nil {
		return nil, err
	}
	return transport.Consume(ctx, consumer, count)
}

func consumeFromRabbitMQWithConfig(ctx context.Context, client *http.Client, rmq rabbitMQConfig, consumer string, count int) ([]Delivery, error) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	queue := queueName(rmq, consumer)
	if err := ensureRabbitMQQueue(ctx, client, rmq, queue); err != nil {
		return nil, err
	}
	if count <= 0 {
		count = 10
	}
	requestPayload := map[string]any{
		"count":    count,
		"ackmode":  "ack_requeue_false",
		"encoding": "auto",
		"truncate": 500000,
	}
	requestBody, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}
	endpoint := rmq.managementURL + "/api/queues/" + url.PathEscape(rmq.vhost) + "/" + url.PathEscape(queue) + "/get"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(rmq.username, rmq.password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rabbitmq consume failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var messages []rabbitMQGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}
	deliveries := make([]Delivery, 0, len(messages))
	for _, message := range messages {
		var delivery Delivery
		if err := json.Unmarshal([]byte(message.Payload), &delivery); err != nil {
			return nil, fmt.Errorf("decode rabbitmq delivery: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, nil
}

func ensureRabbitMQQueue(ctx context.Context, client *http.Client, rmq rabbitMQConfig, queue string) error {
	endpoint := rmq.managementURL + "/api/queues/" + url.PathEscape(rmq.vhost) + "/" + url.PathEscape(queue)
	requestBody, err := json.Marshal(map[string]any{
		"auto_delete": false,
		"durable":     true,
		"arguments":   map[string]any{},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	req.SetBasicAuth(rmq.username, rmq.password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 299 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ensure rabbitmq queue failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
