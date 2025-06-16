package middleware

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

const (
	AlgorithmGzip       = "gzip"
	AlgorithmDeflate    = "deflate"
	AlgorithmBrotli     = "br"
	AlgorithmNone       = "none"
	DefaultLevel        = 6
	DefaultThreshold    = 1024
	MinCompressionRatio = 0.05
)

type EncodingInfo struct {
	Name    string
	Quality float64
}

type CompressionMiddleware struct {
	config            types.ConfigManager
	logger            types.Logger
	metrics           types.MetricsManager
	gzipWriterPool    sync.Pool
	deflateWriterPool sync.Pool
	brotliWriterPool  sync.Pool
	bufferPool        sync.Pool
	compressionConfig *CompressionConfig
}

type CompressionConfig struct {
	Algorithm    string   `json:"algorithm"`
	Level        int      `json:"level"`
	Threshold    int      `json:"threshold"`
	AllowedTypes []string `json:"allowed_types"`
	Enabled      bool     `json:"enabled"`
}

func NewCompressionMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *CompressionMiddleware {
	compressionConfig := &CompressionConfig{
		Algorithm: AlgorithmGzip,
		Level:     DefaultLevel,
		Threshold: DefaultThreshold,
		Enabled:   true,
		AllowedTypes: []string{
			"application/json",
			"application/xml",
			"application/javascript",
			"text/*",
			"application/rss+xml",
			"application/atom+xml",
		},
	}

	if config.GetConfig().Middlewares.Compression.Params != nil {
		if err := utils.UnmarshalConfig(config.GetConfig().Middlewares.Compression.Params, compressionConfig); err != nil {
			logger.Error("Failed to unmarshal compression middleware config", zap.Error(err))
		}
	}

	if err := validateCompressionConfig(compressionConfig); err != nil {
		logger.Error("Invalid compression config, using defaults", zap.Error(err))
		compressionConfig = &CompressionConfig{
			Algorithm: AlgorithmGzip,
			Level:     DefaultLevel,
			Threshold: DefaultThreshold,
			Enabled:   true,
			AllowedTypes: []string{
				"application/json",
				"application/xml",
				"application/javascript",
				"text/*",
			},
		}
	}

	cm := &CompressionMiddleware{
		config:            config,
		logger:            logger,
		metrics:           metrics,
		compressionConfig: compressionConfig,
	}

	cm.initializePools()

	return cm
}

func validateCompressionConfig(config *CompressionConfig) error {
	if config.Level < -1 || config.Level > 9 {
		return fmt.Errorf("invalid compression level: %d (must be between -1 and 9)", config.Level)
	}

	if config.Threshold < 0 {
		return fmt.Errorf("invalid threshold: %d (must be >= 0)", config.Threshold)
	}

	validAlgorithms := map[string]bool{
		AlgorithmGzip:    true,
		AlgorithmDeflate: true,
		AlgorithmBrotli:  true,
	}

	if !validAlgorithms[config.Algorithm] {
		return fmt.Errorf("unsupported algorithm: %s", config.Algorithm)
	}

	return nil
}

func (c *CompressionMiddleware) Name() string { return "compression" }
func (c *CompressionMiddleware) Weight() int  { return 45 }

func (c *CompressionMiddleware) Handle(ctx *fasthttp.RequestCtx, next func(*fasthttp.RequestCtx), _ *types.RouteConfig) {
	if !c.compressionConfig.Enabled {
		next(ctx)
		return
	}

	acceptEncoding := string(ctx.Request.Header.Peek("Accept-Encoding"))
	bestEncoding := c.selectBestEncoding(acceptEncoding)
	if bestEncoding == AlgorithmNone {
		c.recordMetric(AlgorithmNone, "no_encoding_accepted")
		next(ctx)
		return
	}

	start := time.Now()
	next(ctx)

	compressionApplied, compressionStats := c.compressResponse(ctx, bestEncoding)
	duration := time.Since(start)

	if compressionApplied {
		c.recordMetric(bestEncoding, "compressed")
		c.recordCompressionRatio(bestEncoding, compressionStats.Ratio)
		c.recordDuration(bestEncoding, compressionStats.Duration)

		c.logger.Debug("Response compressed successfully",
			zap.String("algorithm", bestEncoding),
			zap.Int("original_size", compressionStats.OriginalSize),
			zap.Int("compressed_size", compressionStats.CompressedSize),
			zap.Float64("ratio", compressionStats.Ratio),
			zap.Duration("compression_time", compressionStats.Duration),
			zap.Duration("total_time", duration))
	} else {
		c.recordMetric(bestEncoding, "skipped")
	}
}

type CompressionStats struct {
	OriginalSize   int
	CompressedSize int
	Ratio          float64
	Duration       time.Duration
}

func (c *CompressionMiddleware) selectBestEncoding(acceptEncoding string) string {
	if acceptEncoding == "" {
		return AlgorithmNone
	}

	encodings := c.parseAcceptEncoding(acceptEncoding)
	if len(encodings) == 0 {
		return AlgorithmNone
	}

	for i := 0; i < len(encodings)-1; i++ {
		for j := i + 1; j < len(encodings); j++ {
			if encodings[i].Quality < encodings[j].Quality {
				encodings[i], encodings[j] = encodings[j], encodings[i]
			}
		}
	}

	for _, encoding := range encodings {
		if encoding.Name == c.compressionConfig.Algorithm && encoding.Quality > 0 {
			return encoding.Name
		}
	}

	supportedAlgorithms := map[string]bool{
		AlgorithmBrotli:  true,
		AlgorithmGzip:    true,
		AlgorithmDeflate: true,
	}

	for _, encoding := range encodings {
		if supportedAlgorithms[encoding.Name] && encoding.Quality > 0 {
			return encoding.Name
		}
	}

	return AlgorithmNone
}

func (c *CompressionMiddleware) parseAcceptEncoding(acceptEncoding string) []EncodingInfo {
	var encodings []EncodingInfo

	if acceptEncoding == "" {
		return encodings
	}

	parts := strings.Split(acceptEncoding, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		encoding := part
		quality := 1.0

		if qIndex := strings.Index(part, ";q="); qIndex != -1 {
			encoding = strings.TrimSpace(part[:qIndex])
			if q, err := strconv.ParseFloat(strings.TrimSpace(part[qIndex+3:]), 64); err == nil && q >= 0 && q <= 1 {
				quality = q
			}
		}

		switch encoding {
		case AlgorithmGzip, AlgorithmBrotli, AlgorithmDeflate:
			encodings = append(encodings, EncodingInfo{
				Name:    encoding,
				Quality: quality,
			})
		}
	}

	return encodings
}

func (c *CompressionMiddleware) compressResponse(ctx *fasthttp.RequestCtx, algorithm string) (bool, CompressionStats) {
	var stats CompressionStats

	bodyBytes := ctx.Response.Body()
	stats.OriginalSize = len(bodyBytes)

	if len(bodyBytes) < c.compressionConfig.Threshold {
		c.logger.Debug("Response too small for compression",
			zap.Int("size", len(bodyBytes)),
			zap.Int("threshold", c.compressionConfig.Threshold))
		return false, stats
	}

	if !c.shouldCompress(ctx) {
		c.logger.Debug("Content type not suitable for compression")
		return false, stats
	}

	start := time.Now()
	compressedData, err := c.compress(bodyBytes, algorithm)
	stats.Duration = time.Since(start)

	if err != nil {
		c.logger.Error("Compression failed",
			zap.String("algorithm", algorithm),
			zap.Error(err))
		return false, stats
	}

	stats.CompressedSize = len(compressedData)
	stats.Ratio = float64(stats.CompressedSize) / float64(stats.OriginalSize)

	if 1.0-stats.Ratio < MinCompressionRatio {
		c.logger.Debug("Compression not effective",
			zap.Float64("ratio", stats.Ratio),
			zap.Float64("min_compression", MinCompressionRatio))
		return false, stats
	}

	c.updateResponseHeaders(ctx, algorithm, len(compressedData))
	ctx.Response.SetBody(compressedData)

	return true, stats
}

func (c *CompressionMiddleware) shouldCompress(ctx *fasthttp.RequestCtx) bool {
	contentType := string(ctx.Response.Header.ContentType())
	if contentType == "" {
		return false
	}

	if semicolon := strings.Index(contentType, ";"); semicolon != -1 {
		contentType = contentType[:semicolon]
	}
	contentType = strings.TrimSpace(strings.ToLower(contentType))

	return c.isContentTypeAllowed(contentType)
}

func (c *CompressionMiddleware) isContentTypeAllowed(contentType string) bool {
	for _, allowedType := range c.compressionConfig.AllowedTypes {
		allowedType = strings.ToLower(allowedType)

		if contentType == allowedType {
			return true
		}

		if strings.HasSuffix(allowedType, "*") {
			prefix := strings.TrimSuffix(allowedType, "*")
			if strings.HasPrefix(contentType, prefix) {
				return true
			}
		}
	}

	return false
}

func (c *CompressionMiddleware) updateResponseHeaders(ctx *fasthttp.RequestCtx, algorithm string, compressedSize int) {
	ctx.Response.Header.Set("Content-Encoding", algorithm)
	ctx.Response.Header.Set("Content-Length", strconv.Itoa(compressedSize))

	if existingVary := string(ctx.Response.Header.Peek("Vary")); existingVary != "" {
		if !strings.Contains(existingVary, "Accept-Encoding") {
			ctx.Response.Header.Set("Vary", existingVary+", Accept-Encoding")
		}
	} else {
		ctx.Response.Header.Set("Vary", "Accept-Encoding")
	}
}

func (c *CompressionMiddleware) compress(data []byte, algorithm string) ([]byte, error) {
	switch algorithm {
	case AlgorithmGzip:
		return c.compressGzip(data)
	case AlgorithmDeflate:
		return c.compressDeflate(data)
	case AlgorithmBrotli:
		return c.compressBrotli(data)
	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %s", algorithm)
	}
}

func (c *CompressionMiddleware) compressGzip(data []byte) ([]byte, error) {
	buf := c.getBuffer()
	defer c.putBuffer(buf)

	writer := c.getGzipWriter(buf)
	defer c.putGzipWriter(writer)

	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("gzip write error: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gzip close error: %w", err)
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

func (c *CompressionMiddleware) compressDeflate(data []byte) ([]byte, error) {
	buf := c.getBuffer()
	defer c.putBuffer(buf)

	writer := c.getDeflateWriter(buf)
	defer c.putDeflateWriter(writer)

	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("deflate write error: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("deflate close error: %w", err)
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

func (c *CompressionMiddleware) compressBrotli(data []byte) ([]byte, error) {
	buf := c.getBuffer()
	defer c.putBuffer(buf)

	writer := c.getBrotliWriter(buf)
	defer c.putBrotliWriter(writer)

	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("brotli write error: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("brotli close error: %w", err)
	}

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

func (c *CompressionMiddleware) initializePools() {
	c.gzipWriterPool = sync.Pool{
		New: func() interface{} {
			writer, _ := gzip.NewWriterLevel(nil, c.compressionConfig.Level)
			return writer
		},
	}

	c.deflateWriterPool = sync.Pool{
		New: func() interface{} {
			writer, _ := flate.NewWriter(nil, c.compressionConfig.Level)
			return writer
		},
	}

	c.brotliWriterPool = sync.Pool{
		New: func() interface{} {
			return brotli.NewWriterLevel(nil, c.compressionConfig.Level)
		},
	}

	c.bufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
}

func (c *CompressionMiddleware) getBuffer() *bytes.Buffer {
	buf := c.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func (c *CompressionMiddleware) putBuffer(buf *bytes.Buffer) {
	c.bufferPool.Put(buf)
}

func (c *CompressionMiddleware) getGzipWriter(buf *bytes.Buffer) *gzip.Writer {
	writer := c.gzipWriterPool.Get().(*gzip.Writer)
	writer.Reset(buf)
	return writer
}

func (c *CompressionMiddleware) putGzipWriter(writer *gzip.Writer) {
	writer.Reset(nil)
	c.gzipWriterPool.Put(writer)
}

func (c *CompressionMiddleware) getDeflateWriter(buf *bytes.Buffer) *flate.Writer {
	writer := c.deflateWriterPool.Get().(*flate.Writer)
	writer.Reset(buf)
	return writer
}

func (c *CompressionMiddleware) putDeflateWriter(writer *flate.Writer) {
	writer.Reset(nil)
	c.deflateWriterPool.Put(writer)
}

func (c *CompressionMiddleware) getBrotliWriter(buf *bytes.Buffer) *brotli.Writer {
	writer := c.brotliWriterPool.Get().(*brotli.Writer)
	writer.Reset(buf)
	return writer
}

func (c *CompressionMiddleware) putBrotliWriter(writer *brotli.Writer) {
	writer.Reset(nil)
	c.brotliWriterPool.Put(writer)
}

func (c *CompressionMiddleware) recordMetric(algorithm, result string) {
	if c.metrics == nil {
		return
	}

	counter := c.metrics.Counter("compression_requests_total", map[string]string{
		"algorithm": algorithm,
		"result":    result,
	})
	counter.Inc()
}

func (c *CompressionMiddleware) recordCompressionRatio(algorithm string, ratio float64) {
	if c.metrics == nil {
		return
	}

	histogram := c.metrics.Histogram("compression_ratio",
		[]float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9},
		map[string]string{"algorithm": algorithm})
	histogram.Observe(ratio)
}

func (c *CompressionMiddleware) recordDuration(algorithm string, duration time.Duration) {
	if c.metrics == nil {
		return
	}

	histogram := c.metrics.Histogram("compression_duration_seconds",
		[]float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		map[string]string{"algorithm": algorithm})
	histogram.Observe(duration.Seconds())
}
