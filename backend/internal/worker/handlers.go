package worker

import (
	"context"
	"fmt"

	"game-admin/backend/internal/eventbus"
)

func NotificationHandler(_ context.Context, delivery eventbus.Delivery) (map[string]any, error) {
	switch delivery.Event.EventType {
	case eventbus.EventWithdrawalCreated,
		eventbus.EventWithdrawalApproved,
		eventbus.EventWithdrawalRejected,
		eventbus.EventWithdrawalPaid,
		eventbus.EventWithdrawalFailed,
		eventbus.EventWithdrawalReturned,
		eventbus.EventSettlementBillConfirmed,
		eventbus.EventRiskCaseCreated,
		eventbus.EventRiskCaseReleased,
		eventbus.EventRiskCaseConfirmed,
		eventbus.EventBindingCreated,
		eventbus.EventAgentInviteApplied,
		eventbus.EventAgentInviteAudited,
		eventbus.EventRecalculationTaskCompleted,
		eventbus.EventRecalculationTaskFailed,
		eventbus.EventActivityRecordGranted,
		eventbus.EventActivityRecordReversed:
		return map[string]any{
			"status":    "processed",
			"channel":   "notification",
			"eventType": delivery.Event.EventType,
		}, nil
	default:
		return map[string]any{
			"status":    "ignored",
			"channel":   "notification",
			"eventType": delivery.Event.EventType,
		}, nil
	}
}

func DataPlatformSyncHandler(_ context.Context, delivery eventbus.Delivery) (map[string]any, error) {
	switch delivery.Event.EventType {
	case eventbus.EventRechargeOrderPaid,
		eventbus.EventRechargeOrderRefunded,
		eventbus.EventTenantCreated,
		eventbus.EventTenantStatusChanged,
		eventbus.EventTenantDeleted,
		eventbus.EventBrandCreated,
		eventbus.EventBrandStatusChanged,
		eventbus.EventBrandDeleted,
		eventbus.EventGameCreated,
		eventbus.EventGameUpdated,
		eventbus.EventGameStatusChanged,
		eventbus.EventGameDeleted,
		eventbus.EventGameCredentialRotated,
		eventbus.EventGameAccessUpserted,
		eventbus.EventGameAccessRevoked,
		eventbus.EventRBACRoleCreated,
		eventbus.EventRBACRoleUpdated,
		eventbus.EventRBACRoleDeleted,
		eventbus.EventRBACUserCreated,
		eventbus.EventRBACUserUpdated,
		eventbus.EventRBACUserDeleted,
		eventbus.EventInviteCodeCreated,
		eventbus.EventPlayerRegisteredWithInvite,
		eventbus.EventBindingCreated,
		eventbus.EventAgentInviteApplied,
		eventbus.EventAgentInviteAudited,
		eventbus.EventAgentRelationCreated,
		eventbus.EventPlatformConfigCreated,
		eventbus.EventPlatformConfigUpdated,
		eventbus.EventRecalculationTaskCreated,
		eventbus.EventRecalculationTaskCompleted,
		eventbus.EventRecalculationTaskFailed,
		eventbus.EventWithdrawalCreated,
		eventbus.EventWithdrawalApproved,
		eventbus.EventWithdrawalRejected,
		eventbus.EventWithdrawalPaid,
		eventbus.EventWithdrawalFailed,
		eventbus.EventWithdrawalReturned,
		eventbus.EventSettlementBillGenerated,
		eventbus.EventSettlementBillConfirmed,
		eventbus.EventRiskCaseCreated,
		eventbus.EventRiskCaseReleased,
		eventbus.EventRiskCaseConfirmed,
		eventbus.EventActivityRuleCreated,
		eventbus.EventActivityRuleStatusChanged,
		eventbus.EventActivityRecordGranted,
		eventbus.EventActivityRecordReversed,
		eventbus.EventAuditLogCreated:
		return map[string]any{
			"status":    "processed",
			"channel":   "data-platform",
			"eventType": delivery.Event.EventType,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported data platform event type: %s", delivery.Event.EventType)
	}
}
