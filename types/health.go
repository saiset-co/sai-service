package types

import (
	"context"
	"time"
)

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusUnknown   HealthStatus = "unknown"
)

type HealthManager interface {
	LifecycleManager
	RegisterChecker(name string, checker HealthChecker)
	Check(ctx context.Context) HealthReport
}

type HealthStatus string

type HealthCheck struct {
	Name      string                 `json:"name"`
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message,omitempty"`
	LastCheck time.Time              `json:"last_check"`
	Duration  time.Duration          `json:"duration"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

type HealthChecker func(ctx context.Context) HealthCheck

type HealthReport struct {
	Status    HealthStatus           `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    time.Duration          `json:"uptime"`
	Service   ServiceInfo            `json:"service"`
	Checks    map[string]HealthCheck `json:"checks"`
	Summary   HealthSummary          `json:"summary"`
}

type ServiceInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
}

type HealthSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
}
