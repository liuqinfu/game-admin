package agent

import (
	"errors"
	"slices"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type HierarchyDepthRange struct {
	Min uint32
	Max *uint32
}

type HierarchyNode struct {
	AncestorAgentID   uint64
	DescendantAgentID uint64
	Depth             uint32
	RelationType      model.RelationType
	Status            model.RelationStatus
	ViaDirectParentID *uint64
}

type TeamStats struct {
	AgentID            uint64
	DirectDescendants  int64
	TotalDescendants   int64
	TotalAncestors     int64
	MaxDescendantDepth uint32
	MaxAncestorDepth   uint32
	LeafDescendants    int64
	DepthBreakdown     []DepthStat
}

type DepthStat struct {
	Depth uint32
	Count int64
}

func (s *Service) Ancestors(scope shared.Scope, agentID uint64, depthRange HierarchyDepthRange) ([]HierarchyNode, error) {
	if _, err := s.ensureAgentInCurrentScope(scope, agentID); err != nil {
		return nil, err
	}
	items, err := s.loadAncestors(agentID, depthRange)
	if err != nil {
		return nil, err
	}
	if shared.Level(scope) == shared.ScopeLevelPlatform {
		return items, nil
	}
	filtered := make([]HierarchyNode, 0, len(items))
	for _, item := range items {
		if _, err := s.ensureAgentInCurrentScope(scope, item.AncestorAgentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func (s *Service) Descendants(scope shared.Scope, agentID uint64, depthRange HierarchyDepthRange) ([]HierarchyNode, error) {
	if _, err := s.ensureAgentInCurrentScope(scope, agentID); err != nil {
		return nil, err
	}
	items, err := s.loadDescendants(agentID, depthRange)
	if err != nil {
		return nil, err
	}
	if shared.Level(scope) == shared.ScopeLevelPlatform {
		return items, nil
	}
	filtered := make([]HierarchyNode, 0, len(items))
	for _, item := range items {
		if _, err := s.ensureAgentInCurrentScope(scope, item.DescendantAgentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func (s *Service) TeamStatsByAgent(scope shared.Scope, agentID uint64, depthRange HierarchyDepthRange) (TeamStats, error) {
	if _, err := s.ensureAgentInCurrentScope(scope, agentID); err != nil {
		return TeamStats{}, err
	}
	if shared.Level(scope) == shared.ScopeLevelPlatform {
		return s.loadTeamStats(agentID, depthRange)
	}
	stats := TeamStats{AgentID: agentID}
	descendants, err := s.Descendants(scope, agentID, depthRange)
	if err != nil {
		return stats, err
	}
	ancestors, err := s.Ancestors(scope, agentID, depthRange)
	if err != nil {
		return stats, err
	}
	stats.TotalDescendants = int64(len(descendants))
	stats.TotalAncestors = int64(len(ancestors))
	breakdown := map[uint32]int64{}
	for _, item := range descendants {
		if item.Depth == 1 {
			stats.DirectDescendants++
		}
		if item.Depth > stats.MaxDescendantDepth {
			stats.MaxDescendantDepth = item.Depth
		}
		breakdown[item.Depth]++
		childCount, err := s.repo.CountDirectActiveDescendants(item.DescendantAgentID)
		if err != nil {
			return stats, err
		}
		if childCount == 0 {
			stats.LeafDescendants++
		}
	}
	for _, item := range ancestors {
		if item.Depth > stats.MaxAncestorDepth {
			stats.MaxAncestorDepth = item.Depth
		}
	}
	depths := make([]uint32, 0, len(breakdown))
	for depth := range breakdown {
		depths = append(depths, depth)
	}
	slices.Sort(depths)
	stats.DepthBreakdown = make([]DepthStat, 0, len(depths))
	for _, depth := range depths {
		stats.DepthBreakdown = append(stats.DepthBreakdown, DepthStat{Depth: depth, Count: breakdown[depth]})
	}
	return stats, nil
}

func (s *Service) loadAncestors(agentID uint64, depthRange HierarchyDepthRange) ([]HierarchyNode, error) {
	closures, err := s.repo.ListAncestorClosures(agentID, depthRange)
	if err != nil {
		return nil, err
	}
	items := make([]HierarchyNode, 0, len(closures))
	for _, closure := range closures {
		items = append(items, HierarchyNode{
			AncestorAgentID:   closure.AncestorAgentID,
			DescendantAgentID: closure.DescendantAgentID,
			Depth:             closure.Depth,
			RelationType:      closure.RelationType,
			Status:            closure.Status,
			ViaDirectParentID: closure.ViaDirectParentID,
		})
	}
	return items, nil
}

func (s *Service) loadDescendants(agentID uint64, depthRange HierarchyDepthRange) ([]HierarchyNode, error) {
	closures, err := s.repo.ListDescendantClosures(agentID, depthRange)
	if err != nil {
		return nil, err
	}
	items := make([]HierarchyNode, 0, len(closures))
	for _, closure := range closures {
		items = append(items, HierarchyNode{
			AncestorAgentID:   closure.AncestorAgentID,
			DescendantAgentID: closure.DescendantAgentID,
			Depth:             closure.Depth,
			RelationType:      closure.RelationType,
			Status:            closure.Status,
			ViaDirectParentID: closure.ViaDirectParentID,
		})
	}
	return items, nil
}

func (s *Service) loadTeamStats(agentID uint64, depthRange HierarchyDepthRange) (TeamStats, error) {
	stats := TeamStats{AgentID: agentID}
	directDescendants, err := s.repo.CountDirectActiveDescendants(agentID)
	if err != nil {
		return stats, err
	}
	stats.DirectDescendants = directDescendants
	totalDescendants, err := s.repo.CountDescendantClosures(agentID, depthRange)
	if err != nil {
		return stats, err
	}
	stats.TotalDescendants = totalDescendants
	totalAncestors, err := s.repo.CountAncestorClosures(agentID, depthRange)
	if err != nil {
		return stats, err
	}
	stats.TotalAncestors = totalAncestors
	maxDescendantDepth, err := s.repo.MaxDescendantDepth(agentID, depthRange)
	if err != nil {
		return stats, err
	}
	stats.MaxDescendantDepth = maxDescendantDepth
	maxAncestorDepth, err := s.repo.MaxAncestorDepth(agentID, depthRange)
	if err != nil {
		return stats, err
	}
	stats.MaxAncestorDepth = maxAncestorDepth
	breakdown, err := s.repo.ListDepthBreakdown(agentID, depthRange)
	if err != nil {
		return stats, err
	}
	stats.DepthBreakdown = breakdown
	leafRows, err := s.repo.ListLeafDescendantIDs(agentID, depthRange)
	if err != nil {
		return stats, err
	}
	for _, row := range leafRows {
		childCount, err := s.repo.CountDirectActiveDescendants(row)
		if err != nil {
			return stats, err
		}
		if childCount == 0 {
			stats.LeafDescendants++
		}
	}
	return stats, nil
}

func applyHierarchyDepthRange(query *gorm.DB, depthRange HierarchyDepthRange) *gorm.DB {
	query = query.Where("depth >= ?", depthRange.Min)
	if depthRange.Max != nil {
		query = query.Where("depth <= ?", *depthRange.Max)
	}
	return query
}
