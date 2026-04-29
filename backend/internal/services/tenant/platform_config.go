package tenant

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	"game-admin/backend/internal/services/shared"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type PlatformConfigListFilter struct {
	TenantCode string
	TenantID   string
	BrandID    string
}

type PlatformConfigListItem struct {
	ID          uint64         `json:"id"`
	TenantID    *uint64        `json:"tenantID,omitempty"`
	TenantCode  string         `json:"tenantCode,omitempty"`
	BrandID     *uint64        `json:"brandID,omitempty"`
	BrandCode   string         `json:"brandCode,omitempty"`
	Key         string         `json:"key"`
	Value       datatypes.JSON `json:"value"`
	Description string         `json:"description,omitempty"`
}

type PlatformConfigInput struct {
	TenantID    *uint64 `json:"tenantID"`
	BrandID     *uint64 `json:"brandID"`
	Key         string  `json:"key"`
	Value       any     `json:"value"`
	Description string  `json:"description"`
}

func (s *Service) ListPlatformConfigs(scope shared.Scope, filter PlatformConfigListFilter) ([]PlatformConfigListItem, error) {
	query := s.db.Model(&model.PlatformConfig{}).
		Select("platform_config.id, platform_config.tenant_id, tenant.code as tenant_code, platform_config.brand_id, brand.code as brand_code, platform_config.key, platform_config.value, platform_config.description").
		Joins("LEFT JOIN tenant ON tenant.id = platform_config.tenant_id").
		Joins("LEFT JOIN brand ON brand.id = platform_config.brand_id").
		Order("platform_config.id desc")
	query = shared.ApplyTenantBrandScope(query, scope, "platform_config.tenant_id", "platform_config.brand_id")
	if value := strings.TrimSpace(filter.TenantCode); value != "" {
		query = query.Where("tenant.code = ?", value)
	}
	if value := strings.TrimSpace(filter.TenantID); value != "" {
		query = query.Where("platform_config.tenant_id = ?", value)
	}
	if value := strings.TrimSpace(filter.BrandID); value != "" {
		query = query.Where("platform_config.brand_id = ?", value)
	}
	var items []PlatformConfigListItem
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) CreatePlatformConfig(scope shared.Scope, input PlatformConfigInput, updatedBy string) (model.PlatformConfig, error) {
	var record model.PlatformConfig
	key := strings.TrimSpace(input.Key)
	if key == "" {
		return record, errors.New("key is required")
	}
	if input.TenantID != nil && *input.TenantID > 0 && len(scope.TenantIDs) > 0 && !slices.Contains(scope.TenantIDs, *input.TenantID) {
		return record, gorm.ErrRecordNotFound
	}
	if input.TenantID != nil && *input.TenantID > 0 {
		if _, err := shared.LoadTenant(s.db, *input.TenantID); err != nil {
			return record, err
		}
	}
	if input.BrandID != nil && *input.BrandID > 0 {
		brand, err := shared.LoadBrand(s.db, *input.BrandID)
		if err != nil {
			return record, err
		}
		if !shared.ScopeAllowsBrand(scope, brand.TenantID, brand.ID) {
			return record, gorm.ErrRecordNotFound
		}
		if input.TenantID != nil && *input.TenantID > 0 && brand.TenantID != *input.TenantID {
			return record, errors.New("brand does not belong to tenant")
		}
	}
	valueJSON, err := json.Marshal(input.Value)
	if err != nil {
		return record, err
	}
	record = model.PlatformConfig{
		TenantID:    input.TenantID,
		BrandID:     input.BrandID,
		Key:         key,
		Value:       datatypes.JSON(valueJSON),
		Description: strings.TrimSpace(input.Description),
		UpdatedBy:   shared.FirstNonEmpty(strings.TrimSpace(updatedBy), "system"),
	}
	if err := s.db.Create(&record).Error; err != nil {
		return model.PlatformConfig{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventPlatformConfigCreated,
		AggregateType:  "platform_config",
		AggregateID:    optionalUint64EventPart(record.TenantID) + ":" + optionalUint64EventPart(record.BrandID) + ":" + record.Key,
		TenantID:       record.TenantID,
		BrandID:        record.BrandID,
		OccurredAt:     record.CreatedAt,
		Producer:       "tenant-service",
		IdempotencyKey: "platform_config.created:" + record.Key + ":" + optionalUint64EventPart(record.TenantID) + ":" + optionalUint64EventPart(record.BrandID),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        record,
	}); err != nil {
		return model.PlatformConfig{}, err
	}
	return record, nil
}

func (s *Service) UpdatePlatformConfig(scope shared.Scope, id uint64, input PlatformConfigInput, updatedBy string) (model.PlatformConfig, model.PlatformConfig, error) {
	var before model.PlatformConfig
	query := shared.ApplyTenantBrandScope(s.db.Model(&model.PlatformConfig{}), scope, "platform_config.tenant_id", "platform_config.brand_id")
	if err := query.Where("platform_config.id = ?", id).First(&before).Error; err != nil {
		return model.PlatformConfig{}, model.PlatformConfig{}, err
	}
	if input.TenantID != nil || input.BrandID != nil || strings.TrimSpace(input.Key) != "" {
		if !shared.EqualOptionalUint64(before.TenantID, input.TenantID) || !shared.EqualOptionalUint64(before.BrandID, input.BrandID) || strings.TrimSpace(input.Key) != before.Key {
			return before, model.PlatformConfig{}, errors.New("platform config key and scope cannot be changed")
		}
	}
	valueJSON, err := json.Marshal(input.Value)
	if err != nil {
		return before, model.PlatformConfig{}, err
	}
	updated := before
	updated.Value = datatypes.JSON(valueJSON)
	updated.Description = strings.TrimSpace(input.Description)
	updated.UpdatedBy = shared.FirstNonEmpty(strings.TrimSpace(updatedBy), before.UpdatedBy, "system")
	if err := s.db.Save(&updated).Error; err != nil {
		return before, model.PlatformConfig{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventPlatformConfigUpdated,
		AggregateType:  "platform_config",
		AggregateID:    optionalUint64EventPart(updated.TenantID) + ":" + optionalUint64EventPart(updated.BrandID) + ":" + updated.Key,
		TenantID:       updated.TenantID,
		BrandID:        updated.BrandID,
		OccurredAt:     updated.UpdatedAt,
		Producer:       "tenant-service",
		IdempotencyKey: "platform_config.updated:" + updated.Key + ":" + optionalUint64EventPart(updated.TenantID) + ":" + optionalUint64EventPart(updated.BrandID) + ":" + updated.UpdatedAt.UTC().Format("20060102150405.000000000"),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        eventbus.PayloadBeforeAfter(before, updated),
	}); err != nil {
		return before, model.PlatformConfig{}, err
	}
	return before, updated, nil
}

func optionalUint64EventPart(value *uint64) string {
	if value == nil {
		return "global"
	}
	return strconv.FormatUint(*value, 10)
}
