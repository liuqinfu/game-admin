package eventbus

import (
	"encoding/json"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
)

const EnvelopeVersion = 1

type EventAggregate struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type EventEnvelope struct {
	EventType      string          `json:"eventType"`
	Aggregate      EventAggregate  `json:"aggregate"`
	Version        int             `json:"version"`
	Producer       string          `json:"producer,omitempty"`
	RequestID      string          `json:"requestID,omitempty"`
	TraceID        string          `json:"traceID,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	OccurredAt     time.Time       `json:"occurredAt"`
	TenantID       *uint64         `json:"tenantID,omitempty"`
	BrandID        *uint64         `json:"brandID,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	Meta           json.RawMessage `json:"meta,omitempty"`
}

func PayloadBeforeAfter(before, after any) map[string]any {
	return map[string]any{
		"before": before,
		"after":  after,
	}
}

func EnvelopeFromEvent(event model.DomainEvent) EventEnvelope {
	var envelope EventEnvelope
	if len(event.Payload) > 0 && json.Unmarshal(event.Payload, &envelope) == nil &&
		strings.TrimSpace(envelope.EventType) != "" &&
		strings.TrimSpace(envelope.Aggregate.Type) != "" &&
		strings.TrimSpace(envelope.Aggregate.ID) != "" {
		if envelope.Version <= 0 {
			envelope.Version = EnvelopeVersion
		}
		if envelope.OccurredAt.IsZero() {
			envelope.OccurredAt = event.OccurredAt
		}
		if envelope.TenantID == nil {
			envelope.TenantID = event.TenantID
		}
		if envelope.BrandID == nil {
			envelope.BrandID = event.BrandID
		}
		if strings.TrimSpace(envelope.Producer) == "" {
			envelope.Producer = event.Producer
		}
		if strings.TrimSpace(envelope.IdempotencyKey) == "" {
			envelope.IdempotencyKey = event.IdempotencyKey
		}
		return envelope
	}
	return EventEnvelope{
		EventType:      event.EventType,
		Aggregate:      EventAggregate{Type: event.AggregateType, ID: event.AggregateID},
		Version:        EnvelopeVersion,
		Producer:       event.Producer,
		IdempotencyKey: event.IdempotencyKey,
		OccurredAt:     event.OccurredAt,
		TenantID:       event.TenantID,
		BrandID:        event.BrandID,
		Payload:        json.RawMessage(append([]byte(nil), event.Payload...)),
		Meta:           json.RawMessage(append([]byte(nil), event.Meta...)),
	}
}

func DeliveryCorrelation(delivery Delivery) (string, string) {
	envelope := EnvelopeFromEvent(delivery.Event)
	requestID := strings.TrimSpace(envelope.RequestID)
	traceID := strings.TrimSpace(envelope.TraceID)
	if traceID == "" {
		traceID = requestID
	}
	return requestID, traceID
}
