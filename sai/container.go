package sai

import (
	"github.com/pkg/errors"
	"github.com/saiset-co/sai-service/database"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/saiset-co/sai-service/action"
	"github.com/saiset-co/sai-service/cache"
	"github.com/saiset-co/sai-service/logger"
	"github.com/saiset-co/sai-service/metrics"
	"github.com/saiset-co/sai-service/types"
)

type Container struct {
	Config        atomic.Pointer[types.ConfigManager]
	Logger        atomic.Pointer[types.LoggerManager]
	AuthProvider  atomic.Pointer[types.AuthProviderManager]
	Router        atomic.Pointer[types.HTTPRouter]
	Cache         atomic.Pointer[types.CacheManager]
	Database      atomic.Pointer[types.DatabaseManager]
	HTTPServer    atomic.Pointer[types.HTTPServer]
	ClientManager atomic.Pointer[types.ClientManager]
	Cron          atomic.Pointer[types.CronManager]
	Metrics       atomic.Pointer[types.MetricsManager]
	Actions       atomic.Pointer[types.ActionBroker]
	Middlewares   atomic.Pointer[types.MiddlewareManager]
	Health        atomic.Pointer[types.HealthManager]
	Documentation atomic.Pointer[types.DocumentationManager]
	TLSManager    atomic.Pointer[types.TLSManager]
	services      map[string]*atomic.Pointer[any]
	mu            sync.RWMutex
}

var globalContainer *Container

func InitContainer() *Container {
	return &Container{
		services: make(map[string]*atomic.Pointer[any]),
	}
}

func SetContainer(container *Container) {
	globalContainer = container
}

func Config() types.ConfigManager {
	if ptr := globalContainer.Config.Load(); ptr != nil {
		return *ptr
	}
	panic("ConfigManager not initialized")
}

func Logger() types.LoggerManager {
	if ptr := globalContainer.Logger.Load(); ptr != nil {
		return *ptr
	}
	panic("Logger not initialized")
}

func Router() types.HTTPRouter {
	if ptr := globalContainer.Router.Load(); ptr != nil {
		return *ptr
	}
	panic("Router not initialized")
}

func ClientManager() types.ClientManager {
	if ptr := globalContainer.ClientManager.Load(); ptr != nil {
		return *ptr
	}
	panic("ClientManager not initialized")
}

func Cron() types.CronManager {
	if ptr := globalContainer.Cron.Load(); ptr != nil {
		return *ptr
	}
	panic("CronManager not initialized")
}

func Database() types.DatabaseManager {
	if ptr := globalContainer.Database.Load(); ptr != nil {
		return *ptr
	}
	panic("DatabaseManager not initialized")
}

func Actions() types.ActionBroker {
	if ptr := globalContainer.Actions.Load(); ptr != nil {
		return *ptr
	}
	panic("ActionBroker not initialized")
}

func Set(name string, value any) {
	globalContainer.Set(name, value)
}

func Get(name string) (any, bool) {
	return globalContainer.Get(name)
}

func Load(name string, target any) bool {
	return globalContainer.Load(name, target)
}

func Has(name string) bool {
	return globalContainer.Has(name)
}

func (c *Container) Set(name string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.services[name] == nil {
		c.services[name] = &atomic.Pointer[any]{}
	}
	c.services[name].Store(&value)
}

func (c *Container) Get(name string) (any, bool) {
	c.mu.RLock()
	ptr := c.services[name]
	c.mu.RUnlock()

	if ptr == nil {
		return nil, false
	}

	if value := ptr.Load(); value != nil {
		return *value, true
	}
	return nil, false
}

func (c *Container) Load(name string, target any) bool {
	c.mu.RLock()
	ptr := c.services[name]
	c.mu.RUnlock()

	if ptr == nil {
		return false
	}

	value := ptr.Load()
	if value == nil {
		return false
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return false
	}

	targetElem := targetValue.Elem()
	if !targetElem.CanSet() {
		return false
	}

	sourceValue := reflect.ValueOf(*value)

	if sourceValue.Type() == targetElem.Type() {
		targetElem.Set(sourceValue)
		return true
	}

	if sourceValue.Type().AssignableTo(targetElem.Type()) {
		targetElem.Set(sourceValue)
		return true
	}

	if sourceValue.Type().ConvertibleTo(targetElem.Type()) {
		converted := sourceValue.Convert(targetElem.Type())
		targetElem.Set(converted)
		return true
	}

	return false
}

func (c *Container) Has(name string) bool {
	_, ok := c.Get(name)
	return ok
}

func RegisterAuthProvider(name string, provider types.AuthProvider) error {
	if ptr := globalContainer.AuthProvider.Load(); ptr != nil {
		providerManager := (*ptr).(types.AuthProviderManager)
		return providerManager.Register(name, provider)
	}

	return errors.New("AuthProvider not initialized")
}

func RegisterMiddleware(middleware types.Middleware) error {
	if ptr := globalContainer.AuthProvider.Load(); ptr != nil {
		middlewareManager := (*ptr).(types.MiddlewareManager)
		return middlewareManager.Register(middleware)
	}

	return errors.New("AuthProvider not initialized")
}

func RegisterActionBroker(actionBrokerName string, creator types.ActionBrokerCreator) {
	action.RegisterActionBroker(actionBrokerName, creator)
}

func RegisterCacheManager(cacheManagerName string, creator types.CacheManagerCreator) {
	cache.RegisterCacheManager(cacheManagerName, creator)
}

func RegisterDatabaseManager(databaseType string, creator types.DatabaseManagerCreator) {
	database.RegisterDatabaseManager(databaseType, creator)
}

func RegisterMetricsManager(metricsManagerName string, creator types.MetricsManagerCreator) {
	metrics.RegisterMetricsManager(metricsManagerName, creator)
}

func RegisterLogger(loggerName string, creator types.LoggerCreator) {
	logger.RegisterLogger(loggerName, creator)
}

func (c *Container) SetConfig(config types.ConfigManager) {
	c.Config.Store(&config)
}

func (c *Container) SetLogger(logger types.LoggerManager) {
	c.Logger.Store(&logger)
}

func (c *Container) SetAuthProvider(authProvider types.AuthProviderManager) {
	c.AuthProvider.Store(&authProvider)
}

func (c *Container) SetRouter(router types.HTTPRouter) {
	c.Router.Store(&router)
}

func (c *Container) SetCache(cache types.CacheManager) {
	c.Cache.Store(&cache)
}

func (c *Container) SetDatabase(database types.DatabaseManager) {
	c.Database.Store(&database)
}

func (c *Container) SetHTTPServer(server types.HTTPServer) {
	c.HTTPServer.Store(&server)
}

func (c *Container) SetClientManager(client types.ClientManager) {
	c.ClientManager.Store(&client)
}

func (c *Container) SetCron(cron types.CronManager) {
	c.Cron.Store(&cron)
}

func (c *Container) SetMetrics(metrics types.MetricsManager) {
	c.Metrics.Store(&metrics)
}

func (c *Container) SetActions(actions types.ActionBroker) {
	c.Actions.Store(&actions)
}

func (c *Container) SetMiddlewares(middlewares types.MiddlewareManager) {
	c.Middlewares.Store(&middlewares)
}

func (c *Container) SetHealth(health types.HealthManager) {
	c.Health.Store(&health)
}

func (c *Container) SetDocumentation(doc types.DocumentationManager) {
	c.Documentation.Store(&doc)
}

func (c *Container) SetTLSManager(tlsManager types.TLSManager) {
	c.TLSManager.Store(&tlsManager)
}
