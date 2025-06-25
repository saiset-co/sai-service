package action

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type WebhookState int32

const (
	WebhookStateStopped WebhookState = iota
	WebhookStateStarting
	WebhookStateRunning
	WebhookStateStopping
)

type WebhookManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	metrics         types.MetricsManager
	db              *sql.DB
	client          *http.Client
	state           atomic.Value
	shutdownTimeout time.Duration
	deliveryTimeout time.Duration
	requestTimeout  time.Duration
}

type Webhook struct {
	ID        string            `json:"id" db:"id"`
	Event     string            `json:"event" db:"event"`
	URL       string            `json:"url" db:"url"`
	Headers   map[string]string `json:"headers" db:"headers"`
	Secret    string            `json:"secret" db:"secret"`
	Enabled   bool              `json:"enabled" db:"enabled"`
	CreatedAt time.Time         `json:"created_at" db:"created_at"`
}

type WebhookCreateRequest struct {
	Event   string            `json:"event" validate:"required"`
	URL     string            `json:"url" validate:"required,url"`
	Headers map[string]string `json:"headers"`
	Enabled *bool             `json:"enabled"`
}

type WebhookUpdateRequest struct {
	Event   *string           `json:"event"`
	URL     *string           `json:"url"`
	Headers map[string]string `json:"headers"`
	Enabled *bool             `json:"enabled"`
}

type WebhookResponse struct {
	Success bool     `json:"success"`
	Data    *Webhook `json:"data,omitempty"`
	Error   string   `json:"error,omitempty"`
}

type WebhookListResponse struct {
	Success bool       `json:"success"`
	Data    []*Webhook `json:"data,omitempty"`
	Total   int        `json:"total"`
	Error   string     `json:"error,omitempty"`
}

func NewWebhookManager(ctx context.Context, logger types.Logger, metrics types.MetricsManager) (*WebhookManager, error) {
	webhookCtx, cancel := context.WithCancel(ctx)

	db, err := sql.Open("sqlite3", "./webhooks.db")
	if err != nil {
		cancel()
		return nil, types.WrapError(err, "failed to open SQLite database")
	}

	wm := &WebhookManager{
		ctx:     webhookCtx,
		cancel:  cancel,
		logger:  logger,
		metrics: metrics,
		db:      db,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		shutdownTimeout: 10 * time.Second,
		deliveryTimeout: 30 * time.Second,
		requestTimeout:  5 * time.Second,
	}

	wm.state.Store(WebhookStateStopped)

	if err := wm.initDatabase(); err != nil {
		cancel()
		if closeErr := db.Close(); closeErr != nil {
			logger.Error("Failed to close database during cleanup", zap.Error(closeErr))
		}
		return nil, types.WrapError(err, "failed to initialize database")
	}

	return wm, nil
}

func (wm *WebhookManager) Start() error {
	if !wm.transitionState(WebhookStateStopped, WebhookStateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if wm.getState() == WebhookStateStarting {
			wm.setState(WebhookStateRunning)
		}
	}()

	wm.logger.Info("Webhook manager started")
	return nil
}

func (wm *WebhookManager) Stop() error {
	if !wm.transitionState(WebhookStateRunning, WebhookStateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		wm.setState(WebhookStateStopped)
		wm.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), wm.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if wm.db != nil {
			if err := wm.db.Close(); err != nil {
				wm.logger.Error("Failed to close database", zap.Error(err))
				return err
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		select {
		case <-gCtx.Done():
			wm.logger.Warn("Webhook manager stop timeout, some components may not have stopped gracefully")
		default:
			wm.logger.Error("Error during webhook manager shutdown", zap.Error(err))
		}
	} else {
		wm.logger.Info("Webhook manager stopped gracefully")
	}

	return nil
}

func (wm *WebhookManager) IsRunning() bool {
	return wm.getState() == WebhookStateRunning
}

func (wm *WebhookManager) getState() WebhookState {
	return wm.state.Load().(WebhookState)
}

func (wm *WebhookManager) setState(newState WebhookState) bool {
	currentState := wm.getState()
	return wm.state.CompareAndSwap(currentState, newState)
}

func (wm *WebhookManager) transitionState(from, to WebhookState) bool {
	return wm.state.CompareAndSwap(from, to)
}

func (wm *WebhookManager) initDatabase() error {
	query := `
	CREATE TABLE IF NOT EXISTS webhooks (
		id TEXT PRIMARY KEY,
		event TEXT NOT NULL,
		url TEXT NOT NULL,
		headers TEXT,
		secret TEXT,
		enabled BOOLEAN DEFAULT true,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_webhooks_event ON webhooks(event);
	CREATE INDEX IF NOT EXISTS idx_webhooks_enabled ON webhooks(enabled);
	`

	_, err := wm.db.Exec(query)
	if err != nil {
		return types.WrapError(err, "failed to create webhooks table")
	}

	return nil
}

func (wm *WebhookManager) notifyWebhooks(event string, payload interface{}) error {
	if !wm.IsRunning() {
		return types.ErrActionNotInitialized
	}

	start := time.Now()
	defer func() {
		wm.recordMetric("notify", "attempt", event, time.Since(start))
	}()

	webhooks, err := wm.getWebhooksByEvent(event)
	if err != nil {
		wm.recordMetric("notify", "error", event, time.Since(start))
		return types.WrapError(err, "failed to get webhooks")
	}

	if len(webhooks) == 0 {
		wm.logger.Debug("No webhooks found for event", zap.String("event", event))
		wm.recordMetric("notify", "no_webhooks", event, time.Since(start))
		return nil
	}

	wm.logger.Debug("Notifying webhooks",
		zap.String("event", event),
		zap.Int("webhook_count", len(webhooks)))

	notifyCtx, cancel := context.WithTimeout(wm.ctx, wm.deliveryTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(notifyCtx)

	var successCount int32
	var errorCount int32

	for _, webhook := range webhooks {
		wh := webhook
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := wm.deliverWebhook(wh, event, payload); err != nil {
					atomic.AddInt32(&errorCount, 1)
					wm.logger.Error("Webhook delivery failed",
						zap.String("webhook_id", wh.ID),
						zap.String("event", event),
						zap.String("url", wh.URL),
						zap.Error(err))
					return err
				} else {
					atomic.AddInt32(&successCount, 1)
					wm.logger.Debug("Webhook delivered successfully",
						zap.String("webhook_id", wh.ID),
						zap.String("event", event))
					return nil
				}
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-notifyCtx.Done():
			wm.recordMetric("notify", "timeout", event, time.Since(start))
			return types.NewErrorf("webhook notification timeout for event: %s", event)
		default:
			if atomic.LoadInt32(&successCount) > 0 {
				wm.logger.Warn("Some webhook deliveries failed",
					zap.String("event", event),
					zap.Int32("success_count", atomic.LoadInt32(&successCount)),
					zap.Int32("error_count", atomic.LoadInt32(&errorCount)),
					zap.Error(err))
				wm.recordMetric("notify", "partial_success", event, time.Since(start))
			} else {
				wm.recordMetric("notify", "error", event, time.Since(start))
				return types.WrapError(err, "all webhook deliveries failed")
			}
		}
	}

	wm.recordMetric("notify", "success", event, time.Since(start))
	return nil
}

func (wm *WebhookManager) deliverWebhook(webhook *Webhook, event string, payload interface{}) error {
	start := time.Now()
	defer func() {
		wm.recordMetric("delivery", "attempt", event, time.Since(start))
	}()

	webhookPayload := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().Unix(),
		"data":      payload,
	}

	jsonData, err := utils.Marshal(webhookPayload)
	if err != nil {
		wm.recordMetric("delivery", "marshal_error", event, time.Since(start))
		return types.WrapError(err, "failed to marshal webhook payload")
	}

	deliveryCtx, cancel := context.WithTimeout(wm.ctx, wm.requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(deliveryCtx, "POST", webhook.URL, strings.NewReader(string(jsonData)))
	if err != nil {
		wm.recordMetric("delivery", "request_error", event, time.Since(start))
		return types.WrapError(err, "failed to create HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SAI-Service-Webhook/2.0")

	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}

	if webhook.Secret != "" {
		signature := wm.generateHMACSignature(webhook.Secret, jsonData)
		req.Header.Set("X-Signature", fmt.Sprintf("sha256=%s", signature))
	}

	resp, err := wm.client.Do(req)
	if err != nil {
		select {
		case <-deliveryCtx.Done():
			wm.recordMetric("delivery", "timeout", event, time.Since(start))
			return types.NewErrorf("webhook delivery timeout for webhook %s", webhook.ID)
		default:
			wm.recordMetric("delivery", "http_error", event, time.Since(start))
			return types.WrapError(err, "HTTP request failed")
		}
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			wm.logger.Error("Failed to close response body", zap.Error(err))
		}
	}(resp.Body)

	if resp.StatusCode >= 400 {
		wm.recordMetric("delivery", "http_error", event, time.Since(start))
		return fmt.Errorf("webhook returned error status: %d %s", resp.StatusCode, resp.Status)
	}

	wm.recordMetric("delivery", "success", event, time.Since(start))
	return nil
}

func (wm *WebhookManager) generateWebhookID() string {
	return fmt.Sprintf("wh_%d", time.Now().UnixNano())
}

func (wm *WebhookManager) getWebhookIDFromPath(ctx *types.RequestCtx) string {
	path := string(ctx.Path())
	parts := strings.Split(path, "/")
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

func (wm *WebhookManager) writeSuccessResponse(ctx *types.RequestCtx, data *Webhook) {
	response := &WebhookResponse{
		Success: true,
		Data:    data,
	}
	wm.writeJSONResponse(ctx, fasthttp.StatusOK, response)
}

func (wm *WebhookManager) writeErrorResponse(ctx *types.RequestCtx, statusCode int, message string, err error) {
	response := &WebhookResponse{
		Success: false,
		Error:   message,
	}

	if err != nil {
		wm.logger.Error("Webhook API error",
			zap.String("message", message),
			zap.Error(err))
	}

	wm.writeJSONResponse(ctx, statusCode, response)
}

func (wm *WebhookManager) writeJSONResponse(ctx *types.RequestCtx, statusCode int, data interface{}) {
	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetStatusCode(statusCode)

	if jsonData, err := utils.Marshal(data); err != nil {
		wm.logger.Error("Failed to marshal JSON response", zap.Error(err))
		ctx.Error(fasthttp.StatusMessage(statusCode), fasthttp.StatusInternalServerError)
	} else {
		if _, err := ctx.Write(jsonData); err != nil {
			wm.logger.Error("Failed to write response", zap.Error(err))
		}
	}
}

func (wm *WebhookManager) recordMetric(operation, result, event string, duration time.Duration) {
	if wm.metrics == nil {
		return
	}

	counter := wm.metrics.Counter("webhook_operations_total", map[string]string{
		"operation": operation,
		"result":    result,
		"event":     event,
	})
	counter.Inc()

	histogram := wm.metrics.Histogram("webhook_operation_duration_seconds",
		[]float64{0.001, 0.01, 0.1, 1.0, 5.0, 10.0, 30.0},
		map[string]string{"operation": operation, "event": event},
	)
	histogram.Observe(duration.Seconds())
}

func (wm *WebhookManager) generateSecret() string {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		wm.logger.Error("Failed to generate random bytes for secret", zap.Error(err))
	}
	return hex.EncodeToString(bytes)
}

func (wm *WebhookManager) getWebhooksByEvent(event string) ([]*Webhook, error) {
	start := time.Now()
	defer func() {
		wm.recordMetric("db_query", "get_by_event", event, time.Since(start))
	}()

	query := `SELECT id, event, url, headers, secret, enabled, created_at 
			  FROM webhooks WHERE event = ? AND enabled = true`

	rows, err := wm.db.Query(query, event)
	if err != nil {
		return nil, types.WrapError(err, "failed to query webhooks")
	}
	defer func(rows *sql.Rows) {
		if err := rows.Close(); err != nil {
			wm.logger.Error("Failed to close database rows", zap.Error(err))
		}
	}(rows)

	var webhooks []*Webhook
	for rows.Next() {
		webhook := &Webhook{}
		var headersJSON string

		err := rows.Scan(&webhook.ID, &webhook.Event, &webhook.URL,
			&headersJSON, &webhook.Secret, &webhook.Enabled, &webhook.CreatedAt)
		if err != nil {
			return nil, types.WrapError(err, "failed to scan webhook")
		}

		if headersJSON != "" {
			if err := utils.Unmarshal([]byte(headersJSON), &webhook.Headers); err != nil {
				wm.logger.Warn("Failed to parse webhook headers",
					zap.String("webhook_id", webhook.ID),
					zap.Error(err))
				webhook.Headers = make(map[string]string)
			}
		} else {
			webhook.Headers = make(map[string]string)
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

func (wm *WebhookManager) registerRoutes(router types.HTTPRouter) {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             wm.requestTimeout,
		DisabledMiddlewares: []string{"cache"},
	}

	router.Add("POST", "/api/webhooks", wm.handleCreateWebhook, config)
	router.Add("GET", "/api/webhooks", wm.handleListWebhooks, config)
	router.Add("GET", "/api/webhooks/get", wm.handleGetWebhook, config)
	router.Add("PUT", "/api/webhooks/update", wm.handleUpdateWebhook, config)
	router.Add("DELETE", "/api/webhooks/delete", wm.handleDeleteWebhook, config)
	router.Add("POST", "/api/webhooks/test", wm.handleTestWebhook, config)
}

func (wm *WebhookManager) handleCreateWebhook(ctx *types.RequestCtx) {
	start := time.Now()
	defer func() {
		wm.recordMetric("api", "create", "webhook", time.Since(start))
	}()

	if !wm.IsRunning() {
		wm.writeErrorResponse(ctx, fasthttp.StatusServiceUnavailable, "Webhook manager is not running", nil)
		return
	}

	var req WebhookCreateRequest
	if err := utils.Unmarshal(ctx.PostBody(), &req); err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	if req.Event == "" || req.URL == "" {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Event and URL are required", nil)
		return
	}

	if exists, err := wm.webhookExists(req.Event, req.URL); err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusInternalServerError, "Failed to check webhook existence", err)
		return
	} else if exists {
		wm.writeErrorResponse(ctx, fasthttp.StatusConflict, "Webhook with this event and URL already exists", nil)
		return
	}

	webhook := &Webhook{
		ID:        wm.generateWebhookID(),
		Event:     req.Event,
		URL:       req.URL,
		Headers:   req.Headers,
		Secret:    wm.generateSecret(),
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	if req.Enabled != nil {
		webhook.Enabled = *req.Enabled
	}

	if err := wm.createWebhook(webhook); err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusInternalServerError, "Failed to create webhook", err)
		return
	}

	wm.logger.Info("Webhook created",
		zap.String("id", webhook.ID),
		zap.String("event", webhook.Event),
		zap.String("url", webhook.URL))

	wm.writeSuccessResponse(ctx, webhook)
}

func (wm *WebhookManager) webhookExists(event, url string) (bool, error) {
	start := time.Now()
	defer func() {
		wm.recordMetric("db_query", "exists_check", event, time.Since(start))
	}()

	query := `SELECT COUNT(*) FROM webhooks WHERE event = ? AND url = ?`

	var count int
	err := wm.db.QueryRow(query, event, url).Scan(&count)
	if err != nil {
		return false, types.WrapError(err, "failed to check webhook existence")
	}

	return count > 0, nil
}

func (wm *WebhookManager) handleListWebhooks(ctx *types.RequestCtx) {
	start := time.Now()
	defer func() {
		wm.recordMetric("api", "list", "webhook", time.Since(start))
	}()

	if !wm.IsRunning() {
		wm.writeErrorResponse(ctx, fasthttp.StatusServiceUnavailable, "Webhook manager is not running", nil)
		return
	}

	webhooks, err := wm.getAllWebhooks()
	if err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusInternalServerError, "Failed to get webhooks", err)
		return
	}

	response := &WebhookListResponse{
		Success: true,
		Data:    webhooks,
		Total:   len(webhooks),
	}

	wm.writeJSONResponse(ctx, fasthttp.StatusOK, response)
}

func (wm *WebhookManager) handleGetWebhook(ctx *types.RequestCtx) {
	start := time.Now()
	defer func() {
		wm.recordMetric("api", "get", "webhook", time.Since(start))
	}()

	if !wm.IsRunning() {
		wm.writeErrorResponse(ctx, fasthttp.StatusServiceUnavailable, "Webhook manager is not running", nil)
		return
	}

	webhookID := wm.getWebhookIDFromPath(ctx)
	if webhookID == "" {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Webhook ID is required", nil)
		return
	}

	webhook, err := wm.getWebhookByID(webhookID)
	if err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusNotFound, "Webhook not found", err)
		return
	}

	wm.writeSuccessResponse(ctx, webhook)
}

func (wm *WebhookManager) handleUpdateWebhook(ctx *types.RequestCtx) {
	start := time.Now()
	defer func() {
		wm.recordMetric("api", "update", "webhook", time.Since(start))
	}()

	if !wm.IsRunning() {
		wm.writeErrorResponse(ctx, fasthttp.StatusServiceUnavailable, "Webhook manager is not running", nil)
		return
	}

	webhookID := wm.getWebhookIDFromPath(ctx)
	if webhookID == "" {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Webhook ID is required", nil)
		return
	}

	var req WebhookUpdateRequest
	if err := utils.Unmarshal(ctx.PostBody(), &req); err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	webhook, err := wm.getWebhookByID(webhookID)
	if err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusNotFound, "Webhook not found", err)
		return
	}

	if req.Event != nil {
		webhook.Event = *req.Event
	}
	if req.URL != nil {
		webhook.URL = *req.URL
	}
	if req.Headers != nil {
		webhook.Headers = req.Headers
	}
	if req.Enabled != nil {
		webhook.Enabled = *req.Enabled
	}

	if err := wm.updateWebhook(webhook); err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusInternalServerError, "Failed to update webhook", err)
		return
	}

	wm.logger.Info("Webhook updated", zap.String("id", webhookID))
	wm.writeSuccessResponse(ctx, webhook)
}

func (wm *WebhookManager) handleDeleteWebhook(ctx *types.RequestCtx) {
	start := time.Now()
	defer func() {
		wm.recordMetric("api", "delete", "webhook", time.Since(start))
	}()

	if !wm.IsRunning() {
		wm.writeErrorResponse(ctx, fasthttp.StatusServiceUnavailable, "Webhook manager is not running", nil)
		return
	}

	webhookID := wm.getWebhookIDFromPath(ctx)
	if webhookID == "" {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Webhook ID is required", nil)
		return
	}

	if err := wm.deleteWebhook(webhookID); err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusInternalServerError, "Failed to delete webhook", err)
		return
	}

	wm.logger.Info("Webhook deleted", zap.String("id", webhookID))

	response := map[string]interface{}{
		"success": true,
		"message": "Webhook deleted successfully",
	}

	wm.writeJSONResponse(ctx, fasthttp.StatusOK, response)
}

func (wm *WebhookManager) handleTestWebhook(ctx *types.RequestCtx) {
	start := time.Now()
	defer func() {
		wm.recordMetric("api", "test", "webhook", time.Since(start))
	}()

	if !wm.IsRunning() {
		wm.writeErrorResponse(ctx, fasthttp.StatusServiceUnavailable, "Webhook manager is not running", nil)
		return
	}

	webhookID := wm.getWebhookIDFromPath(ctx)
	if webhookID == "" {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Webhook ID is required", nil)
		return
	}

	webhook, err := wm.getWebhookByID(webhookID)
	if err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusNotFound, "Webhook not found", err)
		return
	}

	testPayload := map[string]interface{}{
		"test":      true,
		"timestamp": time.Now().Unix(),
		"message":   "This is a test webhook",
	}

	deliveryStart := time.Now()
	err = wm.deliverWebhook(webhook, webhook.Event+"_test", testPayload)

	response := map[string]interface{}{
		"success":      err == nil,
		"delivered_at": deliveryStart,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	wm.writeJSONResponse(ctx, fasthttp.StatusOK, response)
}

func (wm *WebhookManager) createWebhook(webhook *Webhook) error {
	start := time.Now()
	defer func() {
		wm.recordMetric("db_query", "create", webhook.Event, time.Since(start))
	}()

	headersJSON, _ := utils.Marshal(webhook.Headers)

	query := `INSERT INTO webhooks (id, event, url, headers, secret, enabled, created_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := wm.db.Exec(query, webhook.ID, webhook.Event, webhook.URL,
		string(headersJSON), webhook.Secret, webhook.Enabled, webhook.CreatedAt)

	return types.WrapError(err, "failed to insert webhook")
}

func (wm *WebhookManager) getAllWebhooks() ([]*Webhook, error) {
	start := time.Now()
	defer func() {
		wm.recordMetric("db_query", "get_all", "webhook", time.Since(start))
	}()

	query := `SELECT id, event, url, headers, secret, enabled, created_at FROM webhooks ORDER BY created_at DESC`

	rows, err := wm.db.Query(query)
	if err != nil {
		return nil, types.WrapError(err, "failed to query webhooks")
	}
	defer func(rows *sql.Rows) {
		if err := rows.Close(); err != nil {
			wm.logger.Error("Failed to close database rows", zap.Error(err))
		}
	}(rows)

	var webhooks []*Webhook
	for rows.Next() {
		webhook := &Webhook{}
		var headersJSON string

		err := rows.Scan(&webhook.ID, &webhook.Event, &webhook.URL,
			&headersJSON, &webhook.Secret, &webhook.Enabled, &webhook.CreatedAt)
		if err != nil {
			return nil, types.WrapError(err, "failed to scan webhook")
		}

		if headersJSON != "" {
			err := utils.Unmarshal([]byte(headersJSON), &webhook.Headers)
			if err != nil {
				return nil, err
			}
		} else {
			webhook.Headers = make(map[string]string)
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

func (wm *WebhookManager) getWebhookByID(id string) (*Webhook, error) {
	start := time.Now()
	defer func() {
		wm.recordMetric("db_query", "get_by_id", "webhook", time.Since(start))
	}()

	query := `SELECT id, event, url, headers, secret, enabled, created_at 
			  FROM webhooks WHERE id = ?`

	webhook := &Webhook{}
	var headersJSON string

	err := wm.db.QueryRow(query, id).Scan(&webhook.ID, &webhook.Event, &webhook.URL,
		&headersJSON, &webhook.Secret, &webhook.Enabled, &webhook.CreatedAt)

	if err != nil {
		return nil, types.WrapError(err, "failed to get webhook")
	}

	if headersJSON != "" {
		err := utils.Unmarshal([]byte(headersJSON), &webhook.Headers)
		if err != nil {
			return nil, err
		}
	} else {
		webhook.Headers = make(map[string]string)
	}

	return webhook, nil
}

func (wm *WebhookManager) updateWebhook(webhook *Webhook) error {
	start := time.Now()
	defer func() {
		wm.recordMetric("db_query", "update", webhook.Event, time.Since(start))
	}()

	headersJSON, _ := utils.Marshal(webhook.Headers)

	query := `UPDATE webhooks SET event = ?, url = ?, headers = ?, enabled = ? WHERE id = ?`

	_, err := wm.db.Exec(query, webhook.Event, webhook.URL, string(headersJSON), webhook.Enabled, webhook.ID)
	return types.WrapError(err, "failed to update webhook")
}

func (wm *WebhookManager) deleteWebhook(id string) error {
	start := time.Time{}
	defer func() {
		wm.recordMetric("db_query", "delete", "webhook", time.Since(start))
	}()

	start = time.Now()

	query := `DELETE FROM webhooks WHERE id = ?`

	result, err := wm.db.Exec(query, id)
	if err != nil {
		return types.WrapError(err, "failed to delete webhook")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return types.WrapError(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return types.ErrResourceNotFound
	}

	return nil
}

func (wm *WebhookManager) generateHMACSignature(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
