package action

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type BrokerState int32

const (
	BrokerStateStopped BrokerState = iota
	BrokerStateStarting
	BrokerStateRunning
	BrokerStateStopping
	BrokerStateReconnecting
)

type WebSocketConfig struct {
	URL            string        `json:"url"`
	ReconnectDelay time.Duration `json:"reconnect_delay"`
	MaxRetries     int           `json:"max_retries"`
	PingInterval   time.Duration `json:"ping_interval"`
	PongWait       time.Duration `json:"pong_wait"`
	WriteWait      time.Duration `json:"write_wait"`
}

type WebSocketBroker struct {
	ctx               context.Context
	cancel            context.CancelFunc
	logger            types.Logger
	health            types.HealthManager
	config            *WebSocketConfig
	conn              *websocket.Conn
	connMu            sync.RWMutex
	subscriptions     map[string][]types.ActionHandler
	subsMu            sync.RWMutex
	send              chan *types.ActionMessage
	reconnectCh       chan struct{}
	state             atomic.Value
	shutdownTimeout   time.Duration
	messageIDGen      int64
	reconnectAttempts int32
	metrics           types.MetricsManager
}

func NewWebSocketBroker(ctx context.Context, logger types.Logger, config *types.BrokerConfig, health types.HealthManager) (types.ActionBroker, error) {
	wsConfig := &WebSocketConfig{
		URL:            "ws://localhost:8081/ws",
		ReconnectDelay: 5 * time.Second,
		MaxRetries:     10,
		PingInterval:   54 * time.Second,
		PongWait:       60 * time.Second,
		WriteWait:      10 * time.Second,
	}

	if config.Config != nil {
		err := utils.UnmarshalConfig(config.Config, wsConfig)
		if err != nil {
			return nil, types.WrapError(err, "failed to unmarshal WebSocket config")
		}
	}

	brokerCtx, cancel := context.WithCancel(ctx)

	broker := &WebSocketBroker{
		ctx:             brokerCtx,
		cancel:          cancel,
		logger:          logger,
		health:          health,
		config:          wsConfig,
		subscriptions:   make(map[string][]types.ActionHandler),
		send:            make(chan *types.ActionMessage, 256),
		reconnectCh:     make(chan struct{}, 1),
		shutdownTimeout: 10 * time.Second,
	}

	broker.state.Store(BrokerStateStopped)

	logger.Info("WebSocket broker initialized",
		zap.String("url", wsConfig.URL),
		zap.Duration("reconnect_delay", wsConfig.ReconnectDelay),
		zap.Int("max_retries", wsConfig.MaxRetries))

	return broker, nil
}

func (w *WebSocketBroker) Publish(action string, payload interface{}) error {
	if !w.IsRunning() {
		return types.ErrActionNotInitialized
	}

	start := time.Now()
	defer func() {
		w.recordMetric("publish", "attempt", action, time.Since(start))
	}()

	message := &types.ActionMessage{
		Action:    action,
		Payload:   payload,
		Timestamp: time.Now(),
		Source:    "websocket-broker",
		MessageID: w.generateMessageID(),
	}

	select {
	case w.send <- message:
		w.logger.Debug("Message queued for publishing",
			zap.String("action", action),
			zap.String("message_id", message.MessageID))
		w.recordMetric("publish", "success", action, time.Since(start))
		return nil
	case <-w.ctx.Done():
		w.recordMetric("publish", "canceled", action, time.Since(start))
		return types.ErrActionNotInitialized
	default:
		w.logger.Error("Send channel is full, dropping message",
			zap.String("action", action),
			zap.String("message_id", message.MessageID))
		w.recordMetric("publish", "dropped", action, time.Since(start))
		return types.ErrActionPublishFailed
	}
}

func (w *WebSocketBroker) Subscribe(action string, handler types.ActionHandler) error {
	if action == "" || handler == nil {
		return types.ErrActionConfigInvalid
	}

	if w.IsRunning() {
		return types.ErrActionIsRunning
	}

	start := time.Now()
	defer func() {
		w.recordMetric("subscribe", "success", action, time.Since(start))
	}()

	w.subsMu.Lock()
	defer w.subsMu.Unlock()

	if w.subscriptions[action] == nil {
		w.subscriptions[action] = make([]types.ActionHandler, 0)
	}

	wrappedHandler := w.wrapHandler(action, handler)
	w.subscriptions[action] = append(w.subscriptions[action], wrappedHandler)

	w.logger.Debug("Subscribed to action",
		zap.String("action", action),
		zap.Int("total_handlers", len(w.subscriptions[action])))

	return nil
}

func (w *WebSocketBroker) Unsubscribe(action string) error {
	if !w.IsRunning() {
		return types.ErrActionNotInitialized
	}

	start := time.Now()
	defer func() {
		w.recordMetric("unsubscribe", "success", action, time.Since(start))
	}()

	w.subsMu.Lock()
	defer w.subsMu.Unlock()

	handlersCount := len(w.subscriptions[action])
	delete(w.subscriptions, action)

	w.logger.Debug("Unsubscribed from action",
		zap.String("action", action),
		zap.Int("removed_handlers", handlersCount))

	return nil
}

func (w *WebSocketBroker) Start() error {
	if !w.transitionState(BrokerStateStopped, BrokerStateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if w.getState() == BrokerStateStarting {
			w.setState(BrokerStateRunning)
		}
	}()

	if err := w.connect(); err != nil {
		w.setState(BrokerStateStopped)
		w.logger.Error("Failed to establish initial connection", zap.Error(err))
		return types.WrapError(err, "failed to establish initial connection")
	}

	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		w.readPump()
		return nil
	})

	g.Go(func() error {
		w.writePump()
		return nil
	})

	g.Go(func() error {
		w.reconnectLoop()
		return nil
	})

	go func() {
		select {
		case <-gCtx.Done():
		case <-time.After(100 * time.Millisecond):
		}
	}()

	w.logger.Info("WebSocket broker started successfully")
	return nil
}

func (w *WebSocketBroker) Stop() error {
	if !w.transitionState(BrokerStateRunning, BrokerStateStopping) &&
		!w.transitionState(BrokerStateReconnecting, BrokerStateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		w.setState(BrokerStateStopped)
		w.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), w.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		w.connMu.Lock()
		defer w.connMu.Unlock()

		if w.conn != nil {
			if err := w.conn.Close(); err != nil {
				w.logger.Error("Failed to close connection", zap.Error(err))
				return err
			}
			w.conn = nil
		}
		return nil
	})

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			close(w.send)
			close(w.reconnectCh)
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			w.logger.Warn("WebSocket broker stop timeout, some components may not have stopped gracefully")
		default:
			w.logger.Error("Error during broker shutdown", zap.Error(err))
		}
	} else {
		w.logger.Info("WebSocket broker stopped gracefully")
	}

	return nil
}

func (w *WebSocketBroker) IsRunning() bool {
	state := w.getState()
	return state == BrokerStateRunning || state == BrokerStateReconnecting
}

func (w *WebSocketBroker) RegisterRoutes(_ types.HTTPRouter) {}

func (w *WebSocketBroker) getState() BrokerState {
	return w.state.Load().(BrokerState)
}

func (w *WebSocketBroker) setState(newState BrokerState) bool {
	currentState := w.getState()
	return w.state.CompareAndSwap(currentState, newState)
}

func (w *WebSocketBroker) transitionState(from, to BrokerState) bool {
	return w.state.CompareAndSwap(from, to)
}

func (w *WebSocketBroker) connect() error {
	w.logger.Debug("Attempting to connect to WebSocket server",
		zap.String("url", w.config.URL))

	dialCtx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(dialCtx, w.config.URL, nil)
	if err != nil {
		return types.WrapError(err, "failed to dial WebSocket server")
	}

	w.connMu.Lock()
	if w.conn != nil {
		_ = w.conn.Close()
	}
	w.conn = conn
	w.connMu.Unlock()

	_ = conn.SetReadDeadline(time.Now().Add(w.config.PongWait))
	conn.SetPongHandler(func(string) error {
		w.logger.Debug("Received pong from server")
		_ = conn.SetReadDeadline(time.Now().Add(w.config.PongWait))
		return nil
	})

	atomic.StoreInt32(&w.reconnectAttempts, 0)

	w.logger.Info("Successfully connected to WebSocket server")
	return nil
}

func (w *WebSocketBroker) reconnectLoop() {
	defer w.logger.Debug("Reconnect loop stopped")

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.reconnectCh:
			if !w.IsRunning() {
				return
			}

			if w.getState() == BrokerStateRunning {
				w.setState(BrokerStateReconnecting)
			}

			retryCount := atomic.LoadInt32(&w.reconnectAttempts)

			w.logger.Info("Starting reconnection attempt",
				zap.Int32("attempt", retryCount+1),
				zap.Int("max_retries", w.config.MaxRetries))

			if int(retryCount) >= w.config.MaxRetries {
				w.logger.Error("Max reconnection attempts reached, stopping broker")

				if w.transitionState(BrokerStateReconnecting, BrokerStateStopping) {
					w.cancel()
				}
				return
			}

			select {
			case <-time.After(w.config.ReconnectDelay):
			case <-w.ctx.Done():
				return
			}

			atomic.AddInt32(&w.reconnectAttempts, 1)

			if err := w.connect(); err != nil {
				w.logger.Error("Reconnection attempt failed",
					zap.Int32("attempt", atomic.LoadInt32(&w.reconnectAttempts)),
					zap.Error(err))

				w.safeReconnectTrigger()
				continue
			}

			w.setState(BrokerStateRunning)
			w.logger.Info("Successfully reconnected to WebSocket server")

			ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
			g, _ := errgroup.WithContext(ctx)

			g.Go(func() error {
				w.readPump()
				return nil
			})

			g.Go(func() error {
				w.writePump()
				return nil
			})

			cancel()
		}
	}
}

func (w *WebSocketBroker) safeReconnectTrigger() {
	select {
	case w.reconnectCh <- struct{}{}:
	case <-w.ctx.Done():
	default:
	}
}

func (w *WebSocketBroker) readPump() {
	defer w.logger.Debug("Read pump stopped")

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			if !w.IsRunning() {
				return
			}

			success := w.withConnection(func(conn *websocket.Conn) error {
				readCtx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
				defer cancel()

				_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))

				_, messageData, err := conn.ReadMessage()
				if err != nil {
					select {
					case <-readCtx.Done():
						return types.WrapError(readCtx.Err(), "read timeout")
					default:
						if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
							w.logger.Debug("WebSocket connection closed", zap.Error(err))
						}
						return err
					}
				}

				var message types.ActionMessage
				if err := utils.Unmarshal(messageData, &message); err != nil {
					w.logger.Error("Failed to unmarshal message", zap.Error(err))
					return nil
				}

				w.handleIncomingMessage(&message)
				return nil
			})

			if !success && w.IsRunning() {
				w.safeReconnectTrigger()
				return
			}
		}
	}
}

func (w *WebSocketBroker) writePump() {
	ticker := time.NewTicker(w.config.PingInterval)
	defer func() {
		ticker.Stop()
		w.logger.Debug("Write pump stopped")
	}()

	for {
		select {
		case <-w.ctx.Done():
			return
		case message, ok := <-w.send:
			if !ok {
				return
			}

			if !w.IsRunning() {
				w.logger.Debug("Dropping message - broker stopping",
					zap.String("action", message.Action))
				return
			}

			success := w.withConnection(func(conn *websocket.Conn) error {
				writeCtx, cancel := context.WithTimeout(w.ctx, w.config.WriteWait)
				defer cancel()

				_ = conn.SetWriteDeadline(time.Now().Add(w.config.WriteWait))

				data, err := utils.Marshal(message)
				if err != nil {
					w.logger.Error("Failed to marshal outgoing message",
						zap.Error(err),
						zap.String("action", message.Action))
					return nil
				}

				select {
				case <-writeCtx.Done():
					return types.WrapError(writeCtx.Err(), "write timeout")
				default:
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						return err
					}
				}

				w.logger.Debug("Message sent successfully",
					zap.String("action", message.Action),
					zap.String("message_id", message.MessageID))
				return nil
			})

			if !success && w.IsRunning() {
				w.safeReconnectTrigger()
				return
			}

		case <-ticker.C:
			if !w.IsRunning() {
				return
			}

			success := w.withConnection(func(conn *websocket.Conn) error {
				pingCtx, cancel := context.WithTimeout(w.ctx, w.config.WriteWait)
				defer cancel()

				_ = conn.SetWriteDeadline(time.Now().Add(w.config.WriteWait))

				select {
				case <-pingCtx.Done():
					return types.WrapError(pingCtx.Err(), "ping timeout")
				default:
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return err
					}
				}

				w.logger.Debug("Ping sent to server")
				return nil
			})

			if !success && w.IsRunning() {
				w.safeReconnectTrigger()
				return
			}
		}
	}
}

func (w *WebSocketBroker) withConnection(fn func(*websocket.Conn) error) bool {
	w.connMu.RLock()
	defer w.connMu.RUnlock()

	if w.conn == nil {
		return false
	}

	if err := fn(w.conn); err != nil {
		w.logger.Error("WebSocket operation failed", zap.Error(err))
		return false
	}

	return true
}

func (w *WebSocketBroker) handleIncomingMessage(message *types.ActionMessage) {
	start := time.Now()

	w.subsMu.RLock()
	handlers := make([]types.ActionHandler, len(w.subscriptions[message.Action]))
	copy(handlers, w.subscriptions[message.Action])
	w.subsMu.RUnlock()

	if len(handlers) == 0 {
		w.logger.Debug("No handlers found for action",
			zap.String("action", message.Action),
			zap.String("message_id", message.MessageID))
		w.recordMetric("handle", "no_handlers", message.Action, time.Since(start))
		return
	}

	w.logger.Debug("Processing message with handlers",
		zap.String("action", message.Action),
		zap.String("message_id", message.MessageID),
		zap.Int("handler_count", len(handlers)))

	ctx, cancel := context.WithTimeout(w.ctx, 30*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	for i, handler := range handlers {
		h := handler
		handlerIndex := i

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := h(message); err != nil {
					w.logger.Error("Action handler failed",
						zap.String("action", message.Action),
						zap.String("message_id", message.MessageID),
						zap.Int("handler_index", handlerIndex),
						zap.Error(err))
					return err
				}
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		w.recordMetric("handle", "error", message.Action, time.Since(start))
	} else {
		w.recordMetric("handle", "success", message.Action, time.Since(start))
	}
}

func (w *WebSocketBroker) wrapHandler(action string, handler types.ActionHandler) types.ActionHandler {
	return func(payload *types.ActionMessage) error {
		start := time.Now()

		defer func() {
			if r := recover(); r != nil {
				w.logger.Error("Handler panicked",
					zap.String("action", action),
					zap.Any("panic", r))
				w.recordMetric("handle", "panic", action, time.Since(start))
			}
		}()

		err := handler(payload)
		duration := time.Since(start)

		result := "success"
		if err != nil {
			result = "error"
		}

		w.recordMetric("handle", result, action, duration)
		return err
	}
}

func (w *WebSocketBroker) generateMessageID() string {
	id := atomic.AddInt64(&w.messageIDGen, 1)
	return fmt.Sprintf("ws-%d-%d", time.Now().Unix(), id)
}

func (w *WebSocketBroker) recordMetric(operation, result, action string, duration time.Duration) {
	if w.metrics == nil {
		return
	}

	counter := w.metrics.Counter("websocket_operations_total", map[string]string{
		"operation": operation,
		"result":    result,
		"action":    action,
	})
	counter.Inc()

	histogram := w.metrics.Histogram("websocket_operation_duration_seconds",
		[]float64{0.001, 0.01, 0.1, 1.0, 5.0, 10.0, 30.0},
		map[string]string{"operation": operation, "action": action},
	)
	histogram.Observe(duration.Seconds())
}
