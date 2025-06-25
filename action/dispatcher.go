package action

import (
	"context"
	"github.com/saiset-co/sai-service/client"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

type Dispatcher struct {
	ctx             context.Context
	cancel          context.CancelFunc
	config          types.ConfigManager
	actionsConfig   *types.ActionsConfig
	logger          types.Logger
	router          types.HTTPRouter
	metrics         types.MetricsManager
	health          types.HealthManager
	clientManager   types.ClientManager
	broker          atomic.Pointer[types.ActionBroker]
	webhookMgr      *WebhookManager
	state           atomic.Value
	webhookHandlers sync.Map
	shutdownTimeout time.Duration
	publishTimeout  time.Duration
	handlerTimeout  time.Duration
}

var customActionCreators = sync.Map{}

func RegisterActionBroker(actionBrokerName string, creator types.ActionBrokerCreator) {
	customActionCreators.Store(actionBrokerName, creator)
}

func NewDispatcher(
	ctx context.Context,
	config types.ConfigManager,
	logger types.Logger,
	router types.HTTPRouter,
	metrics types.MetricsManager,
	health types.HealthManager,
	clientManager types.ClientManager,
) (types.Dispatcher, error) {
	actionsConfig := config.GetConfig().Actions
	if actionsConfig == nil {
		return nil, types.ErrActionConfigInvalid
	}

	dispatcherCtx, cancel := context.WithCancel(ctx)

	webhookMgr, err := NewWebhookManager(dispatcherCtx, logger, metrics)
	if err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to create webhook manager")
	}

	dispatcher := &Dispatcher{
		ctx:             dispatcherCtx,
		cancel:          cancel,
		config:          config,
		logger:          logger,
		router:          router,
		metrics:         metrics,
		health:          health,
		clientManager:   clientManager,
		webhookMgr:      webhookMgr,
		actionsConfig:   actionsConfig,
		shutdownTimeout: 10 * time.Second,
		publishTimeout:  30 * time.Second,
		handlerTimeout:  30 * time.Second,
	}

	dispatcher.state.Store(StateStopped)

	if actionsConfig.Broker != nil && actionsConfig.Broker.Enabled {
		if err := dispatcher.initializeBroker(); err != nil {
			cancel()
			return nil, types.WrapError(err, "failed to initialize broker")
		}
	}

	return dispatcher, nil
}

func (d *Dispatcher) Start() error {
	if !d.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if d.getState() == StateStarting {
			d.setState(StateRunning)
		}
	}()

	if d.config.GetConfig().Actions.Webhooks.Enabled {
		if err := d.webhookMgr.Start(); err != nil {
			d.setState(StateStopped)
			return types.WrapError(err, "failed to start webhook manager")
		}
	}

	if broker := d.broker.Load(); broker != nil {
		if err := (*broker).(types.LifecycleManager).Start(); err != nil {
			d.logger.Error("Failed to start broker", zap.Error(err))
		} else {
			d.logger.Info("Action broker started")
		}
	}

	d.registerRoutes()

	if err := d.initializeWebhooks(); err != nil {
		d.logger.Error("Failed to initialize webhooks", zap.Error(err))
	}

	d.logger.Info("Event dispatcher started")
	return nil
}

func (d *Dispatcher) Stop() error {
	if !d.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		d.setState(StateStopped)
		d.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), d.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := d.webhookMgr.Stop(); err != nil {
			d.logger.Error("Failed to stop webhook manager", zap.Error(err))
			return err
		}
		return nil
	})

	if broker := d.broker.Load(); broker != nil {
		g.Go(func() error {
			if err := (*broker).(types.LifecycleManager).Stop(); err != nil {
				d.logger.Error("Failed to stop broker", zap.Error(err))
				return err
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			d.logger.Warn("Dispatcher stop timeout, some components may not have stopped gracefully")
		default:
			d.logger.Error("Error during dispatcher shutdown", zap.Error(err))
		}
	} else {
		d.logger.Info("Event dispatcher stopped gracefully")
	}

	d.webhookHandlers = sync.Map{}
	d.broker.Store(nil)

	return nil
}

func (d *Dispatcher) IsRunning() bool {
	return d.getState() == StateRunning
}

func (d *Dispatcher) Publish(action string, payload interface{}) error {
	if !d.IsRunning() {
		return types.ErrActionNotInitialized
	}

	start := time.Now()
	defer func() {
		d.recordMetric("publish", "attempt", action, time.Since(start))
	}()

	d.logger.Debug("Publishing event", zap.String("action", action))

	publishCtx, cancel := context.WithTimeout(d.ctx, d.publishTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(publishCtx)

	var publishedChannels int32

	if broker := d.broker.Load(); broker != nil {
		atomic.AddInt32(&publishedChannels, 1)
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := (*broker).Publish(action, payload); err != nil {
					d.logger.Error("Broker publish failed", zap.String("action", action), zap.Error(err))
					return types.WrapError(err, "broker publish failed")
				}
				return nil
			}
		})
	}

	if d.actionsConfig.Webhooks != nil && d.actionsConfig.Webhooks.Enabled {
		atomic.AddInt32(&publishedChannels, 1)
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := d.webhookMgr.notifyWebhooks(action, payload); err != nil {
					d.logger.Error("Webhook notification failed", zap.String("action", action), zap.Error(err))
					return types.WrapError(err, "webhook notification failed")
				}
				return nil
			}
		})
	}

	if atomic.LoadInt32(&publishedChannels) == 0 {
		d.recordMetric("publish", "no_channels", action, time.Since(start))
		return types.NewErrorf("no publication channels available for action: %s", action)
	}

	if err := g.Wait(); err != nil {
		select {
		case <-publishCtx.Done():
			d.recordMetric("publish", "timeout", action, time.Since(start))
			return types.NewErrorf("publish timeout for action: %s", action)
		default:
			d.logger.Warn("Some publishers failed",
				zap.String("action", action),
				zap.Error(err))
			d.recordMetric("publish", "partial_success", action, time.Since(start))
		}
	}

	d.recordMetric("publish", "success", action, time.Since(start))
	d.logger.Debug("Event published successfully", zap.String("action", action))
	return nil
}

func (d *Dispatcher) Subscribe(action string, handler types.ActionHandler) error {
	if d.IsRunning() {
		return types.ErrActionIsRunning
	}

	start := time.Now()
	wrappedHandler := d.wrapHandler(action, handler)

	var err error
	if broker := d.broker.Load(); broker != nil {
		err = (*broker).Subscribe(action, wrappedHandler)
	} else if d.actionsConfig.Webhooks != nil && d.actionsConfig.Webhooks.Enabled {
		d.webhookHandlers.Store(action, wrappedHandler)
		d.logger.Debug("Webhook handler registered for action", zap.String("action", action))
	} else {
		err = types.NewErrorf("no subscription mechanism available for action: %s", action)
	}

	result := "success"
	if err != nil {
		result = "error"
	}
	d.recordMetric("subscribe", result, action, time.Since(start))

	return err
}

func (d *Dispatcher) Unsubscribe(action string) error {
	if !d.IsRunning() {
		return types.ErrActionNotInitialized
	}

	start := time.Now()
	var err error

	if broker := d.broker.Load(); broker != nil {
		err = (*broker).Unsubscribe(action)
	} else if d.actionsConfig.Webhooks != nil && d.actionsConfig.Webhooks.Enabled {
		d.webhookHandlers.Delete(action)
		d.logger.Debug("Webhook handler unregistered for action", zap.String("action", action))
	} else {
		err = types.NewErrorf("no broker available for unsubscriptions")
	}

	result := "success"
	if err != nil {
		result = "error"
	}
	d.recordMetric("unsubscribe", result, action, time.Since(start))

	return err
}

func (d *Dispatcher) getState() State {
	return d.state.Load().(State)
}

func (d *Dispatcher) setState(newState State) bool {
	currentState := d.getState()
	return d.state.CompareAndSwap(currentState, newState)
}

func (d *Dispatcher) transitionState(from, to State) bool {
	return d.state.CompareAndSwap(from, to)
}

func (d *Dispatcher) initializeBroker() error {
	brokerConfig := d.actionsConfig.Broker
	if brokerConfig == nil {
		return nil
	}

	var broker types.ActionBroker
	var err error

	switch brokerConfig.Type {
	case "websocket":
		broker, err = NewWebSocketBroker(d.ctx, d.logger, brokerConfig, d.health)
	default:
		if creator, exists := customActionCreators.Load(brokerConfig.Type); exists {
			broker, err = creator.(types.ActionBrokerCreator)(brokerConfig.Config)
		} else {
			return types.Errorf(types.ErrActionTypeUnknown, "type: %s", brokerConfig.Type)
		}
	}

	if err != nil {
		return types.WrapError(err, "failed to create action broker")
	}

	d.broker.Store(&broker)
	d.logger.Info("Action broker initialized", zap.String("type", brokerConfig.Type))
	return nil
}

func (d *Dispatcher) initializeWebhooks() error {
	if d.clientManager == nil {
		return nil
	}

	clientConfig := d.config.GetConfig().Client
	if clientConfig == nil || !clientConfig.Enabled || clientConfig.Services == nil {
		return nil
	}

	if d.actionsConfig.Webhooks != nil && d.actionsConfig.Webhooks.Enabled {
		config := &types.RouteConfig{
			Cache: &types.CacheHandlerConfig{
				Enabled: false,
			},
			Timeout:             5 * time.Second,
			DisabledMiddlewares: []string{"cache"},
		}

		d.router.Add("POST", "/webhook/create", d.handleWebhook, config)

		for serviceName, serviceConfig := range clientConfig.Services {
			for _, event := range serviceConfig.Events {
				webhookURL := d.buildWebhookURL(event)

				if _, _, err := d.clientManager.(*client.Manager).RegisterWebhook(serviceName, event, webhookURL); err != nil {
					d.logger.Error("Failed to register webhook for service",
						zap.String("service", serviceName),
						zap.String("event", event),
						zap.String("webhook_url", webhookURL),
						zap.Error(err))
				} else {
					d.logger.Info("Webhook registered for external service",
						zap.String("service", serviceName),
						zap.String("event", event),
						zap.String("webhook_url", webhookURL))
				}
			}
		}
	}

	return nil
}

func (d *Dispatcher) buildWebhookURL(event string) string {
	serverConfig := d.config.GetConfig().Server
	scheme := "http"
	if serverConfig.TLS.Enabled {
		scheme = "https"
	}

	return scheme + "://" + serverConfig.HTTP.Host + ":" +
		strconv.Itoa(serverConfig.HTTP.Port) + "/webhook/" + event
}

func (d *Dispatcher) handleWebhook(ctx *types.RequestCtx) {
	actionID := d.getActionFromPath(ctx)
	if actionID == "" {
		d.logger.Warn("Webhook received without action ID")
		d.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "action ID is required")
		return
	}

	handler, exists := d.webhookHandlers.Load(actionID)
	if !exists {
		d.logger.Debug("No handler found for webhook action", zap.String("action", actionID))
		d.writeErrorResponse(ctx, fasthttp.StatusNotFound, "no handler for this action")
		return
	}

	var webhookPayload map[string]interface{}
	if err := utils.Unmarshal(ctx.PostBody(), &webhookPayload); err != nil {
		d.logger.Error("Failed to parse webhook payload", zap.String("action", actionID), zap.Error(err))
		d.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "invalid JSON payload")
		return
	}

	actionMessage := &types.ActionMessage{
		Action:    actionID,
		Payload:   webhookPayload,
		Timestamp: time.Now(),
		Source:    "webhook",
	}

	go d.processWebhookAsync(actionID, actionMessage, handler.(types.ActionHandler))

	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetContentType("application/json")
	if _, err := ctx.WriteString(`{"status": "received"}`); err != nil {
		d.logger.Error("Failed to write response", zap.Error(err))
	}

	d.logger.Debug("Webhook received and handler triggered", zap.String("action", actionID))
}

func (d *Dispatcher) processWebhookAsync(actionID string, message *types.ActionMessage, handler types.ActionHandler) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(d.ctx, d.handlerTimeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- types.NewErrorf("webhook handler panicked: %v", r)
			}
		}()

		done <- handler(message)
	}()

	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		err = types.NewErrorf("webhook handler timeout for action: %s", actionID)
		d.logger.Error("Webhook handler timeout",
			zap.String("action", actionID),
			zap.Duration("timeout", d.handlerTimeout))
	case <-d.ctx.Done():
		err = types.NewErrorf("dispatcher shutting down, aborting webhook handler for action: %s", actionID)
	}

	result := "success"
	if err != nil {
		result = "error"
		d.logger.Error("Webhook handler failed", zap.String("action", actionID), zap.Error(err))
	}

	d.recordMetric("webhook_handle", result, actionID, time.Since(start))
}

func (d *Dispatcher) writeErrorResponse(ctx *types.RequestCtx, statusCode int, message string) {
	ctx.SetStatusCode(statusCode)
	ctx.SetContentType("application/json")
	response := `{"error": "` + message + `"}`
	if _, err := ctx.WriteString(response); err != nil {
		d.logger.Error("Failed to write error response", zap.Error(err))
	}
}

func (d *Dispatcher) getActionFromPath(ctx *types.RequestCtx) string {
	if id := ctx.UserValue("id"); id != nil {
		return id.(string)
	}

	path := string(ctx.Path())
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[1] == "webhook" {
		return parts[2]
	}

	return ""
}

func (d *Dispatcher) registerRoutes() {
	d.webhookMgr.registerRoutes(d.router)
}

func (d *Dispatcher) wrapHandler(action string, handler types.ActionHandler) types.ActionHandler {
	return func(payload *types.ActionMessage) error {
		start := time.Now()
		err := handler(payload)
		duration := time.Since(start)

		result := "success"
		if err != nil {
			result = "error"
		}

		d.recordMetric("handle", result, action, duration)
		return err
	}
}

func (d *Dispatcher) recordMetric(operation, result, action string, duration time.Duration) {
	if d.metrics == nil {
		return
	}

	counter := d.metrics.Counter("action_operations_total", map[string]string{
		"operation": operation,
		"result":    result,
		"action":    action,
	})
	counter.Inc()

	histogram := d.metrics.Histogram("action_operation_duration_seconds",
		[]float64{0.001, 0.01, 0.1, 1.0, 5.0, 10.0, 30.0},
		map[string]string{"operation": operation, "action": action},
	)
	histogram.Observe(duration.Seconds())
}
