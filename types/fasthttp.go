package types

import (
	"net/http"
	"net/url"

	"github.com/valyala/fasthttp"

	"github.com/saiset-co/sai-service/utils"
)

var headerBytes = []byte("application/json")

type RequestCtx struct {
	*fasthttp.RequestCtx
}

type Response struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

func (ctx *RequestCtx) WriteJSON(data any) (int, error) {
	ctx.SetContentTypeBytes(headerBytes)
	ctx.SetStatusCode(fasthttp.StatusOK)

	bytes, err := utils.Marshal(data)
	if err != nil {
		return 0, err
	}

	return ctx.Write(bytes)
}

func (ctx *RequestCtx) ReadJSON(request any) error {
	if err := utils.Unmarshal(ctx.PostBody(), &request); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.WriteJSON(Response{
			Message: "Invalid JSON",
			Status:  "error",
		})

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

type FastRequestContext struct {
	ctx    *RequestCtx
	header http.Header
	url    *url.URL
}

func (frc *FastRequestContext) Method() string {
	return string(frc.ctx.Method())
}

func (frc *FastRequestContext) URL() *url.URL {
	if frc.url == nil {
		frc.url = &url.URL{
			Path:     string(frc.ctx.Path()),
			RawQuery: string(frc.ctx.QueryArgs().QueryString()),
		}
	}
	return frc.url
}

func (frc *FastRequestContext) Header() http.Header {
	if frc.header == nil {
		frc.header = make(http.Header)
		frc.ctx.Request.Header.VisitAll(func(key, value []byte) {
			frc.header.Set(string(key), string(value))
		})
	}
	return frc.header
}

func CreateErrorResponse(ctx *RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	ctx.SetContentType("application/json")

	ctx.Response.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response.Header.Set("Pragma", "no-cache")
	ctx.Response.Header.Set("Expires", "0")

	if requestID := string(ctx.Request.Header.Peek("X-Request-ID")); requestID != "" {
		ctx.Response.Header.Set("X-Request-ID", requestID)
	}

	ctx.SetBodyString(`{"error":"Internal Server Error","message":"An unexpected error occurred"}`)
}

func CreateUnauthorizedResponse(ctx *RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusUnauthorized)
	ctx.SetContentType("application/json")

	ctx.Response.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Response.Header.Set("Pragma", "no-cache")
	ctx.Response.Header.Set("Expires", "0")

	if requestID := string(ctx.Request.Header.Peek("X-Request-ID")); requestID != "" {
		ctx.Response.Header.Set("X-Request-ID", requestID)
	}

	ctx.SetBodyString(`{"error":"Unauthorized","message":"Authentication required"}`)
}
