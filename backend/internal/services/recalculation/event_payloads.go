package recalculation

import "game-admin/backend/internal/domain/model"

func recalculationTaskPayload(task model.RecalculationTask, resultSummary any) map[string]any {
	return map[string]any{
		"task":          task,
		"resultSummary": resultSummary,
	}
}
