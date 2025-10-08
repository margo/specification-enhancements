package http

import (
	_ "embed"
	"net/http"
	"path"

	"github.com/go-openapi/runtime/middleware"
)

//go:embed openapi.yaml
var openapiYAML []byte

func RegisterOpenAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(openapiYAML)
	})

	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<title>Margo Desired State Manifest API</title>
<link rel="icon" href="data:,">
<style>html,body{margin:0;padding:0;font-family:system-ui,-apple-system,Segoe UI,Roboto,Ubuntu,sans-serif;}#redoc{height:100vh;}header{background:#111;color:#eee;padding:8px 16px;font-size:14px;display:flex;align-items:center;gap:12px;}header a{color:#88c5ff;text-decoration:none;}header a:hover{text-decoration:underline;}</style>
</head>
<body>
<header>
  <strong>Margo Desired State Manifest API</strong>
  <a href="/openapi.yaml" download>Download YAML</a>
  <a href="/swagger">Swagger UI</a>
</header>
<div id="redoc"></div>
<script src="https://cdn.jsdelivr.net/npm/redoc@next/bundles/redoc.standalone.js"></script>
<script>Redoc.init('/openapi.yaml',{hideDownloadButton:true},document.getElementById('redoc'))</script>
</body>
</html>`))
	})

	opts := middleware.SwaggerUIOpts{
		SpecURL: "/openapi.yaml",
		Path:    "/swagger",
		Title:   "Margo Desired State Manifest API",
	}
	mux.Handle(path.Clean(opts.Path), middleware.SwaggerUI(opts, nil))
}
