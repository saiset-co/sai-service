package metrics

import (
	"context"

	"github.com/saiset-co/sai-service/types"
)

var customMetricsCreators = make(map[string]types.MetricsManagerCreator)

func RegisterMetricsManager(metricsManagerName string, creator types.MetricsManagerCreator) {
	customMetricsCreators[metricsManagerName] = creator
}

func NewMetricsManager(ctx context.Context, config types.ConfigManager, logger types.Logger, health types.HealthManager) (types.MetricsManager, error) {
	metricsConfig := config.GetConfig().Metrics

	if !metricsConfig.Enabled {
		return nil, types.ErrMetricsIsDisabled
	}

	metricsManagerName := metricsConfig.Type

	var manager types.MetricsManager
	var err error

	switch metricsManagerName {
	case "memory":
		manager, err = NewMemoryMetrics(ctx, logger, metricsConfig, health)
	case "prometheus":
		manager, err = NewPrometheusMetrics(ctx, logger, metricsConfig, health)
	default:
		if creator, exists := customMetricsCreators[metricsManagerName]; exists {
			manager, err = creator(metricsConfig)
		} else {
			return nil, types.Errorf(types.ErrMetricsTypeUnknown, "type: %s", metricsManagerName)
		}
	}

	if err != nil {
		return nil, err
	}

	return manager, nil
}
