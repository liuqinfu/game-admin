package http

import (
	"strings"

	auditsvc "game-admin/backend/internal/services/audit"
	sharedsvc "game-admin/backend/internal/services/shared"
)

func logAudit(deps RouterDependencies, entry auditEntry) error {
	return auditsvc.NewService(deps.DB).Write(auditsvc.WriteInput{
		OperatorID:   entry.OperatorID,
		OperatorName: entry.OperatorName,
		OperatorRole: entry.OperatorRole,
		Module:       entry.Module,
		Action:       entry.Action,
		TargetType:   entry.TargetType,
		TargetID:     entry.TargetID,
		RequestID:    strings.TrimSpace(sharedsvc.FirstNonEmpty(entry.RequestID, entry.TraceID)),
		TraceID:      strings.TrimSpace(entry.TraceID),
		Result:       entry.Result,
		IP:           entry.IP,
		UserAgent:    entry.UserAgent,
		Before:       entry.Before,
		After:        entry.After,
		ErrorMessage: entry.ErrorMessage,
	})
}
