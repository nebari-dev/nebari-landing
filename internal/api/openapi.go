// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
	"sort"
	"strings"

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

// docsViewer is a server-rendered HTML page that walks the embedded spec and
// emits a clean reference. No JavaScript, no CDN, no third-party deps —
// works offline and behind firewalls. Native <details> handles collapsing.
const docsViewerTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>{{.Title}}</title>
  <style>
    :root {
      --bg: #ffffff; --fg: #1f2328; --muted: #57606a;
      --border: #d0d7de; --card: #f6f8fa; --code: #eaeef2;
      --link: #0969da;
      --get: #0550ae; --get-bg: #ddf4ff;
      --post: #1a7f37; --post-bg: #dafbe1;
      --put: #9a6700; --put-bg: #fff8c5;
      --delete: #cf222e; --delete-bg: #ffebe9;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --bg: #0d1117; --fg: #e6edf3; --muted: #8b949e;
        --border: #30363d; --card: #161b22; --code: #21262d;
        --link: #79c0ff;
        --get: #79c0ff; --get-bg: #1f3a5c;
        --post: #56d364; --post-bg: #1a3e2a;
        --put: #d29922; --put-bg: #3d2e0a;
        --delete: #f85149; --delete-bg: #3d1414;
      }
    }
    * { box-sizing: border-box; }
    body { margin: 0; padding: 2rem; max-width: 980px; margin-inline: auto;
      background: var(--bg); color: var(--fg);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
      line-height: 1.5; }
    h1 { margin: 0 0 .25rem; font-size: 1.6rem; }
    .meta { color: var(--muted); font-size: .9rem; margin-bottom: 2rem; }
    .meta code { background: var(--code); padding: 1px 6px; border-radius: 4px; font-size: .85rem; }
    .meta a { color: var(--link); }
    .desc { color: var(--muted); margin-bottom: 1.5rem; white-space: pre-line; }
    details { margin: 0 0 .5rem; border: 1px solid var(--border); border-radius: 6px;
      background: var(--card); }
    details[open] { background: var(--bg); }
    summary { padding: .65rem .9rem; cursor: pointer; list-style: none;
      display: flex; align-items: center; gap: .75rem; }
    summary::-webkit-details-marker { display: none; }
    .badge { display: inline-block; min-width: 60px; text-align: center;
      padding: 2px 8px; border-radius: 4px; font-family: ui-monospace, "Cascadia Code", Menlo, monospace;
      font-size: .75rem; font-weight: 600; letter-spacing: .04em; }
    .GET { color: var(--get); background: var(--get-bg); }
    .POST { color: var(--post); background: var(--post-bg); }
    .PUT, .PATCH { color: var(--put); background: var(--put-bg); }
    .DELETE { color: var(--delete); background: var(--delete-bg); }
    .path { font-family: ui-monospace, "Cascadia Code", Menlo, monospace;
      font-size: .9rem; color: var(--link); }
    .summary { color: var(--muted); margin-left: auto; font-size: .9rem;
      overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .body { padding: .25rem 1rem 1rem; border-top: 1px solid var(--border); }
    .body p { margin: .75rem 0; }
    .body h4 { margin: 1rem 0 .35rem; font-size: .8rem; text-transform: uppercase;
      letter-spacing: .06em; color: var(--muted); font-weight: 600; }
    pre { background: var(--code); padding: .75rem 1rem; border-radius: 6px;
      overflow-x: auto; margin: 0; font-family: ui-monospace, "Cascadia Code", Menlo, monospace;
      font-size: .85rem; line-height: 1.45; }
    .auth { display: inline-block; padding: 1px 8px; border-radius: 10px;
      font-size: .7rem; background: var(--code); color: var(--muted);
      margin-left: .5rem; }
  </style>
</head>
<body>
  <h1>{{.Title}}</h1>
  <div class="meta">
    <code>{{.Version}}</code>
    {{- if .Server}} &middot; base path: <code>{{.Server}}</code>{{end}}
    &middot; <a href="openapi.json">openapi.json</a>
  </div>
  {{- if .Description}}<div class="desc">{{.Description}}</div>{{end}}

  {{- range .Operations}}
  <details>
    <summary>
      <span class="badge {{.Method}}">{{.Method}}</span>
      <span class="path">{{.Path}}</span>
      <span class="summary">{{.Summary}}{{if .Auth}}<span class="auth">auth</span>{{end}}</span>
    </summary>
    <div class="body">
      {{- if .Description}}<p>{{.Description}}</p>{{end}}
      <h4>curl</h4>
      <pre>{{.Curl}}</pre>
    </div>
  </details>
  {{- end}}
</body>
</html>`

// docsView is the data shape used by the viewer template — populated once at
// startup by parseSpecForViewer.
type docsView struct {
	Title       string
	Version     string
	Server      string
	Description string
	Operations  []docsOp
}

type docsOp struct {
	Method      string
	Path        string
	Summary     string
	Description string
	Auth        bool
	Curl        string
}

var (
	docsTmpl       = template.Must(template.New("docs").Parse(docsViewerTemplate))
	docsViewerHTML []byte // rendered once at init from openapiSpec
)

func init() {
	view, err := parseSpecForViewer(openapiSpec)
	if err != nil {
		// Spec is generated and embedded at build; a parse failure here means
		// the build is broken. Panic so we fail loud rather than serve a bad
		// page in production (where docs are off anyway).
		panic("openapi: parse embedded spec: " + err.Error())
	}
	var buf strings.Builder
	if err := docsTmpl.Execute(&buf, view); err != nil {
		panic("openapi: render embedded spec: " + err.Error())
	}
	docsViewerHTML = []byte(buf.String())
}

// parseSpecForViewer walks the embedded OpenAPI 3.1 spec and produces the
// flat docsView the template expects. Operations are sorted by path then
// canonical HTTP-method order.
func parseSpecForViewer(raw []byte) (docsView, error) {
	var spec struct {
		Info struct {
			Title       string `json:"title"`
			Version     string `json:"version"`
			Description string `json:"description"`
		} `json:"info"`
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
		Paths map[string]map[string]struct {
			Summary     string                `json:"summary"`
			Description string                `json:"description"`
			Security    []map[string][]string `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return docsView{}, err
	}

	v := docsView{
		Title:       spec.Info.Title,
		Version:     spec.Info.Version,
		Description: spec.Info.Description,
	}
	if len(spec.Servers) > 0 {
		v.Server = spec.Servers[0].URL
	}

	methodOrder := map[string]int{"get": 0, "post": 1, "put": 2, "patch": 3, "delete": 4}

	paths := make([]string, 0, len(spec.Paths))
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		methods := spec.Paths[p]
		mkeys := make([]string, 0, len(methods))
		for m := range methods {
			mkeys = append(mkeys, m)
		}
		sort.Slice(mkeys, func(i, j int) bool { return methodOrder[mkeys[i]] < methodOrder[mkeys[j]] })

		for _, m := range mkeys {
			op := methods[m]
			method := strings.ToUpper(m)
			auth := false
			for _, sec := range op.Security {
				if len(sec) > 0 {
					auth = true
					break
				}
			}
			v.Operations = append(v.Operations, docsOp{
				Method:      method,
				Path:        p,
				Summary:     op.Summary,
				Description: op.Description,
				Auth:        auth,
				Curl:        renderCurl(method, v.Server+p, auth),
			})
		}
	}
	return v, nil
}

// renderCurl emits a copy-pasteable shell snippet for the operation. Includes
// a Bearer placeholder when the operation declares any security requirement
// and a JSON Content-Type for write methods.
func renderCurl(method, fullPath string, withAuth bool) string {
	var b strings.Builder
	b.WriteString("curl -s -X ")
	b.WriteString(method)
	b.WriteString(" http://localhost:8080")
	b.WriteString(fullPath)
	if withAuth {
		b.WriteString(" \\\n  -H 'Authorization: Bearer $TOKEN'")
	}
	if method == "POST" || method == "PUT" || method == "PATCH" {
		b.WriteString(" \\\n  -H 'Content-Type: application/json' \\\n  -d '{}'")
	}
	return b.String()
}

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

// handleDocsViewer serves a static, self-contained HTML reference rendered
// from the embedded spec at startup. No JavaScript, no CDN, no external
// resources. Only registered when WithDocsEnabled is set.
func (h *Handler) handleDocsViewer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(docsViewerHTML)
}
