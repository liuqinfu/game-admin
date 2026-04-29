package activity

import "game-admin/backend/internal/domain/model"

func activityRuleCreatedPayload(rule model.ActivityRewardRule) map[string]any {
	return map[string]any{
		"id":           rule.ID,
		"name":         rule.Name,
		"activityType": rule.ActivityType,
		"rewardType":   rule.RewardType,
		"status":       rule.Status,
		"rewardValue":  rule.RewardValue,
		"currency":     rule.Currency,
	}
}

func activityRuleStatusChangedPayload(before, after model.ActivityRewardRule) map[string]any {
	return map[string]any{
		"id":           after.ID,
		"name":         after.Name,
		"activityType": after.ActivityType,
		"beforeStatus": before.Status,
		"status":       after.Status,
	}
}

func activityRecordGrantedPayload(record model.ActivityRewardRecord) map[string]any {
	return map[string]any{
		"id":            record.ID,
		"recordNo":      record.RecordNo,
		"ruleID":        record.RuleID,
		"playerID":      record.PlayerID,
		"agentID":       record.AgentID,
		"activityType":  record.ActivityType,
		"rewardType":    record.RewardType,
		"status":        record.Status,
		"rewardValue":   record.RewardValue,
		"currency":      record.Currency,
		"referenceType": record.ReferenceType,
		"referenceID":   record.ReferenceID,
	}
}

func activityRecordReversedPayload(before, after model.ActivityRewardRecord) map[string]any {
	return map[string]any{
		"id":           after.ID,
		"recordNo":     after.RecordNo,
		"ruleID":       after.RuleID,
		"playerID":     after.PlayerID,
		"agentID":      after.AgentID,
		"beforeStatus": before.Status,
		"status":       after.Status,
	}
}
