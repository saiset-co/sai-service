package types

import (
	"github.com/valyala/fasthttp"
	"net/http"
	"net/url"
)

type FastHTTPHandler func(ctx *fasthttp.RequestCtx)

type FastResponseWriter struct {
	ctx        *fasthttp.RequestCtx
	statusCode int
}

func NewFastResponseWriter(ctx *fasthttp.RequestCtx) *FastResponseWriter {
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
	ctx    *fasthttp.RequestCtx
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
