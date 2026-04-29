package tenant

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	ListTenants(scope shared.Scope) ([]model.Tenant, error)
	FindTenant(id uint64) (model.Tenant, error)
	CreateTenant(*model.Tenant) error
	SaveTenant(*model.Tenant) error
	DeleteTenant(*model.Tenant) error

	ListBrands(scope shared.Scope, tenantID *uint64) ([]model.Brand, error)
	FindBrand(id uint64) (model.Brand, error)
	CreateBrand(*model.Brand) error
	SaveBrand(*model.Brand) error
	DeleteBrand(*model.Brand) error

	EnsureDeleteReferencesClear([]deleteReferenceCheck) error
	ArchiveDeletedScopedCode(tx *gorm.DB, modelValue any, id uint64, column string, current string) error
	WithTx(func(*gorm.DB) error) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListTenants(scope shared.Scope) ([]model.Tenant, error) {
	var items []model.Tenant
	query := r.db.Order("id desc")
	if len(scope.TenantIDs) > 0 {
		query = query.Where("id IN ?", shared.UniqueUint64(scope.TenantIDs))
	}
	return items, query.Find(&items).Error
}

func (r *gormRepository) FindTenant(id uint64) (model.Tenant, error) {
	var tenant model.Tenant
	return tenant, r.db.First(&tenant, id).Error
}

func (r *gormRepository) CreateTenant(tenant *model.Tenant) error {
	return r.db.Create(tenant).Error
}

func (r *gormRepository) SaveTenant(tenant *model.Tenant) error {
	return r.db.Save(tenant).Error
}

func (r *gormRepository) DeleteTenant(tenant *model.Tenant) error {
	return r.db.Delete(tenant).Error
}

func (r *gormRepository) ListBrands(scope shared.Scope, tenantID *uint64) ([]model.Brand, error) {
	query := r.db.Order("id desc")
	if len(scope.TenantIDs) > 0 {
		query = query.Where("tenant_id IN ?", shared.UniqueUint64(scope.TenantIDs))
	}
	if len(scope.BrandIDs) > 0 {
		query = query.Where("id IN ?", shared.UniqueUint64(scope.BrandIDs))
	}
	if tenantID != nil && *tenantID > 0 {
		query = query.Where("tenant_id = ?", *tenantID)
	}
	var items []model.Brand
	return items, query.Find(&items).Error
}

func (r *gormRepository) FindBrand(id uint64) (model.Brand, error) {
	var brand model.Brand
	return brand, r.db.First(&brand, id).Error
}

func (r *gormRepository) CreateBrand(brand *model.Brand) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(brand).Error; err != nil {
			return err
		}
		if !brand.IsDefault {
			return nil
		}
		if err := tx.Model(&model.Brand{}).Where("tenant_id = ? AND id <> ?", brand.TenantID, brand.ID).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.Tenant{}).Where("id = ?", brand.TenantID).Update("default_brand_id", brand.ID).Error
	})
}

func (r *gormRepository) SaveBrand(brand *model.Brand) error {
	return r.db.Save(brand).Error
}

func (r *gormRepository) DeleteBrand(brand *model.Brand) error {
	return r.db.Delete(brand).Error
}

func (r *gormRepository) EnsureDeleteReferencesClear(checks []deleteReferenceCheck) error {
	for _, check := range checks {
		var count int64
		if err := r.db.Model(check.model).Where(check.query, check.args...).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New(check.message)
		}
	}
	return nil
}

func (r *gormRepository) ArchiveDeletedScopedCode(tx *gorm.DB, modelValue any, id uint64, column string, current string) error {
	if strings.TrimSpace(current) == "" {
		return nil
	}
	archived := fmt.Sprintf("%s__deleted_%d_%d", current, id, time.Now().UTC().Unix())
	return tx.Model(modelValue).Where("id = ?", id).Update(column, archived).Error
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error {
	return r.db.Transaction(run)
}
