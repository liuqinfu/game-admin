package activity

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
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type RuleListFilter struct {
	TenantID     string
	BrandID      string
	Status       string
	ActivityType string
	Keyword      string
}

type RuleCreateInput struct {
	TenantID     *uint64
	BrandID      *uint64
	Name         string
	ActivityType string
	RewardType   string
	Status       model.ActivityRewardRuleStatus
	RewardValue  float64
	Currency     string
	TriggerValue float64
	DailyLimit   uint64
	TotalLimit   uint64
	StartAt      string
	EndAt        string
	Remark       string
}

type RecordListFilter struct {
	TenantID string
	BrandID  string
	Status   string
	Keyword  string
}

type RecordGenerateInput struct {
	PlayerID      *uint64
	UserID        *uint64
	AgentID       *uint64
	ReferenceType string
	ReferenceID   string
	Remark        string
}

type RecordReverseInput struct {
	Remark string
}

func NewService(db *gorm.DB) *Service { return &Service{db: db, repo: newRepository(db)} }

func (s *Service) ListRules(scope sharedsvc.Scope, filter RuleListFilter) ([]model.ActivityRewardRule, error) {
	if value := strings.TrimSpace(filter.TenantID); value != "" {
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return nil, errors.New("invalid tenantID")
		}
	}
	if value := strings.TrimSpace(filter.BrandID); value != "" {
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return nil, errors.New("invalid brandID")
		}
	}
	return s.repo.ListRules(scope, filter)
}

func (s *Service) CreateRule(scope sharedsvc.Scope, input RuleCreateInput) (model.ActivityRewardRule, error) {
	startAt, err := sharedsvc.ParseOptionalTime(input.StartAt)
	if err != nil {
		return model.ActivityRewardRule{}, errors.New("invalid startAt")
	}
	endAt, err := sharedsvc.ParseOptionalTime(input.EndAt)
	if err != nil {
		return model.ActivityRewardRule{}, errors.New("invalid endAt")
	}
	rule := model.ActivityRewardRule{
		TenantID:     input.TenantID,
		BrandID:      input.BrandID,
		Name:         strings.TrimSpace(input.Name),
		ActivityType: strings.TrimSpace(input.ActivityType),
		RewardType:   strings.TrimSpace(input.RewardType),
		Status:       defaultRuleStatus(input.Status),
		RewardValue:  input.RewardValue,
		Currency:     sharedsvc.FirstNonEmpty(strings.TrimSpace(input.Currency), "CNY"),
		TriggerValue: input.TriggerValue,
		DailyLimit:   input.DailyLimit,
		TotalLimit:   input.TotalLimit,
		StartAt:      startAt,
		EndAt:        endAt,
		Remark:       strings.TrimSpace(input.Remark),
	}
	rule.BaseModel = model.BaseModel{}
	if rule.Name == "" || rule.ActivityType == "" || rule.RewardType == "" {
		return model.ActivityRewardRule{}, errors.New("name, activityType and rewardType are required")
	}
	if err := s.ensureScopeReferences(scope, input.TenantID, input.BrandID); err != nil {
		return model.ActivityRewardRule{}, err
	}
	if err := s.repo.CreateRule(&rule); err != nil {
		return model.ActivityRewardRule{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventActivityRuleCreated,
		AggregateType:  "activity_reward_rule",
		AggregateID:    strconv.FormatUint(rule.ID, 10),
		TenantID:       rule.TenantID,
		BrandID:        rule.BrandID,
		OccurredAt:     rule.CreatedAt,
		Producer:       "activity-service",
		IdempotencyKey: "activity.rule.created:" + strconv.FormatUint(rule.ID, 10),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        activityRuleCreatedPayload(rule),
	}); err != nil {
		return model.ActivityRewardRule{}, err
	}
	return rule, nil
}

func (s *Service) UpdateRuleStatus(scope sharedsvc.Scope, id uint64, status model.ActivityRewardRuleStatus) (model.ActivityRewardRule, model.ActivityRewardRule, error) {
	rule, err := s.repo.FindRuleInScope(scope, id)
	if err != nil {
		return model.ActivityRewardRule{}, model.ActivityRewardRule{}, err
	}
	before := rule
	rule.Status = defaultRuleStatus(status)
	if err := s.repo.SaveRule(&rule); err != nil {
		return model.ActivityRewardRule{}, model.ActivityRewardRule{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventActivityRuleStatusChanged,
		AggregateType:  "activity_reward_rule",
		AggregateID:    strconv.FormatUint(rule.ID, 10),
		TenantID:       rule.TenantID,
		BrandID:        rule.BrandID,
		OccurredAt:     rule.UpdatedAt,
		Producer:       "activity-service",
		IdempotencyKey: "activity.rule.status:" + strconv.FormatUint(rule.ID, 10) + ":" + string(rule.Status),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        activityRuleStatusChangedPayload(before, rule),
	}); err != nil {
		return model.ActivityRewardRule{}, model.ActivityRewardRule{}, err
	}
	return before, rule, nil
}

func (s *Service) ListRecords(scope sharedsvc.Scope, filter RecordListFilter) ([]model.ActivityRewardRecord, error) {
	if value := strings.TrimSpace(filter.TenantID); value != "" {
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return nil, errors.New("invalid tenantID")
		}
	}
	if value := strings.TrimSpace(filter.BrandID); value != "" {
		if _, err := strconv.ParseUint(value, 10, 64); err != nil {
			return nil, errors.New("invalid brandID")
		}
	}
	return s.repo.ListRecords(scope, filter)
}

func (s *Service) GenerateRecord(scope sharedsvc.Scope, ruleID uint64, input RecordGenerateInput) (model.ActivityRewardRecord, int, error) {
	rule, err := s.repo.FindRuleInScope(scope, ruleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ActivityRewardRecord{}, 404, err
		}
		return model.ActivityRewardRecord{}, 500, err
	}
	return s.generateRecord(rule.ID, input)
}

func (s *Service) ReverseRecord(scope sharedsvc.Scope, recordID uint64, input RecordReverseInput) (model.ActivityRewardRecord, model.ActivityRewardRecord, int, error) {
	record, err := s.repo.FindRecordInScope(scope, recordID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ActivityRewardRecord{}, model.ActivityRewardRecord{}, 404, err
		}
		return model.ActivityRewardRecord{}, model.ActivityRewardRecord{}, 500, err
	}
	return s.reverseRecord(record.ID, input)
}

func (s *Service) generateRecord(ruleID uint64, input RecordGenerateInput) (model.ActivityRewardRecord, int, error) {
	var record model.ActivityRewardRecord
	playerID := input.PlayerID
	if playerID == nil {
		playerID = input.UserID
	}
	if playerID == nil || *playerID == 0 {
		return model.ActivityRewardRecord{}, 400, errors.New("playerID is required")
	}
	if strings.TrimSpace(input.ReferenceID) == "" {
		return model.ActivityRewardRecord{}, 400, errors.New("referenceID is required")
	}
	err := s.repo.WithTx(func(tx *gorm.DB) error {
		rule, err := s.repo.FindRuleTx(tx, ruleID)
		if err != nil {
			return err
		}
		if rule.Status != model.ActivityRewardRuleStatusActive {
			return errors.New("activity reward rule is not active")
		}
		now := time.Now().UTC()
		if rule.StartAt != nil && now.Before(rule.StartAt.UTC()) {
			return errors.New("activity reward rule is not active")
		}
		if rule.EndAt != nil && now.After(rule.EndAt.UTC()) {
			return errors.New("activity reward rule is not active")
		}
		if rule.TotalLimit > 0 {
			total, err := s.repo.CountRewardRecordsByRuleTx(tx, rule.ID)
			if err != nil {
				return err
			}
			if uint64(total) >= rule.TotalLimit {
				return errors.New("activity reward total limit reached")
			}
		}
		if rule.DailyLimit > 0 {
			startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			endOfDay := startOfDay.Add(24 * time.Hour)
			daily, err := s.repo.CountDailyRewardRecordsByRuleTx(tx, rule.ID, startOfDay, endOfDay)
			if err != nil {
				return err
			}
			if uint64(daily) >= rule.DailyLimit {
				return errors.New("activity reward daily limit reached")
			}
		}
		referenceType := strings.TrimSpace(input.ReferenceType)
		if referenceType == "" {
			referenceType = "manual"
		}
		if _, err := s.repo.FindRewardRecordByReferenceTx(tx, rule.ID, referenceType, strings.TrimSpace(input.ReferenceID)); err == nil {
			return errors.New("activity reward record already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		snapshot, err := json.Marshal(map[string]any{
			"ruleID":        rule.ID,
			"tenantID":      rule.TenantID,
			"brandID":       rule.BrandID,
			"ruleName":      rule.Name,
			"activityType":  rule.ActivityType,
			"rewardType":    rule.RewardType,
			"rewardValue":   rule.RewardValue,
			"currency":      rule.Currency,
			"triggerValue":  rule.TriggerValue,
			"dailyLimit":    rule.DailyLimit,
			"totalLimit":    rule.TotalLimit,
			"configPayload": json.RawMessage(rule.ConfigPayload),
		})
		if err != nil {
			return err
		}
		record = model.ActivityRewardRecord{
			RecordNo:        nextRecordNo(now),
			RuleID:          rule.ID,
			TenantID:        rule.TenantID,
			BrandID:         rule.BrandID,
			AgentID:         input.AgentID,
			PlayerID:        playerID,
			ActivityType:    rule.ActivityType,
			RewardType:      rule.RewardType,
			Status:          model.ActivityRewardRecordStatusGranted,
			RewardValue:     rule.RewardValue,
			Currency:        sharedsvc.FirstNonEmpty(strings.TrimSpace(rule.Currency), "CNY"),
			ReferenceType:   referenceType,
			ReferenceID:     strings.TrimSpace(input.ReferenceID),
			RuleName:        rule.Name,
			SnapshotPayload: snapshot,
			GrantedAt:       &now,
			Remark:          strings.TrimSpace(input.Remark),
		}
		if err := s.repo.CreateRewardRecordTx(tx, &record); err != nil {
			return err
		}
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventActivityRecordGranted,
			AggregateType:  "activity_reward_record",
			AggregateID:    strconv.FormatUint(record.ID, 10),
			TenantID:       record.TenantID,
			BrandID:        record.BrandID,
			OccurredAt:     now,
			Producer:       "activity-service",
			IdempotencyKey: "activity.record.granted:" + record.RecordNo,
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        activityRecordGrantedPayload(record),
		})
		return err
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ActivityRewardRecord{}, 404, err
		}
		switch err.Error() {
		case "activity reward rule is not active", "activity reward total limit reached", "activity reward daily limit reached", "activity reward record already exists":
			return model.ActivityRewardRecord{}, 400, err
		default:
			return model.ActivityRewardRecord{}, 500, err
		}
	}
	return record, 201, nil
}

func (s *Service) reverseRecord(recordID uint64, input RecordReverseInput) (model.ActivityRewardRecord, model.ActivityRewardRecord, int, error) {
	var before model.ActivityRewardRecord
	var after model.ActivityRewardRecord
	err := s.repo.WithTx(func(tx *gorm.DB) error {
		loaded, err := s.repo.FindRecordTx(tx, recordID)
		if err != nil {
			return err
		}
		after = loaded
		before = after
		if after.Status != model.ActivityRewardRecordStatusGranted {
			return errors.New("activity reward record is not reversible")
		}
		after.Status = model.ActivityRewardRecordStatusReversed
		if remark := strings.TrimSpace(input.Remark); remark != "" {
			after.Remark = remark
		}
		if err := s.repo.SaveRecordTx(tx, &after); err != nil {
			return err
		}
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventActivityRecordReversed,
			AggregateType:  "activity_reward_record",
			AggregateID:    strconv.FormatUint(after.ID, 10),
			TenantID:       after.TenantID,
			BrandID:        after.BrandID,
			OccurredAt:     time.Now().UTC(),
			Producer:       "activity-service",
			IdempotencyKey: "activity.record.reversed:" + after.RecordNo,
			Consumers:      eventbus.ConsumersNotificationAndDataPlatformSync(),
			Payload:        activityRecordReversedPayload(before, after),
		})
		return err
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return before, after, 404, err
		}
		if err.Error() == "activity reward record is not reversible" {
			return before, after, 400, err
		}
		return before, after, 500, err
	}
	return before, after, 200, nil
}

func (s *Service) ensureScopeReferences(scope sharedsvc.Scope, tenantID, brandID *uint64) error {
	scope = sharedsvc.NormalizeScope(scope)
	if tenantID != nil {
		if len(scope.TenantIDs) > 0 && !slices.Contains(scope.TenantIDs, *tenantID) {
			return gorm.ErrRecordNotFound
		}
		if _, err := sharedsvc.LoadTenant(s.db, *tenantID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("tenant not found")
			}
			return err
		}
	}
	if brandID != nil {
		brand, err := sharedsvc.LoadBrand(s.db, *brandID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("brand not found")
			}
			return err
		}
		if tenantID != nil && brand.TenantID != *tenantID {
			return errors.New("brand does not belong to tenant")
		}
		if !sharedsvc.ScopeAllowsBrand(scope, brand.TenantID, brand.ID) {
			return gorm.ErrRecordNotFound
		}
		if !sharedsvc.ScopeAllowsTenant(scope, brand.TenantID) && !sharedsvc.ScopeAllowsBrand(scope, brand.TenantID, brand.ID) {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

func defaultRuleStatus(status model.ActivityRewardRuleStatus) model.ActivityRewardRuleStatus {
	if strings.TrimSpace(string(status)) == "" {
		return model.ActivityRewardRuleStatusDraft
	}
	return status
}

func nextRecordNo(now time.Time) string {
	return fmt.Sprintf("ARR-%s-%d", now.UTC().Format("20060102150405"), now.UTC().UnixNano()%1000000)
}
