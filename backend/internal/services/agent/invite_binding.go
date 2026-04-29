package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	"game-admin/backend/internal/services/shared"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrAgentInviteRestricted     = errors.New("frozen or inactive agent cannot issue invite bindings")
	ErrInviteCodeUnavailable     = errors.New("invite code is invalid or unavailable")
	ErrDuplicateBinding          = errors.New("player is already bound to an agent")
	ErrInviteCodePrimaryConflict = errors.New("agent already has a primary invite code")
	ErrAgentInviteLoopDetected   = errors.New("agent relation loop detected")
	ErrInviteApplicationDone     = errors.New("agent invite application has already been reviewed")
)

type InviteCodeListFilter struct {
	AgentID string
}

type InviteCodeCreateInput struct {
	AgentID      uint64
	Code         string
	Status       model.InviteCodeStatus
	Channel      string
	Remark       string
	MaxUseCount  uint32
	IsPrimary    bool
	ValidFrom    string
	ValidTo      string
	ChannelScope []string
	GameScope    []uint64
}

type PlayerCreateInput struct {
	TenantID       *uint64
	BrandID        *uint64
	RegisterGameID *uint64
	PlayerNo       string
	PlatformUserID string
	Nickname       string
	Phone          string
	CountryCode    string
	Currency       string
}

type RegisterWithInviteInput struct {
	PlayerNo       string
	PlatformUserID string
	Nickname       string
	Phone          string
	CountryCode    string
	Currency       string
	InviteCode     string
	Remark         string
}

type BindingCreateInput struct {
	PlayerID uint64
	Code     string
	Remark   string
}

type BindingHistoryListFilter struct {
	PlayerID string
}

type InviteApplicationListFilter struct {
	Status string
}

type InviteApplicationCreateInput struct {
	ApplicantAgentID uint64
	InviteCode       string
	ApplyRemark      string
}

type InviteApplicationAuditInput struct {
	Status      model.AgentInviteApplicationStatus
	AuditRemark string
}

func (s *Service) ListInviteCodes(scope shared.Scope, filter InviteCodeListFilter) ([]model.InviteCode, error) {
	return s.repo.ListInviteCodes(scope, filter)
}

func (s *Service) CreateInviteCode(scope shared.Scope, input InviteCodeCreateInput) (*model.InviteCode, error) {
	if _, err := s.ensureAgentInCurrentScope(scope, input.AgentID); err != nil {
		return nil, err
	}
	return s.createInviteCode(input)
}

func (s *Service) ListPlayers(scope shared.Scope) ([]model.Player, error) {
	var agentIDs []uint64
	if scope.AgentID != nil {
		ids, err := s.repo.CurrentAgentScopeIDs(*scope.AgentID)
		if err != nil {
			return nil, err
		}
		agentIDs = ids
	}
	return s.repo.ListPlayers(scope, agentIDs)
}

func (s *Service) CreatePlayer(scope shared.Scope, input PlayerCreateInput) (model.Player, error) {
	tenantID, brandID, err := shared.ResolveTenantBrandScope(s.db, scope, input.TenantID, input.BrandID, false)
	if err != nil {
		return model.Player{}, err
	}
	player := model.Player{
		TenantID:       tenantID,
		BrandID:        brandID,
		RegisterGameID: input.RegisterGameID,
		PlayerNo:       strings.TrimSpace(input.PlayerNo),
		PlatformUserID: strings.TrimSpace(input.PlatformUserID),
		Nickname:       strings.TrimSpace(input.Nickname),
		Phone:          strings.TrimSpace(input.Phone),
		CountryCode:    strings.TrimSpace(input.CountryCode),
		Currency:       shared.DefaultCurrency(input.Currency),
		Status:         "active",
		RegisteredAt:   time.Now().UTC(),
	}
	if err := s.repo.CreatePlayer(&player); err != nil {
		return model.Player{}, err
	}
	return player, nil
}

func (s *Service) CreatePlayerForGame(game model.Game, input PlayerCreateInput) (model.Player, error) {
	player := model.Player{
		TenantID:       game.TenantID,
		BrandID:        game.BrandID,
		PlayerNo:       strings.TrimSpace(input.PlayerNo),
		PlatformUserID: strings.TrimSpace(input.PlatformUserID),
		Nickname:       strings.TrimSpace(input.Nickname),
		Phone:          strings.TrimSpace(input.Phone),
		CountryCode:    strings.TrimSpace(input.CountryCode),
		Currency:       shared.DefaultCurrency(shared.FirstNonEmpty(input.Currency, game.Currency)),
		Status:         "active",
		RegisterGameID: &game.ID,
		RegisteredAt:   time.Now().UTC(),
	}
	if strings.TrimSpace(player.PlatformUserID) == "" {
		return model.Player{}, errors.New("platformUserID is required")
	}
	if strings.TrimSpace(player.PlayerNo) == "" {
		player.PlayerNo = shared.BuildReferenceNo("PLY", player.PlatformUserID, int(time.Now().UTC().UnixNano()))
	}
	if err := s.repo.CreatePlayer(&player); err != nil {
		return model.Player{}, err
	}
	return player, nil
}

func (s *Service) RegisterWithInvite(scope shared.Scope, input RegisterWithInviteInput) (*model.Player, *model.Binding, error) {
	if strings.TrimSpace(input.InviteCode) == "" {
		return nil, nil, ErrInviteCodeUnavailable
	}
	if _, _, err := s.ensureInviteCodeInCurrentScope(scope, input.InviteCode); err != nil {
		return nil, nil, err
	}
	return s.registerWithInvite(input)
}

func (s *Service) RegisterWithInviteForGame(game model.Game, input RegisterWithInviteInput) (*model.Player, *model.Binding, error) {
	if strings.TrimSpace(input.InviteCode) == "" {
		return nil, nil, ErrInviteCodeUnavailable
	}
	return s.registerWithInviteForGame(game, input)
}

func (s *Service) ListBindings(scope shared.Scope) ([]model.Binding, error) {
	query := shared.ApplyTenantBrandScope(s.db.Order("id desc"), scope, "tenant_id", "brand_id")
	var err error
	query, err = s.applyAgentAccountScope(query, scope, "agent_id")
	if err != nil {
		return nil, err
	}
	var items []model.Binding
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) ListBindingHistory(scope shared.Scope, filter BindingHistoryListFilter) ([]model.BindingHistory, error) {
	query := shared.ApplyTenantBrandScope(s.db.Order("id desc"), scope, "tenant_id", "brand_id")
	var err error
	query, err = s.applyAgentAccountScope(query, scope, "to_agent_id")
	if err != nil {
		return nil, err
	}
	if value := strings.TrimSpace(filter.PlayerID); value != "" {
		if parsed, parseErr := strconv.ParseUint(value, 10, 64); parseErr == nil {
			query = query.Where("player_id = ?", parsed)
		}
	}
	var items []model.BindingHistory
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) CreateBinding(scope shared.Scope, input BindingCreateInput) (*model.Binding, error) {
	if input.PlayerID == 0 {
		return nil, errors.New("playerID is required")
	}
	if strings.TrimSpace(input.Code) == "" {
		return nil, ErrInviteCodeUnavailable
	}
	if _, err := s.ensurePlayerInCurrentScope(scope, input.PlayerID); err != nil {
		return nil, err
	}
	if _, _, err := s.ensureInviteCodeInCurrentScope(scope, input.Code); err != nil {
		return nil, err
	}
	return s.createBinding(input)
}

func (s *Service) ListInviteApplications(scope shared.Scope, filter InviteApplicationListFilter) ([]model.AgentInviteApplication, error) {
	return s.repo.ListInviteApplications(scope, filter)
}

func (s *Service) SubmitInviteApplication(scope shared.Scope, input InviteApplicationCreateInput) (*model.AgentInviteApplication, error) {
	if _, err := s.ensureAgentInCurrentScope(scope, input.ApplicantAgentID); err != nil {
		return nil, err
	}
	if _, _, err := s.ensureInviteCodeInCurrentScope(scope, input.InviteCode); err != nil {
		return nil, err
	}
	return s.submitInviteApplication(input)
}

func (s *Service) AuditInviteApplication(scope shared.Scope, id uint64, input InviteApplicationAuditInput, auditBy string) (*model.AgentInviteApplication, *model.AgentInviteApplication, error) {
	application, err := s.repo.FindInviteApplicationInScope(scope, id)
	if err != nil {
		return nil, nil, err
	}
	if _, err := s.ensureAgentInCurrentScope(scope, application.ApplicantAgentID); err != nil {
		return nil, nil, err
	}
	if _, err := s.ensureAgentInCurrentScope(scope, application.InviterAgentID); err != nil {
		return nil, nil, err
	}
	return s.auditInviteApplication(id, input, auditBy)
}

func (s *Service) createInviteCode(input InviteCodeCreateInput) (*model.InviteCode, error) {
	validFrom, err := shared.ParseOptionalTime(input.ValidFrom)
	if err != nil {
		return nil, errors.New("invalid validFrom")
	}
	validTo, err := shared.ParseOptionalTime(input.ValidTo)
	if err != nil {
		return nil, errors.New("invalid validTo")
	}
	if validFrom != nil && validTo != nil && !validTo.After(*validFrom) {
		return nil, errors.New("validTo must be after validFrom")
	}
	channelScope, _ := json.Marshal(shared.UniqueStrings(input.ChannelScope))
	gameScope, _ := json.Marshal(input.GameScope)
	inviteCode := model.InviteCode{
		AgentID:      input.AgentID,
		Code:         strings.TrimSpace(input.Code),
		Status:       defaultInviteStatus(input.Status),
		Channel:      strings.TrimSpace(input.Channel),
		Remark:       strings.TrimSpace(input.Remark),
		MaxUseCount:  input.MaxUseCount,
		IsPrimary:    input.IsPrimary,
		ValidFrom:    validFrom,
		ValidTo:      validTo,
		ChannelScope: datatypes.JSON(channelScope),
		GameScope:    datatypes.JSON(gameScope),
	}
	if inviteCode.Code == "" {
		return nil, errors.New("code is required")
	}
	if inviteCode.AgentID == 0 {
		return nil, errors.New("agentID is required")
	}
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		agent, err := s.repo.FindAgentTx(tx, inviteCode.AgentID)
		if err != nil {
			return err
		}
		if agent.Status != model.AgentStatusActive {
			return ErrAgentInviteRestricted
		}
		inviteCode.TenantID = agent.TenantID
		inviteCode.BrandID = agent.BrandID
		if inviteCode.IsPrimary {
			if _, err := s.repo.FindPrimaryInviteCodeByAgentTx(tx, inviteCode.AgentID); err == nil {
				return ErrInviteCodePrimaryConflict
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if err := s.repo.CreateInviteCodeTx(tx, &inviteCode); err != nil {
			return err
		}
		if _, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventInviteCodeCreated,
			AggregateType:  "invite_code",
			AggregateID:    strconv.FormatUint(inviteCode.ID, 10),
			TenantID:       inviteCode.TenantID,
			BrandID:        inviteCode.BrandID,
			OccurredAt:     inviteCode.CreatedAt,
			Producer:       "agent-service",
			IdempotencyKey: "invite.code.created:" + inviteCode.Code,
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        inviteCode,
		}); err != nil {
			return err
		}
		if inviteCode.IsPrimary {
			if err := tx.Model(&agent).Update("primary_invite_code_id", inviteCode.ID).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &inviteCode, nil
}

func (s *Service) registerWithInvite(input RegisterWithInviteInput) (*model.Player, *model.Binding, error) {
	player := model.Player{
		PlayerNo:       strings.TrimSpace(input.PlayerNo),
		PlatformUserID: strings.TrimSpace(input.PlatformUserID),
		Nickname:       strings.TrimSpace(input.Nickname),
		Phone:          strings.TrimSpace(input.Phone),
		CountryCode:    strings.TrimSpace(input.CountryCode),
		Currency:       shared.DefaultCurrency(input.Currency),
		Status:         "active",
		RegisteredAt:   time.Now().UTC(),
	}
	if player.PlatformUserID == "" {
		return nil, nil, errors.New("platformUserID is required")
	}
	if player.PlayerNo == "" {
		player.PlayerNo = shared.BuildReferenceNo("PLY", player.PlatformUserID, int(time.Now().UTC().UnixNano()))
	}
	if strings.TrimSpace(input.InviteCode) == "" {
		return nil, nil, ErrInviteCodeUnavailable
	}
	var binding *model.Binding
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		inviteCode, err := s.repo.FindInviteCodeByCodeTx(tx, input.InviteCode)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteCodeUnavailable
			}
			return err
		}
		inviter, err := s.repo.FindAgentTx(tx, inviteCode.AgentID)
		if err != nil {
			return err
		}
		if existingPlayer, err := s.repo.FindPlayerByPlatformUserIDTx(tx, player.PlatformUserID); err == nil {
			player = existingPlayer
			if err := shared.EnsureSameScope("player", inviter.TenantID, inviter.BrandID, player.TenantID, player.BrandID); err != nil {
				return err
			}
			if existingBinding, err := s.repo.FindBoundBindingByPlayerTx(tx, player.ID); err == nil {
				if existingBinding.AgentID != inviteCode.AgentID {
					return ErrDuplicateBinding
				}
				if existingBinding.InviteCodeID != nil && *existingBinding.InviteCodeID == inviteCode.ID {
					binding = &existingBinding
					return nil
				}
				return ErrDuplicateBinding
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else {
			player.TenantID = inviter.TenantID
			player.BrandID = inviter.BrandID
			if err := s.repo.CreatePlayerTx(tx, &player); err != nil {
				return err
			}
		}
		createdBinding, err := s.createBindingWithTx(tx, BindingCreateInput{PlayerID: player.ID, Code: input.InviteCode, Remark: strings.TrimSpace(input.Remark)})
		if err != nil {
			return err
		}
		binding = createdBinding
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventPlayerRegisteredWithInvite,
			AggregateType:  "player",
			AggregateID:    strconv.FormatUint(player.ID, 10),
			TenantID:       player.TenantID,
			BrandID:        player.BrandID,
			OccurredAt:     player.CreatedAt,
			Producer:       "agent-service",
			IdempotencyKey: "player.registered_with_invite:" + player.PlatformUserID,
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        playerBindingPayload(player, binding),
		})
		return err
	}); err != nil {
		return nil, nil, err
	}
	return &player, binding, nil
}

func (s *Service) registerWithInviteForGame(game model.Game, input RegisterWithInviteInput) (*model.Player, *model.Binding, error) {
	player, binding, err := s.registerWithInvite(input)
	if err != nil {
		return nil, nil, err
	}
	if player.TenantID != nil && game.TenantID != nil && *player.TenantID != *game.TenantID {
		return nil, nil, errors.New("player does not belong to current game tenant")
	}
	if player.BrandID != nil && game.BrandID != nil && *player.BrandID != *game.BrandID {
		return nil, nil, errors.New("player does not belong to current game brand")
	}
	if player.RegisterGameID == nil || *player.RegisterGameID == 0 {
		if err := s.repo.UpdatePlayerRegisterGameTx(s.db, player.ID, game.ID); err != nil {
			return nil, nil, err
		}
		player.RegisterGameID = &game.ID
	}
	return player, binding, nil
}

func (s *Service) createBinding(input BindingCreateInput) (*model.Binding, error) {
	return s.createBindingWithTx(s.db, input)
}

func (s *Service) createBindingWithTx(tx *gorm.DB, input BindingCreateInput) (*model.Binding, error) {
	if input.PlayerID == 0 {
		return nil, errors.New("playerID is required")
	}
	if strings.TrimSpace(input.Code) == "" {
		return nil, ErrInviteCodeUnavailable
	}
	var binding model.Binding
	now := time.Now().UTC()
	err := tx.Transaction(func(inner *gorm.DB) error {
		player, err := s.repo.FindPlayerTx(inner, input.PlayerID)
		if err != nil {
			return err
		}
		inviteCode, err := s.repo.FindInviteCodeByCodeTx(inner, input.Code)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteCodeUnavailable
			}
			return err
		}
		if inviteCode.Status != model.InviteCodeStatusActive || (inviteCode.MaxUseCount > 0 && inviteCode.UsedCount >= inviteCode.MaxUseCount) || (inviteCode.ValidFrom != nil && inviteCode.ValidFrom.After(now)) || (inviteCode.ValidTo != nil && !inviteCode.ValidTo.After(now)) || (inviteCode.ExpiredAt != nil && inviteCode.ExpiredAt.Before(now)) {
			return ErrInviteCodeUnavailable
		}
		agent, err := s.repo.FindAgentTx(inner, inviteCode.AgentID)
		if err != nil {
			return err
		}
		if agent.Status != model.AgentStatusActive {
			return ErrAgentInviteRestricted
		}
		if err := shared.EnsureSameScope("player", agent.TenantID, agent.BrandID, player.TenantID, player.BrandID); err != nil {
			return err
		}
		if existing, err := s.repo.FindBoundBindingByPlayerTx(inner, input.PlayerID); err == nil {
			binding = existing
			return ErrDuplicateBinding
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		binding = model.Binding{
			TenantID:      agent.TenantID,
			BrandID:       agent.BrandID,
			PlayerID:      input.PlayerID,
			AgentID:       inviteCode.AgentID,
			InviteCodeID:  &inviteCode.ID,
			Status:        model.BindingStatusBound,
			Source:        model.BindingSourceRegister,
			BoundAt:       now,
			EffectiveFrom: now,
			Remark:        strings.TrimSpace(input.Remark),
		}
		if err := s.repo.CreateBindingTx(inner, &binding); err != nil {
			if isUniqueConstraintError(err) {
				if _, findErr := s.repo.FindBoundBindingByPlayerTx(inner, input.PlayerID); findErr == nil {
					return ErrDuplicateBinding
				}
			}
			return err
		}
		if err := s.repo.IncrementInviteCodeUsageTx(inner, inviteCode.ID, now); err != nil {
			return err
		}
		history := model.BindingHistory{
			TenantID:     binding.TenantID,
			BrandID:      binding.BrandID,
			BindingID:    binding.ID,
			PlayerID:     binding.PlayerID,
			ToAgentID:    binding.AgentID,
			InviteCodeID: binding.InviteCodeID,
			Status:       binding.Status,
			Source:       binding.Source,
			ChangedAt:    now,
			ChangedBy:    "system",
			ChangeReason: "initial bind via invite code",
		}
		if err := s.repo.CreateBindingHistoryTx(inner, &history); err != nil {
			return err
		}
		_, err = eventbus.Publish(inner, eventbus.PublishInput{
			EventType:      eventbus.EventBindingCreated,
			AggregateType:  "binding",
			AggregateID:    strconv.FormatUint(binding.ID, 10),
			TenantID:       binding.TenantID,
			BrandID:        binding.BrandID,
			OccurredAt:     binding.CreatedAt,
			Producer:       "agent-service",
			IdempotencyKey: "binding.created:" + strconv.FormatUint(binding.ID, 10),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        binding,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (s *Service) submitInviteApplication(input InviteApplicationCreateInput) (*model.AgentInviteApplication, error) {
	if input.ApplicantAgentID == 0 {
		return nil, errors.New("applicantAgentID is required")
	}
	inviteCodeValue := strings.TrimSpace(input.InviteCode)
	if inviteCodeValue == "" {
		return nil, ErrInviteCodeUnavailable
	}
	var application model.AgentInviteApplication
	err := s.repo.WithTx(func(tx *gorm.DB) error {
		applicant, err := s.repo.FindAgentTx(tx, input.ApplicantAgentID)
		if err != nil {
			return err
		}
		inviteCode, err := s.repo.FindInviteCodeByCodeTx(tx, inviteCodeValue)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteCodeUnavailable
			}
			return err
		}
		if inviteCode.Status != model.InviteCodeStatusActive || (inviteCode.MaxUseCount > 0 && inviteCode.UsedCount >= inviteCode.MaxUseCount) || (inviteCode.ExpiredAt != nil && inviteCode.ExpiredAt.Before(time.Now().UTC())) {
			return ErrInviteCodeUnavailable
		}
		if applicant.ID == inviteCode.AgentID {
			return ErrAgentInviteLoopDetected
		}
		if err := shared.EnsureSameScope("applicant agent", inviteCode.TenantID, inviteCode.BrandID, applicant.TenantID, applicant.BrandID); err != nil {
			return err
		}
		if err := s.detectRelationLoop(tx, inviteCode.AgentID, applicant.ID); err != nil {
			return err
		}
		if existing, err := s.repo.FindPendingInviteApplicationTx(tx, applicant.ID, inviteCode.AgentID); err == nil {
			application = existing
			return nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		application = model.AgentInviteApplication{
			TenantID:         applicant.TenantID,
			BrandID:          applicant.BrandID,
			ApplicantAgentID: applicant.ID,
			InviterAgentID:   inviteCode.AgentID,
			InviteCodeID:     &inviteCode.ID,
			Status:           model.AgentInviteApplicationStatusPending,
			ApplyRemark:      strings.TrimSpace(input.ApplyRemark),
		}
		if err := s.repo.CreateInviteApplicationTx(tx, &application); err != nil {
			return err
		}
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventAgentInviteApplied,
			AggregateType:  "agent_invite_application",
			AggregateID:    strconv.FormatUint(application.ID, 10),
			TenantID:       application.TenantID,
			BrandID:        application.BrandID,
			OccurredAt:     application.CreatedAt,
			Producer:       "agent-service",
			IdempotencyKey: "agent.invite.applied:" + strconv.FormatUint(application.ID, 10),
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        application,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return &application, nil
}

func (s *Service) auditInviteApplication(id uint64, input InviteApplicationAuditInput, auditBy string) (*model.AgentInviteApplication, *model.AgentInviteApplication, error) {
	if input.Status != model.AgentInviteApplicationStatusApproved && input.Status != model.AgentInviteApplicationStatusRejected {
		return nil, nil, errors.New("status must be approved or rejected")
	}
	var application model.AgentInviteApplication
	var before model.AgentInviteApplication
	now := time.Now().UTC()
	err := s.repo.WithTx(func(tx *gorm.DB) error {
		loaded, err := s.repo.ReloadInviteApplicationTx(tx, id)
		if err != nil {
			return err
		}
		application = loaded
		before = application
		if application.Status != model.AgentInviteApplicationStatusPending {
			return ErrInviteApplicationDone
		}
		applicant, err := s.repo.FindAgentTx(tx, application.ApplicantAgentID)
		if err != nil {
			return err
		}
		inviter, err := s.repo.FindAgentTx(tx, application.InviterAgentID)
		if err != nil {
			return err
		}
		if err := shared.EnsureSameScope("applicant agent", inviter.TenantID, inviter.BrandID, applicant.TenantID, applicant.BrandID); err != nil {
			return err
		}
		application.Status = input.Status
		application.AuditRemark = strings.TrimSpace(input.AuditRemark)
		application.AuditBy = strings.TrimSpace(auditBy)
		application.AuditedAt = &now
		if input.Status == model.AgentInviteApplicationStatusApproved {
			relationID, err := s.createAgentRelation(tx, application.InviterAgentID, application.ApplicantAgentID)
			if err != nil {
				return err
			}
			application.ApprovedRelationID = &relationID
		}
		if err := s.repo.SaveInviteApplicationTx(tx, &application); err != nil {
			return err
		}
		if application, err = s.repo.ReloadInviteApplicationTx(tx, application.ID); err != nil {
			return err
		}
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventAgentInviteAudited,
			AggregateType:  "agent_invite_application",
			AggregateID:    strconv.FormatUint(application.ID, 10),
			TenantID:       application.TenantID,
			BrandID:        application.BrandID,
			OccurredAt:     now,
			Producer:       "agent-service",
			IdempotencyKey: "agent.invite.audited:" + strconv.FormatUint(application.ID, 10) + ":" + string(application.Status),
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        eventbus.PayloadBeforeAfter(before, application),
		})
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return &application, &before, nil
}

func (s *Service) ensureAgentInCurrentScope(scope shared.Scope, agentID uint64) (model.Agent, error) {
	agent, err := s.repo.FindAgent(agentID)
	if err != nil {
		return model.Agent{}, err
	}
	if !shared.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) {
		return model.Agent{}, gorm.ErrRecordNotFound
	}
	if scope.AgentID != nil {
		ids, err := s.repo.CurrentAgentScopeIDs(*scope.AgentID)
		if err != nil {
			return model.Agent{}, err
		}
		if !slices.Contains(ids, agentID) {
			return model.Agent{}, gorm.ErrRecordNotFound
		}
	}
	return agent, nil
}

func (s *Service) ensurePlayerInCurrentScope(scope shared.Scope, playerID uint64) (model.Player, error) {
	player, err := s.repo.FindPlayer(playerID)
	if err != nil {
		return model.Player{}, err
	}
	if !shared.MatchesScopedRecord(scope, player.TenantID, player.BrandID) {
		return model.Player{}, gorm.ErrRecordNotFound
	}
	if scope.AgentID != nil {
		ids, err := s.repo.CurrentAgentScopeIDs(*scope.AgentID)
		if err != nil {
			return model.Player{}, err
		}
		count, err := s.repo.CountBoundPlayerBindingsInAgentIDs(player.ID, ids)
		if err != nil {
			return model.Player{}, err
		}
		if count == 0 {
			return model.Player{}, gorm.ErrRecordNotFound
		}
	}
	return player, nil
}

func (s *Service) ensureInviteCodeInCurrentScope(scope shared.Scope, code string) (model.InviteCode, model.Agent, error) {
	inviteCode, err := s.repo.FindInviteCodeByCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.InviteCode{}, model.Agent{}, ErrInviteCodeUnavailable
		}
		return model.InviteCode{}, model.Agent{}, err
	}
	agent, err := s.ensureAgentInCurrentScope(scope, inviteCode.AgentID)
	if err != nil {
		return model.InviteCode{}, model.Agent{}, err
	}
	return inviteCode, agent, nil
}

func (s *Service) currentAgentScopeIDs(agentID uint64) ([]uint64, error) {
	return s.repo.CurrentAgentScopeIDs(agentID)
}

func (s *Service) applyAgentAccountScope(query *gorm.DB, scope shared.Scope, column string) (*gorm.DB, error) {
	if scope.AgentID == nil {
		return query, nil
	}
	ids, err := s.currentAgentScopeIDs(*scope.AgentID)
	if err != nil {
		return nil, err
	}
	return query.Where(column+" IN ?", ids), nil
}

func (s *Service) applyAgentSelfScope(query *gorm.DB, scope shared.Scope, column string) *gorm.DB {
	if scope.AgentID != nil {
		return query.Where(column+" = ?", *scope.AgentID)
	}
	return query
}

func (s *Service) createAgentRelation(tx *gorm.DB, inviterAgentID, applicantAgentID uint64) (uint64, error) {
	if err := s.detectRelationLoop(tx, inviterAgentID, applicantAgentID); err != nil {
		return 0, err
	}
	inviter, err := s.repo.FindAgentTx(tx, inviterAgentID)
	if err != nil {
		return 0, err
	}
	applicant, err := s.repo.FindAgentTx(tx, applicantAgentID)
	if err != nil {
		return 0, err
	}
	if err := shared.EnsureSameScope("applicant agent", inviter.TenantID, inviter.BrandID, applicant.TenantID, applicant.BrandID); err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	if err := s.repo.DeactivateActiveDirectRelationsTx(tx, applicantAgentID, now); err != nil {
		return 0, err
	}
	if err := s.repo.UpdateAgentParentTx(tx, applicantAgentID, inviterAgentID, now); err != nil {
		return 0, err
	}
	relation := model.Relation{
		TenantID:          inviter.TenantID,
		BrandID:           inviter.BrandID,
		AncestorAgentID:   inviterAgentID,
		DescendantAgentID: applicantAgentID,
		Depth:             1,
		DirectParentID:    &inviterAgentID,
		RelationType:      model.RelationTypeDirect,
		Status:            model.RelationStatusActive,
		EffectiveFrom:     now,
	}
	if err := s.repo.CreateRelationTx(tx, &relation); err != nil {
		return 0, err
	}
	if err := rebuildAgentRelationClosure(tx); err != nil {
		return 0, err
	}
	if _, err := eventbus.Publish(tx, eventbus.PublishInput{
		EventType:      eventbus.EventAgentRelationCreated,
		AggregateType:  "agent_relation",
		AggregateID:    strconv.FormatUint(relation.ID, 10),
		TenantID:       relation.TenantID,
		BrandID:        relation.BrandID,
		OccurredAt:     now,
		Producer:       "agent-service",
		IdempotencyKey: "agent.relation.created:" + strconv.FormatUint(relation.ID, 10),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        relation,
	}); err != nil {
		return 0, err
	}
	return relation.ID, nil
}

func (s *Service) detectRelationLoop(tx *gorm.DB, inviterAgentID, applicantAgentID uint64) error {
	if inviterAgentID == 0 || applicantAgentID == 0 {
		return nil
	}
	if inviterAgentID == applicantAgentID {
		return ErrAgentInviteLoopDetected
	}
	exists, err := s.repo.RelationLoopExistsTx(tx, applicantAgentID, inviterAgentID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAgentInviteLoopDetected
	}
	return nil
}

func rebuildAgentRelationClosure(tx *gorm.DB) error {
	repo := &gormRepository{}
	if err := repo.ClearRelationClosuresTx(tx); err != nil {
		return err
	}
	agents, err := repo.ListAgentsForClosureRebuildTx(tx)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return nil
	}
	now := time.Now().UTC()
	agentByID := make(map[uint64]model.Agent, len(agents))
	for _, agent := range agents {
		agentByID[agent.ID] = agent
	}
	closures := make([]model.AgentRelationClosure, 0, len(agents)*2)
	seen := make(map[string]struct{}, len(agents)*2)
	appendClosure := func(ancestorID, descendantID uint64, depth uint32, path []uint64, viaDirectParentID *uint64) error {
		key := strconv.FormatUint(ancestorID, 10) + ":" + strconv.FormatUint(descendantID, 10) + ":" + strconv.FormatUint(uint64(depth), 10)
		if _, exists := seen[key]; exists {
			return errors.New("duplicate closure path generated: " + key)
		}
		seen[key] = struct{}{}
		closures = append(closures, model.AgentRelationClosure{
			TenantID:          agentByID[descendantID].TenantID,
			BrandID:           agentByID[descendantID].BrandID,
			AncestorAgentID:   ancestorID,
			DescendantAgentID: descendantID,
			Depth:             depth,
			PathSnapshot:      agentPathSnapshot(path),
			ViaDirectParentID: viaDirectParentID,
			RelationType:      relationTypeForDepth(depth),
			Status:            model.RelationStatusActive,
			EffectiveFrom:     now,
		})
		return nil
	}
	for _, agent := range agents {
		lineagePath := []uint64{agent.ID}
		if err := appendClosure(agent.ID, agent.ID, 0, lineagePath, nil); err != nil {
			return err
		}
		depth := uint32(1)
		current := agent.ParentAgentID
		lineageSeen := map[uint64]struct{}{agent.ID: {}}
		for current != nil && *current != 0 {
			ancestorID := *current
			if _, exists := lineageSeen[ancestorID]; exists {
				return ErrAgentInviteLoopDetected
			}
			lineageSeen[ancestorID] = struct{}{}
			lineagePath = append([]uint64{ancestorID}, lineagePath...)
			if err := appendClosure(ancestorID, agent.ID, depth, lineagePath, agent.ParentAgentID); err != nil {
				return err
			}
			parentAgent, ok := agentByID[ancestorID]
			if !ok || parentAgent.ParentAgentID == nil || *parentAgent.ParentAgentID == 0 {
				break
			}
			current = parentAgent.ParentAgentID
			depth++
		}
	}
	for i := range closures {
		if err := repo.CreateRelationClosureTx(tx, &closures[i]); err != nil {
			return fmt.Errorf("insert closure[%d] ancestor=%d descendant=%d depth=%d via=%v failed: %w", i, closures[i].AncestorAgentID, closures[i].DescendantAgentID, closures[i].Depth, closures[i].ViaDirectParentID, err)
		}
	}
	return nil
}

func relationTypeForDepth(depth uint32) model.RelationType {
	if depth == 1 {
		return model.RelationTypeDirect
	}
	return model.RelationTypeClosure
}

func agentPathSnapshot(path []uint64) string {
	parts := make([]string, 0, len(path))
	for _, id := range path {
		parts = append(parts, strconv.FormatUint(id, 10))
	}
	return strings.Join(parts, "/")
}

func defaultInviteStatus(status model.InviteCodeStatus) model.InviteCodeStatus {
	if strings.TrimSpace(string(status)) == "" {
		return model.InviteCodeStatusActive
	}
	return status
}

func isUniqueConstraintError(err error) bool { return shared.IsUniqueConstraintError(err) }
