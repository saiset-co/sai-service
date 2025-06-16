package logger

import (
	"github.com/saiset-co/sai-service/types"
)

var customLoggerCreators = make(map[string]types.LoggerCreator)

func RegisterLogger(loggerName string, creator types.LoggerCreator) {
	customLoggerCreators[loggerName] = creator
}

func NewLogger(config types.ConfigManager) (types.Logger, error) {
	loggerConfig := config.GetConfig().Logger
	loggerName := "default"

	if loggerConfig.Type != "" {
		loggerName = loggerConfig.Type
	}

	switch loggerName {
	case "default":
		return NewDefaultLogger(loggerConfig)
	default:
		if creator, exists := customLoggerCreators[loggerName]; exists {
			return creator(loggerConfig.Config)
		} else {
			return nil, types.Errorf(types.ErrLoggerTypeUnknown, "logger type: %s", loggerName)
		}
	}
}
