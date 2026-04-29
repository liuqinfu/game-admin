package http

import "game-admin/backend/internal/domain/model"

type auditEntry struct {
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
