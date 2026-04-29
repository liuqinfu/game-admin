package servicedef

import "game-admin/backend/internal/routing"

type HTTPService struct {
	Name        string
	DefaultPort string
	Modules     []string
	RoutingKey  routing.ServiceKey
}

type WorkerService struct {
	Name         string
	DefaultPort  string
	ConsumerName string
}

var httpServices = map[string]HTTPService{
	"identity-service":   {Name: "identity-service", DefaultPort: "8081", Modules: []string{"auth", "rbac"}, RoutingKey: routing.ServiceIdentity},
	"tenant-service":     {Name: "tenant-service", DefaultPort: "8082", Modules: []string{"tenant"}, RoutingKey: routing.ServiceTenant},
	"agent-service":      {Name: "agent-service", DefaultPort: "8083", Modules: []string{"agent"}, RoutingKey: routing.ServiceAgent},
	"relation-service":   {Name: "relation-service", DefaultPort: "8084", Modules: []string{"relation"}, RoutingKey: routing.ServiceRelation},
	"game-service":       {Name: "game-service", DefaultPort: "8085", Modules: []string{"game"}, RoutingKey: routing.ServiceGame},
	"rule-service":       {Name: "rule-service", DefaultPort: "8086", Modules: []string{"rule"}, RoutingKey: routing.ServiceRule},
	"activity-service":   {Name: "activity-service", DefaultPort: "8087", Modules: []string{"activity"}, RoutingKey: routing.ServiceActivity},
	"recharge-service":   {Name: "recharge-service", DefaultPort: "8088", Modules: []string{"recharge", "openapi"}, RoutingKey: routing.ServiceRecharge},
	"settlement-service": {Name: "settlement-service", DefaultPort: "8089", Modules: []string{"settlement"}, RoutingKey: routing.ServiceSettlement},
	"account-service":    {Name: "account-service", DefaultPort: "8090", Modules: []string{"account"}, RoutingKey: routing.ServiceAccount},
	"withdrawal-service": {Name: "withdrawal-service", DefaultPort: "8091", Modules: []string{"withdrawal"}, RoutingKey: routing.ServiceWithdrawal},
	"risk-service":       {Name: "risk-service", DefaultPort: "8092", Modules: []string{"risk"}, RoutingKey: routing.ServiceRisk},
	"report-service":     {Name: "report-service", DefaultPort: "8093", Modules: []string{"report"}, RoutingKey: routing.ServiceReport},
	"audit-service":      {Name: "audit-service", DefaultPort: "8094", Modules: []string{"audit"}, RoutingKey: routing.ServiceAudit},
}

var workerServices = map[string]WorkerService{
	"notification-service":       {Name: "notification-service", DefaultPort: "8095", ConsumerName: "notification-service"},
	"data-platform-sync-service": {Name: "data-platform-sync-service", DefaultPort: "8096", ConsumerName: "data-platform-sync-service"},
}

func HTTP(name string) (HTTPService, bool) {
	def, ok := httpServices[name]
	return def, ok
}

func MustHTTP(name string) HTTPService {
	def, ok := HTTP(name)
	if !ok {
		panic("unknown http service: " + name)
	}
	return def
}

func Worker(name string) (WorkerService, bool) {
	def, ok := workerServices[name]
	return def, ok
}

func MustWorker(name string) WorkerService {
	def, ok := Worker(name)
	if !ok {
		panic("unknown worker service: " + name)
	}
	return def
}

func HTTPList() []HTTPService {
	items := make([]HTTPService, 0, len(httpServices))
	for _, item := range httpServices {
		items = append(items, item)
	}
	return items
}

func HTTPNames() []string {
	items := make([]string, 0, len(httpServices))
	for name := range httpServices {
		items = append(items, name)
	}
	return items
}

func WorkerNames() []string {
	items := make([]string, 0, len(workerServices))
	for name := range workerServices {
		items = append(items, name)
	}
	return items
}

func DefaultURL(name string) string {
	if def, ok := HTTP(name); ok {
		return "http://localhost:" + def.DefaultPort
	}
	if def, ok := Worker(name); ok {
		return "http://localhost:" + def.DefaultPort
	}
	return ""
}
