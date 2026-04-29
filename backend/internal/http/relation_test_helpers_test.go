package http

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	sharedsvc "game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

func ensureAgentExists(db *gorm.DB, agentID uint64) error {
	var agent model.Agent
	return db.Select("id").First(&agent, agentID).Error
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

func loadAgentForScope(tx *gorm.DB, agentID uint64) (model.Agent, error) {
	var agent model.Agent
	if err := tx.First(&agent, agentID).Error; err != nil {
		return model.Agent{}, err
	}
	return agent, nil
}

func loadGameForScope(tx *gorm.DB, gameID uint64) (model.Game, error) {
	var game model.Game
	if err := tx.First(&game, gameID).Error; err != nil {
		return model.Game{}, err
	}
	return game, nil
}

func rebuildAgentRelationClosure(tx *gorm.DB) error {
	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&model.AgentRelationClosure{}).Error; err != nil {
		return err
	}

	var agents []model.Agent
	if err := tx.Select("id,parent_agent_id,tenant_id,brand_id").Order("id asc").Find(&agents).Error; err != nil {
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
				return errAgentInviteLoopDetected
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

	if len(closures) == 0 {
		return nil
	}

	for i := range closures {
		if err := tx.Omit("BaseModel").Create(&closures[i]).Error; err != nil {
			return fmt.Errorf("insert closure[%d] ancestor=%d descendant=%d depth=%d via=%v failed: %w", i, closures[i].AncestorAgentID, closures[i].DescendantAgentID, closures[i].Depth, closures[i].ViaDirectParentID, err)
		}
	}
	return nil
}

func detectRelationLoop(tx *gorm.DB, inviterAgentID, applicantAgentID uint64) error {
	if inviterAgentID == 0 || applicantAgentID == 0 {
		return nil
	}
	if inviterAgentID == applicantAgentID {
		return errAgentInviteLoopDetected
	}
	var count int64
	if err := tx.Model(&model.AgentRelationClosure{}).
		Where("ancestor_agent_id = ? AND descendant_agent_id = ? AND status = ?", applicantAgentID, inviterAgentID, model.RelationStatusActive).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errAgentInviteLoopDetected
	}
	return nil
}

func createAgentRelation(tx *gorm.DB, inviterAgentID, applicantAgentID uint64) (uint64, error) {
	if err := detectRelationLoop(tx, inviterAgentID, applicantAgentID); err != nil {
		return 0, err
	}
	inviter, err := loadAgentForScope(tx, inviterAgentID)
	if err != nil {
		return 0, err
	}
	applicant, err := loadAgentForScope(tx, applicantAgentID)
	if err != nil {
		return 0, err
	}
	if err := sharedsvc.EnsureSameScope("applicant agent", inviter.TenantID, inviter.BrandID, applicant.TenantID, applicant.BrandID); err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	if err := tx.Model(&model.Relation{}).
		Where("descendant_agent_id = ? AND depth = ? AND status = ?", applicantAgentID, 1, model.RelationStatusActive).
		Updates(map[string]any{"status": model.RelationStatusInactive, "effective_to": now, "updated_at": now}).Error; err != nil {
		return 0, err
	}
	if err := tx.Model(&model.Agent{}).
		Where("id = ?", applicantAgentID).
		Updates(map[string]any{"parent_agent_id": inviterAgentID, "updated_at": now}).Error; err != nil {
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
	if err := tx.Create(&relation).Error; err != nil {
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
		Producer:       "http-relation-helper",
		IdempotencyKey: "agent.relation.created:" + strconv.FormatUint(relation.ID, 10),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        relation,
	}); err != nil {
		return 0, err
	}
	return relation.ID, nil
}

func applyHierarchyDepthRange(query *gorm.DB, depthRange hierarchyDepthRange) *gorm.DB {
	query = query.Where("depth >= ?", depthRange.Min)
	if depthRange.Max != nil {
		query = query.Where("depth <= ?", *depthRange.Max)
	}
	return query
}
