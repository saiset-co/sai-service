package admin

import (
	"bytes"
	"embed"
	"mime"
	"path/filepath"
	"strings"

	"github.com/saiset-co/sai-service/types"
)

//go:embed assets/*
//go:embed assets/fonts/*
var embeddedAssets embed.FS

func (b *Builder) mountAssets(group types.GroupBuilder) {
	group.GET("/assets/tailwind.js", b.serveEmbeddedAsset("assets/tailwind.js"))
	group.GET("/assets/fonts/inter.css", b.serveEmbeddedAsset("assets/fonts/inter.css"))
	group.GET("/assets/fonts/inter-cyrillic.woff2", b.serveEmbeddedAsset("assets/fonts/inter-cyrillic.woff2"))
	group.GET("/assets/fonts/inter-latin.woff2", b.serveEmbeddedAsset("assets/fonts/inter-latin.woff2"))
}

func (b *Builder) serveEmbeddedAsset(name string) types.FastHTTPHandler {
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}

	return func(ctx *types.RequestCtx) {
		data, contentEncoding, err := readEmbeddedAssetForRequest(ctx, name)
		if err != nil {
			ctx.Error(err, 404)
			return
		}

		if contentEncoding != "" {
			ctx.Response.Header.Set("Content-Encoding", contentEncoding)
			ctx.Response.Header.Add("Vary", "Accept-Encoding")
		}
		ctx.Response.Header.Set("Cache-Control", "public, max-age=86400")
		_, _ = ctx.Success(data, []byte(contentType))
	}
}

func readEmbeddedAssetForRequest(ctx *types.RequestCtx, name string) ([]byte, string, error) {
	acceptEncoding := string(ctx.Request.Header.Peek("Accept-Encoding"))
	if strings.Contains(strings.ToLower(acceptEncoding), "gzip") {
		if gzData, err := embeddedAssets.ReadFile(name + ".gz"); err == nil {
			return gzData, "gzip", nil
		}
	}

	data, err := embeddedAssets.ReadFile(name)
	if err != nil {
		return nil, "", err
	}

	return bytes.Clone(data), "", nil
}
