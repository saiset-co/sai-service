package sai

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/saiset-co/sai-service/types"
)

type Container struct {
	config         atomic.Pointer[types.ConfigManager]
	logger         atomic.Pointer[types.Logger]
	router         atomic.Pointer[types.HTTPRouter]
	cache          atomic.Pointer[types.CacheManager]
	httpServer     atomic.Pointer[types.HTTPServer]
	clientManager  atomic.Pointer[types.ClientManager]
	cron           atomic.Pointer[types.CronManager]
	metrics        atomic.Pointer[types.MetricsManager]
	actions        atomic.Pointer[types.ActionBroker]
	middlewares    atomic.Pointer[types.MiddlewareManager]
	health         atomic.Pointer[types.HealthManager]
	documentation  atomic.Pointer[types.DocumentationManager]
	tlsManager     atomic.Pointer[types.TLSManager]
	componentCache map[string]unsafe.Pointer
	cacheMu        sync.RWMutex
	initialized    int32
}

var globalContainer *Container

func InitContainer() *Container {
	globalContainer = &Container{
		componentCache: make(map[string]unsafe.Pointer),
	}

	return globalContainer
}

func SetContainer(container *Container) {
	globalContainer = container
	atomic.StoreInt32(&container.initialized, 1)
}

func Config() types.ConfigManager {
	if ptr := globalContainer.config.Load(); ptr != nil {
		return *ptr
	}
	panic("ConfigManager not initialized")
}

func Logger() types.Logger {
	if ptr := globalContainer.logger.Load(); ptr != nil {
		return *ptr
	}
	panic("Logger not initialized")
}

func Cache() types.CacheManager {
	if ptr := globalContainer.cache.Load(); ptr != nil {
		return *ptr
	}
	panic("CacheManager not initialized")
}

func Router() types.HTTPRouter {
	if ptr := globalContainer.router.Load(); ptr != nil {
		return *ptr
	}
	panic("Router not initialized")
}

func HTTPServer() types.HTTPServer {
	if ptr := globalContainer.httpServer.Load(); ptr != nil {
		return *ptr
	}
	panic("HTTPServer not initialized")
}

func ClientManager() types.ClientManager {
	if ptr := globalContainer.clientManager.Load(); ptr != nil {
		return *ptr
	}
	panic("ClientManager not initialized")
}

func Cron() types.CronManager {
	if ptr := globalContainer.cron.Load(); ptr != nil {
		return *ptr
	}
	panic("CronManager not initialized")
}

func Metrics() types.MetricsManager {
	if ptr := globalContainer.metrics.Load(); ptr != nil {
		return *ptr
	}
	panic("MetricsManager not initialized")
}

func Actions() types.ActionBroker {
	if ptr := globalContainer.actions.Load(); ptr != nil {
		return *ptr
	}
	panic("ActionBroker not initialized")
}

func Middlewares() types.MiddlewareManager {
	if ptr := globalContainer.middlewares.Load(); ptr != nil {
		return *ptr
	}
	panic("MiddlewareManager not initialized")
}

func Health() types.HealthManager {
	if ptr := globalContainer.health.Load(); ptr != nil {
		return *ptr
	}
	panic("HealthManager not initialized")
}

func Documentation() types.DocumentationManager {
	if ptr := globalContainer.documentation.Load(); ptr != nil {
		return *ptr
	}
	panic("DocumentationManager not initialized")
}

func TLSManager() types.TLSManager {
	if ptr := globalContainer.tlsManager.Load(); ptr != nil {
		return *ptr
	}
	panic("TLSManager not initialized")
}

func (fc *Container) SetConfig(config types.ConfigManager) {
	fc.config.Store(&config)
}

func (fc *Container) SetLogger(logger types.Logger) {
	fc.logger.Store(&logger)
}

func (fc *Container) SetRouter(router types.HTTPRouter) {
	fc.router.Store(&router)
}

func (fc *Container) SetCache(cache types.CacheManager) {
	fc.cache.Store(&cache)
}

func (fc *Container) SetHTTPServer(server types.HTTPServer) {
	fc.httpServer.Store(&server)
}

func (fc *Container) SetClientManager(client types.ClientManager) {
	fc.clientManager.Store(&client)
}

func (fc *Container) SetCron(cron types.CronManager) {
	fc.cron.Store(&cron)
}

func (fc *Container) SetMetrics(metrics types.MetricsManager) {
	fc.metrics.Store(&metrics)
}

func (fc *Container) SetActions(actions types.ActionBroker) {
	fc.actions.Store(&actions)
}

func (fc *Container) SetMiddlewares(middlewares types.MiddlewareManager) {
	fc.middlewares.Store(&middlewares)
}

func (fc *Container) SetHealth(health types.HealthManager) {
	fc.health.Store(&health)
}

func (fc *Container) SetDocumentation(doc types.DocumentationManager) {
	fc.documentation.Store(&doc)
}

func (fc *Container) SetTLSManager(tlsManager types.TLSManager) {
	fc.tlsManager.Store(&tlsManager)
}

func (fc *Container) IsInitialized() bool {
	return atomic.LoadInt32(&fc.initialized) == 1
}
