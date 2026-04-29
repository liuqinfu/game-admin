package withdrawal

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	accountcontract "game-admin/backend/internal/contract/account"
	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWithdrawalLifecyclePublishesEvents(t *testing.T) {
	t.Parallel()

	db := openWithdrawalTestDB(t)
	agent := model.Agent{AgentNo: "AG-WD-1", Name: "Agent WD", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)
	account := model.AgentAccount{
		AgentID:            agent.ID,
		AccountNo:          "ACC-WD-1",
		Status:             model.AccountStatusActive,
		Currency:           "CNY",
		Balance:            200,
		AvailableBalance:   200,
		WithdrawableAmount: 200,
		Version:            1,
	}
	require.NoError(t, db.Create(&account).Error)

	service := NewService(db)
	scope := sharedsvc.Scope{}

	created, err := service.Create(context.Background(), scope, CreateInput{
		AgentID:         agent.ID,
		Amount:          100,
		Currency:        "CNY",
		IdempotencyKey:  "wd-create-1",
		TenantCode:      "default",
		BankAccountName: "A",
		BankAccountNo:   "6222",
		BankName:        "Test Bank",
	}, "tester")
	require.NoError(t, err)

	_, approved, err := service.Review(context.Background(), scope, created.ID, ReviewInput{Action: "approve", Remark: "ok"}, "reviewer")
	require.NoError(t, err)
	require.Equal(t, model.WithdrawalStatusApproved, approved.Status)

	_, paid, err := service.Payout(context.Background(), scope, created.ID, PayoutInput{Action: "success", Reference: "PAY-1"}, "operator")
	require.NoError(t, err)
	require.Equal(t, model.WithdrawalStatusPaid, paid.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 3)
	require.Equal(t, "withdrawal.request.created", events[0].EventType)
	require.Equal(t, "withdrawal.request.approved", events[1].EventType)
	require.Equal(t, "withdrawal.request.paid", events[2].EventType)

	var deliveries int64
	require.NoError(t, db.Model(&model.DomainEventDelivery{}).Count(&deliveries).Error)
	require.Equal(t, int64(6), deliveries)
}

func TestCreateCompensatesFreezeWhenTransactionFails(t *testing.T) {
	t.Parallel()

	db := openWithdrawalTestDB(t)
	agent := model.Agent{AgentNo: "AG-WD-CREATE-FAIL", Name: "Agent WD Create Fail", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)

	accountClient := &stubAccountClient{
		createLedgerFn: func(_ context.Context, _ sharedsvc.Scope, input accountcontract.LedgerCreateInput) (model.AgentAccountLedger, error) {
			return model.AgentAccountLedger{AgentID: input.AgentID, Currency: input.Currency}, nil
		},
	}
	service := NewServiceWithClients(db, accountClient, &stubRiskClient{})
	service.repo = failingWithdrawalRepository{Repository: service.repo, failCreate: true}

	_, err := service.Create(context.Background(), sharedsvc.Scope{}, CreateInput{
		AgentID:         agent.ID,
		Amount:          88,
		Currency:        "CNY",
		IdempotencyKey:  "wd-create-compensate-1",
		TenantCode:      "default",
		BankAccountName: "A",
		BankAccountNo:   "6222",
		BankName:        "Test Bank",
	}, "tester")
	require.Error(t, err)
	require.Len(t, accountClient.inputs, 2)
	require.Equal(t, "withdrawal-freeze:wd-create-compensate-1", accountClient.inputs[0].IdempotencyKey)
	require.Equal(t, model.LedgerTypeFreeze, accountClient.inputs[0].LedgerType)
	require.Equal(t, "withdrawal-compensate-create:wd-create-compensate-1", accountClient.inputs[1].IdempotencyKey)
	require.Equal(t, model.LedgerTypeUnfreeze, accountClient.inputs[1].LedgerType)
}

func TestPayoutCompensatesRiskReleaseWhenTransactionFails(t *testing.T) {
	t.Parallel()

	db := openWithdrawalTestDB(t)
	agent := model.Agent{AgentNo: "AG-WD-PAYOUT-FAIL", Name: "Agent WD Payout Fail", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)
	withdrawal := model.WithdrawalRequest{
		RequestNo:     "WD-PAYOUT-FAIL-1",
		AgentID:       agent.ID,
		Currency:      "CNY",
		Amount:        100,
		PayableAmount: 100,
		Status:        model.WithdrawalStatusApproved,
		ApprovedLedgerID: func() *uint64 {
			value := uint64(9)
			return &value
		}(),
		IdempotencyKey: "wd-payout-compensate-1",
	}
	require.NoError(t, db.Create(&withdrawal).Error)

	accountClient := &stubAccountClient{}
	accountClient.createLedgerFn = func(_ context.Context, _ sharedsvc.Scope, input accountcontract.LedgerCreateInput) (model.AgentAccountLedger, error) {
		return model.AgentAccountLedger{AgentID: input.AgentID, Currency: input.Currency}, nil
	}
	riskClient := &stubRiskClient{
		releaseFn: func(_ context.Context, _ sharedsvc.Scope, agentID uint64, remark, reviewer string) ([]string, error) {
			require.Equal(t, agent.ID, agentID)
			require.Equal(t, "finance", reviewer)
			return []string{"RISK-COMP-1", "RISK-COMP-2"}, nil
		},
	}
	service := NewServiceWithClients(db, accountClient, riskClient)
	service.repo = failingWithdrawalRepository{Repository: service.repo, failSave: true}

	_, _, err := service.Payout(context.Background(), sharedsvc.Scope{}, withdrawal.ID, PayoutInput{Action: "success"}, "finance")
	require.Error(t, err)

	require.Len(t, riskClient.released, 1)
	require.Equal(t, []string{"RISK-COMP-1", "RISK-COMP-2"}, riskClient.released[0])
	require.Len(t, riskClient.restored, 1)
	require.Equal(t, []string{"RISK-COMP-1", "RISK-COMP-2"}, riskClient.restored[0])

	require.Len(t, accountClient.inputs, 4)
	require.Equal(t, "withdrawal-complete-unfreeze:wd-payout-compensate-1", accountClient.inputs[0].IdempotencyKey)
	require.Equal(t, "withdrawal-complete:wd-payout-compensate-1", accountClient.inputs[1].IdempotencyKey)
	require.Equal(t, "withdrawal-compensate-complete-reverse:wd-payout-compensate-1", accountClient.inputs[2].IdempotencyKey)
	require.Equal(t, "withdrawal-compensate-complete-unfreeze:wd-payout-compensate-1", accountClient.inputs[3].IdempotencyKey)
	require.Equal(t, model.LedgerTypeIncome, accountClient.inputs[2].LedgerType)
	require.Equal(t, model.LedgerTypeFreeze, accountClient.inputs[3].LedgerType)
}

func TestPayoutFailureWithLocalClientsRestoresRiskCaseAndEvents(t *testing.T) {
	t.Parallel()

	db := openWithdrawalTestDB(t)
	agent := model.Agent{AgentNo: "AG-WD-PAYOUT-LOCAL", Name: "Agent WD Payout Local", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)
	account := model.AgentAccount{
		AgentID:            agent.ID,
		AccountNo:          "ACC-WD-PAYOUT-LOCAL",
		Status:             model.AccountStatusActive,
		Currency:           "CNY",
		Balance:            300,
		AvailableBalance:   150,
		WithdrawableAmount: 150,
		FrozenBalance:      150,
		Version:            1,
	}
	require.NoError(t, db.Create(&account).Error)
	withdrawal := model.WithdrawalRequest{
		RequestNo:     "WD-PAYOUT-LOCAL-1",
		AgentID:       agent.ID,
		AccountID:     account.ID,
		Currency:      "CNY",
		Amount:        100,
		PayableAmount: 100,
		Status:        model.WithdrawalStatusApproved,
		ApprovedLedgerID: func() *uint64 {
			value := uint64(19)
			return &value
		}(),
		IdempotencyKey: "wd-payout-local-compensate-1",
	}
	require.NoError(t, db.Create(&withdrawal).Error)
	riskCase := model.RiskCase{
		CaseNo:          "RC-WD-LOCAL-1",
		AgentID:         agent.ID,
		Amount:          50,
		Currency:        "CNY",
		Reason:          "pending risk review",
		Status:          model.RiskCaseStatusPending,
		FreezeRequested: true,
	}
	require.NoError(t, db.Create(&riskCase).Error)

	service := NewService(db)
	service.repo = failingWithdrawalRepository{Repository: service.repo, failSave: true}

	_, _, err := service.Payout(context.Background(), sharedsvc.Scope{}, withdrawal.ID, PayoutInput{Action: "success", Remark: "trigger rollback"}, "finance")
	require.Error(t, err)

	var updatedRiskCase model.RiskCase
	require.NoError(t, db.First(&updatedRiskCase, riskCase.ID).Error)
	require.Equal(t, model.RiskCaseStatusPending, updatedRiskCase.Status)

	var updatedWithdrawal model.WithdrawalRequest
	require.NoError(t, db.First(&updatedWithdrawal, withdrawal.ID).Error)
	require.Equal(t, model.WithdrawalStatusApproved, updatedWithdrawal.Status)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	eventTypes := make([]string, 0, len(events))
	for _, event := range events {
		eventTypes = append(eventTypes, event.EventType)
	}
	require.Contains(t, eventTypes, "risk.case.released")
	require.Contains(t, eventTypes, "risk.case.restored")
	require.NotContains(t, eventTypes, "withdrawal.request.paid")
}

type stubAccountClient struct {
	inputs         []accountcontract.LedgerCreateInput
	createLedgerFn func(context.Context, sharedsvc.Scope, accountcontract.LedgerCreateInput) (model.AgentAccountLedger, error)
}

func (s *stubAccountClient) CreateLedger(ctx context.Context, scope sharedsvc.Scope, input accountcontract.LedgerCreateInput) (model.AgentAccountLedger, error) {
	s.inputs = append(s.inputs, input)
	if s.createLedgerFn != nil {
		return s.createLedgerFn(ctx, scope, input)
	}
	return model.AgentAccountLedger{}, nil
}

func (s *stubAccountClient) ListRiskAccounts(context.Context, sharedsvc.Scope, string) ([]accountcontract.RiskAccountSnapshot, error) {
	return nil, nil
}

func (s *stubAccountClient) GetRiskAccountSnapshot(context.Context, sharedsvc.Scope, uint64) (accountcontract.RiskAccountSnapshot, error) {
	return accountcontract.RiskAccountSnapshot{}, nil
}

type stubRiskClient struct {
	released  [][]string
	restored  [][]string
	releaseFn func(context.Context, sharedsvc.Scope, uint64, string, string) ([]string, error)
	restoreFn func(context.Context, sharedsvc.Scope, []string, string, string) (int, error)
}

func (s *stubRiskClient) ReleasePendingCases(ctx context.Context, scope sharedsvc.Scope, agentID uint64, remark, reviewer string) ([]string, error) {
	if s.releaseFn != nil {
		caseNos, err := s.releaseFn(ctx, scope, agentID, remark, reviewer)
		if err == nil {
			s.released = append(s.released, append([]string(nil), caseNos...))
		}
		return caseNos, err
	}
	return nil, nil
}

func (s *stubRiskClient) RestorePendingCases(ctx context.Context, scope sharedsvc.Scope, caseNos []string, remark, reviewer string) (int, error) {
	s.restored = append(s.restored, append([]string(nil), caseNos...))
	if s.restoreFn != nil {
		return s.restoreFn(ctx, scope, caseNos, remark, reviewer)
	}
	return len(caseNos), nil
}

type failingWithdrawalRepository struct {
	Repository
	failCreate bool
	failSave   bool
}

func (r failingWithdrawalRepository) CreateWithdrawalTx(tx *gorm.DB, withdrawal *model.WithdrawalRequest) error {
	if r.failCreate {
		return errors.New("create withdrawal failed")
	}
	return r.Repository.CreateWithdrawalTx(tx, withdrawal)
}

func (r failingWithdrawalRepository) SaveWithdrawalTx(tx *gorm.DB, withdrawal *model.WithdrawalRequest) error {
	if r.failSave {
		return errors.New("save withdrawal failed")
	}
	return r.Repository.SaveWithdrawalTx(tx, withdrawal)
}

func openWithdrawalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "withdrawal.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}
