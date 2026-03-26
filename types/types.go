package types

import (
	"context"
)

type LifecycleManager interface {
	Start() error
	Stop() error
	IsRunning() bool
}

// Database Manager types - compatible with sai-storage API

type DatabaseManager interface {
	LifecycleManager
	CreateDocuments(ctx context.Context, request CreateDocumentsRequest) ([]string, error)
	ReadDocuments(ctx context.Context, request ReadDocumentsRequest) ([]map[string]interface{}, int64, error)
	UpdateDocuments(ctx context.Context, request UpdateDocumentsRequest) (int64, error)
	DeleteDocuments(ctx context.Context, request DeleteDocumentsRequest) (int64, error)
	CreateCollection(collectionName string) error
	DropCollection(collectionName string) error
}

type CreateDocumentsRequest struct {
	Collection string        `json:"collection"`
	Data       []interface{} `json:"data"`
}

type ReadDocumentsRequest struct {
	Collection string                 `json:"collection" validate:"required"`
	Filter     map[string]interface{} `json:"filter,omitempty"`
	Sort       map[string]int         `json:"sort,omitempty"`
	Limit      int                    `json:"limit,omitempty"`
	Skip       int                    `json:"skip,omitempty"`
}

type UpdateDocumentsRequest struct {
	Collection string                 `json:"collection"`
	Filter     map[string]interface{} `json:"filter"`
	Data       interface{}            `json:"data"`
	Upsert     bool                   `json:"upsert,omitempty"`
}

type DeleteDocumentsRequest struct {
	Collection string                 `json:"collection"`
	Filter     map[string]interface{} `json:"filter"`
}

type DatabaseManagerCreator func(*DatabaseConfig) (DatabaseManager, error)
