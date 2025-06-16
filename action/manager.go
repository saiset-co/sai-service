package action

import (
	"context"

	"github.com/saiset-co/sai-service/types"
)

var customActionCreators = make(map[string]types.ActionBrokerCreator)

func RegisterActionBroker(actionBrokerName string, creator types.ActionBrokerCreator) {
	customActionCreators[actionBrokerName] = creator
}

func NewActionBroker(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, health types.HealthManager) (types.ActionBroker, error) {
	actionsConfig := config.GetConfig().Actions

	if !actionsConfig.Enabled {
		return nil, types.ErrActionIsDisabled
	}

	return NewEventDispatcher(ctx, config, logger, metrics, health)
}
