package middleware

import (
	"bytes"
	"context"
	"errors"
	"hash"
	"hash/fnv"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

const (
	shardCount       = 128
	maxRetryAttempts = 3
)

type RateLimitMiddleware struct {
	ctx             context.Context
	config          types.ConfigManager
	logger          types.Logger
	metrics         types.MetricsManager
	shards          [shardCount]*RateLimitShard
	stopCleanup     chan struct{}
	rateLimitConfig *RateLimitConfig
	workerGroup     sync.WaitGroup
	shutdown        int32
	name            string
	weight          int
	hasherPool      sync.Pool
}

type RateLimitShard struct {
	clients map[string]*FastRateLimit
	mu      sync.RWMutex
	_       [56]byte
}

type FastRateLimit struct {
	counter      int64
	windowStart  int64
	blocked      int64
	blockedUntil int64
	lastAccess   int64
	_            [24]byte
}

type RateLimitConfig struct {
	RequestsPerMinute int64         `json:"requests_per_minute"`
	BurstSize         int64         `json:"burst_size"`
	WindowSize        time.Duration `json:"window_size"`
}

var (
	realIPHeader    = []byte("X-Real-IP")
	forwardedHeader = []byte("X-Forwarded-For")
	commaBytes      = []byte(",")
)

func NewRateLimitMiddleware(ctx context.Context, config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *RateLimitMiddleware {
	var rateLimitConfig = &RateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         20,
		WindowSize:        time.Minute,
	}

	if config.GetConfig().Middlewares.RateLimit.Params != nil {
		err := utils.UnmarshalConfig(config.GetConfig().Middlewares.RateLimit.Params, rateLimitConfig)
		if err != nil {
			logger.Error("Failed to unmarshal RateLimit middleware config", zap.Error(err))
		}
	}

	rl := &RateLimitMiddleware{
		name:            "rate-limit",
		weight:          config.GetConfig().Middlewares.RateLimit.Weight,
		ctx:             ctx,
		config:          config,
		logger:          logger,
		metrics:         metrics,
		stopCleanup:     make(chan struct{}),
		rateLimitConfig: rateLimitConfig,

		hasherPool: sync.Pool{
			New: func() interface{} {
				return fnv.New32a()
			},
		},
	}

	for i := range rl.shards {
		rl.shards[i] = &RateLimitShard{
			clients: make(map[string]*FastRateLimit, 256),
		}
	}

	rl.workerGroup.Add(1)
	go rl.cleanupWorker()

	return rl
}

func (rl *RateLimitMiddleware) Name() string          { return rl.name }
func (rl *RateLimitMiddleware) Weight() int           { return rl.weight }
func (rl *RateLimitMiddleware) Provider() interface{} { return nil }

func (rl *RateLimitMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), _ *types.RouteConfig) {
	clientIP := rl.extractRealIP(ctx)

	if !rl.isAllowed(clientIP) {
		rl.createRateLimitResponse(ctx)
		return
	}

	next(ctx)
}

func (rl *RateLimitMiddleware) extractRealIP(ctx *types.RequestCtx) []byte {
	if realIP := ctx.Request.Header.PeekBytes(realIPHeader); len(realIP) > 0 {
		return realIP
	}

	if forwarded := ctx.Request.Header.PeekBytes(forwardedHeader); len(forwarded) > 0 {
		if comma := bytes.Index(forwarded, commaBytes); comma > 0 {
			return bytes.TrimSpace(forwarded[:comma])
		}
		return bytes.TrimSpace(forwarded)
	}

	return ctx.RemoteIP()
}

func (rl *RateLimitMiddleware) isAllowed(clientIP []byte) bool {
	clientIPStr := utils.BytesToString(clientIP)
	shard, err := rl.getShard(clientIP)
	if err != nil {
		return false
	}

	now := time.Now().UnixNano()

	shard.mu.RLock()
	limiter, exists := shard.clients[clientIPStr]
	shard.mu.RUnlock()

	if !exists {
		limiter = &FastRateLimit{
			counter:     1,
			windowStart: now,
			lastAccess:  now,
		}

		shard.mu.Lock()
		if existing, exists := shard.clients[clientIPStr]; exists {
			shard.mu.Unlock()
			return rl.checkLimit(existing, now)
		} else {
			shard.clients[clientIPStr] = limiter
			shard.mu.Unlock()
			return true
		}
	}

	return rl.checkLimit(limiter, now)
}

func (rl *RateLimitMiddleware) getShard(clientIP []byte) (*RateLimitShard, error) {
	hasher := rl.hasherPool.Get().(hash.Hash32)
	defer rl.hasherPool.Put(hasher)

	hasher.Reset()
	_, err := hasher.Write(clientIP)
	if err != nil {
		return nil, err
	}
	_hash := hasher.Sum32()

	return rl.shards[_hash&(shardCount-1)], nil
}

func (rl *RateLimitMiddleware) checkLimit(limiter *FastRateLimit, now int64) bool {
	atomic.StoreInt64(&limiter.lastAccess, now)

	if atomic.LoadInt64(&limiter.blocked) == 1 {
		blockedUntil := atomic.LoadInt64(&limiter.blockedUntil)
		if now < blockedUntil {
			return false
		}

		atomic.StoreInt64(&limiter.blocked, 0)
		atomic.StoreInt64(&limiter.counter, 0)
		atomic.StoreInt64(&limiter.windowStart, now)
	}

	windowStart := atomic.LoadInt64(&limiter.windowStart)
	windowSize := int64(rl.rateLimitConfig.WindowSize)

	if now-windowStart > windowSize {
		if atomic.CompareAndSwapInt64(&limiter.windowStart, windowStart, now) {
			atomic.StoreInt64(&limiter.counter, 1)
			return true
		}
		return rl.checkLimitWithRetry(limiter, now, 0)
	}

	counter := atomic.AddInt64(&limiter.counter, 1)

	if counter > rl.rateLimitConfig.RequestsPerMinute {
		atomic.StoreInt64(&limiter.blocked, 1)
		atomic.StoreInt64(&limiter.blockedUntil, now+int64(time.Minute))
		return false
	}

	return true
}

func (rl *RateLimitMiddleware) checkLimitWithRetry(limiter *FastRateLimit, now int64, attempts int) bool {
	if attempts >= maxRetryAttempts {
		return false
	}

	windowStart := atomic.LoadInt64(&limiter.windowStart)
	windowSize := int64(rl.rateLimitConfig.WindowSize)

	if now-windowStart > windowSize {
		if atomic.CompareAndSwapInt64(&limiter.windowStart, windowStart, now) {
			atomic.StoreInt64(&limiter.counter, 1)
			return true
		}
		return rl.checkLimitWithRetry(limiter, now, attempts+1)
	}

	counter := atomic.AddInt64(&limiter.counter, 1)
	return counter <= rl.rateLimitConfig.RequestsPerMinute
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
			return
		case <-rl.stopCleanup:
			return
		}
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
}

func (rl *RateLimitMiddleware) createRateLimitResponse(ctx *types.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusTooManyRequests)
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Response.Header.Set("Retry-After", "60")
	ctx.Response.Header.Set("X-RateLimit-Limit", strconv.Itoa(int(rl.rateLimitConfig.RequestsPerMinute)))

	ctx.SetBodyString(`{"error":"Rate limit exceeded","message":"Too many requests","retry_after":60}`)
}

func (rl *RateLimitMiddleware) Stop() error {
	if !atomic.CompareAndSwapInt32(&rl.shutdown, 0, 1) {
		return nil
	}

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
