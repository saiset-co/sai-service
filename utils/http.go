package utils

import "github.com/valyala/fasthttp"

func CreateErrorResponse(ctx *fasthttp.RequestCtx) {
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

func CreateUnauthorizedResponse(ctx *fasthttp.RequestCtx) {
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
