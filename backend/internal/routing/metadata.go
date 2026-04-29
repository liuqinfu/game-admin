package routing

import "strings"

type ServiceKey string

const (
	ServiceUnknown    ServiceKey = ""
	ServiceIdentity   ServiceKey = "identity"
	ServiceTenant     ServiceKey = "tenant"
	ServiceAgent      ServiceKey = "agent"
	ServiceRelation   ServiceKey = "relation"
	ServiceGame       ServiceKey = "game"
	ServiceRule       ServiceKey = "rule"
	ServiceActivity   ServiceKey = "activity"
	ServiceRecharge   ServiceKey = "recharge"
	ServiceSettlement ServiceKey = "settlement"
	ServiceAccount    ServiceKey = "account"
	ServiceWithdrawal ServiceKey = "withdrawal"
	ServiceRisk       ServiceKey = "risk"
	ServiceReport     ServiceKey = "report"
	ServiceAudit      ServiceKey = "audit"
)

type Match struct {
	Module  string
	Service ServiceKey
	Shared  bool
	Found   bool
}

func MatchPath(path string) Match {
	switch {
	case path == "/healthz":
		return Match{Shared: true, Found: true}
	case path == "/api/enum-dictionaries":
		return Match{Shared: true, Found: true, Service: ServiceTenant}
	case path == "/openapi/game/enum-dictionaries":
		return Match{Shared: true, Found: true, Service: ServiceRecharge}
	case strings.HasPrefix(path, "/api/auth/") || path == "/api/auth/login":
		return Match{Module: "auth", Service: ServiceIdentity, Found: true}
	case strings.HasPrefix(path, "/api/rbac/"):
		return Match{Module: "rbac", Service: ServiceIdentity, Found: true}
	case strings.HasPrefix(path, "/api/agents/") && isRelationPath(path):
		return Match{Module: "relation", Service: ServiceRelation, Found: true}
	case strings.HasPrefix(path, "/api/agents"),
		strings.HasPrefix(path, "/api/invite-codes"),
		strings.HasPrefix(path, "/api/players"),
		strings.HasPrefix(path, "/api/bindings"),
		strings.HasPrefix(path, "/api/binding-history"),
		strings.HasPrefix(path, "/api/agent-invite-applications"):
		return Match{Module: "agent", Service: ServiceAgent, Found: true}
	case strings.HasPrefix(path, "/api/tenants"),
		strings.HasPrefix(path, "/api/brands"),
		strings.HasPrefix(path, "/api/platform-configs"):
		return Match{Module: "tenant", Service: ServiceTenant, Found: true}
	case strings.HasPrefix(path, "/api/games"),
		strings.HasPrefix(path, "/api/agent-game-access"):
		return Match{Module: "game", Service: ServiceGame, Found: true}
	case strings.HasPrefix(path, "/api/rules"):
		return Match{Module: "rule", Service: ServiceRule, Found: true}
	case strings.HasPrefix(path, "/api/activity-reward-"):
		return Match{Module: "activity", Service: ServiceActivity, Found: true}
	case strings.HasPrefix(path, "/api/orders"),
		strings.HasPrefix(path, "/api/recharge/"),
		strings.HasPrefix(path, "/openapi/game/"):
		return Match{Module: "recharge", Service: ServiceRecharge, Found: true}
	case strings.HasPrefix(path, "/api/commissions"),
		strings.HasPrefix(path, "/api/settlement-bills"),
		strings.HasPrefix(path, "/api/recalculation-tasks"):
		return Match{Module: "settlement", Service: ServiceSettlement, Found: true}
	case strings.HasPrefix(path, "/api/ledger"),
		strings.HasPrefix(path, "/api/agent-accounts"):
		return Match{Module: "account", Service: ServiceAccount, Found: true}
	case strings.HasPrefix(path, "/api/withdrawals"),
		strings.HasPrefix(path, "/api/withdrawal-requests"):
		return Match{Module: "withdrawal", Service: ServiceWithdrawal, Found: true}
	case strings.HasPrefix(path, "/api/risk/"):
		return Match{Module: "risk", Service: ServiceRisk, Found: true}
	case strings.HasPrefix(path, "/api/report/"):
		return Match{Module: "report", Service: ServiceReport, Found: true}
	case strings.HasPrefix(path, "/api/audit"):
		return Match{Module: "audit", Service: ServiceAudit, Found: true}
	default:
		return Match{Shared: true}
	}
}

func isRelationPath(path string) bool {
	return strings.HasSuffix(path, "/ancestors") || strings.HasSuffix(path, "/descendants") || strings.HasSuffix(path, "/team-stats")
}
