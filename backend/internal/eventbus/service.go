package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type PublishInput struct {
	Topic          string
	EventType      string
	AggregateType  string
	AggregateID    string
	Version        int
	RequestID      string
	TraceID        string
	TenantID       *uint64
	BrandID        *uint64
	OccurredAt     time.Time
	Producer       string
	IdempotencyKey string
	Payload        any
	Meta           any
	Consumers      []string
}

type Delivery struct {
	model.DomainEventDelivery
	Event model.DomainEvent
}

type Handler func(context.Context, Delivery) (map[string]any, error)

const defaultDeliveryMaxAttempts uint32 = 20

type FailureOutcome struct {
	Status        model.DomainEventDeliveryStatus
	RetryDelay    time.Duration
	NextAttemptAt time.Time
}

func Publish(tx *gorm.DB, input PublishInput) (model.DomainEvent, error) {
	if tx == nil {
		return model.DomainEvent{}, fmt.Errorf("publish event: db is nil")
	}
	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		return model.DomainEvent{}, errors.New("publish event: event type is required")
	}
	aggregateType := strings.TrimSpace(input.AggregateType)
	if aggregateType == "" {
		return model.DomainEvent{}, errors.New("publish event: aggregate type is required")
	}
	aggregateID := strings.TrimSpace(input.AggregateID)
	if aggregateID == "" {
		return model.DomainEvent{}, errors.New("publish event: aggregate id is required")
	}
	consumers := normalizeConsumers(input.Consumers)
	if len(consumers) == 0 {
		return model.DomainEvent{}, errors.New("publish event: at least one consumer is required")
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey != "" {
		var existing model.DomainEvent
		if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&existing).Error; err == nil {
			return existing, nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.DomainEvent{}, err
		}
	}
	payload, err := buildEnvelopePayload(input)
	if err != nil {
		return model.DomainEvent{}, err
	}
	meta, err := marshalOptionalJSON(input.Meta)
	if err != nil {
		return model.DomainEvent{}, err
	}
	occurredAt := input.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	event := model.DomainEvent{
		Topic:          firstNonEmpty(strings.TrimSpace(input.Topic), TopicDomain),
		EventType:      eventType,
		AggregateType:  aggregateType,
		AggregateID:    aggregateID,
		TenantID:       input.TenantID,
		BrandID:        input.BrandID,
		OccurredAt:     occurredAt,
		Producer:       strings.TrimSpace(input.Producer),
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
		Meta:           meta,
	}
	if err := tx.Create(&event).Error; err != nil {
		return model.DomainEvent{}, err
	}
	deliveries := make([]model.DomainEventDelivery, 0, len(consumers))
	for _, consumer := range consumers {
		deliveries = append(deliveries, model.DomainEventDelivery{
			EventID:     event.ID,
			Consumer:    consumer,
			Status:      model.DomainEventDeliveryStatusPending,
			MaxAttempts: defaultDeliveryMaxAttempts,
			AvailableAt: occurredAt,
		})
	}
	if err := tx.Create(&deliveries).Error; err != nil {
		return model.DomainEvent{}, err
	}
	return event, nil
}

func ClaimPendingDeliveries(db *gorm.DB, consumer, workerName string, batchSize int, now time.Time) ([]Delivery, error) {
	return claimPendingDeliveries(db, consumer, workerName, "", "", batchSize, now)
}

func ClaimPendingDeliveriesWithCorrelation(db *gorm.DB, consumer, workerName, requestID, traceID string, batchSize int, now time.Time) ([]Delivery, error) {
	return claimPendingDeliveries(db, consumer, workerName, requestID, traceID, batchSize, now)
}

func claimPendingDeliveries(db *gorm.DB, consumer, workerName, requestID, traceID string, batchSize int, now time.Time) ([]Delivery, error) {
	if db == nil {
		return nil, fmt.Errorf("claim deliveries: db is nil")
	}
	consumer = strings.TrimSpace(consumer)
	if consumer == "" {
		return nil, errors.New("claim deliveries: consumer is required")
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	workerName = strings.TrimSpace(workerName)
	if workerName == "" {
		workerName = consumer
	}

	claimed := make([]Delivery, 0, batchSize)
	err := db.Transaction(func(tx *gorm.DB) error {
		var candidates []model.DomainEventDelivery
		if err := tx.
			Where("consumer = ? AND status IN ? AND available_at <= ?", consumer, []model.DomainEventDeliveryStatus{
				model.DomainEventDeliveryStatusPending,
				model.DomainEventDeliveryStatusFailed,
			}, now).
			Where("dead_lettered_at IS NULL").
			Where("max_attempts = 0 OR attempts < max_attempts").
			Order("available_at asc, id asc").
			Limit(batchSize).
			Find(&candidates).Error; err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}
		claimedIDs := make([]uint64, 0, len(candidates))
		for _, candidate := range candidates {
			update := tx.Model(&model.DomainEventDelivery{}).
				Where("id = ? AND consumer = ? AND status IN ? AND available_at <= ?", candidate.ID, consumer, []model.DomainEventDeliveryStatus{
					model.DomainEventDeliveryStatusPending,
					model.DomainEventDeliveryStatusFailed,
				}, now).
				Where("dead_lettered_at IS NULL").
				Where("max_attempts = 0 OR attempts < max_attempts").
				Updates(map[string]any{
					"status":                 model.DomainEventDeliveryStatusProcessing,
					"attempts":               gorm.Expr("attempts + 1"),
					"last_attempted_at":      now,
					"last_worker_request_id": firstNonEmpty(strings.TrimSpace(requestID), workerName),
					"last_worker_trace_id":   firstNonEmpty(strings.TrimSpace(traceID), strings.TrimSpace(requestID), workerName),
					"locked_at":              now,
					"locked_by":              workerName,
					"updated_at":             now,
				})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 1 {
				claimedIDs = append(claimedIDs, candidate.ID)
			}
		}
		if len(claimedIDs) == 0 {
			return nil
		}
		var rows []model.DomainEventDelivery
		if err := tx.Where("id IN ?", claimedIDs).Order("id asc").Find(&rows).Error; err != nil {
			return err
		}
		eventIDs := make([]uint64, 0, len(rows))
		for _, row := range rows {
			eventIDs = append(eventIDs, row.EventID)
		}
		var events []model.DomainEvent
		if err := tx.Where("id IN ?", eventIDs).Find(&events).Error; err != nil {
			return err
		}
		eventByID := make(map[uint64]model.DomainEvent, len(events))
		for _, event := range events {
			eventByID[event.ID] = event
		}
		for _, row := range rows {
			event, ok := eventByID[row.EventID]
			if !ok {
				return fmt.Errorf("claim deliveries: missing event %d", row.EventID)
			}
			claimed = append(claimed, Delivery{DomainEventDelivery: row, Event: event})
		}
		return nil
	})
	return claimed, err
}

func MarkDelivered(db *gorm.DB, deliveryID uint64, result any, now time.Time) error {
	if db == nil {
		return fmt.Errorf("mark delivered: db is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	payload, err := marshalOptionalJSON(result)
	if err != nil {
		return err
	}
	return db.Model(&model.DomainEventDelivery{}).Where("id = ?", deliveryID).Updates(map[string]any{
		"status":             model.DomainEventDeliveryStatusSucceeded,
		"delivered_at":       now,
		"locked_at":          nil,
		"locked_by":          "",
		"last_error":         "",
		"result_payload":     payload,
		"dead_letter_reason": "",
		"updated_at":         now,
	}).Error
}

func MarkQueued(db *gorm.DB, deliveryID uint64, result any, now time.Time) error {
	if db == nil {
		return fmt.Errorf("mark queued: db is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	payload, err := marshalOptionalJSON(result)
	if err != nil {
		return err
	}
	return db.Model(&model.DomainEventDelivery{}).Where("id = ?", deliveryID).Updates(map[string]any{
		"status":            model.DomainEventDeliveryStatusQueued,
		"locked_at":         nil,
		"locked_by":         "",
		"last_error":        "",
		"result_payload":    payload,
		"last_published_at": now,
		"source_transport":  "rabbitmq",
		"updated_at":        now,
	}).Error
}

func BeginQueuedDelivery(db *gorm.DB, deliveryID uint64, workerName, requestID, traceID string, now time.Time) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("begin queued delivery: db is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	update := db.Model(&model.DomainEventDelivery{}).
		Where("id = ? AND status = ? AND dead_lettered_at IS NULL", deliveryID, model.DomainEventDeliveryStatusQueued).
		Updates(map[string]any{
			"status":                 model.DomainEventDeliveryStatusProcessing,
			"last_attempted_at":      now,
			"last_worker_request_id": firstNonEmpty(strings.TrimSpace(requestID), workerName),
			"last_worker_trace_id":   firstNonEmpty(strings.TrimSpace(traceID), strings.TrimSpace(requestID), workerName),
			"locked_at":              now,
			"locked_by":              workerName,
			"updated_at":             now,
		})
	if update.Error != nil {
		return false, update.Error
	}
	return update.RowsAffected == 1, nil
}

func MarkFailed(db *gorm.DB, deliveryID uint64, cause error, retryDelay time.Duration, result any, now time.Time) (FailureOutcome, error) {
	if db == nil {
		return FailureOutcome{}, fmt.Errorf("mark failed: db is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	payload, err := marshalOptionalJSON(result)
	if err != nil {
		return FailureOutcome{}, err
	}
	lastError := ""
	if cause != nil {
		lastError = truncateString(cause.Error(), 255)
	}
	outcome := FailureOutcome{
		Status:     model.DomainEventDeliveryStatusFailed,
		RetryDelay: retryDelay,
	}
	var delivery model.DomainEventDelivery
	if err := db.First(&delivery, deliveryID).Error; err != nil {
		return FailureOutcome{}, err
	}
	if retryDelay <= 0 {
		retryDelay = NextRetryDelay(delivery.Attempts)
	}
	availableAt := now.Add(retryDelay)
	status := model.DomainEventDeliveryStatusFailed
	deadLetteredAt := any(nil)
	deadLetterReason := ""
	deadLetterPayload := datatypes.JSON(nil)
	if delivery.MaxAttempts > 0 && delivery.Attempts >= delivery.MaxAttempts {
		status = model.DomainEventDeliveryStatusDeadLetter
		deadLetteredAt = now
		deadLetterReason = lastError
		deadLetterPayload = payload
		availableAt = now
		retryDelay = 0
	}
	outcome.Status = status
	outcome.RetryDelay = retryDelay
	outcome.NextAttemptAt = availableAt
	if err := db.Model(&model.DomainEventDelivery{}).Where("id = ?", deliveryID).Updates(map[string]any{
		"status":              status,
		"available_at":        availableAt,
		"locked_at":           nil,
		"locked_by":           "",
		"last_error":          lastError,
		"result_payload":      payload,
		"dead_lettered_at":    deadLetteredAt,
		"dead_letter_reason":  deadLetterReason,
		"dead_letter_payload": deadLetterPayload,
		"updated_at":          now,
	}).Error; err != nil {
		return FailureOutcome{}, err
	}
	return outcome, nil
}

func DeadLetterDeliveries(db *gorm.DB, consumer string, limit int) ([]Delivery, error) {
	if db == nil {
		return nil, fmt.Errorf("dead letter deliveries: db is nil")
	}
	consumer = strings.TrimSpace(consumer)
	if limit <= 0 {
		limit = 50
	}
	var rows []model.DomainEventDelivery
	query := db.Model(&model.DomainEventDelivery{}).
		Where("status = ?", model.DomainEventDeliveryStatusDeadLetter).
		Order("dead_lettered_at desc, id desc").
		Limit(limit)
	if consumer != "" {
		query = query.Where("consumer = ?", consumer)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	eventIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		eventIDs = append(eventIDs, row.EventID)
	}
	var events []model.DomainEvent
	if err := db.Where("id IN ?", eventIDs).Find(&events).Error; err != nil {
		return nil, err
	}
	eventByID := make(map[uint64]model.DomainEvent, len(events))
	for _, event := range events {
		eventByID[event.ID] = event
	}
	items := make([]Delivery, 0, len(rows))
	for _, row := range rows {
		items = append(items, Delivery{DomainEventDelivery: row, Event: eventByID[row.EventID]})
	}
	return items, nil
}

func normalizeConsumers(items []string) []string {
	set := map[string]struct{}{}
	consumers := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := set[item]; exists {
			continue
		}
		set[item] = struct{}{}
		consumers = append(consumers, item)
	}
	return consumers
}

func marshalJSON(value any) (datatypes.JSON, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(payload), nil
}

func marshalOptionalJSON(value any) (datatypes.JSON, error) {
	if value == nil {
		return nil, nil
	}
	return marshalJSON(value)
}

func buildEnvelopePayload(input PublishInput) (datatypes.JSON, error) {
	payloadBody, err := marshalJSON(input.Payload)
	if err != nil {
		return nil, err
	}
	metaBody, err := marshalOptionalJSON(input.Meta)
	if err != nil {
		return nil, err
	}
	occurredAt := input.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	requestID, traceID := normalizeCorrelationFields(input.RequestID, input.TraceID, input.Meta)
	envelope := EventEnvelope{
		EventType:      strings.TrimSpace(input.EventType),
		Aggregate:      EventAggregate{Type: strings.TrimSpace(input.AggregateType), ID: strings.TrimSpace(input.AggregateID)},
		Version:        normalizeEnvelopeVersion(input.Version),
		Producer:       strings.TrimSpace(input.Producer),
		RequestID:      requestID,
		TraceID:        traceID,
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		OccurredAt:     occurredAt,
		TenantID:       input.TenantID,
		BrandID:        input.BrandID,
		Payload:        json.RawMessage(payloadBody),
		Meta:           json.RawMessage(metaBody),
	}
	return marshalJSON(envelope)
}

func normalizeEnvelopeVersion(version int) int {
	if version <= 0 {
		return EnvelopeVersion
	}
	return version
}

func normalizeCorrelationFields(requestID, traceID string, meta any) (string, string) {
	requestID = strings.TrimSpace(requestID)
	traceID = strings.TrimSpace(traceID)
	if requestID == "" || traceID == "" {
		metaRequestID, metaTraceID := extractCorrelation(meta)
		if requestID == "" {
			requestID = metaRequestID
		}
		if traceID == "" {
			traceID = metaTraceID
		}
	}
	if traceID == "" {
		traceID = requestID
	}
	return requestID, traceID
}

func extractCorrelation(meta any) (string, string) {
	switch typed := meta.(type) {
	case map[string]any:
		requestID, _ := typed["requestID"].(string)
		traceID, _ := typed["traceID"].(string)
		return strings.TrimSpace(requestID), strings.TrimSpace(traceID)
	case map[string]string:
		return strings.TrimSpace(typed["requestID"]), strings.TrimSpace(typed["traceID"])
	default:
		return "", ""
	}
}

func NextRetryDelay(attempts uint32) time.Duration {
	if attempts <= 1 {
		return 5 * time.Second
	}
	shift := attempts - 1
	if shift > 4 {
		shift = 4
	}
	return 5 * time.Second * time.Duration(1<<shift)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncateString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
