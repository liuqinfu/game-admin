package http

import (
	gamesvc "game-admin/backend/internal/services/game"
	identitysvc "game-admin/backend/internal/services/identity"
	recalcsvc "game-admin/backend/internal/services/recalculation"
	reportsvc "game-admin/backend/internal/services/report"
	risksvc "game-admin/backend/internal/services/risk"
	settlementsvc "game-admin/backend/internal/services/settlement"
	sharedsvc "game-admin/backend/internal/services/shared"
	withdrawalsvc "game-admin/backend/internal/services/withdrawal"
)

func toServiceScope(scope TenantScope) sharedsvc.Scope {
	return sharedsvc.Scope{
		TenantIDs:   append([]uint64(nil), scope.TenantIDs...),
		TenantCodes: append([]string(nil), scope.TenantCodes...),
		BrandIDs:    append([]uint64(nil), scope.BrandIDs...),
		AgentID:     scope.AgentID,
	}
}

func fromServiceAccount(account identitysvc.Account) authAccount {
	return authAccount{
		UserID:       account.UserID,
		Username:     account.Username,
		DisplayName:  account.DisplayName,
		PasswordHash: account.PasswordHash,
		AgentID:      account.AgentID,
		Token:        account.Token,
		Role:         account.Role,
		Permissions:  account.Permissions,
		TenantScope: TenantScope{
			TenantIDs:   append([]uint64(nil), account.TenantScope.TenantIDs...),
			TenantCodes: append([]string(nil), account.TenantScope.TenantCodes...),
			BrandIDs:    append([]uint64(nil), account.TenantScope.BrandIDs...),
			AgentID:     account.TenantScope.AgentID,
		},
	}
}

func toHTTPAuthLoginResponse(response identitysvc.AuthLoginResponse) authLoginResponse {
	return authLoginResponse{
		Token:       response.Token,
		Role:        response.Role,
		Permissions: append([]string(nil), response.Permissions...),
		Identity: authLoginIdentity{
			Username:    response.User.Username,
			Role:        response.Role,
			Permissions: append([]string(nil), response.Permissions...),
		},
		User: authUserResponse{
			ID:          response.User.ID,
			Username:    response.User.Username,
			DisplayName: response.User.DisplayName,
			Roles:       append([]string(nil), response.User.Roles...),
			Permissions: append([]string(nil), response.User.Permissions...),
			Scope: authUserScopeResponse{
				Level:      ScopeLevel(response.User.Scope.Level),
				TenantID:   response.User.Scope.TenantID,
				TenantName: response.User.Scope.TenantName,
				TenantCode: response.User.Scope.TenantCode,
				BrandID:    response.User.Scope.BrandID,
				BrandName:  response.User.Scope.BrandName,
				BrandCode:  response.User.Scope.BrandCode,
				AgentID:    response.User.Scope.AgentID,
			},
		},
	}
}

func toHTTPRBACUser(user identitysvc.User) rbacUserResponse {
	return rbacUserResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Status:      user.Status,
		TenantID:    user.TenantID,
		BrandID:     user.BrandID,
		AgentID:     user.AgentID,
		Roles:       append([]string(nil), user.Roles...),
		LastLoginAt: user.LastLoginAt,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

func toHTTPGameCredential(view *gamesvc.CredentialView) *gameCredentialResponse {
	if view == nil {
		return nil
	}
	return &gameCredentialResponse{
		ID:         view.ID,
		GameID:     view.GameID,
		Name:       view.Name,
		AccessKey:  view.AccessKey,
		SecretKey:  view.SecretKey,
		Status:     view.Status,
		Scopes:     append([]string(nil), view.Scopes...),
		CreatedAt:  view.CreatedAt,
		LastUsedAt: view.LastUsedAt,
		RotatedAt:  view.RotatedAt,
		ExpiresAt:  view.ExpiresAt,
		Remark:     view.Remark,
	}
}

func toHTTPAgentPerformanceItem(item reportsvc.AgentPerformanceItem) agentPerformanceReportItem {
	return agentPerformanceReportItem{
		AgentID:            item.AgentID,
		AgentName:          item.AgentName,
		Currency:           item.Currency,
		Balance:            item.Balance,
		FrozenBalance:      item.FrozenBalance,
		WithdrawableAmount: item.WithdrawableAmount,
		TotalEntries:       item.TotalEntries,
		IncomeAmount:       item.IncomeAmount,
		FreezeAmount:       item.FreezeAmount,
		UnfreezeAmount:     item.UnfreezeAmount,
		DebitAmount:        item.DebitAmount,
		ReverseAmount:      item.ReverseAmount,
		AdjustAmount:       item.AdjustAmount,
	}
}

func toHTTPGameSettlementItem(item reportsvc.GameSettlementItem) gameSettlementReportItem {
	return gameSettlementReportItem{
		Currency:              item.Currency,
		TotalBills:            item.TotalBills,
		ConfirmedBills:        item.ConfirmedBills,
		PendingBills:          item.PendingBills,
		GeneratedBills:        item.GeneratedBills,
		CancelledBills:        item.CancelledBills,
		TotalCommissionAmount: item.TotalCommissionAmount,
		TotalAdjustmentAmount: item.TotalAdjustmentAmount,
		TotalPayableAmount:    item.TotalPayableAmount,
	}
}

func toHTTPTeamPerformanceItem(item reportsvc.TeamPerformanceItem) teamPerformanceReportItem {
	return teamPerformanceReportItem{
		AgentID:                 item.AgentID,
		AgentName:               item.AgentName,
		Level:                   item.Level,
		Currency:                item.Currency,
		DirectDescendants:       item.DirectDescendants,
		TotalDescendants:        item.TotalDescendants,
		ActiveDescendants:       item.ActiveDescendants,
		LeafDescendants:         item.LeafDescendants,
		BoundPlayers:            item.BoundPlayers,
		TotalTeamBalance:        item.TotalTeamBalance,
		TotalTeamFrozenBalance:  item.TotalTeamFrozenBalance,
		TotalWithdrawableAmount: item.TotalWithdrawableAmount,
		PendingWithdrawals:      item.PendingWithdrawals,
		PendingWithdrawalAmount: item.PendingWithdrawalAmount,
		ConfirmedBills:          item.ConfirmedBills,
		ConfirmedBillAmount:     item.ConfirmedBillAmount,
	}
}

func toHTTPSettlementProgressItem(item reportsvc.SettlementProgressItem) settlementProgressReportItem {
	return settlementProgressReportItem{
		Currency:                item.Currency,
		TotalBills:              item.TotalBills,
		GeneratedBills:          item.GeneratedBills,
		ConfirmedBills:          item.ConfirmedBills,
		CancelledBills:          item.CancelledBills,
		ProgressPercent:         item.ProgressPercent,
		PendingCommissionAmount: item.PendingCommissionAmount,
		PendingPayableAmount:    item.PendingPayableAmount,
		CompletedPayableAmount:  item.CompletedPayableAmount,
		LastGeneratedAt:         item.LastGeneratedAt,
		LastConfirmedAt:         item.LastConfirmedAt,
	}
}

func toHTTPAgentAccountRiskItem(item risksvc.AgentAccountRiskItem) agentAccountRiskListItem {
	return agentAccountRiskListItem{
		AgentAccount: item.AgentAccount,
		AgentName:    item.AgentName,
		RiskLevel:    item.RiskLevel,
		FrozenRatio:  item.FrozenRatio,
	}
}

func toHTTPRiskCaseItem(item risksvc.RiskCaseItem) riskCaseListItem {
	return riskCaseListItem{
		CaseNo:                    item.CaseNo,
		Status:                    item.Status,
		AgentID:                   item.AgentID,
		AgentName:                 item.AgentName,
		Currency:                  item.Currency,
		RiskLevel:                 item.RiskLevel,
		Reason:                    item.Reason,
		FreezeRequested:           item.FreezeRequested,
		FrozenBalance:             item.FrozenBalance,
		WithdrawableAmount:        item.WithdrawableAmount,
		FrozenRatio:               item.FrozenRatio,
		LatestWithdrawalRequestID: item.LatestWithdrawalRequestID,
		LatestWithdrawalRequestNo: item.LatestWithdrawalRequestNo,
		LatestWithdrawalAmount:    item.LatestWithdrawalAmount,
		RiskNote:                  item.RiskNote,
		CreatedAt:                 item.CreatedAt,
		UpdatedAt:                 item.UpdatedAt,
	}
}

func toHTTPRiskIntelligenceItem(item risksvc.IntelligenceItem) riskIntelligenceItem {
	return riskIntelligenceItem{
		SourceType:        item.SourceType,
		SourceID:          item.SourceID,
		AgentID:           item.AgentID,
		AgentName:         item.AgentName,
		RiskLevel:         item.RiskLevel,
		Score:             item.Score,
		Reason:            item.Reason,
		RecommendedAction: item.RecommendedAction,
		Intercepted:       item.Intercepted,
		Status:            item.Status,
		CreatedAt:         item.CreatedAt,
	}
}

func toHTTPWithdrawalListItem(item withdrawalsvc.ListItem) withdrawalListItem {
	return withdrawalListItem{
		WithdrawalRequest: item.WithdrawalRequest,
		AgentName:         item.AgentName,
	}
}

func toHTTPSettlementBillListItem(item settlementsvc.BillListItem) settlementBillListItem {
	details := make([]settlementBillDetailItem, 0, len(item.Details))
	for _, detail := range item.Details {
		details = append(details, settlementBillDetailItem{
			SettlementBillDetail: detail.SettlementBillDetail,
			RecordNo:             detail.RecordNo,
			OrderNo:              detail.OrderNo,
		})
	}
	return settlementBillListItem{
		SettlementBill: item.SettlementBill,
		AgentName:      item.AgentName,
		Details:        details,
	}
}

func toHTTPRecalculationTaskItem(item recalcsvc.ListItem) recalculationTaskListItem {
	return recalculationTaskListItem{
		RecalculationTask: item.RecalculationTask,
		AgentName:         item.AgentName,
		BillNo:            item.BillNo,
	}
}
