package tenant

import "game-admin/backend/internal/domain/model"

func tenantCreatedPayload(tenant model.Tenant) map[string]any {
	return map[string]any{
		"id":          tenant.ID,
		"code":        tenant.Code,
		"name":        tenant.Name,
		"displayName": tenant.DisplayName,
		"status":      tenant.Status,
	}
}

func tenantStatusChangedPayload(before, after model.Tenant) map[string]any {
	return map[string]any{
		"id":           after.ID,
		"code":         after.Code,
		"beforeStatus": before.Status,
		"status":       after.Status,
	}
}

func tenantDeletedPayload(tenant model.Tenant) map[string]any {
	return map[string]any{
		"id":     tenant.ID,
		"code":   tenant.Code,
		"name":   tenant.Name,
		"status": tenant.Status,
	}
}

func brandCreatedPayload(brand model.Brand) map[string]any {
	return map[string]any{
		"id":          brand.ID,
		"tenantID":    brand.TenantID,
		"code":        brand.Code,
		"name":        brand.Name,
		"displayName": brand.DisplayName,
		"status":      brand.Status,
		"isDefault":   brand.IsDefault,
	}
}

func brandStatusChangedPayload(before, after model.Brand) map[string]any {
	return map[string]any{
		"id":           after.ID,
		"tenantID":     after.TenantID,
		"code":         after.Code,
		"beforeStatus": before.Status,
		"status":       after.Status,
	}
}

func brandDeletedPayload(brand model.Brand) map[string]any {
	return map[string]any{
		"id":       brand.ID,
		"tenantID": brand.TenantID,
		"code":     brand.Code,
		"name":     brand.Name,
		"status":   brand.Status,
	}
}
