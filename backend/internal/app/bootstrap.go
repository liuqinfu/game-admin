package app

import (
	"fmt"
	"strings"
	"time"

	"game-admin/backend/internal/config"
	httptransport "game-admin/backend/internal/http"
	gametrics "game-admin/backend/internal/metrics"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type healthAdapter struct {
	runtime HealthRuntime
}

func newHealthAdapter(cfg config.Config) healthAdapter {
	return healthAdapter{runtime: NewHealthRuntime(HealthStore{
		Service:      cfg.Service.Name,
		Version:      "dev",
		Dependencies: healthDependencies(cfg),
	})}
}

func (h healthAdapter) Live(now time.Time, requestID, traceID string) httptransport.HealthResponse {
	return toHTTPHealthResponse(h.runtime.Live(now, requestID, traceID))
}

func (h healthAdapter) Ready(now time.Time, requestID, traceID string) httptransport.HealthResponse {
	return toHTTPHealthResponse(h.runtime.Ready(now, requestID, traceID))
}

func toHTTPHealthResponse(status HealthStatus) httptransport.HealthResponse {
	dependencies := make([]httptransport.HealthDependency, 0, len(status.Dependencies))
	for _, dependency := range status.Dependencies {
		dependencies = append(dependencies, httptransport.HealthDependency{
			Name:      dependency.Name,
			Status:    dependency.Status,
			Message:   dependency.Message,
			Optional:  dependency.Optional,
			CheckedAt: dependency.CheckedAt,
		})
	}
	return httptransport.HealthResponse{
		Status:       status.Status,
		Service:      status.Service,
		Version:      status.Version,
		Timestamp:    status.Timestamp,
		RequestID:    status.RequestID,
		TraceID:      status.TraceID,
		Dependencies: dependencies,
	}
}

type Application struct {
	Config config.Config
	DB     *gorm.DB
	Redis  *RedisRuntime
	Router *gin.Engine
}

func Bootstrap(cfg config.Config) (*Application, error) {
	db, err := OpenDatabase(cfg.Database)
	if err != nil {
		return nil, err
	}

	redisRuntime, err := NewRedisRuntime(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("init redis runtime: %w", err)
	}
	if err := VerifyMQ(cfg.MQ); err != nil {
		return nil, fmt.Errorf("verify mq: %w", err)
	}

	if err := AutoMigrateModules(db, cfg.Service.Modules); err != nil {
		return nil, err
	}

	router := httptransport.NewRouter(httptransport.RouterDependencies{
		Health:  newHealthAdapter(cfg),
		Metrics: gametrics.NewRuntime(cfg.Service.Name, "http"),
		DB:      db,
		Redis:   redisRuntime,
		Config:  cfg,
	}, httptransport.RouterConfig{
		ServiceName:    cfg.Service.Name,
		EnabledModules: cfg.Service.Modules,
	})

	return &Application{
		Config: cfg,
		DB:     db,
		Redis:  redisRuntime,
		Router: router,
	}, nil
}

func (a *Application) Run() error {
	if a == nil {
		return fmt.Errorf("application is nil")
	}
	return a.Router.Run(a.Config.Address())
}

func healthDependencies(cfg config.Config) []DependencyHealth {
	dependencies := []DependencyHealth{
		CheckDependency("database", false, func() error {
			_, err := OpenDatabase(cfg.Database)
			return err
		}),
	}
	if cfg.Redis.Enabled {
		dependencies = append(dependencies, CheckDependency("redis", false, func() error {
			return VerifyRedis(cfg.Redis)
		}))
	}
	if cfg.MQ.Enabled {
		dependencies = append(dependencies, CheckDependency("mq", false, func() error {
			return VerifyMQ(cfg.MQ)
		}))
	}
	if strings.TrimSpace(cfg.Service.Name) == "gateway-service" {
		for _, item := range gatewayDependencies(cfg) {
			dependencies = append(dependencies, item)
		}
	}
	return dependencies
}

func gatewayDependencies(cfg config.Config) []DependencyHealth {
	targets := []struct {
		name string
		url  string
	}{
		{name: "identity-service", url: cfg.Gateway.IdentityURL},
		{name: "tenant-service", url: cfg.Gateway.TenantURL},
		{name: "agent-service", url: cfg.Gateway.AgentURL},
		{name: "relation-service", url: cfg.Gateway.RelationURL},
		{name: "game-service", url: cfg.Gateway.GameURL},
		{name: "rule-service", url: cfg.Gateway.RuleURL},
		{name: "activity-service", url: cfg.Gateway.ActivityURL},
		{name: "recharge-service", url: cfg.Gateway.RechargeURL},
		{name: "settlement-service", url: cfg.Gateway.SettlementURL},
		{name: "account-service", url: cfg.Gateway.AccountURL},
		{name: "withdrawal-service", url: cfg.Gateway.WithdrawalURL},
		{name: "risk-service", url: cfg.Gateway.RiskURL},
		{name: "report-service", url: cfg.Gateway.ReportURL},
		{name: "audit-service", url: cfg.Gateway.AuditURL},
	}
	dependencies := make([]DependencyHealth, 0, len(targets))
	for _, target := range targets {
		url := strings.TrimSpace(target.url)
		dependencies = append(dependencies, CheckDependency(target.name, false, func() error {
			return VerifyHTTPHealth(url)
		}))
	}
	return dependencies
}
