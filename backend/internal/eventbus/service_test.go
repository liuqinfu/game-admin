package eventbus

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"game-admin/backend/internal/domain/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPublishClaimAndAckDelivery(t *testing.T) {
	t.Parallel()

	db := openEventBusTestDB(t)
	now := time.Now().UTC()

	event, err := Publish(db, PublishInput{
		EventType:      EventRechargeOrderPaid,
		AggregateType:  "recharge_order",
		AggregateID:    "1001",
		OccurredAt:     now,
		Producer:       "test",
		IdempotencyKey: "evt-1001",
		Consumers:      []string{ConsumerNotification, ConsumerDataPlatformSync},
		Payload:        map[string]any{"orderNo": "R-1001"},
	})
	require.NoError(t, err)
	require.NotZero(t, event.ID)

	duplicate, err := Publish(db, PublishInput{
		EventType:      EventRechargeOrderPaid,
		AggregateType:  "recharge_order",
		AggregateID:    "1001",
		OccurredAt:     now,
		IdempotencyKey: "evt-1001",
		Consumers:      []string{ConsumerNotification},
		Payload:        map[string]any{"orderNo": "R-1001"},
	})
	require.NoError(t, err)
	require.Equal(t, event.ID, duplicate.ID)

	deliveries, err := ClaimPendingDeliveries(db, ConsumerNotification, "notification-service", 10, now.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	require.Equal(t, EventRechargeOrderPaid, deliveries[0].Event.EventType)
	require.Equal(t, model.DomainEventDeliveryStatusProcessing, deliveries[0].Status)
	envelope := EnvelopeFromEvent(deliveries[0].Event)
	require.Equal(t, EventRechargeOrderPaid, envelope.EventType)
	require.Equal(t, "recharge_order", envelope.Aggregate.Type)
	require.Equal(t, "1001", envelope.Aggregate.ID)
	require.Equal(t, "evt-1001", envelope.IdempotencyKey)
	require.Equal(t, EnvelopeVersion, envelope.Version)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(envelope.Payload, &payload))
	require.Equal(t, "R-1001", payload["orderNo"])

	require.NoError(t, MarkDelivered(db, deliveries[0].ID, map[string]any{"status": "processed"}, now.Add(2*time.Second)))

	var stored model.DomainEventDelivery
	require.NoError(t, db.First(&stored, deliveries[0].ID).Error)
	require.Equal(t, model.DomainEventDeliveryStatusSucceeded, stored.Status)
	require.NotNil(t, stored.DeliveredAt)
}

func TestMarkFailedUsesBackoffAndDeadLettersAtMaxAttempts(t *testing.T) {
	t.Parallel()

	db := openEventBusTestDB(t)
	now := time.Now().UTC()

	_, err := Publish(db, PublishInput{
		EventType:      EventRechargeOrderPaid,
		AggregateType:  "recharge_order",
		AggregateID:    "2001",
		OccurredAt:     now,
		IdempotencyKey: "evt-2001",
		Consumers:      []string{ConsumerNotification},
		Payload:        map[string]any{"orderNo": "R-2001"},
	})
	require.NoError(t, err)

	deliveries, err := ClaimPendingDeliveriesWithCorrelation(db, ConsumerNotification, "notification-service", "req-1", "trace-1", 1, now.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, deliveries, 1)

	outcome, err := MarkFailed(db, deliveries[0].ID, errors.New("temporary"), 0, map[string]any{"stage": "consume"}, now.Add(2*time.Second))
	require.NoError(t, err)
	require.Equal(t, model.DomainEventDeliveryStatusFailed, outcome.Status)
	require.Equal(t, 5*time.Second, outcome.RetryDelay)

	var stored model.DomainEventDelivery
	require.NoError(t, db.First(&stored, deliveries[0].ID).Error)
	require.Equal(t, model.DomainEventDeliveryStatusFailed, stored.Status)
	require.Equal(t, "req-1", stored.LastWorkerRequestID)
	require.Equal(t, "trace-1", stored.LastWorkerTraceID)

	require.NoError(t, db.Model(&model.DomainEventDelivery{}).Where("id = ?", deliveries[0].ID).Updates(map[string]any{
		"status":       model.DomainEventDeliveryStatusProcessing,
		"attempts":     stored.MaxAttempts,
		"max_attempts": stored.MaxAttempts,
	}).Error)
	outcome, err = MarkFailed(db, deliveries[0].ID, errors.New("terminal"), 0, map[string]any{"stage": "consume"}, now.Add(3*time.Second))
	require.NoError(t, err)
	require.Equal(t, model.DomainEventDeliveryStatusDeadLetter, outcome.Status)
	require.Zero(t, outcome.RetryDelay)

	require.NoError(t, db.First(&stored, deliveries[0].ID).Error)
	require.Equal(t, model.DomainEventDeliveryStatusDeadLetter, stored.Status)
	require.NotNil(t, stored.DeadLetteredAt)
	require.Equal(t, "terminal", stored.DeadLetterReason)
}

func TestBeginQueuedDeliveryPreventsDuplicateConsume(t *testing.T) {
	t.Parallel()

	db := openEventBusTestDB(t)
	now := time.Now().UTC()

	_, err := Publish(db, PublishInput{
		EventType:      EventRechargeOrderPaid,
		AggregateType:  "recharge_order",
		AggregateID:    "3001",
		OccurredAt:     now,
		IdempotencyKey: "evt-3001",
		Consumers:      []string{ConsumerNotification},
		Payload:        map[string]any{"orderNo": "R-3001"},
	})
	require.NoError(t, err)

	deliveries, err := ClaimPendingDeliveries(db, ConsumerNotification, "dispatch-worker", 1, now.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	require.NoError(t, MarkQueued(db, deliveries[0].ID, map[string]any{"stage": "queued"}, now.Add(2*time.Second)))

	acquired, err := BeginQueuedDelivery(db, deliveries[0].ID, "consumer-a", "req-a", "trace-a", now.Add(3*time.Second))
	require.NoError(t, err)
	require.True(t, acquired)

	acquired, err = BeginQueuedDelivery(db, deliveries[0].ID, "consumer-b", "req-b", "trace-b", now.Add(4*time.Second))
	require.NoError(t, err)
	require.False(t, acquired)

	var stored model.DomainEventDelivery
	require.NoError(t, db.First(&stored, deliveries[0].ID).Error)
	require.Equal(t, model.DomainEventDeliveryStatusProcessing, stored.Status)
	require.Equal(t, "req-a", stored.LastWorkerRequestID)
	require.Equal(t, "trace-a", stored.LastWorkerTraceID)
}

func openEventBusTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "eventbus.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
