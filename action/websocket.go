package action

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
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
	ctx           context.Context
	logger        types.Logger
	health        types.HealthManager
	config        *WebSocketConfig
	conn          *websocket.Conn
	connMu        sync.RWMutex
	subscriptions map[string][]types.ActionHandler
	subsMu        sync.RWMutex
	send          chan *types.ActionMessage
	running       int32
	reconnectCh   chan struct{}
	stopCh        chan struct{}
	messageIDGen  int64
	readDone      chan struct{}
	writeDone     chan struct{}
	reconnectDone chan struct{}
}

func NewWebSocketBroker(ctx context.Context, logger types.Logger, config *types.ActionsConfig, health types.HealthManager) (types.ActionBroker, error) {
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
			return nil, types.WrapError(err, "failed to marshal WebSocket config")
		}
	}

	broker := &WebSocketBroker{
		ctx:           ctx,
		logger:        logger,
		health:        health,
		config:        wsConfig,
		subscriptions: make(map[string][]types.ActionHandler),
		send:          make(chan *types.ActionMessage, 256),
		reconnectCh:   make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
	}

	logger.Info("WebSocket broker initialized",
		zap.String("url", wsConfig.URL),
		zap.Duration("reconnect_delay", wsConfig.ReconnectDelay),
		zap.Int("max_retries", wsConfig.MaxRetries))

	return broker, nil
}

func (w *WebSocketBroker) Publish(action string, payload interface{}) error {
	if !w.IsRunning() {
		w.logger.Warn("Attempted to publish message while broker is not running",
			zap.String("action", action))
		return types.ErrActionNotInitialized
	}

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
		return nil
	default:
		w.logger.Error("Send channel is full, dropping message",
			zap.String("action", action),
			zap.String("message_id", message.MessageID))
		return types.ErrActionPublishFailed
	}
}

func (w *WebSocketBroker) Subscribe(action string, handler types.ActionHandler) error {
	if action == "" || handler == nil {
		w.logger.Error("Invalid subscription parameters",
			zap.String("action", action),
			zap.Bool("handler_nil", handler == nil))
		return types.ErrActionConfigInvalid
	}

	w.subsMu.Lock()
	defer w.subsMu.Unlock()

	if w.subscriptions[action] == nil {
		w.subscriptions[action] = make([]types.ActionHandler, 0)
	}

	w.subscriptions[action] = append(w.subscriptions[action], handler)

	w.logger.Debug("Subscribed to action",
		zap.String("action", action),
		zap.Int("total_handlers", len(w.subscriptions[action])))

	return nil
}

func (w *WebSocketBroker) Unsubscribe(action string) error {
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
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		w.logger.Warn("WebSocket broker is already running")
		return types.ErrServerAlreadyRunning
	}

	if err := w.connect(); err != nil {
		atomic.StoreInt32(&w.running, 0)
		w.logger.Error("Failed to establish initial connection", zap.Error(err))
		return types.ErrActionConnectionFailed
	}

	go func() {
		defer close(w.readDone)
		w.readPump()
	}()

	go func() {
		defer close(w.writeDone)
		w.writePump()
	}()

	go func() {
		defer close(w.reconnectDone)
		w.reconnectLoop()
	}()

	w.logger.Info("WebSocket broker started successfully")
	return nil
}

func (w *WebSocketBroker) Stop() error {
	if !atomic.CompareAndSwapInt32(&w.running, 1, 0) {
		w.logger.Warn("WebSocket broker is not running")
		return types.ErrServerNotRunning
	}

	close(w.stopCh)

	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	select {
	case <-w.readDone:
	case <-w.writeDone:
	case <-w.reconnectDone:
	case <-timeout.C:
		w.logger.Warn("WebSocket broker stop timeout, some goroutines may not have finished")
	}

	w.connMu.Lock()
	if w.conn != nil {
		err := w.conn.Close()
		if err != nil {
			w.logger.Error("Failed to close connection", zap.Error(err))
		}
		w.conn = nil
	}
	w.connMu.Unlock()

	w.logger.Info("WebSocket broker stopped")
	return nil
}

func (w *WebSocketBroker) IsRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

func (w *WebSocketBroker) RegisterRoutes(_ types.HTTPRouter) {
}

func (w *WebSocketBroker) connect() error {
	w.logger.Debug("Attempting to connect to WebSocket server",
		zap.String("url", w.config.URL))

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(w.config.URL, nil)
	if err != nil {
		w.logger.Error("Failed to dial WebSocket server",
			zap.String("url", w.config.URL),
			zap.Error(err))
		return err
	}

	w.connMu.Lock()
	if w.conn != nil {
		err = w.conn.Close()
		if err != nil {
			w.logger.Error("Failed to close connection", zap.Error(err))
		}
	}
	w.conn = conn
	w.connMu.Unlock()

	err = conn.SetReadDeadline(time.Now().Add(w.config.PongWait))
	if err != nil {
		w.logger.Error("Failed to set read deadline", zap.Error(err))
		return err
	}

	conn.SetPongHandler(func(string) error {
		w.logger.Debug("Received pong from server")
		return conn.SetReadDeadline(time.Now().Add(w.config.PongWait))
	})

	w.logger.Info("Successfully connected to WebSocket server")
	return nil
}

func (w *WebSocketBroker) reconnectLoop() {
	retryCount := 0

	for {
		select {
		case <-w.reconnectCh:
			if !w.IsRunning() {
				return
			}

			w.logger.Info("Starting reconnection attempt",
				zap.Int("attempt", retryCount+1),
				zap.Int("max_retries", w.config.MaxRetries))

			if retryCount >= w.config.MaxRetries {
				w.logger.Error("Max reconnection attempts reached, stopping broker",
					zap.Int("max_retries", w.config.MaxRetries))
				err := w.Stop()
				if err != nil {
					w.logger.Error("Failed to stop broker", zap.Error(err))
				}
				return
			}

			time.Sleep(w.config.ReconnectDelay)

			if err := w.connect(); err != nil {
				retryCount++
				w.logger.Error("Reconnection attempt failed",
					zap.Int("attempt", retryCount),
					zap.Error(err))

				select {
				case w.reconnectCh <- struct{}{}:
				default:
				}
				continue
			}

			retryCount = 0
			w.logger.Info("Successfully reconnected to WebSocket server")

			go func() {
				defer close(w.readDone)
				w.readPump()
			}()

			go func() {
				defer close(w.writeDone)
				w.writePump()
			}()

		case <-w.stopCh:
			return
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *WebSocketBroker) triggerReconnect() {
	select {
	case w.reconnectCh <- struct{}{}:
		w.logger.Debug("Reconnection triggered")
	default:
		w.logger.Debug("Reconnection already in progress")
	}
}

func (w *WebSocketBroker) readPump() {
	defer w.logger.Debug("Read pump stopped")

	for {
		select {
		case <-w.ctx.Done():
		case <-w.stopCh:
			return
		default:
			if !w.withConnection(func(conn *websocket.Conn) error {
				err := conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				if err != nil {
					w.logger.Error("WebSocket failed to set read deadline", zap.Error(err))
				}

				_, messageData, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						w.logger.Debug("WebSocket connection closed", zap.Error(err))
						w.triggerReconnect()
						return err
					}
					return err
				}

				var message types.ActionMessage
				if err := utils.Unmarshal(messageData, &message); err != nil {
					w.logger.Error("Failed to unmarshal message", zap.Error(err))
					return nil
				}

				w.handleIncomingMessage(&message)
				return nil
			}) {
				w.triggerReconnect()
				return
			}
		}
	}
}

func (w *WebSocketBroker) withConnection(fn func(*websocket.Conn) error) bool {
	w.connMu.RLock()
	conn := w.conn
	w.connMu.RUnlock()

	if conn == nil {
		return false
	}

	if err := fn(conn); err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
			w.logger.Error("WebSocket error", zap.Error(err))
		}
		return false
	}

	return true
}

func (w *WebSocketBroker) writePump() {
	ticker := time.NewTicker(w.config.PingInterval)
	defer func() {
		ticker.Stop()
		w.logger.Debug("Write pump stopped")

		if w.IsRunning() {
			w.triggerReconnect()
		}
	}()

	for {
		select {
		case message := <-w.send:
			w.connMu.RLock()
			conn := w.conn
			w.connMu.RUnlock()

			if conn == nil {
				w.logger.Debug("Connection is nil in write pump, dropping message",
					zap.String("action", message.Action),
					zap.String("message_id", message.MessageID))
				return
			}

			err := conn.SetWriteDeadline(time.Now().Add(w.config.WriteWait))
			if err != nil {
				w.logger.Error("Failed to set write deadline", zap.Error(err))
				return
			}

			data, err := utils.Marshal(message)
			if err != nil {
				w.logger.Error("Failed to marshal outgoing message",
					zap.Error(err),
					zap.String("action", message.Action),
					zap.String("message_id", message.MessageID))
				continue
			}

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				w.logger.Error("Failed to write message to WebSocket",
					zap.Error(err),
					zap.String("action", message.Action),
					zap.String("message_id", message.MessageID))
				return
			}

			w.logger.Debug("Message sent successfully",
				zap.String("action", message.Action),
				zap.String("message_id", message.MessageID))

		case <-ticker.C:
			w.connMu.RLock()
			conn := w.conn
			w.connMu.RUnlock()

			if conn == nil {
				w.logger.Debug("Connection is nil, skipping ping")
				return
			}

			_ = conn.SetWriteDeadline(time.Now().Add(w.config.WriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				w.logger.Error("Failed to send ping", zap.Error(err))
				return
			}
			w.logger.Debug("Ping sent to server")
		case <-w.stopCh:
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *WebSocketBroker) handleIncomingMessage(message *types.ActionMessage) {
	w.subsMu.RLock()
	handlers := w.subscriptions[message.Action]
	w.subsMu.RUnlock()

	if len(handlers) == 0 {
		w.logger.Debug("No handlers found for action",
			zap.String("action", message.Action),
			zap.String("message_id", message.MessageID))
		return
	}

	w.logger.Debug("Processing message with handlers",
		zap.String("action", message.Action),
		zap.String("message_id", message.MessageID),
		zap.Int("handler_count", len(handlers)))

	for i, handler := range handlers {
		go func(h types.ActionHandler, handlerIndex int) {
			if err := h(message); err != nil {
				w.logger.Error("Action handler failed",
					zap.String("action", message.Action),
					zap.String("message_id", message.MessageID),
					zap.Int("handler_index", handlerIndex),
					zap.Error(err))
			} else {
				w.logger.Debug("Action handler completed successfully",
					zap.String("action", message.Action),
					zap.String("message_id", message.MessageID),
					zap.Int("handler_index", handlerIndex))
			}
		}(handler, i)
	}
}

func (w *WebSocketBroker) generateMessageID() string {
	id := atomic.AddInt64(&w.messageIDGen, 1)
	return fmt.Sprintf("ws-%d-%d", time.Now().Unix(), id)
}
