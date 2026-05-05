// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	_ "embed"
	"net/http"

	// Side-effect import: registers the swag-generated SwaggerInfo so the spec
	// content is available without an explicit dependency from this file.
	_ "github.com/nebari-dev/nebari-landing/internal/api/docs"
)

// openapiSpec is the generated OpenAPI 3.1 spec, embedded at build time.
// Regenerate with `make generate-docs` after editing handler annotations or
// changing the @general info block in cmd/main.go.
//
//go:embed docs/swagger.json
var openapiSpec []byte

// scalarViewerHTML is a single-page HTML viewer that loads the spec from
// /api/v1/docs/openapi.json via the Scalar JS bundle hosted on a CDN. No Go
// dependencies are needed — the browser does the rendering.
const scalarViewerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>Nebari Landing API</title>
</head>
<body>
  <div id="scalar"></div>
  <script id="api-reference" data-url="/api/v1/docs/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

// handleOpenAPISpec serves the embedded OpenAPI 3.1 spec as application/json.
// Only registered when WithDocsEnabled is set on the Handler.
func (h *Handler) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(openapiSpec)
}

// handleScalarViewer serves a static HTML page that renders the spec via the
// Scalar API Reference CDN bundle. Only registered when WithDocsEnabled is set.
func (h *Handler) handleScalarViewer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(scalarViewerHTML))
}
