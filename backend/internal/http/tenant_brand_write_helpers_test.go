package http

import (
	"errors"

	"game-admin/backend/internal/domain/model"
	"gorm.io/gorm"
)

func createBrand(db *gorm.DB, brand *model.Brand) error {
	if db == nil {
		return errors.New("db is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(brand).Error; err != nil {
			return err
		}
		if !brand.IsDefault {
			return nil
		}
		if err := tx.Model(&model.Brand{}).
			Where("tenant_id = ? AND id <> ?", brand.TenantID, brand.ID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.Tenant{}).
			Where("id = ?", brand.TenantID).
			Update("default_brand_id", brand.ID).Error
	})
}
