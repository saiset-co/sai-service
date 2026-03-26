package database

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/ostafen/clover"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
)

type CloverDB struct {
	db     *clover.DB
	logger types.Logger
	config *types.DatabaseConfig
	state  atomic.Value
}

func NewCloverDB(ctx context.Context, logger types.Logger, config *types.DatabaseConfig, metrics types.MetricsManager, health types.HealthManager) (types.DatabaseManager, error) {
	var db *clover.DB
	var err error

	// Open or create CloverDB
	if config.Path == "" {
		// In-memory database
		db, err = clover.Open("")
	} else {
		// Persistent database
		db, err = clover.Open(config.Path)
	}

	if err != nil {
		return nil, types.WrapError(err, "failed to open CloverDB")
	}

	cdb := &CloverDB{
		db:     db,
		logger: logger,
		config: config,
	}

	cdb.state.Store(StateStopped)
	return cdb, nil
}

func (c *CloverDB) Start() error {
	if !c.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if c.getState() == StateStarting {
			c.setState(StateRunning)
		}
	}()

	c.logger.Info("CloverDB started", zap.String("path", c.config.Path))
	return nil
}

func (c *CloverDB) Stop() error {
	if !c.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		c.setState(StateStopped)
	}()

	err := c.db.Close()
	if err != nil {
		return types.WrapError(err, "failed to close CloverDB")
	}

	c.logger.Info("CloverDB stopped gracefully")
	return nil
}

func (c *CloverDB) IsRunning() bool {
	return c.getState() == StateRunning
}

func (c *CloverDB) CreateCollection(collectionName string) error {
	exists, err := c.db.HasCollection(collectionName)
	if err != nil {
		return types.WrapError(err, "failed to check collection existence")
	}

	if exists {
		return types.ErrDatabaseCollectionExists
	}

	err = c.db.CreateCollection(collectionName)
	if err != nil {
		return types.WrapError(err, "failed to create collection")
	}

	return nil
}

func (c *CloverDB) DropCollection(collectionName string) error {
	err := c.db.DropCollection(collectionName)
	if err != nil {
		return types.WrapError(err, "failed to drop collection")
	}

	return nil
}

func (c *CloverDB) CreateDocuments(ctx context.Context, request types.CreateDocumentsRequest) ([]string, error) {
	if len(request.Data) == 0 {
		return []string{}, nil
	}

	// Ensure collection exists
	exists, err := c.db.HasCollection(request.Collection)
	if err != nil {
		return nil, types.WrapError(err, "failed to check collection existence")
	}

	if !exists {
		err = c.db.CreateCollection(request.Collection)
		if err != nil {
			return nil, types.WrapError(err, "failed to create collection")
		}
	}

	var docs []*clover.Document
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

		// Create CloverDB document
		doc := clover.NewDocument()
		for key, value := range dataMap {
			doc.Set(key, value)
		}

		docs = append(docs, doc)
		ids = append(ids, internalID)
	}

	// Insert documents
	err = c.db.Insert(request.Collection, docs...)
	if err != nil {
		return nil, types.WrapError(err, "failed to insert documents")
	}

	return ids, nil
}

func (c *CloverDB) ReadDocuments(ctx context.Context, request types.ReadDocumentsRequest) ([]map[string]interface{}, int64, error) {
	// Check if collection exists
	exists, err := c.db.HasCollection(request.Collection)
	if err != nil {
		return nil, 0, types.WrapError(err, "failed to check collection existence")
	}

	if !exists {
		return []map[string]interface{}{}, 0, nil
	}

	// Build query
	query := c.db.Query(request.Collection)

	// Apply filters
	if request.Filter != nil && len(request.Filter) > 0 {
		query = c.applyFilters(query, request.Filter)
	}

	// Apply sorting
	if request.Sort != nil && len(request.Sort) > 0 {
		for field, order := range request.Sort {
			query = query.Sort(clover.SortOption{Field: field, Direction: order})
		}
	}

	// Apply pagination
	if request.Skip > 0 {
		query = query.Skip(request.Skip)
	}

	if request.Limit > 0 {
		query = query.Limit(request.Limit)
	}

	// Execute query
	cloverDocs, err := query.FindAll()
	if err != nil {
		return nil, 0, types.WrapError(err, "failed to find documents")
	}

	// Get total count (without pagination)
	totalQuery := c.db.Query(request.Collection)
	if request.Filter != nil && len(request.Filter) > 0 {
		totalQuery = c.applyFilters(totalQuery, request.Filter)
	}

	totalCount, err := totalQuery.Count()
	if err != nil {
		return nil, 0, types.WrapError(err, "failed to count documents")
	}

	// Convert to map format
	var results []map[string]interface{}
	for _, doc := range cloverDocs {
		docMap := make(map[string]interface{})

		// Use Unmarshal to convert document to map
		err = doc.Unmarshal(&docMap)
		if err != nil {
			continue
		}
		
		// Remove CloverDB internal fields
		delete(docMap, "_id")

		results = append(results, docMap)
	}

	return results, int64(totalCount), nil
}

func (c *CloverDB) UpdateDocuments(ctx context.Context, request types.UpdateDocumentsRequest) (int64, error) {
	// Check if collection exists
	exists, err := c.db.HasCollection(request.Collection)
	if err != nil {
		return 0, types.WrapError(err, "failed to check collection existence")
	}

	if !exists && !request.Upsert {
		return 0, nil
	}

	// Create collection if it doesn't exist and upsert is enabled
	if !exists && request.Upsert {
		err = c.db.CreateCollection(request.Collection)
		if err != nil {
			return 0, types.WrapError(err, "failed to create collection")
		}
	}

	// Build query
	query := c.db.Query(request.Collection)

	// Apply filters
	if request.Filter != nil && len(request.Filter) > 0 {
		query = c.applyFilters(query, request.Filter)
	}

	// Check if documents exist
	count, err := query.Count()
	if err != nil {
		return 0, types.WrapError(err, "failed to count matching documents")
	}

	if count == 0 && !request.Upsert {
		return 0, nil
	}

	// Handle upsert case
	if count == 0 && request.Upsert {
		// Create new document
		newDoc := make(map[string]interface{})

		// Apply update operations
		if err := c.applyUpdateOperations(newDoc, request.Data); err != nil {
			return 0, err
		}

		// Add metadata
		newDoc["internal_id"] = uuid.New().String()
		newDoc["cr_time"] = time.Now().UnixNano()
		newDoc["ch_time"] = time.Now().UnixNano()

		// Create CloverDB document
		doc := clover.NewDocument()
		for key, value := range newDoc {
			doc.Set(key, value)
		}

		// Insert new document
		err = c.db.Insert(request.Collection, doc)
		if err != nil {
			return 0, types.WrapError(err, "failed to insert upserted document")
		}

		return 1, nil
	}

	// Prepare update data
	updateMap := make(map[string]interface{})
	if err := c.applyUpdateOperations(updateMap, request.Data); err != nil {
		return 0, err
	}

	// Add timestamp
	updateMap["ch_time"] = time.Now().UnixNano()

	// Execute update
	err = query.Update(updateMap)
	if err != nil {
		return 0, types.WrapError(err, "failed to update documents")
	}

	return int64(count), nil
}

func (c *CloverDB) DeleteDocuments(ctx context.Context, request types.DeleteDocumentsRequest) (int64, error) {
	// Check if collection exists
	exists, err := c.db.HasCollection(request.Collection)
	if err != nil {
		return 0, types.WrapError(err, "failed to check collection existence")
	}

	if !exists {
		return 0, nil
	}

	// Build query
	query := c.db.Query(request.Collection)

	// Apply filters
	if request.Filter != nil && len(request.Filter) > 0 {
		query = c.applyFilters(query, request.Filter)
	}

	// Count documents to be deleted
	count, err := query.Count()
	if err != nil {
		return 0, types.WrapError(err, "failed to count matching documents")
	}

	if count == 0 {
		return 0, nil
	}

	// Execute deletion
	err = query.Delete()
	if err != nil {
		return 0, types.WrapError(err, "failed to delete documents")
	}

	return int64(count), nil
}

// Helper methods

func (c *CloverDB) applyFilters(query *clover.Query, filter map[string]interface{}) *clover.Query {
	for key, value := range filter {
		query = c.applyFieldFilter(query, key, value)
	}
	return query
}

func (c *CloverDB) applyFieldFilter(query *clover.Query, key string, value interface{}) *clover.Query {
	switch val := value.(type) {
	case map[string]interface{}:
		// MongoDB-style operators
		for op, opValue := range val {
			switch op {
			case "$eq":
				query = query.Where(clover.Field(key).Eq(opValue))
			case "$ne":
				query = query.Where(clover.Field(key).Neq(opValue))
			case "$gt":
				query = query.Where(clover.Field(key).Gt(opValue))
			case "$gte":
				query = query.Where(clover.Field(key).GtEq(opValue))
			case "$lt":
				query = query.Where(clover.Field(key).Lt(opValue))
			case "$lte":
				query = query.Where(clover.Field(key).LtEq(opValue))
			case "$in":
				if arr, ok := opValue.([]interface{}); ok {
					query = query.Where(clover.Field(key).In(arr...))
				}
			case "$nin":
				if arr, ok := opValue.([]interface{}); ok {
					// NotIn doesn't exist, use negation of In
					query = query.Where(clover.Field(key).In(arr...).Not())
				}
			case "$exists":
				if exists, ok := opValue.(bool); ok {
					if exists {
						query = query.Where(clover.Field(key).Exists())
					} else {
						query = query.Where(clover.Field(key).NotExists())
					}
				}
			case "$regex":
				if regexStr, ok := opValue.(string); ok {
					query = query.Where(clover.Field(key).Like(regexStr))
				}
			}
		}
	default:
		// Direct equality comparison
		query = query.Where(clover.Field(key).Eq(value))
	}

	return query
}

func (c *CloverDB) applyUpdateOperations(doc map[string]interface{}, update interface{}) error {
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
					if incVal, ok := c.toFloat64(val); ok {
						if current, exists := doc[key]; exists {
							if currentVal, ok := c.toFloat64(current); ok {
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

func (c *CloverDB) toFloat64(v interface{}) (float64, bool) {
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

// State management helpers

func (c *CloverDB) getState() State {
	return c.state.Load().(State)
}

func (c *CloverDB) setState(newState State) bool {
	currentState := c.getState()
	return c.state.CompareAndSwap(currentState, newState)
}

func (c *CloverDB) transitionState(from, to State) bool {
	return c.state.CompareAndSwap(from, to)
}
