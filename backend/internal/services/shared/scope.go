package shared

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"game-admin/backend/internal/domain/model"
	"gorm.io/gorm"
)

type Scope struct {
	TenantIDs   []uint64
	TenantCodes []string
	BrandIDs    []uint64
	AgentID     *uint64
}

type ScopeLevel string

const (
	ScopeLevelPlatform ScopeLevel = "platform"
	ScopeLevelTenant   ScopeLevel = "tenant"
	ScopeLevelBrand    ScopeLevel = "brand"
	ScopeLevelAgent    ScopeLevel = "agent"
)

func NormalizeScope(scope Scope) Scope {
	scope.TenantIDs = UniqueUint64(scope.TenantIDs)
	scope.BrandIDs = UniqueUint64(scope.BrandIDs)
	scope.TenantCodes = UniqueStrings(scope.TenantCodes)
	return scope
}

func Level(scope Scope) ScopeLevel {
	scope = NormalizeScope(scope)
	if scope.AgentID != nil {
		return ScopeLevelAgent
	}
	if len(scope.TenantIDs) > 0 {
		return ScopeLevelTenant
	}
	if len(scope.BrandIDs) > 0 {
		return ScopeLevelBrand
	}
	return ScopeLevelPlatform
}

func UniqueUint64(values []uint64) []uint64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(values))
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func UniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func ScopeAllowsTenant(scope Scope, tenantID uint64) bool {
	scope = NormalizeScope(scope)
	if tenantID == 0 {
		return true
	}
	if len(scope.TenantIDs) == 0 {
		return len(scope.BrandIDs) == 0
	}
	return slices.Contains(scope.TenantIDs, tenantID)
}

func ScopeAllowsBrand(scope Scope, tenantID, brandID uint64) bool {
	scope = NormalizeScope(scope)
	if brandID == 0 {
		return true
	}
	if len(scope.TenantIDs) > 0 && tenantID > 0 && slices.Contains(scope.TenantIDs, tenantID) {
		return true
	}
	if len(scope.BrandIDs) == 0 {
		return len(scope.TenantIDs) == 0
	}
	return slices.Contains(scope.BrandIDs, brandID)
}

func MatchesScopedRecord(scope Scope, tenantID, brandID *uint64) bool {
	scope = NormalizeScope(scope)
	if len(scope.TenantIDs) == 0 && len(scope.BrandIDs) == 0 {
		return true
	}
	if tenantID != nil && brandID != nil && ScopeAllowsBrand(scope, *tenantID, *brandID) {
		return true
	}
	if tenantID != nil && ScopeAllowsTenant(scope, *tenantID) {
		return true
	}
	return false
}

func ScopeUint64Ptr(value uint64) *uint64 {
	if value == 0 {
		return nil
	}
	return &value
}

func EqualOptionalUint64(left, right *uint64) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return *left == *right
}

func EqualScopeIDs(leftTenantID, leftBrandID, rightTenantID, rightBrandID *uint64) bool {
	switch {
	case leftTenantID == nil && rightTenantID != nil, leftTenantID != nil && rightTenantID == nil:
		return false
	case leftTenantID != nil && rightTenantID != nil && *leftTenantID != *rightTenantID:
		return false
	}
	switch {
	case leftBrandID == nil && rightBrandID != nil, leftBrandID != nil && rightBrandID == nil:
		return false
	case leftBrandID != nil && rightBrandID != nil && *leftBrandID != *rightBrandID:
		return false
	}
	return true
}

func EnsureSameScope(label string, expectedTenantID, expectedBrandID, actualTenantID, actualBrandID *uint64) error {
	if EqualScopeIDs(expectedTenantID, expectedBrandID, actualTenantID, actualBrandID) {
		return nil
	}
	return fmt.Errorf("%s does not belong to the active tenant or brand", label)
}

func ParseScopeUint64(text string) (*uint64, error) {
	value := strings.TrimSpace(text)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return nil, err
	}
	return ScopeUint64Ptr(parsed), nil
}

func ApplyScopedQueryFilters(query *gorm.DB, tenantField, brandField, tenantIDText, brandIDText string) (*gorm.DB, error) {
	if query == nil {
		return nil, nil
	}
	tenantID, err := ParseScopeUint64(tenantIDText)
	if err != nil {
		return nil, err
	}
	brandID, err := ParseScopeUint64(brandIDText)
	if err != nil {
		return nil, err
	}
	if tenantID != nil && tenantField != "" {
		query = query.Where(tenantField+" = ?", *tenantID)
	}
	if brandID != nil && brandField != "" {
		query = query.Where(brandField+" = ?", *brandID)
	}
	return query, nil
}

func CurrentAgentScopeIDs(db *gorm.DB, agentID uint64) ([]uint64, error) {
	ids := []uint64{agentID}
	if db == nil {
		return ids, nil
	}
	var descendants []uint64
	if err := db.Model(&model.AgentRelationClosure{}).
		Where("ancestor_agent_id = ? AND depth > ? AND status = ?", agentID, 0, model.RelationStatusActive).
		Pluck("descendant_agent_id", &descendants).Error; err != nil {
		return nil, err
	}
	return UniqueUint64(append(ids, descendants...)), nil
}

func ApplyExactTenantBrandScope(query *gorm.DB, tenantID, brandID *uint64, tenantColumn, brandColumn string) *gorm.DB {
	if query == nil {
		return nil
	}
	if tenantID == nil {
		query = query.Where(tenantColumn + " IS NULL")
	} else {
		query = query.Where(tenantColumn+" = ?", *tenantID)
	}
	if strings.TrimSpace(brandColumn) == "" {
		return query
	}
	if brandID == nil {
		return query.Where(brandColumn + " IS NULL")
	}
	return query.Where(brandColumn+" = ?", *brandID)
}

func ApplyRecordAndOwnerTenantBrandScope(query *gorm.DB, recordTenantID, recordBrandID, ownerTenantID, ownerBrandID *uint64, recordTenantColumn, recordBrandColumn, ownerTenantColumn, ownerBrandColumn string) *gorm.DB {
	if query == nil {
		return nil
	}
	query = ApplyExactTenantBrandScope(query, recordTenantID, recordBrandID, recordTenantColumn, recordBrandColumn)
	return ApplyExactTenantBrandScope(query, ownerTenantID, ownerBrandID, ownerTenantColumn, ownerBrandColumn)
}

func ApplyTenantBrandScope(query *gorm.DB, scope Scope, tenantColumn, brandColumn string) *gorm.DB {
	if query == nil {
		return nil
	}
	scope = NormalizeScope(scope)
	switch {
	case len(scope.TenantIDs) > 0 && len(scope.BrandIDs) > 0 && strings.TrimSpace(brandColumn) != "":
		return query.Where("("+tenantColumn+" IN ? OR "+brandColumn+" IN ?)", scope.TenantIDs, scope.BrandIDs)
	case len(scope.TenantIDs) > 0:
		return query.Where(tenantColumn+" IN ?", scope.TenantIDs)
	case len(scope.BrandIDs) > 0 && strings.TrimSpace(brandColumn) != "":
		return query.Where(brandColumn+" IN ?", scope.BrandIDs)
	default:
		return query
	}
}

func ApplyTenantBrandScopeWithLegacyCode(query *gorm.DB, scope Scope, tenantColumn, brandColumn, legacyCodeColumn string) *gorm.DB {
	if query == nil {
		return nil
	}
	scope = NormalizeScope(scope)
	legacyCodeColumn = strings.TrimSpace(legacyCodeColumn)
	if legacyCodeColumn == "" {
		scope.TenantCodes = nil
	}
	baseScopedClause := ""
	baseArgs := make([]any, 0, 3)
	switch {
	case len(scope.TenantIDs) > 0 && len(scope.TenantCodes) > 0:
		baseScopedClause = "(" + tenantColumn + " IN ? OR ((" + tenantColumn + " IS NULL OR " + tenantColumn + " = 0) AND (" + legacyCodeColumn + " IN ? OR TRIM(" + legacyCodeColumn + ") = '')))"
		baseArgs = append(baseArgs, scope.TenantIDs, scope.TenantCodes)
	case len(scope.TenantIDs) > 0:
		baseScopedClause = tenantColumn + " IN ?"
		baseArgs = append(baseArgs, scope.TenantIDs)
	case len(scope.TenantCodes) > 0:
		baseScopedClause = "(" + legacyCodeColumn + " IN ? OR TRIM(" + legacyCodeColumn + ") = '')"
		baseArgs = append(baseArgs, scope.TenantCodes)
	}
	switch {
	case baseScopedClause != "" && len(scope.BrandIDs) > 0 && strings.TrimSpace(brandColumn) != "":
		args := append(baseArgs, scope.BrandIDs)
		return query.Where("("+baseScopedClause+" OR "+brandColumn+" IN ?)", args...)
	case baseScopedClause != "":
		return query.Where(baseScopedClause, baseArgs...)
	case len(scope.BrandIDs) > 0 && strings.TrimSpace(brandColumn) != "":
		return query.Where(brandColumn+" IN ?", scope.BrandIDs)
	default:
		return query
	}
}

func LoadTenant(db *gorm.DB, tenantID uint64) (*model.Tenant, error) {
	var tenant model.Tenant
	if err := db.First(&tenant, tenantID).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func LoadBrand(db *gorm.DB, brandID uint64) (*model.Brand, error) {
	var brand model.Brand
	if err := db.First(&brand, brandID).Error; err != nil {
		return nil, err
	}
	return &brand, nil
}

func DefaultBrandForTenant(db *gorm.DB, tenantID uint64) (*uint64, error) {
	if tenantID == 0 {
		return nil, nil
	}
	tenant, err := LoadTenant(db, tenantID)
	if err != nil {
		return nil, err
	}
	if tenant.DefaultBrandID != nil && *tenant.DefaultBrandID > 0 {
		return tenant.DefaultBrandID, nil
	}
	var brand model.Brand
	if err := db.Where("tenant_id = ? AND is_default = ?", tenantID, true).First(&brand).Error; err == nil {
		return &brand.ID, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, nil
}

func ResolveTenantBrandScope(db *gorm.DB, scope Scope, tenantID, brandID *uint64, requireResolved bool) (*uint64, *uint64, error) {
	scope = NormalizeScope(scope)
	var resolvedTenantID *uint64
	var resolvedBrandID *uint64

	if brandID != nil && *brandID > 0 {
		brand, err := LoadBrand(db, *brandID)
		if err != nil {
			return nil, nil, err
		}
		if tenantID != nil && *tenantID > 0 && brand.TenantID != *tenantID {
			return nil, nil, errors.New("brand does not belong to tenant")
		}
		if !ScopeAllowsTenant(scope, brand.TenantID) && !ScopeAllowsBrand(scope, brand.TenantID, brand.ID) {
			return nil, nil, gorm.ErrRecordNotFound
		}
		return ScopeUint64Ptr(brand.TenantID), ScopeUint64Ptr(brand.ID), nil
	}

	if tenantID != nil && *tenantID > 0 {
		if !ScopeAllowsTenant(scope, *tenantID) {
			return nil, nil, gorm.ErrRecordNotFound
		}
		resolvedTenantID = ScopeUint64Ptr(*tenantID)
		defaultBrandID, err := DefaultBrandForTenant(db, *tenantID)
		if err != nil {
			return nil, nil, err
		}
		resolvedBrandID = defaultBrandID
		if requireResolved && resolvedBrandID == nil {
			return nil, nil, errors.New("default brand is required for tenant scoped writes")
		}
		return resolvedTenantID, resolvedBrandID, nil
	}

	if len(scope.TenantIDs) == 0 && len(scope.BrandIDs) == 1 {
		brand, err := LoadBrand(db, scope.BrandIDs[0])
		if err != nil {
			return nil, nil, err
		}
		return ScopeUint64Ptr(brand.TenantID), ScopeUint64Ptr(brand.ID), nil
	}
	if len(scope.TenantIDs) == 1 {
		resolvedTenantID = ScopeUint64Ptr(scope.TenantIDs[0])
		defaultBrandID, err := DefaultBrandForTenant(db, *resolvedTenantID)
		if err != nil {
			return nil, nil, err
		}
		resolvedBrandID = defaultBrandID
		if requireResolved && resolvedBrandID == nil {
			return nil, nil, errors.New("default brand is required for tenant scoped writes")
		}
		return resolvedTenantID, resolvedBrandID, nil
	}
	if requireResolved {
		return nil, nil, errors.New("tenant and brand scope are required")
	}
	return nil, nil, nil
}
