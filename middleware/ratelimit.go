package middleware

import (
	"context"
	"errors"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

const (
	shardCount = 64
)

type RateLimitMiddleware struct {
	ctx             context.Context
	config          types.ConfigManager
	logger          types.Logger
	metrics         types.MetricsManager
	shards          [shardCount]*RateLimitShard
	cleanupTicker   *time.Ticker
	stopCleanup     chan struct{}
	rateLimitConfig *RateLimitConfig
	workerGroup     sync.WaitGroup
	shutdown        int32
	metricsChan     chan MetricEvent
}

type RateLimitShard struct {
	clients map[string]*FastRateLimit
	mu      sync.RWMutex
	_       [64]byte
}

type FastRateLimit struct {
	counter      int64
	windowStart  int64
	blocked      int64
	blockedUntil int64
	lastAccess   int64
}

type MetricEvent struct {
	Type   string
	Labels map[string]string
	Value  float64
}

type RateLimitConfig struct {
	RequestsPerMinute int64           `json:"requests_per_minute"`
	BurstSize         int64           `json:"burst_size"`
	WindowSize        time.Duration   `json:"window_size"`
	TrustedServices   map[string]bool `json:"trusted_services"`
}

var (
	trustedLabels = map[string]string{"result": "trusted"}
	blockedLabels = map[string]string{"result": "blocked"}
	allowedLabels = map[string]string{"result": "allowed"}
)

func NewRateLimitMiddleware(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *RateLimitMiddleware {
	var rateLimitConfig = &RateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         20,
		WindowSize:        time.Minute,
		TrustedServices:   make(map[string]bool),
	}

	if config.GetConfig().Middlewares.RateLimit.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.RateLimit.Params, rateLimitConfig)
		if err != nil {
			logger.Error("Failed to unmarshal RateLimit middleware config", zap.Error(err))
		}
	}

	rl := &RateLimitMiddleware{
		ctx:             ctx,
		config:          config,
		logger:          logger,
		metrics:         metrics,
		metricsChan:     make(chan MetricEvent, 100),
		stopCleanup:     make(chan struct{}),
		rateLimitConfig: rateLimitConfig,
	}

	for i := range rl.shards {
		rl.shards[i] = &RateLimitShard{
			clients: make(map[string]*FastRateLimit),
		}
	}

	rl.workerGroup.Add(2)
	go rl.metricsWorker()
	go rl.cleanupWorker()

	return rl
}

func (rl *RateLimitMiddleware) Name() string { return "rate-limit" }
func (rl *RateLimitMiddleware) Weight() int  { return 25 }

func (rl *RateLimitMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), _ *types.RouteConfig) {
	start := time.Now()
	clientIP := rl.getClientIP(ctx)

	if rl.isTrustedService(ctx) {
		rl.recordMetricZeroAlloc("trusted", 1)
		next(ctx)
		return
	}

	if !rl.isAllowed(clientIP) {
		duration := time.Since(start).Seconds()
		rl.recordMetricZeroAlloc("blocked", 1)
		rl.recordDurationZeroAlloc("blocked", duration)

		rl.createRateLimitResponse(ctx)
		return
	}

	next(ctx)

	duration := time.Since(start).Seconds()
	rl.recordMetricZeroAlloc("allowed", 1)
	rl.recordDurationZeroAlloc("allowed", duration)
}

func (rl *RateLimitMiddleware) recordMetricZeroAlloc(result string, value float64) {
	if rl.metrics == nil {
		return
	}

	var labels map[string]string
	switch result {
	case "trusted":
		labels = trustedLabels
	case "blocked":
		labels = blockedLabels
	case "allowed":
		labels = allowedLabels
	default:
		return
	}

	counter := rl.metrics.Counter("rate_limit_requests_total", labels)
	counter.Add(value)
}

func (rl *RateLimitMiddleware) recordDurationZeroAlloc(result string, duration float64) {
	if rl.metrics == nil {
		return
	}

	var labels map[string]string
	switch result {
	case "blocked":
		labels = blockedLabels
	case "allowed":
		labels = allowedLabels
	default:
		return
	}

	histogram := rl.metrics.Histogram("rate_limit_duration_seconds",
		[]float64{0.001, 0.01, 0.1, 0.5}, labels)
	histogram.Observe(duration)
}

func (rl *RateLimitMiddleware) isAllowed(clientIP string) bool {
	shard := rl.getShard(clientIP)
	now := time.Now().UnixNano()

	shard.mu.RLock()
	limiter, exists := shard.clients[clientIP]
	shard.mu.RUnlock()

	if !exists {
		limiter = &FastRateLimit{
			counter:     1,
			windowStart: now,
			lastAccess:  now,
		}

		shard.mu.Lock()
		if existing, exists := shard.clients[clientIP]; exists {
			shard.mu.Unlock()
			return rl.checkLimitLockFree(existing, now)
		} else {
			shard.clients[clientIP] = limiter
			shard.mu.Unlock()
			return true
		}
	}

	return rl.checkLimitLockFree(limiter, now)
}

func (rl *RateLimitMiddleware) checkLimitLockFree(limiter *FastRateLimit, now int64) bool {
	atomic.StoreInt64(&limiter.lastAccess, now)

	if atomic.LoadInt64(&limiter.blocked) == 1 {
		blockedUntil := atomic.LoadInt64(&limiter.blockedUntil)
		if now < blockedUntil {
			return false
		}

		if atomic.CompareAndSwapInt64(&limiter.blocked, 1, 0) {
			atomic.StoreInt64(&limiter.counter, 0)
			atomic.StoreInt64(&limiter.windowStart, now)
		} else {
			return rl.checkLimitLockFree(limiter, now)
		}
	}

	for {
		windowStart := atomic.LoadInt64(&limiter.windowStart)
		windowSize := int64(rl.rateLimitConfig.WindowSize)

		if now-windowStart > windowSize {
			if atomic.CompareAndSwapInt64(&limiter.windowStart, windowStart, now) {
				atomic.StoreInt64(&limiter.counter, 1)
				return true
			}
			continue
		}

		break
	}

	counter := atomic.AddInt64(&limiter.counter, 1)

	if counter > rl.rateLimitConfig.RequestsPerMinute {
		atomic.StoreInt64(&limiter.blocked, 1)
		atomic.StoreInt64(&limiter.blockedUntil, now+int64(time.Minute))
		return false
	}

	return true
}

func (rl *RateLimitMiddleware) getShard(clientIP string) *RateLimitShard {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(clientIP))
	return rl.shards[hash.Sum32()&(shardCount-1)]
}

func (rl *RateLimitMiddleware) metricsWorker() {
	defer rl.workerGroup.Done()

	buffer := make([]MetricEvent, 0, 1000)
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		if atomic.LoadInt32(&rl.shutdown) == 1 {
			rl.logger.Debug("Metrics worker shutting down")
			return
		}

		select {
		case event := <-rl.metricsChan:
			buffer = append(buffer, event)

		case <-ticker.C:
			if len(buffer) > 0 {
				rl.flushMetrics(buffer)
				buffer = buffer[:0]
			}
		case <-rl.ctx.Done():
			rl.logger.Debug("Metrics worker stopping due to context")
			return
		}
	}
}

func (rl *RateLimitMiddleware) flushMetrics(events []MetricEvent) {
	metrics := rl.metrics

	for _, event := range events {
		switch event.Type {
		case "histogram":
			histogram := metrics.Histogram("rate_limit_duration_seconds",
				[]float64{0.001, 0.01, 0.1, 0.5}, event.Labels)
			histogram.Observe(event.Value)
		}
	}
}

func (rl *RateLimitMiddleware) cleanupWorker() {
	defer rl.workerGroup.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.ctx.Done():
		case <-rl.stopCleanup:
			return
		}
	}
}

func (rl *RateLimitMiddleware) Stop() error {
	atomic.StoreInt32(&rl.shutdown, 1)

	close(rl.stopCleanup)

	done := make(chan struct{})
	go func() {
		rl.workerGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		rl.logger.Info("Rate limit middleware stopped gracefully")
		return nil
	case <-time.After(5 * time.Second):
		rl.logger.Warn("Rate limit middleware stop timeout")
		return errors.New("timeout waiting for workers to stop")
	}
}

func (rl *RateLimitMiddleware) cleanup() {
	now := time.Now().UnixNano()
	cutoff := now - int64(time.Hour)

	var totalClients, cleanedClients int

	for _, shard := range rl.shards {
		shard.mu.Lock()
		for ip, limiter := range shard.clients {
			totalClients++
			lastAccess := atomic.LoadInt64(&limiter.lastAccess)
			blocked := atomic.LoadInt64(&limiter.blocked)

			if lastAccess < cutoff && blocked == 0 {
				delete(shard.clients, ip)
				cleanedClients++
			}
		}
		shard.mu.Unlock()
	}

	rl.logger.Debug("Rate limit cleanup completed",
		zap.Int("total_clients", totalClients),
		zap.Int("cleaned_clients", cleanedClients))
}

func (rl *RateLimitMiddleware) getClientIP(ctx *fasthttp.RequestCtx) string {
	if forwarded := string(ctx.Request.Header.Peek("X-Forwarded-For")); forwarded != "" {
		if commaIndex := strings.IndexByte(forwarded, ','); commaIndex != -1 {
			ip := forwarded[:commaIndex]
			return strings.TrimSpace(ip)
		}
		return strings.TrimSpace(forwarded)
	}

	if realIP := string(ctx.Request.Header.Peek("X-Real-IP")); realIP != "" {
		return realIP
	}

	return ctx.RemoteIP().String()
}

func (rl *RateLimitMiddleware) isTrustedService(ctx *fasthttp.RequestCtx) bool {
	userAgent := string(ctx.UserAgent())
	for serviceName := range rl.rateLimitConfig.TrustedServices {
		if strings.Contains(strings.ToLower(userAgent), strings.ToLower(serviceName)) {
			return true
		}
	}

	serviceHeader := string(ctx.Request.Header.Peek("X-Service-Name"))
	if serviceHeader != "" {
		return rl.rateLimitConfig.TrustedServices[serviceHeader]
	}

	apiKey := string(ctx.Request.Header.Peek("X-Internal-API-Key"))
	if apiKey != "" {
		return rl.validateInternalAPIKey(apiKey)
	}

	return false
}

func (rl *RateLimitMiddleware) validateInternalAPIKey(apiKey string) bool {
	internalKeys := map[string]bool{
		"internal-service-key-123": true,
		"monitoring-key-456":       true,
	}

	return internalKeys[apiKey]
}

func (rl *RateLimitMiddleware) createRateLimitResponse(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusTooManyRequests)
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Response.Header.Set("Retry-After", "60")
	ctx.Response.Header.Set("X-RateLimit-Limit", strconv.Itoa(int(rl.rateLimitConfig.RequestsPerMinute)))

	ctx.SetBodyString(`{"error":"Rate limit exceeded","message":"Too many requests","retry_after":60}`)
}
