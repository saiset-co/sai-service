package database

import (
	"context"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

var customDatabaseCreators = make(map[string]types.DatabaseManagerCreator)

func RegisterDatabaseManager(databaseType string, creator types.DatabaseManagerCreator) {
	customDatabaseCreators[databaseType] = creator
}

func NewManager(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, health types.HealthManager) (types.DatabaseManager, error) {
	dbConfig := config.GetConfig().Database

	if !dbConfig.Enabled {
		return nil, types.ErrDatabaseIsDisabled
	}

	databaseType := dbConfig.Type

	var impl types.DatabaseManager
	var err error

	switch databaseType {
	case "clover":
		impl, err = NewCloverDB(ctx, logger, dbConfig, metrics, health)
	case "memory":
		impl, err = NewMemoryDB(ctx, logger, dbConfig, metrics, health)
	default:
		if creator, exists := customDatabaseCreators[databaseType]; exists {
			impl, err = creator(dbConfig)
		} else {
			return nil, types.Errorf(types.ErrDatabaseTypeUnknown, "type: %s", databaseType)
		}
	}

	if err != nil {
		return nil, err
	}

	return newInstrumentedDatabaseManager(logger, impl), nil
}

type instrumentedDatabaseManager struct {
	impl   types.DatabaseManager
	logger types.Logger
	state  atomic.Value
}

func newInstrumentedDatabaseManager(logger types.Logger, impl types.DatabaseManager) types.DatabaseManager {
	instrumented := &instrumentedDatabaseManager{
		impl:   impl,
		logger: logger,
	}

	instrumented.state.Store(StateStopped)
	return instrumented
}

func (dm *instrumentedDatabaseManager) Start() error {
	if !dm.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if dm.getState() == StateStarting {
			dm.setState(StateRunning)
		}
	}()

	err := dm.impl.Start()
	if err != nil {
		dm.setState(StateStopped)
		return err
	}

	dm.logger.Info("Database manager started")
	return nil
}

func (dm *instrumentedDatabaseManager) Stop() error {
	if !dm.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		dm.setState(StateStopped)
	}()

	err := dm.impl.Stop()
	if err != nil {
		dm.logger.Error("Failed to stop database implementation", zap.Error(err))
		return err
	}

	dm.logger.Info("Database manager stopped gracefully")
	return nil
}

func (dm *instrumentedDatabaseManager) IsRunning() bool {
	return dm.getState() == StateRunning
}

func (dm *instrumentedDatabaseManager) CreateDocuments(ctx context.Context, request types.CreateDocumentsRequest) ([]string, error) {
	return dm.impl.CreateDocuments(ctx, request)
}

func (dm *instrumentedDatabaseManager) ReadDocuments(ctx context.Context, request types.ReadDocumentsRequest) ([]map[string]interface{}, int64, error) {
	return dm.impl.ReadDocuments(ctx, request)
}

func (dm *instrumentedDatabaseManager) UpdateDocuments(ctx context.Context, request types.UpdateDocumentsRequest) (int64, error) {
	return dm.impl.UpdateDocuments(ctx, request)
}

func (dm *instrumentedDatabaseManager) DeleteDocuments(ctx context.Context, request types.DeleteDocumentsRequest) (int64, error) {
	return dm.impl.DeleteDocuments(ctx, request)
}

func (dm *instrumentedDatabaseManager) CreateCollection(collectionName string) error {
	return dm.impl.CreateCollection(collectionName)
}

func (dm *instrumentedDatabaseManager) DropCollection(collectionName string) error {
	return dm.impl.DropCollection(collectionName)
}

// Helper methods for state management

func (dm *instrumentedDatabaseManager) getState() State {
	return dm.state.Load().(State)
}

func (dm *instrumentedDatabaseManager) setState(newState State) bool {
	currentState := dm.getState()
	return dm.state.CompareAndSwap(currentState, newState)
}

func (dm *instrumentedDatabaseManager) transitionState(from, to State) bool {
	return dm.state.CompareAndSwap(from, to)
}
