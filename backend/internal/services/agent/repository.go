package agent

import (
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Repository interface {
	CreateAgent(*model.Agent) error
	FindAgent(id uint64) (model.Agent, error)
	SaveAgent(*model.Agent) error
	WithTx(func(*gorm.DB) error) error
	ListAgents(scope shared.Scope, filter AgentListFilter) ([]model.Agent, error)

	ListInviteCodes(scope shared.Scope, filter InviteCodeListFilter) ([]model.InviteCode, error)
	ListPlayers(scope shared.Scope, agentIDs []uint64) ([]model.Player, error)
	CreatePlayer(*model.Player) error
	ListBindings(scope shared.Scope) ([]model.Binding, error)
	ListBindingHistory(scope shared.Scope, filter BindingHistoryListFilter) ([]model.BindingHistory, error)
	ListInviteApplications(scope shared.Scope, filter InviteApplicationListFilter) ([]model.AgentInviteApplication, error)
	FindInviteApplicationInScope(scope shared.Scope, id uint64) (model.AgentInviteApplication, error)

	FindPlayer(id uint64) (model.Player, error)
	FindInviteCodeByCode(code string) (model.InviteCode, error)
	CountBoundPlayerBindingsInAgentIDs(playerID uint64, agentIDs []uint64) (int64, error)
	CurrentAgentScopeIDs(agentID uint64) ([]uint64, error)

	FindAgentTx(tx *gorm.DB, id uint64) (model.Agent, error)
	FindPlayerTx(tx *gorm.DB, id uint64) (model.Player, error)
	FindInviteCodeByCodeTx(tx *gorm.DB, code string) (model.InviteCode, error)
	FindPrimaryInviteCodeByAgentTx(tx *gorm.DB, agentID uint64) (model.InviteCode, error)
	FindPlayerByPlatformUserIDTx(tx *gorm.DB, platformUserID string) (model.Player, error)
	FindBoundBindingByPlayerTx(tx *gorm.DB, playerID uint64) (model.Binding, error)
	FindPendingInviteApplicationTx(tx *gorm.DB, applicantAgentID, inviterAgentID uint64) (model.AgentInviteApplication, error)
	CreateInviteCodeTx(tx *gorm.DB, inviteCode *model.InviteCode) error
	CreatePlayerTx(tx *gorm.DB, player *model.Player) error
	UpdatePlayerRegisterGameTx(tx *gorm.DB, playerID, gameID uint64) error
	CreateBindingTx(tx *gorm.DB, binding *model.Binding) error
	IncrementInviteCodeUsageTx(tx *gorm.DB, inviteCodeID uint64, at time.Time) error
	CreateBindingHistoryTx(tx *gorm.DB, history *model.BindingHistory) error
	CreateInviteApplicationTx(tx *gorm.DB, application *model.AgentInviteApplication) error
	SaveInviteApplicationTx(tx *gorm.DB, application *model.AgentInviteApplication) error
	ReloadInviteApplicationTx(tx *gorm.DB, id uint64) (model.AgentInviteApplication, error)
	DeactivateActiveDirectRelationsTx(tx *gorm.DB, applicantAgentID uint64, at time.Time) error
	UpdateAgentParentTx(tx *gorm.DB, agentID, parentAgentID uint64, at time.Time) error
	CreateRelationTx(tx *gorm.DB, relation *model.Relation) error
	RelationLoopExistsTx(tx *gorm.DB, applicantAgentID, inviterAgentID uint64) (bool, error)
	ClearRelationClosuresTx(tx *gorm.DB) error
	ListAgentsForClosureRebuildTx(tx *gorm.DB) ([]model.Agent, error)
	CreateRelationClosureTx(tx *gorm.DB, closure *model.AgentRelationClosure) error

	ListAncestorClosures(agentID uint64, depthRange HierarchyDepthRange) ([]model.AgentRelationClosure, error)
	ListDescendantClosures(agentID uint64, depthRange HierarchyDepthRange) ([]model.AgentRelationClosure, error)
	CountDirectActiveDescendants(agentID uint64) (int64, error)
	CountDescendantClosures(agentID uint64, depthRange HierarchyDepthRange) (int64, error)
	CountAncestorClosures(agentID uint64, depthRange HierarchyDepthRange) (int64, error)
	MaxDescendantDepth(agentID uint64, depthRange HierarchyDepthRange) (uint32, error)
	MaxAncestorDepth(agentID uint64, depthRange HierarchyDepthRange) (uint32, error)
	ListDepthBreakdown(agentID uint64, depthRange HierarchyDepthRange) ([]DepthStat, error)
	ListLeafDescendantIDs(agentID uint64, depthRange HierarchyDepthRange) ([]uint64, error)
}

type gormRepository struct {
	db *gorm.DB
}

func newRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) CreateAgent(agent *model.Agent) error {
	return r.db.Create(agent).Error
}

func (r *gormRepository) FindAgent(id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, r.db.First(&agent, id).Error
}

func (r *gormRepository) SaveAgent(agent *model.Agent) error {
	return r.db.Save(agent).Error
}

func (r *gormRepository) WithTx(run func(*gorm.DB) error) error {
	return r.db.Transaction(run)
}

func (r *gormRepository) ListAgents(scope shared.Scope, filter AgentListFilter) ([]model.Agent, error) {
	query := shared.ApplyTenantBrandScope(r.db.Order("id desc"), scope, "tenant_id", "brand_id")
	if scope.AgentID != nil {
		ids, err := r.CurrentAgentScopeIDs(*scope.AgentID)
		if err != nil {
			return nil, err
		}
		query = query.Where("id IN ?", ids)
	}
	if value := strings.TrimSpace(filter.TenantID); value != "" {
		query = query.Where("tenant_id = ?", value)
	}
	if value := strings.TrimSpace(filter.BrandID); value != "" {
		query = query.Where("brand_id = ?", value)
	}
	var items []model.Agent
	return items, query.Find(&items).Error
}

func (r *gormRepository) ListInviteCodes(scope shared.Scope, filter InviteCodeListFilter) ([]model.InviteCode, error) {
	query := shared.ApplyTenantBrandScope(r.db.Order("id desc"), scope, "tenant_id", "brand_id")
	if scope.AgentID != nil {
		query = query.Where("agent_id = ?", *scope.AgentID)
	}
	if value := strings.TrimSpace(filter.AgentID); value != "" {
		query = query.Where("agent_id = ?", value)
	}
	var items []model.InviteCode
	return items, query.Find(&items).Error
}

func (r *gormRepository) ListPlayers(scope shared.Scope, agentIDs []uint64) ([]model.Player, error) {
	query := shared.ApplyTenantBrandScope(r.db.Model(&model.Player{}).Order("player.id desc"), scope, "player.tenant_id", "player.brand_id")
	if len(agentIDs) > 0 {
		query = query.Joins("JOIN user_agent_binding pab ON pab.player_id = player.id AND pab.status = ?", model.BindingStatusBound).Where("pab.agent_id IN ?", agentIDs)
	}
	var items []model.Player
	return items, query.Find(&items).Error
}

func (r *gormRepository) CreatePlayer(player *model.Player) error {
	return r.db.Create(player).Error
}

func (r *gormRepository) ListBindings(scope shared.Scope) ([]model.Binding, error) {
	query := shared.ApplyTenantBrandScope(r.db.Order("id desc"), scope, "tenant_id", "brand_id")
	var items []model.Binding
	return items, query.Find(&items).Error
}

func (r *gormRepository) ListBindingHistory(scope shared.Scope, filter BindingHistoryListFilter) ([]model.BindingHistory, error) {
	query := shared.ApplyTenantBrandScope(r.db.Order("id desc"), scope, "tenant_id", "brand_id")
	if value := strings.TrimSpace(filter.PlayerID); value != "" {
		query = query.Where("player_id = ?", value)
	}
	var items []model.BindingHistory
	return items, query.Find(&items).Error
}

func (r *gormRepository) ListInviteApplications(scope shared.Scope, filter InviteApplicationListFilter) ([]model.AgentInviteApplication, error) {
	query := shared.ApplyTenantBrandScope(r.db.Order("id desc"), scope, "tenant_id", "brand_id")
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	var items []model.AgentInviteApplication
	return items, query.Find(&items).Error
}

func (r *gormRepository) FindInviteApplicationInScope(scope shared.Scope, id uint64) (model.AgentInviteApplication, error) {
	query := shared.ApplyTenantBrandScope(r.db.Model(&model.AgentInviteApplication{}), scope, "tenant_id", "brand_id")
	var item model.AgentInviteApplication
	return item, query.First(&item, id).Error
}

func (r *gormRepository) FindPlayer(id uint64) (model.Player, error) {
	var player model.Player
	return player, r.db.First(&player, id).Error
}

func (r *gormRepository) FindInviteCodeByCode(code string) (model.InviteCode, error) {
	var inviteCode model.InviteCode
	return inviteCode, r.db.Where("code = ?", strings.TrimSpace(code)).First(&inviteCode).Error
}

func (r *gormRepository) CountBoundPlayerBindingsInAgentIDs(playerID uint64, agentIDs []uint64) (int64, error) {
	var count int64
	err := r.db.Model(&model.Binding{}).Where("player_id = ? AND agent_id IN ? AND status = ?", playerID, agentIDs, model.BindingStatusBound).Count(&count).Error
	return count, err
}

func (r *gormRepository) CurrentAgentScopeIDs(agentID uint64) ([]uint64, error) {
	return shared.CurrentAgentScopeIDs(r.db, agentID)
}

func (r *gormRepository) FindAgentTx(tx *gorm.DB, id uint64) (model.Agent, error) {
	var agent model.Agent
	return agent, tx.First(&agent, id).Error
}

func (r *gormRepository) FindPlayerTx(tx *gorm.DB, id uint64) (model.Player, error) {
	var player model.Player
	return player, tx.First(&player, id).Error
}

func (r *gormRepository) FindInviteCodeByCodeTx(tx *gorm.DB, code string) (model.InviteCode, error) {
	var inviteCode model.InviteCode
	return inviteCode, tx.Where("code = ?", strings.TrimSpace(code)).First(&inviteCode).Error
}

func (r *gormRepository) FindPrimaryInviteCodeByAgentTx(tx *gorm.DB, agentID uint64) (model.InviteCode, error) {
	var inviteCode model.InviteCode
	return inviteCode, tx.Where("agent_id = ? AND is_primary = ?", agentID, true).First(&inviteCode).Error
}

func (r *gormRepository) FindPlayerByPlatformUserIDTx(tx *gorm.DB, platformUserID string) (model.Player, error) {
	var player model.Player
	return player, tx.Where("platform_user_id = ?", strings.TrimSpace(platformUserID)).First(&player).Error
}

func (r *gormRepository) FindBoundBindingByPlayerTx(tx *gorm.DB, playerID uint64) (model.Binding, error) {
	var binding model.Binding
	return binding, tx.Where("player_id = ? AND status = ?", playerID, model.BindingStatusBound).First(&binding).Error
}

func (r *gormRepository) FindPendingInviteApplicationTx(tx *gorm.DB, applicantAgentID, inviterAgentID uint64) (model.AgentInviteApplication, error) {
	var application model.AgentInviteApplication
	return application, tx.Where("applicant_agent_id = ? AND inviter_agent_id = ? AND status = ?", applicantAgentID, inviterAgentID, model.AgentInviteApplicationStatusPending).First(&application).Error
}

func (r *gormRepository) CreateInviteCodeTx(tx *gorm.DB, inviteCode *model.InviteCode) error {
	return tx.Create(inviteCode).Error
}

func (r *gormRepository) CreatePlayerTx(tx *gorm.DB, player *model.Player) error {
	return tx.Create(player).Error
}

func (r *gormRepository) UpdatePlayerRegisterGameTx(tx *gorm.DB, playerID, gameID uint64) error {
	return tx.Model(&model.Player{}).Where("id = ?", playerID).Update("register_game_id", gameID).Error
}

func (r *gormRepository) CreateBindingTx(tx *gorm.DB, binding *model.Binding) error {
	return tx.Create(binding).Error
}

func (r *gormRepository) IncrementInviteCodeUsageTx(tx *gorm.DB, inviteCodeID uint64, at time.Time) error {
	return tx.Model(&model.InviteCode{}).Where("id = ?", inviteCodeID).Updates(map[string]any{
		"used_count":   gorm.Expr("used_count + 1"),
		"last_used_at": at,
	}).Error
}

func (r *gormRepository) CreateBindingHistoryTx(tx *gorm.DB, history *model.BindingHistory) error {
	return tx.Create(history).Error
}

func (r *gormRepository) CreateInviteApplicationTx(tx *gorm.DB, application *model.AgentInviteApplication) error {
	return tx.Create(application).Error
}

func (r *gormRepository) SaveInviteApplicationTx(tx *gorm.DB, application *model.AgentInviteApplication) error {
	return tx.Save(application).Error
}

func (r *gormRepository) ReloadInviteApplicationTx(tx *gorm.DB, id uint64) (model.AgentInviteApplication, error) {
	var application model.AgentInviteApplication
	return application, tx.First(&application, id).Error
}

func (r *gormRepository) DeactivateActiveDirectRelationsTx(tx *gorm.DB, applicantAgentID uint64, at time.Time) error {
	return tx.Model(&model.Relation{}).
		Where("descendant_agent_id = ? AND depth = ? AND status = ?", applicantAgentID, 1, model.RelationStatusActive).
		Updates(map[string]any{"status": model.RelationStatusInactive, "effective_to": at, "updated_at": at}).Error
}

func (r *gormRepository) UpdateAgentParentTx(tx *gorm.DB, agentID, parentAgentID uint64, at time.Time) error {
	return tx.Model(&model.Agent{}).
		Where("id = ?", agentID).
		Updates(map[string]any{"parent_agent_id": parentAgentID, "updated_at": at}).Error
}

func (r *gormRepository) CreateRelationTx(tx *gorm.DB, relation *model.Relation) error {
	return tx.Create(relation).Error
}

func (r *gormRepository) RelationLoopExistsTx(tx *gorm.DB, applicantAgentID, inviterAgentID uint64) (bool, error) {
	var count int64
	err := tx.Model(&model.AgentRelationClosure{}).
		Where("ancestor_agent_id = ? AND descendant_agent_id = ? AND status = ?", applicantAgentID, inviterAgentID, model.RelationStatusActive).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) ClearRelationClosuresTx(tx *gorm.DB) error {
	return tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&model.AgentRelationClosure{}).Error
}

func (r *gormRepository) ListAgentsForClosureRebuildTx(tx *gorm.DB) ([]model.Agent, error) {
	var agents []model.Agent
	return agents, tx.Select("id,parent_agent_id,tenant_id,brand_id").Order("id asc").Find(&agents).Error
}

func (r *gormRepository) CreateRelationClosureTx(tx *gorm.DB, closure *model.AgentRelationClosure) error {
	return tx.Omit("BaseModel").Create(closure).Error
}

func (r *gormRepository) ListAncestorClosures(agentID uint64, depthRange HierarchyDepthRange) ([]model.AgentRelationClosure, error) {
	var closures []model.AgentRelationClosure
	query := r.db.Where("descendant_agent_id = ? AND status = ?", agentID, model.RelationStatusActive)
	query = applyHierarchyDepthRange(query, depthRange)
	return closures, query.Order("depth asc, ancestor_agent_id asc").Find(&closures).Error
}

func (r *gormRepository) ListDescendantClosures(agentID uint64, depthRange HierarchyDepthRange) ([]model.AgentRelationClosure, error) {
	var closures []model.AgentRelationClosure
	query := r.db.Where("ancestor_agent_id = ? AND status = ?", agentID, model.RelationStatusActive)
	query = applyHierarchyDepthRange(query, depthRange)
	return closures, query.Order("depth asc, descendant_agent_id asc").Find(&closures).Error
}

func (r *gormRepository) CountDirectActiveDescendants(agentID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&model.AgentRelationClosure{}).Where("ancestor_agent_id = ? AND depth = ? AND status = ?", agentID, 1, model.RelationStatusActive).Count(&count).Error
	return count, err
}

func (r *gormRepository) CountDescendantClosures(agentID uint64, depthRange HierarchyDepthRange) (int64, error) {
	var count int64
	query := applyHierarchyDepthRange(r.db.Model(&model.AgentRelationClosure{}).Where("ancestor_agent_id = ? AND status = ?", agentID, model.RelationStatusActive), depthRange)
	return count, query.Count(&count).Error
}

func (r *gormRepository) CountAncestorClosures(agentID uint64, depthRange HierarchyDepthRange) (int64, error) {
	var count int64
	query := applyHierarchyDepthRange(r.db.Model(&model.AgentRelationClosure{}).Where("descendant_agent_id = ? AND status = ?", agentID, model.RelationStatusActive), depthRange)
	return count, query.Count(&count).Error
}

func (r *gormRepository) MaxDescendantDepth(agentID uint64, depthRange HierarchyDepthRange) (uint32, error) {
	type row struct{ MaxDepth uint32 }
	var result row
	query := applyHierarchyDepthRange(r.db.Model(&model.AgentRelationClosure{}).Select("COALESCE(MAX(depth), 0) AS max_depth").Where("ancestor_agent_id = ? AND status = ?", agentID, model.RelationStatusActive), depthRange)
	return result.MaxDepth, query.Scan(&result).Error
}

func (r *gormRepository) MaxAncestorDepth(agentID uint64, depthRange HierarchyDepthRange) (uint32, error) {
	type row struct{ MaxDepth uint32 }
	var result row
	query := applyHierarchyDepthRange(r.db.Model(&model.AgentRelationClosure{}).Select("COALESCE(MAX(depth), 0) AS max_depth").Where("descendant_agent_id = ? AND status = ?", agentID, model.RelationStatusActive), depthRange)
	return result.MaxDepth, query.Scan(&result).Error
}

func (r *gormRepository) ListDepthBreakdown(agentID uint64, depthRange HierarchyDepthRange) ([]DepthStat, error) {
	type row struct {
		Depth uint32
		Count int64
	}
	var rows []row
	query := applyHierarchyDepthRange(r.db.Model(&model.AgentRelationClosure{}).Select("depth, COUNT(*) AS count").Where("ancestor_agent_id = ? AND status = ?", agentID, model.RelationStatusActive), depthRange)
	if err := query.Group("depth").Order("depth asc").Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]DepthStat, 0, len(rows))
	for _, item := range rows {
		items = append(items, DepthStat{Depth: item.Depth, Count: item.Count})
	}
	return items, nil
}

func (r *gormRepository) ListLeafDescendantIDs(agentID uint64, depthRange HierarchyDepthRange) ([]uint64, error) {
	type row struct{ DescendantAgentID uint64 }
	var rows []row
	query := applyHierarchyDepthRange(r.db.Model(&model.AgentRelationClosure{}).Select("descendant_agent_id").Where("ancestor_agent_id = ? AND status = ?", agentID, model.RelationStatusActive), depthRange)
	if err := query.Group("descendant_agent_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]uint64, 0, len(rows))
	for _, item := range rows {
		items = append(items, item.DescendantAgentID)
	}
	return items, nil
}
