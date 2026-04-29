package game

import (
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	ListGames(scope sharedsvc.Scope, filter ListFilter) ([]model.Game, error)
	CreateGame(game *model.Game) error
	FindGame(id uint64) (model.Game, error)
	SaveGame(game *model.Game) error
	DeleteReferenceCount(modelValue any, query string, args ...any) (int64, error)
	CreateArchivedGameAndDeleteTx(tx *gorm.DB, game *model.Game) error
	ListIntegrationKeys(game model.Game) ([]model.GameIntegrationKey, error)
	FindIntegrationKey(game model.Game, keyID uint64) (model.GameIntegrationKey, error)
	FindIntegrationKeyByAccessKey(accessKey string) (model.GameIntegrationKey, error)
	UpdateIntegrationKeySecret(keyID uint64, secret string, rotatedAt time.Time) error
	TouchIntegrationKeyLastUsed(keyID uint64, usedAt time.Time) error
	ListAccess(scope sharedsvc.Scope, filter AccessFilter) ([]AccessListItem, error)
	ListAccessForGame(game model.Game, filter OpenAPIAccessFilter) ([]AccessListItem, error)
	FindAgent(id uint64) (model.Agent, error)
	FindGameTx(tx *gorm.DB, id uint64) (model.Game, error)
	FindAgentTx(tx *gorm.DB, id uint64) (model.Agent, error)
	FindAgentGameAccessTx(tx *gorm.DB, agentID, gameID uint64) (model.AgentGameAccess, error)
	CreateAgentGameAccessTx(tx *gorm.DB, access *model.AgentGameAccess) error
	SaveAgentGameAccessTx(tx *gorm.DB, access *model.AgentGameAccess) error
	FindAgentGameAccess(agentID, gameID uint64) (model.AgentGameAccess, error)
	DeleteAgentGameAccess(access *model.AgentGameAccess) error
	CreateGameIntegrationKey(credential *model.GameIntegrationKey) error
	WithTx(func(*gorm.DB) error) error
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) ListGames(scope sharedsvc.Scope, filter ListFilter) ([]model.Game, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Order("id desc"), scope, "tenant_id", "brand_id")
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("game_code LIKE ? OR name LIKE ? OR vendor LIKE ? OR category LIKE ?", like, like, like, like)
	}
	var items []model.Game
	return items, query.Find(&items).Error
}

func (r *gormRepository) CreateGame(game *model.Game) error {
	return r.db.Create(game).Error
}

func (r *gormRepository) FindGame(id uint64) (model.Game, error) {
	var game model.Game
	return game, r.db.First(&game, id).Error
}

func (r *gormRepository) SaveGame(game *model.Game) error {
	return r.db.Save(game).Error
}

func (r *gormRepository) DeleteReferenceCount(modelValue any, query string, args ...any) (int64, error) {
	var count int64
	return count, r.db.Model(modelValue).Where(query, args...).Count(&count).Error
}

func (r *gormRepository) CreateArchivedGameAndDeleteTx(tx *gorm.DB, game *model.Game) error {
	if err := archiveDeletedScopedCode(tx, &model.Game{}, game.ID, "game_code", game.GameCode); err != nil {
		return err
	}
	return tx.Delete(game).Error
}

func (r *gormRepository) ListIntegrationKeys(game model.Game) ([]model.GameIntegrationKey, error) {
	query := r.db.Where("game_id = ?", game.ID).Order("id desc")
	query = sharedsvc.ApplyExactTenantBrandScope(query, game.TenantID, game.BrandID, "tenant_id", "brand_id")
	var credentials []model.GameIntegrationKey
	return credentials, query.Find(&credentials).Error
}

func (r *gormRepository) FindIntegrationKey(game model.Game, keyID uint64) (model.GameIntegrationKey, error) {
	query := r.db.Where("id = ? AND game_id = ?", keyID, game.ID)
	query = sharedsvc.ApplyExactTenantBrandScope(query, game.TenantID, game.BrandID, "tenant_id", "brand_id")
	var credential model.GameIntegrationKey
	return credential, query.First(&credential).Error
}

func (r *gormRepository) FindIntegrationKeyByAccessKey(accessKey string) (model.GameIntegrationKey, error) {
	var credential model.GameIntegrationKey
	return credential, r.db.Where("access_key = ?", strings.TrimSpace(accessKey)).First(&credential).Error
}

func (r *gormRepository) UpdateIntegrationKeySecret(keyID uint64, secret string, rotatedAt time.Time) error {
	return r.db.Model(&model.GameIntegrationKey{}).Where("id = ?", keyID).Updates(map[string]any{
		"secret_ciphertext": secret,
		"rotated_at":        &rotatedAt,
	}).Error
}

func (r *gormRepository) TouchIntegrationKeyLastUsed(keyID uint64, usedAt time.Time) error {
	return r.db.Model(&model.GameIntegrationKey{}).Where("id = ?", keyID).Update("last_used_at", usedAt).Error
}

func (r *gormRepository) ListAccess(scope sharedsvc.Scope, filter AccessFilter) ([]AccessListItem, error) {
	query := sharedsvc.ApplyTenantBrandScope(r.db.Table("agent_game_access AS aga"), scope, "aga.tenant_id", "aga.brand_id").
		Select("aga.*, agent.name AS agent_name, game.game_code AS game_code, game.name AS game_name").
		Joins("JOIN agent ON agent.id = aga.agent_id").
		Joins("JOIN game ON game.id = aga.game_id").
		Order("aga.id desc")
	query = query.Where("((aga.tenant_id IS NULL AND agent.tenant_id IS NULL) OR aga.tenant_id = agent.tenant_id)")
	query = query.Where("((aga.brand_id IS NULL AND agent.brand_id IS NULL) OR aga.brand_id = agent.brand_id)")
	query = query.Where("((aga.tenant_id IS NULL AND game.tenant_id IS NULL) OR aga.tenant_id = game.tenant_id)")
	query = query.Where("((aga.brand_id IS NULL AND game.brand_id IS NULL) OR aga.brand_id = game.brand_id)")
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("aga.agent_id = ?", value)
	}
	if value := strings.TrimSpace(filter.GameID); value != "" {
		query = query.Where("aga.game_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("aga.status = ?", value)
	}
	var items []AccessListItem
	return items, query.Scan(&items).Error
}

func (r *gormRepository) ListAccessForGame(game model.Game, filter OpenAPIAccessFilter) ([]AccessListItem, error) {
	query := r.db.Table("agent_game_access AS aga").
		Select("aga.*, agent.name AS agent_name, game.game_code AS game_code, game.name AS game_name").
		Joins("JOIN agent ON agent.id = aga.agent_id").
		Joins("JOIN game ON game.id = aga.game_id").
		Where("aga.game_id = ?", game.ID).
		Order("aga.id desc")
	query = sharedsvc.ApplyRecordAndOwnerTenantBrandScope(query, game.TenantID, game.BrandID, game.TenantID, game.BrandID, "aga.tenant_id", "aga.brand_id", "game.tenant_id", "game.brand_id")
	query = sharedsvc.ApplyExactTenantBrandScope(query, game.TenantID, game.BrandID, "agent.tenant_id", "agent.brand_id")
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("aga.agent_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("aga.status = ?", value)
	}
	var items []AccessListItem
	return items, query.Scan(&items).Error
}

func (r *gormRepository) FindAgent(id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, r.db.First(&agent, id).Error
}

func (r *gormRepository) FindGameTx(tx *gorm.DB, id uint64) (model.Game, error) {
	var game model.Game
	return game, tx.First(&game, id).Error
}

func (r *gormRepository) FindAgentTx(tx *gorm.DB, id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, tx.First(&agent, id).Error
}

func (r *gormRepository) FindAgentGameAccessTx(tx *gorm.DB, agentID, gameID uint64) (model.AgentGameAccess, error) {
	var access model.AgentGameAccess
	return access, tx.Where("agent_id = ? AND game_id = ?", agentID, gameID).First(&access).Error
}

func (r *gormRepository) CreateAgentGameAccessTx(tx *gorm.DB, access *model.AgentGameAccess) error {
	return tx.Create(access).Error
}

func (r *gormRepository) SaveAgentGameAccessTx(tx *gorm.DB, access *model.AgentGameAccess) error {
	return tx.Save(access).Error
}

func (r *gormRepository) FindAgentGameAccess(agentID, gameID uint64) (model.AgentGameAccess, error) {
	var access model.AgentGameAccess
	return access, r.db.Where("agent_id = ? AND game_id = ?", agentID, gameID).First(&access).Error
}

func (r *gormRepository) DeleteAgentGameAccess(access *model.AgentGameAccess) error {
	return r.db.Delete(access).Error
}

func (r *gormRepository) CreateGameIntegrationKey(credential *model.GameIntegrationKey) error {
	return r.db.Create(credential).Error
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error {
	return r.db.Transaction(run)
}
