package agent

import (
	"errors"
	"strings"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type CreateInput struct {
	TenantID              *uint64
	BrandID               *uint64
	AgentNo               string
	Name                  string
	DisplayName           string
	Phone                 string
	Email                 string
	Status                model.AgentStatus
	Level                 uint32
	SettlementAccountNo   string
	SettlementAccountName string
	Remark                string
}

type AgentListFilter struct {
	TenantID string
	BrandID  string
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, repo: newRepository(db)}
}

func (s *Service) ListAgents(scope shared.Scope, filter AgentListFilter) ([]model.Agent, error) {
	return s.repo.ListAgents(scope, filter)
}

func (s *Service) Create(scope shared.Scope, input CreateInput) (model.Agent, error) {
	tenantID, brandID, err := shared.ResolveTenantBrandScope(s.db, scope, input.TenantID, input.BrandID, false)
	if err != nil {
		return model.Agent{}, err
	}
	agent := model.Agent{
		TenantID:              tenantID,
		BrandID:               brandID,
		AgentNo:               input.AgentNo,
		Name:                  input.Name,
		DisplayName:           input.DisplayName,
		Phone:                 input.Phone,
		Email:                 input.Email,
		Status:                defaultAgentStatus(input.Status),
		Level:                 defaultLevel(input.Level),
		SettlementAccountNo:   strings.TrimSpace(input.SettlementAccountNo),
		SettlementAccountName: strings.TrimSpace(input.SettlementAccountName),
		Remark:                input.Remark,
	}
	return agent, s.repo.CreateAgent(&agent)
}

func (s *Service) UpdateStatus(scope shared.Scope, id uint64, status model.AgentStatus) (model.Agent, model.Agent, error) {
	agent, err := s.repo.FindAgent(id)
	if err != nil {
		return model.Agent{}, model.Agent{}, err
	}
	if !shared.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
		return model.Agent{}, model.Agent{}, gorm.ErrRecordNotFound
	}
	before := agent
	if status == "" {
		return model.Agent{}, model.Agent{}, errors.New("status is required")
	}
	agent.Status = status
	return before, agent, s.repo.SaveAgent(&agent)
}

func defaultAgentStatus(status model.AgentStatus) model.AgentStatus {
	if status == "" {
		return model.AgentStatusActive
	}
	return status
}

func defaultLevel(level uint32) uint32 {
	if level == 0 {
		return 1
	}
	return level
}
