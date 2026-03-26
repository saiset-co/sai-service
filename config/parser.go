package config

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/saiset-co/sai-service/types"
)

type ParserState int32

const (
	ParserStateStopped ParserState = iota
	ParserStateStarting
	ParserStateRunning
	ParserStateStopping
)

type Parser struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          *map[string]interface{}
	state           atomic.Value
	mu              sync.RWMutex
	shutdownTimeout time.Duration
	parseTimeout    time.Duration
}

func NewParser(config *map[string]interface{}) *Parser {
	ctx, cancel := context.WithCancel(context.Background())

	parser := &Parser{
		ctx:             ctx,
		cancel:          cancel,
		config:          config,
		shutdownTimeout: 5 * time.Second,
		parseTimeout:    10 * time.Second,
	}

	parser.state.Store(ParserStateStopped)

	return parser
}

func (p *Parser) Start() error {
	if !p.transitionState(ParserStateStopped, ParserStateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if p.getState() == ParserStateStarting {
			p.setState(ParserStateRunning)
		}
	}()

	return nil
}

func (p *Parser) Stop() error {
	if !p.transitionState(ParserStateRunning, ParserStateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		p.setState(ParserStateStopped)
		p.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), p.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			p.mu.Lock()
			defer p.mu.Unlock()
			p.config = nil
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
		default:
		}
	}

	return nil
}

func (p *Parser) IsRunning() bool {
	return p.getState() == ParserStateRunning
}

func (p *Parser) GetValue(path string, defaultValue interface{}) interface{} {
	getValue := func() interface{} {
		p.mu.RLock()
		defer p.mu.RUnlock()
		return p.navigateToPath(path)
	}

	parseCtx, cancel := context.WithTimeout(p.ctx, 1*time.Second)
	defer cancel()

	done := make(chan interface{}, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- defaultValue
			}
		}()

		value := getValue()
		if value == nil {
			done <- defaultValue
		} else {
			done <- value
		}
	}()

	select {
	case value := <-done:
		return value
	case <-parseCtx.Done():
		return defaultValue
	case <-p.ctx.Done():
		return defaultValue
	}
}

func (p *Parser) GetAs(path string, target interface{}) error {
	if target == nil {
		return types.NewErrorf("target cannot be nil for path: %s", path)
	}

	getAs := func() error {
		p.mu.RLock()
		defer p.mu.RUnlock()

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

	parseCtx, cancel := context.WithTimeout(p.ctx, 2*time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- types.NewErrorf("get as panicked for path %s: %v", path, r)
			}
		}()

		done <- getAs()
	}()

	select {
	case err := <-done:
		return err
	case <-parseCtx.Done():
		return types.WrapError(parseCtx.Err(), "get as timeout for path: "+path)
	case <-p.ctx.Done():
		return types.WrapError(p.ctx.Err(), "parser shutting down")
	}
}

func (p *Parser) navigateToPath(path string) interface{} {
	if path == "" {
		return p.config
	}

	if p.config == nil {
		return nil
	}

	parts := strings.Split(path, ".")
	var current interface{} = *p.config

	for _, part := range parts {
		if part == "" {
			continue
		}

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

func (p *Parser) ValidatePath(path string) error {
	if path == "" {
		return nil
	}

	validateCtx, cancel := context.WithTimeout(p.ctx, 1*time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- types.NewErrorf("path validation panicked: %v", r)
			}
		}()

		parts := strings.Split(path, ".")
		for _, part := range parts {
			if strings.TrimSpace(part) == "" {
				done <- types.NewErrorf("invalid path part in: %s", path)
				return
			}
		}
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-validateCtx.Done():
		return types.WrapError(validateCtx.Err(), "path validation timeout")
	case <-p.ctx.Done():
		return types.WrapError(p.ctx.Err(), "parser shutting down")
	}
}

func (p *Parser) GetAllPaths() ([]string, error) {
	if !p.IsRunning() {
		return nil, types.ErrActionNotInitialized
	}

	pathsCtx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()

	done := make(chan []string, 1)
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- types.NewErrorf("get all paths panicked: %v", r)
			}
		}()

		p.mu.RLock()
		defer p.mu.RUnlock()

		if p.config == nil {
			done <- []string{}
			return
		}

		paths := p.collectPaths("", *p.config)
		done <- paths
	}()

	select {
	case paths := <-done:
		return paths, nil
	case err := <-errChan:
		return nil, err
	case <-pathsCtx.Done():
		return nil, types.WrapError(pathsCtx.Err(), "get all paths timeout")
	case <-p.ctx.Done():
		return nil, types.WrapError(p.ctx.Err(), "parser shutting down")
	}
}

func (p *Parser) collectPaths(prefix string, data interface{}) []string {
	var paths []string

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			currentPath := key
			if prefix != "" {
				currentPath = prefix + "." + key
			}
			paths = append(paths, currentPath)

			if subPaths := p.collectPaths(currentPath, value); len(subPaths) > 0 {
				paths = append(paths, subPaths...)
			}
		}
	case map[interface{}]interface{}:
		for k, value := range v {
			if key, ok := k.(string); ok {
				currentPath := key
				if prefix != "" {
					currentPath = prefix + "." + key
				}
				paths = append(paths, currentPath)

				if subPaths := p.collectPaths(currentPath, value); len(subPaths) > 0 {
					paths = append(paths, subPaths...)
				}
			}
		}
	}

	return paths
}

func (p *Parser) getState() ParserState {
	return p.state.Load().(ParserState)
}

func (p *Parser) setState(newState ParserState) bool {
	currentState := p.getState()
	return p.state.CompareAndSwap(currentState, newState)
}

func (p *Parser) transitionState(from, to ParserState) bool {
	return p.state.CompareAndSwap(from, to)
}
