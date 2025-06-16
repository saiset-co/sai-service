package config

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/saiset-co/sai-service/types"
)

type Parser struct {
	config *types.ServiceConfig
	data   map[string]interface{}
}

func NewParser(config *types.ServiceConfig) *Parser {
	parser := &Parser{
		config: config,
		data:   make(map[string]interface{}),
	}

	configBytes, err := yaml.Marshal(config)
	if err != nil {
		return parser
	}

	if err := yaml.Unmarshal(configBytes, &parser.data); err != nil {
		parser.data = make(map[string]interface{})
	}

	return parser
}

func (p *Parser) GetValue(path string, defaultValue interface{}) interface{} {
	value := p.navigateToPath(path)
	if value == nil {
		return defaultValue
	}
	return value
}

func (p *Parser) GetAs(path string, target interface{}) error {
	value := p.navigateToPath(path)
	if value == nil {
		return types.Errorf(types.ErrConfigNotFound, "path: %s", path)
	}

	valueBytes, err := yaml.Marshal(value)
	if err != nil {
		return types.WrapError(err, "failed to marshal config value")
	}

	if err = yaml.Unmarshal(valueBytes, target); err != nil {
		return types.WrapError(err, "failed to unmarshal config value")
	}

	return nil
}

func (p *Parser) navigateToPath(path string) interface{} {
	if path == "" {
		return p.data
	}

	parts := strings.Split(path, ".")
	var current interface{} = p.data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, exists := v[part]; exists {
				current = val
			} else {
				return nil
			}
		case map[interface{}]interface{}:
			if val, exists := v[part]; exists {
				current = val
			} else {
				return nil
			}
		default:
			return nil
		}

		if current == nil {
			return nil
		}
	}

	return current
}
