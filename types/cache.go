package types

import (
	"time"
)

type CacheManager interface {
	LifecycleManager
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error
	Invalidate(keys ...string) error
	GetRevision(key string) uint64
	SetRevision(key string, revision uint64)
	BuildCacheKey(requestPath []byte, dependencies []string, metadata map[string][]byte) string
}

type CacheManagerCreator func(config interface{}) (CacheManager, error)

type CacheEntry struct {
	Key          string            `json:"key"`
	Value        interface{}       `json:"value"`
	TTL          time.Duration     `json:"ttl"`
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Dependencies []string          `json:"dependencies"`
	Metadata     map[string]string `json:"metadata"`
}
