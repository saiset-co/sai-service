package database

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/saiset-co/sai-service/types"
)

type MemoryDB struct {
	collections map[string]map[string]map[string]interface{}
	mutex       sync.RWMutex
	logger      types.Logger
	config      *types.DatabaseConfig
	state       atomic.Value
}

func NewMemoryDB(ctx context.Context, logger types.Logger, config *types.DatabaseConfig, metrics types.MetricsManager, health types.HealthManager) (types.DatabaseManager, error) {
	mdb := &MemoryDB{
		collections: make(map[string]map[string]map[string]interface{}),
		logger:      logger,
		config:      config,
	}

	mdb.state.Store(StateStopped)
	return mdb, nil
}

func (m *MemoryDB) Start() error {
	if !m.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if m.getState() == StateStarting {
			m.setState(StateRunning)
		}
	}()

	m.logger.Info("MemoryDB started")
	return nil
}

func (m *MemoryDB) Stop() error {
	if !m.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		m.setState(StateStopped)
	}()

	m.mutex.Lock()
	m.collections = make(map[string]map[string]map[string]interface{})
	m.mutex.Unlock()

	m.logger.Info("MemoryDB stopped gracefully")
	return nil
}

func (m *MemoryDB) IsRunning() bool {
	return m.getState() == StateRunning
}

func (m *MemoryDB) CreateCollection(collectionName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.collections[collectionName]; exists {
		return types.ErrDatabaseCollectionExists
	}

	m.collections[collectionName] = make(map[string]map[string]interface{})
	return nil
}

func (m *MemoryDB) DropCollection(collectionName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.collections, collectionName)
	return nil
}

func (m *MemoryDB) CreateDocuments(ctx context.Context, request types.CreateDocumentsRequest) ([]string, error) {
	if len(request.Data) == 0 {
		return []string{}, nil
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create collection if it doesn't exist
	if _, exists := m.collections[request.Collection]; !exists {
		m.collections[request.Collection] = make(map[string]map[string]interface{})
	}

	var ids []string
	now := time.Now().UnixNano()

	for i, data := range request.Data {
		// Convert to map
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			return nil, types.NewError("data must be a map")
		}

		// Generate internal_id if not provided
		internalID := uuid.New().String()
		dataMap["internal_id"] = internalID
		dataMap["cr_time"] = now + int64(i)
		dataMap["ch_time"] = now + int64(i)

		// Deep copy the document
		docCopy := make(map[string]interface{})
		m.deepCopy(dataMap, docCopy)

		// Store document
		m.collections[request.Collection][internalID] = docCopy
		ids = append(ids, internalID)
	}

	return ids, nil
}

func (m *MemoryDB) ReadDocuments(ctx context.Context, request types.ReadDocumentsRequest) ([]map[string]interface{}, int64, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Check if collection exists
	collection, exists := m.collections[request.Collection]
	if !exists {
		return []map[string]interface{}{}, 0, nil
	}

	var allDocs []map[string]interface{}

	// Apply filters and collect matching documents
	for _, doc := range collection {
		if m.matchesFilter(doc, request.Filter) {
			// Deep copy the document
			docCopy := make(map[string]interface{})
			m.deepCopy(doc, docCopy)
			allDocs = append(allDocs, docCopy)
		}
	}

	total := int64(len(allDocs))

	// Apply sorting
	if request.Sort != nil && len(request.Sort) > 0 {
		m.sortDocuments(allDocs, request.Sort)
	}

	// Apply pagination
	if request.Skip > 0 {
		if request.Skip >= len(allDocs) {
			return []map[string]interface{}{}, total, nil
		}
		allDocs = allDocs[request.Skip:]
	}

	if request.Limit > 0 && request.Limit < len(allDocs) {
		allDocs = allDocs[:request.Limit]
	}

	return allDocs, total, nil
}

func (m *MemoryDB) UpdateDocuments(ctx context.Context, request types.UpdateDocumentsRequest) (int64, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if collection exists
	collection, exists := m.collections[request.Collection]
	if !exists && !request.Upsert {
		return 0, nil
	}

	// Create collection if it doesn't exist and upsert is enabled
	if !exists && request.Upsert {
		m.collections[request.Collection] = make(map[string]map[string]interface{})
		collection = m.collections[request.Collection]
	}

	var matchingDocs []string
	for id, doc := range collection {
		if m.matchesFilter(doc, request.Filter) {
			matchingDocs = append(matchingDocs, id)
		}
	}

	if len(matchingDocs) == 0 && !request.Upsert {
		return 0, nil
	}

	// Handle upsert case
	if len(matchingDocs) == 0 && request.Upsert {
		// Create new document
		newDoc := make(map[string]interface{})

		// Apply update operations
		if err := m.applyUpdateOperations(newDoc, request.Data); err != nil {
			return 0, err
		}

		// Add metadata
		internalID := uuid.New().String()
		newDoc["internal_id"] = internalID
		newDoc["cr_time"] = time.Now().UnixNano()
		newDoc["ch_time"] = time.Now().UnixNano()

		// Store new document
		collection[internalID] = newDoc
		return 1, nil
	}

	// Update existing documents
	now := time.Now().UnixNano()
	for _, id := range matchingDocs {
		doc := collection[id]

		// Apply update operations
		if err := m.applyUpdateOperations(doc, request.Data); err != nil {
			continue
		}

		// Update timestamp
		doc["ch_time"] = now
	}

	return int64(len(matchingDocs)), nil
}

func (m *MemoryDB) DeleteDocuments(ctx context.Context, request types.DeleteDocumentsRequest) (int64, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if collection exists
	collection, exists := m.collections[request.Collection]
	if !exists {
		return 0, nil
	}

	var toDelete []string
	for id, doc := range collection {
		if m.matchesFilter(doc, request.Filter) {
			toDelete = append(toDelete, id)
		}
	}

	// Delete documents
	for _, id := range toDelete {
		delete(collection, id)
	}

	return int64(len(toDelete)), nil
}

// Helper methods (similar to Redis implementation)

func (m *MemoryDB) deepCopy(src, dst map[string]interface{}) {
	for k, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			nestedDst := make(map[string]interface{})
			m.deepCopy(val, nestedDst)
			dst[k] = nestedDst
		default:
			dst[k] = v
		}
	}
}

func (m *MemoryDB) matchesFilter(doc map[string]interface{}, filter map[string]interface{}) bool {
	if filter == nil {
		return true
	}

	for key, value := range filter {
		if !m.matchesField(doc, key, value) {
			return false
		}
	}
	return true
}

func (m *MemoryDB) matchesField(doc map[string]interface{}, key string, filterValue interface{}) bool {
	// Handle nested keys (e.g., "user.id")
	keys := strings.Split(key, ".")
	current := doc

	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, compare value
			docValue, exists := current[k]
			if !exists {
				return false
			}
			return m.compareValues(docValue, filterValue)
		} else {
			// Navigate deeper
			next, exists := current[k]
			if !exists {
				return false
			}
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return false
			}
		}
	}

	return false
}

func (m *MemoryDB) compareValues(docValue, filterValue interface{}) bool {
	// Handle different comparison types
	switch filter := filterValue.(type) {
	case map[string]interface{}:
		// MongoDB-style operators
		for op, value := range filter {
			switch op {
			case "$eq":
				return docValue == value
			case "$ne":
				return docValue != value
			case "$gt":
				return m.compareNumbers(docValue, value, ">")
			case "$gte":
				return m.compareNumbers(docValue, value, ">=")
			case "$lt":
				return m.compareNumbers(docValue, value, "<")
			case "$lte":
				return m.compareNumbers(docValue, value, "<=")
			case "$in":
				if arr, ok := value.([]interface{}); ok {
					for _, v := range arr {
						if docValue == v {
							return true
						}
					}
				}
				return false
			case "$nin":
				if arr, ok := value.([]interface{}); ok {
					for _, v := range arr {
						if docValue == v {
							return false
						}
					}
				}
				return true
			}
		}
		return false
	default:
		// Direct equality comparison
		return docValue == filterValue
	}
}

func (m *MemoryDB) compareNumbers(a, b interface{}, op string) bool {
	aVal, aOk := m.toFloat64(a)
	bVal, bOk := m.toFloat64(b)

	if !aOk || !bOk {
		return false
	}

	switch op {
	case ">":
		return aVal > bVal
	case ">=":
		return aVal >= bVal
	case "<":
		return aVal < bVal
	case "<=":
		return aVal <= bVal
	}
	return false
}

func (m *MemoryDB) toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func (m *MemoryDB) sortDocuments(docs []map[string]interface{}, sort map[string]int) {
	// Simple sorting implementation - would need more robust implementation for production
	// This is a basic implementation for demonstration
}

func (m *MemoryDB) applyUpdateOperations(doc map[string]interface{}, update interface{}) error {
	updateMap, ok := update.(map[string]interface{})
	if !ok {
		return types.NewError("update data must be a map")
	}

	for op, value := range updateMap {
		switch op {
		case "$set":
			if setMap, ok := value.(map[string]interface{}); ok {
				for key, val := range setMap {
					doc[key] = val
				}
			}
		case "$unset":
			if unsetMap, ok := value.(map[string]interface{}); ok {
				for key := range unsetMap {
					delete(doc, key)
				}
			}
		case "$inc":
			if incMap, ok := value.(map[string]interface{}); ok {
				for key, val := range incMap {
					if incVal, ok := m.toFloat64(val); ok {
						if current, exists := doc[key]; exists {
							if currentVal, ok := m.toFloat64(current); ok {
								doc[key] = currentVal + incVal
							}
						} else {
							doc[key] = incVal
						}
					}
				}
			}
		default:
			// Direct field assignment
			doc[op] = value
		}
	}

	return nil
}

// State management helpers

func (m *MemoryDB) getState() State {
	return m.state.Load().(State)
}

func (m *MemoryDB) setState(newState State) bool {
	currentState := m.getState()
	return m.state.CompareAndSwap(currentState, newState)
}

func (m *MemoryDB) transitionState(from, to State) bool {
	return m.state.CompareAndSwap(from, to)
}
