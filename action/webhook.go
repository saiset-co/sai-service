package action

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

type WebhookManager struct {
	ctx     context.Context
	logger  types.Logger
	metrics types.MetricsManager
	db      *sql.DB
	client  *http.Client
	running int32
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
	db, err := sql.Open("sqlite3", "./webhooks.db")
	if err != nil {
		return nil, types.WrapError(err, "failed to open SQLite database")
	}

	wm := &WebhookManager{
		ctx:     ctx,
		logger:  logger,
		metrics: metrics,
		db:      db,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		running: 0,
	}

	if err := wm.initDatabase(); err != nil {
		db.Close()
		return nil, types.WrapError(err, "failed to initialize database")
	}

	return wm, nil
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

func (wm *WebhookManager) NotifyWebhooks(event string, payload interface{}) error {
	webhooks, err := wm.getWebhooksByEvent(event)
	if err != nil {
		return types.WrapError(err, "failed to get webhooks")
	}

	if len(webhooks) == 0 {
		wm.logger.Debug("No webhooks found for event", zap.String("event", event))
		return nil
	}

	wm.logger.Debug("Notifying webhooks",
		zap.String("event", event),
		zap.Int("webhook_count", len(webhooks)))

	for _, webhook := range webhooks {
		go func(wh *Webhook) {
			if err := wm.deliverWebhook(wh, event, payload); err != nil {
				wm.logger.Error("Webhook delivery failed",
					zap.String("webhook_id", wh.ID),
					zap.String("event", event),
					zap.String("url", wh.URL),
					zap.Error(err))
				wm.recordMetric("delivery", "error", event)
			} else {
				wm.logger.Debug("Webhook delivered successfully",
					zap.String("webhook_id", wh.ID),
					zap.String("event", event))
				wm.recordMetric("delivery", "success", event)
			}
		}(webhook)
	}

	return nil
}

func (wm *WebhookManager) deliverWebhook(webhook *Webhook, event string, payload interface{}) error {
	webhookPayload := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().Unix(),
		"data":      payload,
	}

	jsonData, err := json.Marshal(webhookPayload)
	if err != nil {
		return types.WrapError(err, "failed to marshal webhook payload")
	}

	req, err := http.NewRequestWithContext(wm.ctx, "POST", webhook.URL, strings.NewReader(string(jsonData)))
	if err != nil {
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
		return types.WrapError(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned error status: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}

func (wm *WebhookManager) generateHMACSignature(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (wm *WebhookManager) generateSecret() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (wm *WebhookManager) getWebhooksByEvent(event string) ([]*Webhook, error) {
	query := `SELECT id, event, url, headers, secret, enabled, created_at 
			  FROM webhooks WHERE event = ? AND enabled = true`

	rows, err := wm.db.Query(query, event)
	if err != nil {
		return nil, types.WrapError(err, "failed to query webhooks")
	}
	defer rows.Close()

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
			if err := json.Unmarshal([]byte(headersJSON), &webhook.Headers); err != nil {
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

func (wm *WebhookManager) RegisterRoutes(router types.HTTPRouter) {
	config := &types.RouteConfig{
		Cache: &types.CacheHandlerConfig{
			Enabled: false,
		},
		Timeout:             time.Duration(5) * time.Second,
		DisabledMiddlewares: []string{"Auth", "BodyLimit", "Cache"},
		Doc:                 nil,
	}

	router.Add("POST", "/api/webhooks", wm.handleCreateWebhook, config)
	router.Add("GET", "/api/webhooks", wm.handleListWebhooks, config)
	router.Add("GET", "/api/webhooks/{id}", wm.handleGetWebhook, config)
	router.Add("PUT", "/api/webhooks/{id}", wm.handleUpdateWebhook, config)
	router.Add("DELETE", "/api/webhooks/{id}", wm.handleDeleteWebhook, config)
	router.Add("POST", "/api/webhooks/{id}/test", wm.handleTestWebhook, config)
}

func (wm *WebhookManager) handleCreateWebhook(ctx *fasthttp.RequestCtx) {
	var req WebhookCreateRequest
	if err := utils.Unmarshal(ctx.PostBody(), &req); err != nil {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Invalid JSON payload", err)
		return
	}

	if req.Event == "" || req.URL == "" {
		wm.writeErrorResponse(ctx, fasthttp.StatusBadRequest, "Event and URL are required", nil)
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

func (wm *WebhookManager) handleListWebhooks(ctx *fasthttp.RequestCtx) {
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

func (wm *WebhookManager) handleGetWebhook(ctx *fasthttp.RequestCtx) {
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

func (wm *WebhookManager) handleUpdateWebhook(ctx *fasthttp.RequestCtx) {
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

func (wm *WebhookManager) handleDeleteWebhook(ctx *fasthttp.RequestCtx) {
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

func (wm *WebhookManager) handleTestWebhook(ctx *fasthttp.RequestCtx) {
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

	start := time.Now()
	err = wm.deliverWebhook(webhook, webhook.Event+"_test", testPayload)

	response := map[string]interface{}{
		"success":      err == nil,
		"delivered_at": start,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	wm.writeJSONResponse(ctx, fasthttp.StatusOK, response)
}

func (wm *WebhookManager) createWebhook(webhook *Webhook) error {
	headersJSON, _ := json.Marshal(webhook.Headers)

	query := `INSERT INTO webhooks (id, event, url, headers, secret, enabled, created_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := wm.db.Exec(query, webhook.ID, webhook.Event, webhook.URL,
		string(headersJSON), webhook.Secret, webhook.Enabled, webhook.CreatedAt)

	return types.WrapError(err, "failed to insert webhook")
}

func (wm *WebhookManager) getAllWebhooks() ([]*Webhook, error) {
	query := `SELECT id, event, url, headers, secret, enabled, created_at FROM webhooks ORDER BY created_at DESC`

	rows, err := wm.db.Query(query)
	if err != nil {
		return nil, types.WrapError(err, "failed to query webhooks")
	}
	defer rows.Close()

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
			json.Unmarshal([]byte(headersJSON), &webhook.Headers)
		} else {
			webhook.Headers = make(map[string]string)
		}

		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

func (wm *WebhookManager) getWebhookByID(id string) (*Webhook, error) {
	query := `SELECT id, event, url, headers, secret, enabled, created_at 
			  FROM webhooks WHERE id = ?`

	webhook := &Webhook{}
	var headersJSON string

	err := wm.db.QueryRow(query, id).Scan(&webhook.ID, &webhook.Event, &webhook.URL,
		&headersJSON, &webhook.Secret, &webhook.Enabled, &webhook.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, types.ErrResourceNotFound
	}
	if err != nil {
		return nil, types.WrapError(err, "failed to get webhook")
	}

	if headersJSON != "" {
		json.Unmarshal([]byte(headersJSON), &webhook.Headers)
	} else {
		webhook.Headers = make(map[string]string)
	}

	return webhook, nil
}

func (wm *WebhookManager) updateWebhook(webhook *Webhook) error {
	headersJSON, _ := json.Marshal(webhook.Headers)

	query := `UPDATE webhooks SET event = ?, url = ?, headers = ?, enabled = ? WHERE id = ?`

	_, err := wm.db.Exec(query, webhook.Event, webhook.URL, string(headersJSON), webhook.Enabled, webhook.ID)
	return types.WrapError(err, "failed to update webhook")
}

func (wm *WebhookManager) deleteWebhook(id string) error {
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

func (wm *WebhookManager) generateWebhookID() string {
	return fmt.Sprintf("wh_%d", time.Now().UnixNano())
}

func (wm *WebhookManager) getWebhookIDFromPath(ctx *fasthttp.RequestCtx) string {
	path := string(ctx.Path())
	parts := strings.Split(path, "/")
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

func (wm *WebhookManager) writeSuccessResponse(ctx *fasthttp.RequestCtx, data *Webhook) {
	response := &WebhookResponse{
		Success: true,
		Data:    data,
	}
	wm.writeJSONResponse(ctx, fasthttp.StatusOK, response)
}

func (wm *WebhookManager) writeErrorResponse(ctx *fasthttp.RequestCtx, statusCode int, message string, err error) {
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

func (wm *WebhookManager) writeJSONResponse(ctx *fasthttp.RequestCtx, statusCode int, data interface{}) {
	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetStatusCode(statusCode)

	if jsonData, err := utils.Marshal(data); err != nil {
		wm.logger.Error("Failed to marshal JSON response", zap.Error(err))
		ctx.Error(fasthttp.StatusMessage(statusCode), fasthttp.StatusInternalServerError)
	} else {
		ctx.Write(jsonData)
	}
}

func (wm *WebhookManager) recordMetric(operation, result, event string) {
	if wm.metrics == nil {
		return
	}

	counter := wm.metrics.Counter("webhook_operations_total", map[string]string{
		"operation": operation,
		"result":    result,
		"event":     event,
	})
	counter.Inc()
}

func (wm *WebhookManager) Start() error {
	if !atomic.CompareAndSwapInt32(&wm.running, 0, 1) {
		return types.ErrServerAlreadyRunning
	}

	wm.logger.Info("Webhook manager started")
	return nil
}

func (wm *WebhookManager) Stop() error {
	if !atomic.CompareAndSwapInt32(&wm.running, 1, 0) {
		return types.ErrServerNotRunning
	}

	if wm.db != nil {
		wm.db.Close()
	}

	wm.logger.Info("Webhook manager stopped")
	return nil
}

func (wm *WebhookManager) IsRunning() bool {
	return atomic.LoadInt32(&wm.running) == 1
}
