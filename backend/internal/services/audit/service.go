package audit

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

var ErrPlatformScopeRequired = errors.New("audit log is platform scoped")

type Service struct {
	db   *gorm.DB
	repo Repository
}

type ListFilter struct {
	Module   string
	Action   string
	TargetID string
}

type WriteInput struct {
	OperatorID   string
	OperatorName string
	OperatorRole string
	Module       model.AuditModule
	Action       string
	TargetType   string
	TargetID     string
	RequestID    string
	TraceID      string
	Result       model.AuditResult
	IP           string
	UserAgent    string
	Before       any
	After        any
	ErrorMessage string
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, repo: newRepository(db)}
}

func (s *Service) List(scope shared.Scope, filter ListFilter) ([]model.OperationAuditLog, error) {
	if shared.Level(scope) != shared.ScopeLevelPlatform {
		return nil, ErrPlatformScopeRequired
	}
	return s.repo.List(scope, filter)
}

func (s *Service) Write(input WriteInput) error {
	afterPayload, err := json.Marshal(input.After)
	if err != nil {
		return err
	}
	var beforePayload []byte
	if input.Before != nil {
		beforePayload, err = json.Marshal(input.Before)
		if err != nil {
			return err
		}
	}
	log := model.OperationAuditLog{
		OperatorID:    shared.FirstNonEmpty(input.OperatorID, "system"),
		OperatorName:  shared.FirstNonEmpty(input.OperatorName, "system"),
		OperatorRole:  shared.FirstNonEmpty(input.OperatorRole, "system"),
		Module:        input.Module,
		Action:        input.Action,
		TargetType:    input.TargetType,
		TargetID:      input.TargetID,
		RequestID:     input.RequestID,
		TraceID:       input.TraceID,
		Result:        input.Result,
		IP:            input.IP,
		UserAgent:     input.UserAgent,
		BeforePayload: beforePayload,
		AfterPayload:  afterPayload,
		ErrorMessage:  input.ErrorMessage,
		OccurredAt:    time.Now().UTC(),
	}
	if log.Result == "" {
		log.Result = model.AuditResultSuccess
	}
	if err := s.repo.Create(&log); err != nil {
		return err
	}
	_, err = eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventAuditLogCreated,
		AggregateType:  "operation_audit_log",
		AggregateID:    strconv.FormatUint(log.ID, 10),
		OccurredAt:     log.OccurredAt,
		Producer:       "audit-service",
		IdempotencyKey: "audit.log.created:" + strconv.FormatUint(log.ID, 10),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload: map[string]any{
			"id":           log.ID,
			"module":       log.Module,
			"action":       log.Action,
			"targetType":   log.TargetType,
			"targetID":     log.TargetID,
			"operatorID":   log.OperatorID,
			"operatorRole": log.OperatorRole,
			"result":       log.Result,
			"requestID":    log.RequestID,
		},
	})
	return err
}
