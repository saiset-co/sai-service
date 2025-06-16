package action

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
)

type EventDispatcher struct {
	ctx        context.Context
	logger     types.Logger
	metrics    types.MetricsManager
	health     types.HealthManager
	broker     types.ActionBroker
	webhookMgr *WebhookManager
	mu         sync.RWMutex
	running    int32
}

func NewEventDispatcher(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager, health types.HealthManager) (types.ActionBroker, error) {
	actionsConfig := config.GetConfig().Actions

	if !actionsConfig.Enabled {
		return nil, types.ErrActionIsDisabled
	}

	webhookMgr, err := NewWebhookManager(ctx, logger, metrics)
	if err != nil {
		return nil, types.WrapError(err, "failed to create webhook manager")
	}

	dispatcher := &EventDispatcher{
		ctx:        ctx,
		logger:     logger,
		metrics:    metrics,
		health:     health,
		webhookMgr: webhookMgr,
		running:    0,
	}

	if actionsConfig.Type != "" {
		var broker types.ActionBroker
		var err error

		switch actionsConfig.Type {
		case "websocket":
			broker, err = NewWebSocketBroker(ctx, logger, actionsConfig, health)
		default:
			if creator, exists := customActionCreators[actionsConfig.Type]; exists {
				broker, err = creator(actionsConfig.Config)
			} else {
				return nil, types.Errorf(types.ErrActionTypeUnknown, "type: %s", actionsConfig.Type)
			}
		}

		if err != nil {
			return nil, types.WrapError(err, "failed to create action broker")
		}

		dispatcher.broker = broker
	}

	return newInstrumentedEventDispatcher(logger, metrics, dispatcher), nil
}

func (ed *EventDispatcher) Publish(action string, payload interface{}) error {
	if !ed.IsRunning() {
		return types.ErrActionNotInitialized
	}

	ed.logger.Debug("Publishing event", zap.String("action", action))

	var wg sync.WaitGroup
	var errors []error
	var errorsMu sync.Mutex

	ed.mu.RLock()
	broker := ed.broker
	ed.mu.RUnlock()

	if broker != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := broker.Publish(action, payload); err != nil {
				errorsMu.Lock()
				errors = append(errors, types.WrapError(err, "broker failed"))
				errorsMu.Unlock()
				ed.logger.Error("Broker publish failed",
					zap.String("action", action),
					zap.Error(err))
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ed.webhookMgr.NotifyWebhooks(action, payload); err != nil {
			errorsMu.Lock()
			errors = append(errors, types.WrapError(err, "webhooks failed"))
			errorsMu.Unlock()
			ed.logger.Error("Webhook notification failed",
				zap.String("action", action),
				zap.Error(err))
		}
	}()

	wg.Wait()

	if len(errors) > 0 {
		ed.logger.Warn("Some publishers failed",
			zap.String("action", action),
			zap.Int("failed_count", len(errors)))
	}

	ed.logger.Debug("Event published successfully", zap.String("action", action))
	return nil
}

func (ed *EventDispatcher) Subscribe(action string, handler types.ActionHandler) error {
	if !ed.IsRunning() {
		return types.ErrActionNotInitialized
	}

	ed.mu.RLock()
	broker := ed.broker
	ed.mu.RUnlock()

	if broker == nil {
		return types.NewErrorf("no broker available for subscriptions")
	}

	return broker.Subscribe(action, handler)
}

func (ed *EventDispatcher) Unsubscribe(action string) error {
	if !ed.IsRunning() {
		return types.ErrActionNotInitialized
	}

	ed.mu.RLock()
	broker := ed.broker
	ed.mu.RUnlock()

	if broker == nil {
		return types.NewErrorf("no broker available for unsubscriptions")
	}

	return broker.Unsubscribe(action)
}

func (ed *EventDispatcher) SetBroker(broker types.ActionBroker) error {
	if ed.IsRunning() {
		return types.NewErrorf("cannot set broker while dispatcher is running")
	}

	ed.mu.Lock()
	defer ed.mu.Unlock()

	ed.broker = broker
	ed.logger.Info("Action broker set", zap.String("type", fmt.Sprintf("%T", broker)))

	return nil
}

func (ed *EventDispatcher) Start() error {
	if !atomic.CompareAndSwapInt32(&ed.running, 0, 1) {
		return types.ErrServerAlreadyRunning
	}

	if err := ed.webhookMgr.Start(); err != nil {
		atomic.StoreInt32(&ed.running, 0)
		return types.WrapError(err, "failed to start webhook manager")
	}

	ed.mu.RLock()
	broker := ed.broker
	ed.mu.RUnlock()

	if broker != nil {
		if err := broker.Start(); err != nil {
			ed.logger.Error("Failed to start broker", zap.Error(err))
		} else {
			ed.logger.Info("Action broker started")
		}
	}

	ed.logger.Info("Event dispatcher started")
	return nil
}

func (ed *EventDispatcher) Stop() error {
	if !atomic.CompareAndSwapInt32(&ed.running, 1, 0) {
		return types.ErrServerNotRunning
	}

	if err := ed.webhookMgr.Stop(); err != nil {
		ed.logger.Error("Failed to stop webhook manager", zap.Error(err))
	}

	ed.mu.RLock()
	broker := ed.broker
	ed.mu.RUnlock()

	if broker != nil {
		if err := broker.Stop(); err != nil {
			ed.logger.Error("Failed to stop broker", zap.Error(err))
		}
	}

	ed.logger.Info("Event dispatcher stopped")
	return nil
}

func (ed *EventDispatcher) IsRunning() bool {
	return atomic.LoadInt32(&ed.running) == 1
}

func (ed *EventDispatcher) RegisterRoutes(router types.HTTPRouter) {
	ed.webhookMgr.RegisterRoutes(router)
}

type instrumentedEventDispatcher struct {
	impl    *EventDispatcher
	logger  types.Logger
	metrics types.MetricsManager
}

func newInstrumentedEventDispatcher(logger types.Logger, metrics types.MetricsManager, impl *EventDispatcher) types.ActionBroker {
	return &instrumentedEventDispatcher{
		impl:    impl,
		logger:  logger,
		metrics: metrics,
	}
}

func (ied *instrumentedEventDispatcher) Publish(action string, payload interface{}) error {
	start := time.Now()
	err := ied.impl.Publish(action, payload)
	duration := time.Since(start)

	result := "success"
	if err != nil {
		result = "error"
	}

	ied.recordMetric("publish", result, action, duration)
	return err
}

func (ied *instrumentedEventDispatcher) Subscribe(action string, handler types.ActionHandler) error {
	start := time.Now()

	wrappedHandler := ied.wrapHandler(action, handler)
	err := ied.impl.Subscribe(action, wrappedHandler)
	duration := time.Since(start)

	result := "success"
	if err != nil {
		result = "error"
	}

	ied.recordMetric("subscribe", result, action, duration)
	return err
}

func (ied *instrumentedEventDispatcher) Unsubscribe(action string) error {
	start := time.Now()
	err := ied.impl.Unsubscribe(action)
	duration := time.Since(start)

	result := "success"
	if err != nil {
		result = "error"
	}

	ied.recordMetric("unsubscribe", result, action, duration)
	return err
}

func (ied *instrumentedEventDispatcher) Start() error {
	return ied.impl.Start()
}

func (ied *instrumentedEventDispatcher) Stop() error {
	return ied.impl.Stop()
}

func (ied *instrumentedEventDispatcher) IsRunning() bool {
	return ied.impl.IsRunning()
}

func (ied *instrumentedEventDispatcher) RegisterRoutes(router types.HTTPRouter) {
	ied.impl.RegisterRoutes(router)
}

func (ied *instrumentedEventDispatcher) wrapHandler(action string, handler types.ActionHandler) types.ActionHandler {
	return func(payload *types.ActionMessage) error {
		start := time.Now()
		err := handler(payload)
		duration := time.Since(start)

		result := "success"
		if err != nil {
			result = "error"
		}

		ied.recordMetric("handle", result, action, duration)
		return err
	}
}

func (ied *instrumentedEventDispatcher) recordMetric(operation, result, action string, duration time.Duration) {
	if ied.metrics == nil {
		return
	}

	counter := ied.metrics.Counter("action_operations_total", map[string]string{
		"operation": operation,
		"result":    result,
		"action":    action,
	})
	counter.Inc()

	histogram := ied.metrics.Histogram("action_operation_duration_seconds",
		[]float64{0.001, 0.01, 0.1, 1.0, 5.0},
		map[string]string{"operation": operation, "action": action},
	)
	histogram.Observe(duration.Seconds())
}
