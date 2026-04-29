package eventbus

const (
	TopicDomain = "domain.events"
)

const (
	ConsumerNotification     = "notification-service"
	ConsumerDataPlatformSync = "data-platform-sync-service"
)

func ConsumersDataPlatformSync() []string {
	return []string{ConsumerDataPlatformSync}
}

func ConsumersNotificationAndDataPlatformSync() []string {
	return []string{ConsumerNotification, ConsumerDataPlatformSync}
}

const (
	EventRechargeOrderPaid          = "recharge.order.paid"
	EventRechargeOrderRefunded      = "recharge.order.refunded"
	EventWithdrawalCreated          = "withdrawal.request.created"
	EventWithdrawalApproved         = "withdrawal.request.approved"
	EventWithdrawalRejected         = "withdrawal.request.rejected"
	EventWithdrawalPaid             = "withdrawal.request.paid"
	EventWithdrawalFailed           = "withdrawal.request.failed"
	EventWithdrawalReturned         = "withdrawal.request.returned"
	EventSettlementBillGenerated    = "settlement.bill.generated"
	EventSettlementBillConfirmed    = "settlement.bill.confirmed"
	EventRiskCaseCreated            = "risk.case.created"
	EventRiskCaseReleased           = "risk.case.released"
	EventRiskCaseRestored           = "risk.case.restored"
	EventRiskCaseConfirmed          = "risk.case.confirmed"
	EventTenantCreated              = "tenant.created"
	EventTenantStatusChanged        = "tenant.status_changed"
	EventTenantDeleted              = "tenant.deleted"
	EventBrandCreated               = "brand.created"
	EventBrandStatusChanged         = "brand.status_changed"
	EventBrandDeleted               = "brand.deleted"
	EventGameCreated                = "game.created"
	EventGameUpdated                = "game.updated"
	EventGameStatusChanged          = "game.status_changed"
	EventGameDeleted                = "game.deleted"
	EventGameCredentialRotated      = "game.credential.rotated"
	EventGameAccessUpserted         = "game.access.upserted"
	EventGameAccessRevoked          = "game.access.revoked"
	EventRBACRoleCreated            = "rbac.role.created"
	EventRBACRoleUpdated            = "rbac.role.updated"
	EventRBACRoleDeleted            = "rbac.role.deleted"
	EventRBACUserCreated            = "rbac.user.created"
	EventRBACUserUpdated            = "rbac.user.updated"
	EventRBACUserDeleted            = "rbac.user.deleted"
	EventInviteCodeCreated          = "invite.code.created"
	EventPlayerRegisteredWithInvite = "player.registered_with_invite"
	EventBindingCreated             = "binding.created"
	EventAgentInviteApplied         = "agent.invite.applied"
	EventAgentInviteAudited         = "agent.invite.audited"
	EventAgentRelationCreated       = "agent.relation.created"
	EventPlatformConfigCreated      = "platform_config.created"
	EventPlatformConfigUpdated      = "platform_config.updated"
	EventRecalculationTaskCreated   = "recalculation.task.created"
	EventRecalculationTaskCompleted = "recalculation.task.completed"
	EventRecalculationTaskFailed    = "recalculation.task.failed"
	EventActivityRuleCreated        = "activity.rule.created"
	EventActivityRuleStatusChanged  = "activity.rule.status_changed"
	EventActivityRecordGranted      = "activity.record.granted"
	EventActivityRecordReversed     = "activity.record.reversed"
	EventAuditLogCreated            = "audit.log.created"
)
