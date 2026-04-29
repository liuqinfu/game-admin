package app

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type DependencyHealth struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Optional  bool   `json:"optional,omitempty"`
	CheckedAt string `json:"checkedAt,omitempty"`
}

type HealthStatus struct {
	Status       string             `json:"status"`
	Service      string             `json:"service"`
	Version      string             `json:"version"`
	Timestamp    time.Time          `json:"timestamp"`
	RequestID    string             `json:"requestID,omitempty"`
	TraceID      string             `json:"traceID,omitempty"`
	Dependencies []DependencyHealth `json:"dependencies,omitempty"`
}

type HealthStore struct {
	Service      string
	Version      string
	Dependencies []DependencyHealth
}

type HealthRuntime struct {
	store HealthStore
}

func NewHealthRuntime(store HealthStore) HealthRuntime {
	return HealthRuntime{store: store}
}

func (h HealthStore) Status(now time.Time) HealthStatus {
	return h.StatusWithContext(now, "", "")
}

func (h HealthStore) StatusWithContext(now time.Time, requestID, traceID string) HealthStatus {
	service := h.Service
	if service == "" {
		service = "game-admin-backend"
	}
	version := h.Version
	if version == "" {
		version = "dev"
	}
	dependencies := cloneDependencies(h.Dependencies)

	status := "ok"
	for _, dependency := range dependencies {
		if dependency.Optional {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(dependency.Status), "ok") {
			continue
		}
		status = "degraded"
		break
	}

	return HealthStatus{
		Status:       status,
		Service:      service,
		Version:      version,
		Timestamp:    now.UTC(),
		RequestID:    strings.TrimSpace(requestID),
		TraceID:      strings.TrimSpace(traceID),
		Dependencies: dependencies,
	}
}

func (h HealthRuntime) Live(now time.Time, requestID, traceID string) HealthStatus {
	status := h.store.StatusWithContext(now, requestID, traceID)
	status.Dependencies = nil
	status.Status = "ok"
	return status
}

func (h HealthRuntime) Ready(now time.Time, requestID, traceID string) HealthStatus {
	return h.store.StatusWithContext(now, requestID, traceID)
}

func cloneDependencies(items []DependencyHealth) []DependencyHealth {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]DependencyHealth, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, DependencyHealth{
			Name:      strings.TrimSpace(item.Name),
			Status:    normalizeDependencyStatus(item.Status),
			Message:   strings.TrimSpace(item.Message),
			Optional:  item.Optional,
			CheckedAt: strings.TrimSpace(item.CheckedAt),
		})
	}
	return cloned
}

func normalizeDependencyStatus(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "ok") {
		return "ok"
	}
	return "error"
}

func CheckDependency(name string, optional bool, check func() error) DependencyHealth {
	dependency := DependencyHealth{
		Name:      strings.TrimSpace(name),
		Status:    "ok",
		Optional:  optional,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if check == nil {
		dependency.Status = "error"
		dependency.Message = "check is nil"
		return dependency
	}
	if err := check(); err != nil {
		dependency.Status = "error"
		dependency.Message = healthErrorMessage(err)
	}
	return dependency
}

func healthErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "unknown error"
	}
	return message
}

func RequireHealthy(status HealthStatus) error {
	if strings.EqualFold(strings.TrimSpace(status.Status), "ok") {
		return nil
	}
	failed := make([]string, 0, len(status.Dependencies))
	for _, dependency := range status.Dependencies {
		if dependency.Optional || strings.EqualFold(strings.TrimSpace(dependency.Status), "ok") {
			continue
		}
		message := dependency.Name
		if strings.TrimSpace(dependency.Message) != "" {
			message = fmt.Sprintf("%s=%s", dependency.Name, dependency.Message)
		}
		failed = append(failed, message)
	}
	if len(failed) == 0 {
		return errors.New("service not ready")
	}
	return fmt.Errorf("service not ready: %s", strings.Join(failed, "; "))
}
