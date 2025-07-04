package logger

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

type Manager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	config          types.ConfigManager
	state           atomic.Value
	shutdownTimeout time.Duration
}

var customLoggerCreators = make(map[string]types.LoggerCreator)

func RegisterLogger(loggerName string, creator types.LoggerCreator) {
	customLoggerCreators[loggerName] = creator
}

func NewManager(ctx context.Context, config types.ConfigManager) (types.LoggerManager, error) {
	loggerConfig := config.GetConfig().Logger
	if loggerConfig == nil {
		return nil, types.ErrLoggerConfigInvalid
	}

	managerCtx, cancel := context.WithCancel(ctx)

	logger, err := createLogger(loggerConfig)
	if err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to create logger")
	}

	manager := &Manager{
		ctx:             managerCtx,
		cancel:          cancel,
		logger:          logger,
		config:          config,
		shutdownTimeout: 10 * time.Second,
	}

	manager.state.Store(StateStopped)

	return manager, nil
}

func (m *Manager) Start() error {
	if !m.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if m.getState() == StateStarting {
			m.setState(StateRunning)
		}
	}()

	return nil
}

func (m *Manager) Stop() error {
	if !m.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		m.setState(StateStopped)
		m.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			if syncer, hasSyncer := m.logger.(interface{ Sync() error }); hasSyncer {
				_ = syncer.Sync()
			}
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
		default:
		}
	}

	return nil
}

func (m *Manager) IsRunning() bool {
	return m.getState() == StateRunning
}

func (m *Manager) Error(msg string, fields ...zap.Field) {
	m.logger.Error(msg, fields...)
}

func (m *Manager) ErrorWithErrStack(msg string, err error, fields ...zap.Field) {
	m.logger.ErrorWithErrStack(msg, err, fields...)
}

func (m *Manager) ErrorWithStack(msg string, stack string, fields ...zap.Field) {
	m.logger.ErrorWithStack(msg, stack, fields...)
}

func (m *Manager) Warn(msg string, fields ...zap.Field) {
	m.logger.Warn(msg, fields...)
}

func (m *Manager) Info(msg string, fields ...zap.Field) {
	m.logger.Info(msg, fields...)
}

func (m *Manager) Debug(msg string, fields ...zap.Field) {
	m.logger.Debug(msg, fields...)
}

func (m *Manager) Log(lvl zapcore.Level, msg string, fields ...zap.Field) {
	m.logger.Log(lvl, msg, fields...)
}

func (m *Manager) getState() State {
	return m.state.Load().(State)
}

func (m *Manager) setState(newState State) bool {
	currentState := m.getState()
	return m.state.CompareAndSwap(currentState, newState)
}

func (m *Manager) transitionState(from, to State) bool {
	return m.state.CompareAndSwap(from, to)
}

func createLogger(loggerConfig *types.LoggerConfig) (types.Logger, error) {
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
