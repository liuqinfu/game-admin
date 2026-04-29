package audit

import (
	"strings"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	List(scope shared.Scope, filter ListFilter) ([]model.OperationAuditLog, error)
	Create(log *model.OperationAuditLog) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) List(scope shared.Scope, filter ListFilter) ([]model.OperationAuditLog, error) {
	query := r.db.Order("id desc")
	if value := strings.TrimSpace(filter.Module); value != "" {
		query = query.Where("module = ?", value)
	}
	if value := strings.TrimSpace(filter.Action); value != "" {
		query = query.Where("action = ?", value)
	}
	if value := strings.TrimSpace(filter.TargetID); value != "" {
		query = query.Where("target_id = ?", value)
	}
	var items []model.OperationAuditLog
	return items, query.Find(&items).Error
}

func (r *gormRepository) Create(log *model.OperationAuditLog) error {
	return r.db.Create(log).Error
}
