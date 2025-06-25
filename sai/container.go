package sai

import (
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
	HTTPServer    atomic.Pointer[types.HTTPServer]
	ClientManager atomic.Pointer[types.ClientManager]
	Cron          atomic.Pointer[types.CronManager]
	Metrics       atomic.Pointer[types.MetricsManager]
	Actions       atomic.Pointer[types.ActionBroker]
	Middlewares   atomic.Pointer[types.MiddlewareManager]
	Health        atomic.Pointer[types.HealthManager]
	Documentation atomic.Pointer[types.DocumentationManager]
	TLSManager    atomic.Pointer[types.TLSManager]
}

var globalContainer *Container

func InitContainer() *Container {
	return &Container{}
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

func Actions() types.ActionBroker {
	if ptr := globalContainer.Actions.Load(); ptr != nil {
		return *ptr
	}
	panic("ActionBroker not initialized")
}

func RegisterActionBroker(actionBrokerName string, creator types.ActionBrokerCreator) {
	action.RegisterActionBroker(actionBrokerName, creator)
}

func RegisterCacheManager(cacheManagerName string, creator types.CacheManagerCreator) {
	cache.RegisterCacheManager(cacheManagerName, creator)
}

func RegisterMetricsManager(metricsManagerName string, creator types.MetricsManagerCreator) {
	metrics.RegisterMetricsManager(metricsManagerName, creator)
}

func RegisterLogger(loggerName string, creator types.LoggerCreator) {
	logger.RegisterLogger(loggerName, creator)
}

func (fc *Container) SetConfig(config types.ConfigManager) {
	fc.Config.Store(&config)
}

func (fc *Container) SetLogger(logger types.LoggerManager) {
	fc.Logger.Store(&logger)
}

func (fc *Container) SetAuthProvider(authProvider types.AuthProviderManager) {
	fc.AuthProvider.Store(&authProvider)
}

func (fc *Container) SetRouter(router types.HTTPRouter) {
	fc.Router.Store(&router)
}

func (fc *Container) SetCache(cache types.CacheManager) {
	fc.Cache.Store(&cache)
}

func (fc *Container) SetHTTPServer(server types.HTTPServer) {
	fc.HTTPServer.Store(&server)
}

func (fc *Container) SetClientManager(client types.ClientManager) {
	fc.ClientManager.Store(&client)
}

func (fc *Container) SetCron(cron types.CronManager) {
	fc.Cron.Store(&cron)
}

func (fc *Container) SetMetrics(metrics types.MetricsManager) {
	fc.Metrics.Store(&metrics)
}

func (fc *Container) SetActions(actions types.ActionBroker) {
	fc.Actions.Store(&actions)
}

func (fc *Container) SetMiddlewares(middlewares types.MiddlewareManager) {
	fc.Middlewares.Store(&middlewares)
}

func (fc *Container) SetHealth(health types.HealthManager) {
	fc.Health.Store(&health)
}

func (fc *Container) SetDocumentation(doc types.DocumentationManager) {
	fc.Documentation.Store(&doc)
}

func (fc *Container) SetTLSManager(tlsManager types.TLSManager) {
	fc.TLSManager.Store(&tlsManager)
}
