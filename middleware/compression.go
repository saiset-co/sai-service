package middleware

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"go.uber.org/zap"

	"github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-service/utils"
)

const (
	AlgorithmGzip       = "gzip"
	AlgorithmDeflate    = "deflate"
	AlgorithmBrotli     = "br"
	DefaultLevel        = 6
	DefaultThreshold    = 1024
	MinCompressionRatio = 0.05
	SmallBufferSize     = 4096
	MediumBufferSize    = 16384
	LargeBufferSize     = 65536
	StreamingThreshold  = 1024 * 1024
)

type CompressionMiddleware struct {
	config             types.ConfigManager
	logger             types.Logger
	metrics            types.MetricsManager
	algorithm          []byte
	compressionConfig  *CompressionConfig
	name               string
	weight             int
	gzipWriterPool     sync.Pool
	deflateWriterPool  sync.Pool
	brotliWriterPool   sync.Pool
	bufferPools        []*sync.Pool
	stringPool         sync.Pool
	varyHeaderValue    []byte
	compressFunc       func([]byte, int) ([]byte, error)
	streamCompressFunc func(io.Writer, io.Reader) error
}

type CompressionConfig struct {
	Algorithm    string   `json:"algorithm"`
	Level        int      `json:"level"`
	Threshold    int      `json:"threshold"`
	AllowedTypes []string `json:"allowed_types"`
	Timeout      int      `json:"timeout"`
}

func NewCompressionMiddleware(config types.ConfigManager, logger types.Logger, metrics types.MetricsManager) *CompressionMiddleware {
	compressionConfig := &CompressionConfig{}

	if config.GetConfig().Middlewares.Compression.Params != nil {
		if err := utils.UnmarshalConfig(config.GetConfig().Middlewares.Compression.Params, compressionConfig); err != nil {
			logger.Error("Failed to unmarshal compression middleware config", zap.Error(err))
		}
	}

	if err := validateCompressionConfig(compressionConfig); err != nil {
		logger.Warn("Invalid compression config, using defaults", zap.Error(err))
		compressionConfig = &CompressionConfig{
			Algorithm: AlgorithmBrotli,
			Level:     DefaultLevel,
			Threshold: DefaultThreshold,
			Timeout:   30,
			AllowedTypes: []string{
				"application/json",
				"application/xml",
				"application/javascript",
				"text/*",
				"application/rss+xml",
				"application/atom+xml",
			},
		}
	}

	cm := &CompressionMiddleware{
		name:              "compression",
		weight:            config.GetConfig().Middlewares.Compression.Weight,
		config:            config,
		logger:            logger,
		metrics:           metrics,
		compressionConfig: compressionConfig,
		algorithm:         []byte(compressionConfig.Algorithm),
		varyHeaderValue:   []byte("Accept-Encoding"),
	}

	cm.initializePools()

	switch compressionConfig.Algorithm {
	case AlgorithmGzip:
		cm.compressFunc = cm.compressGzipOptimized
		cm.streamCompressFunc = cm.streamCompressGzip
	case AlgorithmDeflate:
		cm.compressFunc = cm.compressDeflateOptimized
		cm.streamCompressFunc = cm.streamCompressDeflate
	case AlgorithmBrotli:
		cm.compressFunc = cm.compressBrotliOptimized
		cm.streamCompressFunc = cm.streamCompressBrotli
	}

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

	if config.Timeout <= 0 {
		return fmt.Errorf("invalid timeout: %d (must be > 0)", config.Timeout)
	}

	return nil
}

func (c *CompressionMiddleware) Name() string          { return c.name }
func (c *CompressionMiddleware) Weight() int           { return c.weight }
func (c *CompressionMiddleware) Provider() interface{} { return nil }

func (c *CompressionMiddleware) Handle(ctx *types.RequestCtx, next func(*types.RequestCtx), _ *types.RouteConfig) {
	acceptEncoding := ctx.Request.Header.Peek("Accept-Encoding")
	if !c.supportsCompression(acceptEncoding) {
		next(ctx)
		return
	}

	next(ctx)

	if len(ctx.Response.Header.Peek("Content-Encoding")) > 0 {
		return
	}

	contentType := ctx.Response.Header.Peek("Content-Type")
	if !c.shouldCompress(contentType) {
		return
	}

	c.compressResponse(ctx)
}

func (c *CompressionMiddleware) supportsCompression(acceptEncoding []byte) bool {
	if len(acceptEncoding) == 0 {
		return false
	}

	return bytes.Contains(acceptEncoding, c.algorithm)
}

func (c *CompressionMiddleware) shouldCompress(contentType []byte) bool {
	if len(contentType) == 0 {
		return false
	}

	ctStr := string(contentType)

	if semicolon := strings.Index(ctStr, ";"); semicolon != -1 {
		ctStr = ctStr[:semicolon]
	}
	ctStr = strings.TrimSpace(strings.ToLower(ctStr))

	for _, allowedType := range c.compressionConfig.AllowedTypes {
		if allowedType == ctStr {
			return true
		}
		if strings.HasSuffix(allowedType, "*") {
			prefix := strings.TrimSuffix(allowedType, "*")
			if strings.HasPrefix(ctStr, prefix) {
				return true
			}
		}
	}
	return false
}

func (c *CompressionMiddleware) compressResponse(ctx *types.RequestCtx) {
	bodyBytes := ctx.Response.Body()
	originalSize := len(bodyBytes)

	if originalSize < c.compressionConfig.Threshold {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(),
		time.Duration(c.compressionConfig.Timeout)*time.Second)
	defer cancel()

	if originalSize > StreamingThreshold {
		c.compressResponseStreaming(ctx, bodyBytes, timeoutCtx)
	} else {
		c.compressResponseInMemory(ctx, bodyBytes, timeoutCtx)
	}
}

func (c *CompressionMiddleware) compressResponseInMemory(ctx *types.RequestCtx, bodyBytes []byte, timeoutCtx context.Context) {
	originalSize := len(bodyBytes)

	done := make(chan struct {
		data []byte
		err  error
	}, 1)

	go func() {
		data, err := c.compressFunc(bodyBytes, originalSize)
		done <- struct {
			data []byte
			err  error
		}{data, err}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			return
		}

		compressedSize := len(result.data)
		ratio := float64(compressedSize) / float64(originalSize)

		if 1.0-ratio < MinCompressionRatio {
			return
		}

		c.updateResponseHeaders(ctx, compressedSize)
		ctx.Response.SetBody(result.data)

	case <-timeoutCtx.Done():
		c.logger.Warn("Compression timeout", zap.Int("size", originalSize))
		return
	}
}

func (c *CompressionMiddleware) compressResponseStreaming(ctx *types.RequestCtx, bodyBytes []byte, timeoutCtx context.Context) {
	originalSize := len(bodyBytes)

	done := make(chan struct {
		buf *bytes.Buffer
		err error
	}, 1)

	go func() {
		buf := c.getBuffer(originalSize / 2)
		defer c.putBuffer(buf, originalSize/2)

		reader := bytes.NewReader(bodyBytes)
		err := c.streamCompressFunc(buf, reader)

		result := struct {
			buf *bytes.Buffer
			err error
		}{buf, err}
		done <- result
	}()

	select {
	case result := <-done:
		if result.err != nil {
			return
		}

		compressedSize := result.buf.Len()
		ratio := float64(compressedSize) / float64(originalSize)

		if 1.0-ratio < MinCompressionRatio {
			return
		}

		c.updateResponseHeaders(ctx, compressedSize)
		ctx.Response.SetBody(result.buf.Bytes())

	case <-timeoutCtx.Done():
		c.logger.Warn("Streaming compression timeout", zap.Int("size", originalSize))
		return
	}
}

func (c *CompressionMiddleware) compressGzipOptimized(data []byte, size int) ([]byte, error) {
	estimatedSize := size / 3
	buf := c.getBufferWithCapacity(estimatedSize)
	defer c.putBuffer(buf, estimatedSize)

	writer := c.getGzipWriter(buf)
	defer c.putGzipWriter(writer)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	result := buf.Bytes()
	return append([]byte(nil), result...), nil
}

func (c *CompressionMiddleware) compressDeflateOptimized(data []byte, size int) ([]byte, error) {
	estimatedSize := size / 3
	buf := c.getBufferWithCapacity(estimatedSize)
	defer c.putBuffer(buf, estimatedSize)

	writer := c.getDeflateWriter(buf)
	defer c.putDeflateWriter(writer)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	result := buf.Bytes()
	return append([]byte(nil), result...), nil
}

func (c *CompressionMiddleware) compressBrotliOptimized(data []byte, size int) ([]byte, error) {
	estimatedSize := size / 4
	buf := c.getBufferWithCapacity(estimatedSize)
	defer c.putBuffer(buf, estimatedSize)

	writer := c.getBrotliWriter(buf)
	defer c.putBrotliWriter(writer)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	result := buf.Bytes()
	return append([]byte(nil), result...), nil
}

func (c *CompressionMiddleware) streamCompressGzip(w io.Writer, r io.Reader) error {
	writer, err := gzip.NewWriterLevel(w, c.compressionConfig.Level)
	if err != nil {
		return err
	}
	defer func(writer *gzip.Writer) {
		err := writer.Close()
		if err != nil {
			c.logger.Error("Failed to close writer", zap.Error(err))
		}
	}(writer)

	_, err = io.Copy(writer, r)
	return err
}

func (c *CompressionMiddleware) streamCompressDeflate(w io.Writer, r io.Reader) error {
	writer, err := flate.NewWriter(w, c.compressionConfig.Level)
	if err != nil {
		return err
	}

	defer func(writer *flate.Writer) {
		err := writer.Close()
		if err != nil {
			c.logger.Error("Failed to close writer", zap.Error(err))
		}
	}(writer)

	_, err = io.Copy(writer, r)
	return err
}

func (c *CompressionMiddleware) streamCompressBrotli(w io.Writer, r io.Reader) error {
	writer := brotli.NewWriterLevel(w, c.compressionConfig.Level)

	defer func(writer *brotli.Writer) {
		err := writer.Close()
		if err != nil {
			c.logger.Error("Failed to close writer", zap.Error(err))
		}
	}(writer)

	_, err := io.Copy(writer, r)
	return err
}

func (c *CompressionMiddleware) updateResponseHeaders(ctx *types.RequestCtx, compressedSize int) {
	ctx.Response.Header.SetContentEncoding(c.compressionConfig.Algorithm)
	ctx.Response.Header.SetContentLength(compressedSize)

	existingVary := ctx.Response.Header.Peek("Vary")
	if len(existingVary) > 0 {
		if !bytes.Contains(existingVary, c.varyHeaderValue) {
			buf := c.stringPool.Get().(*[]byte)
			defer func() {
				*buf = (*buf)[:0]
				c.stringPool.Put(buf)
			}()

			*buf = append(*buf, existingVary...)
			*buf = append(*buf, ", "...)
			*buf = append(*buf, c.varyHeaderValue...)
			ctx.Response.Header.SetBytesV("Vary", *buf)
		}
	} else {
		ctx.Response.Header.SetBytesV("Vary", c.varyHeaderValue)
	}
}

func (c *CompressionMiddleware) getBuffer(dataSize int) *bytes.Buffer {
	poolIndex := c.getPoolIndex(dataSize)
	buf := c.bufferPools[poolIndex].Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func (c *CompressionMiddleware) getBufferWithCapacity(estimatedSize int) *bytes.Buffer {
	poolIndex := c.getPoolIndex(estimatedSize)
	buf := c.bufferPools[poolIndex].Get().(*bytes.Buffer)
	buf.Reset()

	if buf.Cap() < estimatedSize {
		buf.Grow(estimatedSize - buf.Cap())
	}

	return buf
}

func (c *CompressionMiddleware) putBuffer(buf *bytes.Buffer, dataSize int) {
	poolIndex := c.getPoolIndex(dataSize)
	c.bufferPools[poolIndex].Put(buf)
}

func (c *CompressionMiddleware) getPoolIndex(size int) int {
	if size <= SmallBufferSize {
		return 0
	} else if size <= MediumBufferSize {
		return 1
	} else if size <= LargeBufferSize {
		return 2
	}
	return 3
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

	c.bufferPools = make([]*sync.Pool, 4)

	c.bufferPools[0] = &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, SmallBufferSize))
		},
	}

	c.bufferPools[1] = &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, MediumBufferSize))
		},
	}

	c.bufferPools[2] = &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, LargeBufferSize))
		},
	}

	c.bufferPools[3] = &sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, LargeBufferSize*4))
		},
	}

	c.stringPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 256)
		},
	}
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
