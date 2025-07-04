package types

import (
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"net/http"

	"github.com/saiset-co/sai-service/utils"
)

var jsonHeaderBytes = []byte("application/json")
var textHeaderBytes = []byte("text/html; charset=UTF-8")

type RequestCtx struct {
	*fasthttp.RequestCtx
}

type ErrorResponse struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func (ctx *RequestCtx) Error(err error, statusCode int) {
	ctx.Response.Reset()

	ctx.Response.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response.Header.Set("Pragma", "no-cache")
	ctx.Response.Header.Set("Expires", "0")

	ctx.SetUserValue("error", err)
	ctx.SetStatusCode(statusCode)
	ctx.SetContentTypeBytes(jsonHeaderBytes)

	bytes, _ := utils.Marshal(ErrorResponse{Message: errors.Cause(err).Error(), Error: fasthttp.StatusMessage(statusCode)})
	ctx.Write(bytes)
}

func (ctx *RequestCtx) SuccessJSON(data any) (int, error) {
	ctx.SetContentTypeBytes(jsonHeaderBytes)
	ctx.SetStatusCode(fasthttp.StatusOK)

	bytes, err := utils.Marshal(data)
	if err != nil {
		return 0, err
	}

	return ctx.Write(bytes)
}

func (ctx *RequestCtx) Success(data []byte, header []byte) (int, error) {
	if len(header) == 0 {
		header = textHeaderBytes
	}

	ctx.SetContentTypeBytes(header)
	ctx.SetStatusCode(fasthttp.StatusOK)

	return ctx.Write(data)
}

func (ctx *RequestCtx) ReadJSON(request any) error {
	if err := utils.Unmarshal(ctx.PostBody(), &request); err != nil {
		return err
	}

	return nil
}

func (ctx *RequestCtx) Marshal(data any) ([]byte, error) {
	return utils.Marshal(data)
}

func (ctx *RequestCtx) Unmarshal(data []byte, request any) error {
	if err := utils.Unmarshal(data, &request); err != nil {
		return err
	}

	return nil
}

type FastHTTPHandler func(ctx *RequestCtx)

type FastResponseWriter struct {
	ctx        *RequestCtx
	statusCode int
}

func NewFastResponseWriter(ctx *RequestCtx) *FastResponseWriter {
	return &FastResponseWriter{
		ctx:        ctx,
		statusCode: 200,
	}
}

func (frw *FastResponseWriter) Header() http.Header {
	header := make(http.Header)
	frw.ctx.Response.Header.VisitAll(func(key, value []byte) {
		header.Set(string(key), string(value))
	})
	return header
}

func (frw *FastResponseWriter) Write(data []byte) (int, error) {
	return frw.ctx.Write(data)
}

func (frw *FastResponseWriter) WriteHeader(statusCode int) {
	frw.statusCode = statusCode
	frw.ctx.SetStatusCode(statusCode)
}
