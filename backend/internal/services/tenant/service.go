package tenant

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type TenantInput struct {
	Code        string
	Name        string
	DisplayName string
	Status      model.TenantStatus
	Remark      string
}

type BrandInput struct {
	TenantID    uint64
	Code        string
	Name        string
	DisplayName string
	Status      model.BrandStatus
	Domain      string
	IsDefault   bool
	Remark      string
}

type EnumDictionaryItem = shared.EnumDictionaryItem
type EnumDictionaryResponse = shared.EnumDictionaryResponse

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, repo: newRepository(db)}
}

func (s *Service) ListEnumDictionaries(codes []string) ([]EnumDictionaryResponse, error) {
	return shared.ListEnumDictionaries(s.db, codes)
}

func (s *Service) ListTenants(scope shared.Scope) ([]model.Tenant, error) {
	return s.repo.ListTenants(scope)
}

func (s *Service) CreateTenant(scope shared.Scope, input TenantInput) (model.Tenant, error) {
	if shared.Level(scope) != shared.ScopeLevelPlatform {
		return model.Tenant{}, errors.New("tenant can only be created by platform scope")
	}
	tenant := model.Tenant{
		Code:        strings.TrimSpace(input.Code),
		Name:        strings.TrimSpace(input.Name),
		DisplayName: strings.TrimSpace(input.DisplayName),
		Status:      defaultTenantStatus(input.Status),
		Remark:      strings.TrimSpace(input.Remark),
	}
	if tenant.Code == "" || tenant.Name == "" {
		return model.Tenant{}, errors.New("tenant code and name are required")
	}
	if err := s.repo.CreateTenant(&tenant); err != nil {
		return model.Tenant{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventTenantCreated,
		AggregateType:  "tenant",
		AggregateID:    fmt.Sprintf("%d", tenant.ID),
		OccurredAt:     tenant.CreatedAt,
		Producer:       "tenant-service",
		IdempotencyKey: "tenant.created:" + tenant.Code,
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        tenantCreatedPayload(tenant),
	}); err != nil {
		return model.Tenant{}, err
	}
	return tenant, nil
}

func (s *Service) UpdateTenantStatus(scope shared.Scope, id uint64, status model.TenantStatus) (model.Tenant, model.Tenant, error) {
	var tenant model.Tenant
	if loaded, err := s.repo.FindTenant(id); err != nil {
		return model.Tenant{}, model.Tenant{}, err
	} else {
		tenant = loaded
	}
	if !shared.ScopeAllowsTenant(scope, tenant.ID) {
		return model.Tenant{}, model.Tenant{}, gorm.ErrRecordNotFound
	}
	before := tenant
	tenant.Status = defaultTenantStatus(status)
	if err := s.repo.SaveTenant(&tenant); err != nil {
		return model.Tenant{}, model.Tenant{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventTenantStatusChanged,
		AggregateType:  "tenant",
		AggregateID:    fmt.Sprintf("%d", tenant.ID),
		OccurredAt:     tenant.UpdatedAt,
		Producer:       "tenant-service",
		IdempotencyKey: "tenant.status:" + fmt.Sprintf("%d", tenant.ID) + ":" + string(tenant.Status),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        tenantStatusChangedPayload(before, tenant),
	}); err != nil {
		return model.Tenant{}, model.Tenant{}, err
	}
	return before, tenant, nil
}

func (s *Service) DeleteTenant(scope shared.Scope, id uint64) (model.Tenant, error) {
	if level := shared.Level(scope); level != shared.ScopeLevelPlatform && level != shared.ScopeLevelTenant {
		return model.Tenant{}, gorm.ErrRecordNotFound
	}
	var tenant model.Tenant
	if loaded, err := s.repo.FindTenant(id); err != nil {
		return model.Tenant{}, err
	} else {
		tenant = loaded
	}
	if !shared.ScopeAllowsTenant(scope, tenant.ID) {
		return model.Tenant{}, gorm.ErrRecordNotFound
	}
	if err := s.repo.EnsureDeleteReferencesClear([]deleteReferenceCheck{
		{model: &model.Brand{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active brands"},
		{model: &model.Agent{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active agents"},
		{model: &model.Player{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active players"},
		{model: &model.Game{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active games"},
		{model: &model.PlatformConfig{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active platform configs"},
		{model: &model.CommissionRule{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active commission rules"},
		{model: &model.WithdrawalRequest{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active withdrawal requests"},
		{model: &model.RechargeOrder{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active recharge orders"},
		{model: &model.CommissionRecord{}, query: "tenant_id = ?", args: []any{tenant.ID}, message: "tenant has active commission records"},
	}); err != nil {
		return model.Tenant{}, err
	}
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		if err := s.repo.ArchiveDeletedScopedCode(tx, &model.Tenant{}, tenant.ID, "code", tenant.Code); err != nil {
			return err
		}
		if err := tx.Delete(&tenant).Error; err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventTenantDeleted,
			AggregateType:  "tenant",
			AggregateID:    fmt.Sprintf("%d", tenant.ID),
			OccurredAt:     time.Now().UTC(),
			Producer:       "tenant-service",
			IdempotencyKey: "tenant.deleted:" + fmt.Sprintf("%d", tenant.ID),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        tenantDeletedPayload(tenant),
		})
		return err
	}); err != nil {
		return model.Tenant{}, err
	}
	return tenant, nil
}

func (s *Service) ListBrands(scope shared.Scope, tenantID *uint64) ([]model.Brand, error) {
	return s.repo.ListBrands(scope, tenantID)
}

func (s *Service) CreateBrand(scope shared.Scope, input BrandInput) (model.Brand, error) {
	if !shared.ScopeAllowsTenant(scope, input.TenantID) {
		return model.Brand{}, gorm.ErrRecordNotFound
	}
	if _, err := s.repo.FindTenant(input.TenantID); err != nil {
		return model.Brand{}, errors.New("tenant not found")
	}
	brand := model.Brand{
		TenantID:    input.TenantID,
		Code:        strings.TrimSpace(input.Code),
		Name:        strings.TrimSpace(input.Name),
		DisplayName: strings.TrimSpace(input.DisplayName),
		Status:      defaultBrandStatus(input.Status),
		Domain:      strings.TrimSpace(input.Domain),
		IsDefault:   input.IsDefault,
		Remark:      strings.TrimSpace(input.Remark),
	}
	if brand.Code == "" || brand.Name == "" {
		return model.Brand{}, errors.New("brand code and name are required")
	}
	if err := s.repo.CreateBrand(&brand); err != nil {
		return model.Brand{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventBrandCreated,
		AggregateType:  "brand",
		AggregateID:    fmt.Sprintf("%d", brand.ID),
		TenantID:       &brand.TenantID,
		BrandID:        &brand.ID,
		OccurredAt:     brand.CreatedAt,
		Producer:       "tenant-service",
		IdempotencyKey: "brand.created:" + brand.Code,
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        brandCreatedPayload(brand),
	}); err != nil {
		return model.Brand{}, err
	}
	return brand, nil
}

func (s *Service) UpdateBrandStatus(scope shared.Scope, id uint64, status model.BrandStatus) (model.Brand, model.Brand, error) {
	var brand model.Brand
	if loaded, err := s.repo.FindBrand(id); err != nil {
		return model.Brand{}, model.Brand{}, err
	} else {
		brand = loaded
	}
	if !shared.MatchesScopedRecord(scope, &brand.TenantID, &brand.ID) {
		return model.Brand{}, model.Brand{}, gorm.ErrRecordNotFound
	}
	before := brand
	brand.Status = defaultBrandStatus(status)
	if err := s.repo.SaveBrand(&brand); err != nil {
		return model.Brand{}, model.Brand{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventBrandStatusChanged,
		AggregateType:  "brand",
		AggregateID:    fmt.Sprintf("%d", brand.ID),
		TenantID:       &brand.TenantID,
		BrandID:        &brand.ID,
		OccurredAt:     brand.UpdatedAt,
		Producer:       "tenant-service",
		IdempotencyKey: "brand.status:" + fmt.Sprintf("%d", brand.ID) + ":" + string(brand.Status),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        brandStatusChangedPayload(before, brand),
	}); err != nil {
		return model.Brand{}, model.Brand{}, err
	}
	return before, brand, nil
}

func (s *Service) DeleteBrand(scope shared.Scope, id uint64) (model.Brand, error) {
	if shared.Level(scope) == shared.ScopeLevelAgent {
		return model.Brand{}, gorm.ErrRecordNotFound
	}
	var brand model.Brand
	if loaded, err := s.repo.FindBrand(id); err != nil {
		return model.Brand{}, err
	} else {
		brand = loaded
	}
	if !shared.ScopeAllowsBrand(scope, brand.TenantID, brand.ID) {
		return model.Brand{}, gorm.ErrRecordNotFound
	}
	if brand.IsDefault {
		return model.Brand{}, errors.New("brand is tenant default")
	}
	if err := s.repo.EnsureDeleteReferencesClear([]deleteReferenceCheck{
		{model: &model.Tenant{}, query: "default_brand_id = ?", args: []any{brand.ID}, message: "brand is tenant default"},
		{model: &model.Agent{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active agents"},
		{model: &model.Player{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active players"},
		{model: &model.Game{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active games"},
		{model: &model.PlatformConfig{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active platform configs"},
		{model: &model.InviteCode{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active invite codes"},
		{model: &model.Binding{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active bindings"},
		{model: &model.WithdrawalRequest{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active withdrawal requests"},
		{model: &model.RechargeOrder{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active recharge orders"},
		{model: &model.CommissionRecord{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active commission records"},
		{model: &model.CommissionRule{}, query: "brand_id = ?", args: []any{brand.ID}, message: "brand has active commission rules"},
	}); err != nil {
		return model.Brand{}, err
	}
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		if err := s.repo.ArchiveDeletedScopedCode(tx, &model.Brand{}, brand.ID, "code", brand.Code); err != nil {
			return err
		}
		if err := tx.Delete(&brand).Error; err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventBrandDeleted,
			AggregateType:  "brand",
			AggregateID:    fmt.Sprintf("%d", brand.ID),
			TenantID:       &brand.TenantID,
			BrandID:        &brand.ID,
			OccurredAt:     time.Now().UTC(),
			Producer:       "tenant-service",
			IdempotencyKey: "brand.deleted:" + fmt.Sprintf("%d", brand.ID),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        brandDeletedPayload(brand),
		})
		return err
	}); err != nil {
		return model.Brand{}, err
	}
	return brand, nil
}

type deleteReferenceCheck struct {
	model   any
	query   string
	args    []any
	message string
}

func defaultTenantStatus(status model.TenantStatus) model.TenantStatus {
	if status == "" {
		return model.TenantStatusActive
	}
	return status
}

func defaultBrandStatus(status model.BrandStatus) model.BrandStatus {
	if status == "" {
		return model.BrandStatusActive
	}
	return status
}
