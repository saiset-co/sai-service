package config

import (
	"sync"

	"github.com/saiset-co/sai-service/types"
)

type ConfigurationManager struct {
	config     *types.ServiceConfig
	configPath string
	loader     *Loader
	parser     *Parser
	mu         sync.RWMutex
}

func NewConfigurationManager(configPath string) (*ConfigurationManager, error) {
	cm := &ConfigurationManager{
		configPath: configPath,
		loader:     NewLoader(),
		mu:         sync.RWMutex{},
	}

	if err := cm.Load(); err != nil {
		return nil, types.WrapError(err, "failed to load initial configuration")
	}

	return cm, nil
}

func (cm *ConfigurationManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	config, err := cm.loader.LoadFromFile(cm.configPath)
	if err != nil {
		return types.WrapError(err, "failed to load configuration from file")
	}

	cm.config = config
	cm.parser = NewParser(config)

	return nil
}

func (cm *ConfigurationManager) GetConfig() *types.ServiceConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

func (cm *ConfigurationManager) GetValue(path string, defaultValue interface{}) interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.parser.GetValue(path, defaultValue)
}

func (cm *ConfigurationManager) GetAs(path string, target interface{}) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.parser.GetAs(path, target)
}
