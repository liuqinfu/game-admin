package http

import (
	"strings"

	accountcontract "game-admin/backend/internal/contract/account"
	reportcontract "game-admin/backend/internal/contract/report"
	riskcontract "game-admin/backend/internal/contract/risk"
	settlementcontract "game-admin/backend/internal/contract/settlement"
	withdrawalcontract "game-admin/backend/internal/contract/withdrawal"
	"game-admin/backend/internal/servicedef"
	accountsvc "game-admin/backend/internal/services/account"
	reportsvc "game-admin/backend/internal/services/report"
	risksvc "game-admin/backend/internal/services/risk"
	settlementsvc "game-admin/backend/internal/services/settlement"
	"game-admin/backend/internal/syncclient"
)

func accountContractClient(deps RouterDependencies, cfg RouterConfig) accountcontract.Client {
	if cfg.moduleEnabled(RouterModuleAccount) {
		return accountcontract.NewInProcessClient(accountsvc.NewService(deps.DB))
	}
	target := strings.TrimSpace(deps.Config.Gateway.AccountURL)
	if target == "" {
		target = servicedef.DefaultURL("account-service")
	}
	return accountcontract.NewHTTPClient(target, syncclient.Options{})
}

func reportContractClient(deps RouterDependencies, cfg RouterConfig) reportcontract.Client {
	if cfg.moduleEnabled(RouterModuleReport) {
		return reportcontract.NewInProcessClient(reportsvc.NewService(deps.DB))
	}
	target := strings.TrimSpace(deps.Config.Gateway.ReportURL)
	if target == "" {
		target = servicedef.DefaultURL("report-service")
	}
	return reportcontract.NewHTTPClient(target, syncclient.Options{})
}

func riskContractClient(deps RouterDependencies, cfg RouterConfig) riskcontract.Client {
	if cfg.moduleEnabled(RouterModuleRisk) {
		return riskcontract.NewInProcessClient(risksvc.NewServiceWithClients(deps.DB, reportContractClient(deps, cfg), accountContractClient(deps, cfg), withdrawalContractClient(deps, cfg)))
	}
	target := strings.TrimSpace(deps.Config.Gateway.RiskURL)
	if target == "" {
		target = servicedef.DefaultURL("risk-service")
	}
	return riskcontract.NewHTTPClient(target, syncclient.Options{})
}

func withdrawalContractClient(deps RouterDependencies, cfg RouterConfig) withdrawalcontract.Client {
	if cfg.moduleEnabled(RouterModuleWithdrawal) {
		return withdrawalcontract.NewLocalDBClient(deps.DB)
	}
	target := strings.TrimSpace(deps.Config.Gateway.WithdrawalURL)
	if target == "" {
		target = servicedef.DefaultURL("withdrawal-service")
	}
	return withdrawalcontract.NewHTTPClient(target, syncclient.Options{})
}

func settlementContractClient(deps RouterDependencies, cfg RouterConfig) settlementcontract.Client {
	if cfg.moduleEnabled(RouterModuleSettlement) {
		return settlementcontract.NewInProcessClient(settlementsvc.NewService(deps.DB))
	}
	target := strings.TrimSpace(deps.Config.Gateway.SettlementURL)
	if target == "" {
		target = servicedef.DefaultURL("settlement-service")
	}
	return settlementcontract.NewHTTPClient(target, syncclient.Options{})
}
