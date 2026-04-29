package modeltest

import (
	"reflect"
	"testing"

	"game-admin/backend/internal/domain/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPhase1ModelsAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(model.Phase1Models...)
	require.NoError(t, err)

	for _, table := range []string{
		"agent",
		"agent_invite_code",
		"player",
		"user_agent_binding",
		"user_agent_binding_history",
		"agent_relation",
		"agent_relation_closure",
		"agent_invite_application",
		"game",
		"agent_game_access",
		"commission_rule",
		"rule_snapshot",
		"recharge_order",
		"recharge_callback_log",
		"commission_record",
		"agent_account",
		"agent_account_ledger",
		"settlement_bill",
		"settlement_bill_detail",
		"recalculation_task",
		"operation_audit_log",
		"admin_user",
		"admin_role",
		"admin_permission",
		"admin_user_role",
		"admin_role_permission",
		"domain_event",
		"domain_event_delivery",
	} {
		require.Truef(t, db.Migrator().HasTable(table), "expected table %s to exist", table)
	}

	for _, column := range []string{"invited_by_agent_id", "settlement_account_no", "settlement_account_name"} {
		require.Truef(t, db.Migrator().HasColumn(&model.Agent{}, column), "expected agent.%s to exist", column)
	}
	require.True(t, db.Migrator().HasColumn(&model.AdminUser{}, "agent_id"), "expected admin_user.agent_id to exist")
	for _, column := range []string{"valid_from", "valid_to", "channel_scope", "game_scope"} {
		require.Truef(t, db.Migrator().HasColumn(&model.InviteCode{}, column), "expected agent_invite_code.%s to exist", column)
	}
	for _, column := range []string{"settlement_rate"} {
		require.Truef(t, db.Migrator().HasColumn(&model.CommissionRule{}, column), "expected commission_rule.%s to exist", column)
	}
	for _, column := range []string{"paid_amount"} {
		require.Truef(t, db.Migrator().HasColumn(&model.RechargeOrder{}, column), "expected recharge_order.%s to exist", column)
	}
	for _, column := range []string{"source_agent_id", "settlement_rate", "relation_snapshot"} {
		require.Truef(t, db.Migrator().HasColumn(&model.CommissionRecord{}, column), "expected commission_record.%s to exist", column)
	}
	for _, column := range []string{"available_balance", "total_income", "total_reversed"} {
		require.Truef(t, db.Migrator().HasColumn(&model.AgentAccount{}, column), "expected agent_account.%s to exist", column)
	}
}

func TestPhase2ModelsIncludedInPhase1Models(t *testing.T) {
	phase1Types := make(map[string]struct{}, len(model.Phase1Models))
	for _, item := range model.Phase1Models {
		phase1Types[reflect.TypeOf(item).String()] = struct{}{}
	}

	for _, item := range []any{
		&model.Relation{},
		&model.AgentRelationClosure{},
		&model.AgentInviteApplication{},
		&model.Game{},
		&model.AgentGameAccess{},
		&model.SettlementBill{},
		&model.SettlementBillDetail{},
		&model.RecalculationTask{},
	} {
		_, ok := phase1Types[reflect.TypeOf(item).String()]
		require.Truef(t, ok, "expected %T to be included in Phase1Models", item)
	}
}
