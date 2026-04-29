package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"game-admin/backend/internal/routing"
	"game-admin/backend/internal/servicedef"
)

type Config struct {
	Env       string
	HTTPPort  string
	Service   ServiceConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	MQ        MQConfig
	Gateway   GatewayConfig
	Telemetry TelemetryConfig
}

type ServiceConfig struct {
	Name    string
	Modules []string
}

type DatabaseConfig struct {
	Driver string
	DSN    string
}

type RedisConfig struct {
	Enabled  bool
	Addr     string
	Password string
	DB       int
	Prefix   string
}

type MQConfig struct {
	Enabled bool
	Driver  string
	URL     string
	Topic   string
}

type GatewayConfig struct {
	IdentityURL     string
	TenantURL       string
	AgentURL        string
	RelationURL     string
	GameURL         string
	RuleURL         string
	ActivityURL     string
	RechargeURL     string
	SettlementURL   string
	AccountURL      string
	WithdrawalURL   string
	RiskURL         string
	ReportURL       string
	AuditURL        string
	NotificationURL string
	DataSyncURL     string
}

type TelemetryConfig struct {
	Enabled        bool
	Exporter       string
	OTLPEndpoint   string
	OTLPInsecure   bool
	ServiceVersion string
}

func Load() Config {
	return Config{
		Env:      getEnv("APP_ENV", "development"),
		HTTPPort: getEnv("HTTP_PORT", "8080"),
		Service: ServiceConfig{
			Name:    getEnv("SERVICE_NAME", "server"),
			Modules: parseCSVEnv("SERVICE_MODULES"),
		},
		Database: DatabaseConfig{
			Driver: getEnv("DATABASE_DRIVER", "sqlite"),
			DSN:    databaseDSN(),
		},
		Redis: RedisConfig{
			Enabled:  getEnvBool("REDIS_ENABLED", false),
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			Prefix:   getEnv("REDIS_PREFIX", "game-admin:"),
		},
		MQ: MQConfig{
			Enabled: getEnvBool("MQ_ENABLED", false),
			Driver:  getEnv("MQ_DRIVER", "rabbitmq"),
			URL:     getEnv("MQ_URL", "amqp://guest:***@localhost:5672/"),
			Topic:   getEnv("MQ_TOPIC", "game-admin.events"),
		},
		Gateway: GatewayConfig{
			IdentityURL:     getEnv("GATEWAY_IDENTITY_URL", servicedef.DefaultURL("identity-service")),
			TenantURL:       getEnv("GATEWAY_TENANT_URL", servicedef.DefaultURL("tenant-service")),
			AgentURL:        getEnv("GATEWAY_AGENT_URL", servicedef.DefaultURL("agent-service")),
			RelationURL:     getEnv("GATEWAY_RELATION_URL", servicedef.DefaultURL("relation-service")),
			GameURL:         getEnv("GATEWAY_GAME_URL", servicedef.DefaultURL("game-service")),
			RuleURL:         getEnv("GATEWAY_RULE_URL", servicedef.DefaultURL("rule-service")),
			ActivityURL:     getEnv("GATEWAY_ACTIVITY_URL", servicedef.DefaultURL("activity-service")),
			RechargeURL:     getEnv("GATEWAY_RECHARGE_URL", servicedef.DefaultURL("recharge-service")),
			SettlementURL:   getEnv("GATEWAY_SETTLEMENT_URL", servicedef.DefaultURL("settlement-service")),
			AccountURL:      getEnv("GATEWAY_ACCOUNT_URL", servicedef.DefaultURL("account-service")),
			WithdrawalURL:   getEnv("GATEWAY_WITHDRAWAL_URL", servicedef.DefaultURL("withdrawal-service")),
			RiskURL:         getEnv("GATEWAY_RISK_URL", servicedef.DefaultURL("risk-service")),
			ReportURL:       getEnv("GATEWAY_REPORT_URL", servicedef.DefaultURL("report-service")),
			AuditURL:        getEnv("GATEWAY_AUDIT_URL", servicedef.DefaultURL("audit-service")),
			NotificationURL: getEnv("GATEWAY_NOTIFICATION_URL", servicedef.DefaultURL("notification-service")),
			DataSyncURL:     getEnv("GATEWAY_DATA_SYNC_URL", servicedef.DefaultURL("data-platform-sync-service")),
		},
		Telemetry: TelemetryConfig{
			Enabled:        getEnvBool("TELEMETRY_ENABLED", false),
			Exporter:       getEnv("TELEMETRY_EXPORTER", "none"),
			OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			OTLPInsecure:   getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", false),
			ServiceVersion: getEnv("SERVICE_VERSION", "dev"),
		},
	}
}

func databaseDSN() string {
	raw := os.Getenv("DATABASE_DSN")
	if raw != "" {
		return raw
	}

	return filepath.Join("data", "app.db")
}

func (c Config) Address() string {
	return fmt.Sprintf(":%s", c.HTTPPort)
}

func (c Config) BackendRoot() string {
	return getEnv("BACKEND_ROOT", ".")
}

func (c Config) FrontendRoot() string {
	return getEnv("FRONTEND_ROOT", filepath.Join("..", "frontend"))
}

func (c Config) InitSummary() string {
	return strings.Join([]string{
		fmt.Sprintf("APP_ENV=%s", c.Env),
		fmt.Sprintf("SERVICE_NAME=%s", c.Service.Name),
		fmt.Sprintf("SERVICE_MODULES=%s", strings.Join(c.Service.Modules, ",")),
		fmt.Sprintf("HTTP_PORT=%s", c.HTTPPort),
		fmt.Sprintf("DATABASE_DRIVER=%s", c.Database.Driver),
		fmt.Sprintf("DATABASE_DSN=%s", c.Database.DSN),
		fmt.Sprintf("REDIS_ENABLED=%t", c.Redis.Enabled),
		fmt.Sprintf("MQ_ENABLED=%t", c.MQ.Enabled),
		fmt.Sprintf("TELEMETRY_ENABLED=%t", c.Telemetry.Enabled),
		fmt.Sprintf("TELEMETRY_EXPORTER=%s", c.Telemetry.Exporter),
	}, "\n")
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value == "1" || value == "true" || value == "TRUE" || value == "True"
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func parseCSVEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func (g GatewayConfig) TargetFor(service routing.ServiceKey) string {
	switch service {
	case routing.ServiceIdentity:
		return g.IdentityURL
	case routing.ServiceTenant:
		return g.TenantURL
	case routing.ServiceAgent:
		return g.AgentURL
	case routing.ServiceRelation:
		return g.RelationURL
	case routing.ServiceGame:
		return g.GameURL
	case routing.ServiceRule:
		return g.RuleURL
	case routing.ServiceActivity:
		return g.ActivityURL
	case routing.ServiceRecharge:
		return g.RechargeURL
	case routing.ServiceSettlement:
		return g.SettlementURL
	case routing.ServiceAccount:
		return g.AccountURL
	case routing.ServiceWithdrawal:
		return g.WithdrawalURL
	case routing.ServiceRisk:
		return g.RiskURL
	case routing.ServiceReport:
		return g.ReportURL
	case routing.ServiceAudit:
		return g.AuditURL
	default:
		return ""
	}
}
