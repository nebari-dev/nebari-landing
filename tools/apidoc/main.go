//go:build ignore

// apidoc generates docs/api.md from the structured route definitions below.
//
// Run via go generate (preferred):
//
//	go generate ./internal/api/...
//
// or directly from the repo root:
//
//	go run tools/apidoc/main.go
//
// The tool writes output relative to the current working directory, so it must
// be run either via go generate (which sets cwd to the package dir) or from
// the repository root with the explicit path above.

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

// ─── Data model ───────────────────────────────────────────────────────────────

// Auth describes the authentication requirement for a route.
type Auth struct {
	Required bool
	Note     string // extra context, e.g. "+ admin group"
}

func (a Auth) String() string {
	if !a.Required {
		return "None / optional — passes through when `--enable-auth=false`"
	}
	s := "Required — `Authorization: Bearer <token>`"
	if a.Note != "" {
		s += " + " + a.Note
	}
	return s
}

// Param describes a path or query parameter.
type Param struct {
	Name        string
	In          string // path | query
	Description string
}

// Example holds a JSON body sample.
type Example struct {
	Description string // optional label shown before the code block
	Body        string // raw JSON
}

// Route documents a single HTTP endpoint.
type Route struct {
	Method      string
	Path        string
	Summary     string
	Description string   // additional prose
	Auth        Auth
	PathParams  []Param
	QueryParams []Param
	Request     *Example
	Responses   []string // e.g. "`200` — ...", "`404` — ..."
	Response    *Example // primary success response body
	Notes       []string // rendered as a blockquote bullet list
}

// Group is a named section containing related routes.
type Group struct {
	Name   string
	Routes []Route
}

// ─── Route definitions ────────────────────────────────────────────────────────

var groups = []Group{
	{
		Name: "Services",
		Routes: []Route{
			{
				Method:  "GET",
				Path:    "/api/v1/services",
				Summary: "List services",
				Auth:    Auth{Required: false},
				Responses: []string{
					"`200` — array of service objects",
				},
				Response: &Example{Body: `{
  "services": [
    {
      "id": "<uid>",
      "name": "JupyterHub",
      "status": "healthy",
      "description": "Interactive notebooks",
      "category": ["compute"],
      "pinned": false,
      "image": "<icon-url>",
      "url": "https://..."
    }
  ]
}`},
				Notes: []string{
					"Services whose `spec.landingPage.visibility` is `private` are filtered to members of the required Keycloak groups.",
					"The `pinned` field reflects the calling user's pin state; always `false` for unauthenticated callers.",
					"The `status` field reflects the latest health-probe result: `\"unknown\"` (no probe configured or not yet run), `\"healthy\"` (HTTP 2xx–3xx), or `\"unhealthy\"` (connection error or HTTP 4xx/5xx). Probes are configured per service via `spec.landingPage.healthCheck` in the NebariApp CRD.",
				},
			},
			{
				Method:  "GET",
				Path:    "/api/v1/services/{id}",
				Summary: "Get a service by UID",
				Auth:    Auth{Required: false},
				PathParams: []Param{
					{Name: "id", In: "path", Description: "Service UID (`NebariApp.metadata.uid`)"},
				},
				Responses: []string{
					"`200` — service object (same shape as list item)",
					"`404` — not found or not visible to the caller",
				},
			},
			{
				Method:  "POST",
				Path:    "/api/v1/services/{id}/request_access",
				Summary: "Submit an access request for a service",
				Auth:    Auth{Required: true},
				PathParams: []Param{
					{Name: "id", In: "path", Description: "Service UID"},
				},
				Request: &Example{
					Description: "All fields optional.",
					Body:        `{ "message": "I need access for research purposes." }`,
				},
				Responses: []string{
					"`202` — request created",
					"`409` — a pending request from this user for this service already exists",
					"`501` — access-request store not configured",
				},
			},
		},
	},
	{
		Name: "Categories",
		Routes: []Route{
			{
				Method:  "GET",
				Path:    "/api/v1/categories",
				Summary: "List categories derived from visible services",
				Auth:    Auth{Required: false},
				Response: &Example{Body: `{ "categories": ["compute", "data", "monitoring"] }`},
				Responses: []string{
					"`200` — distinct category strings",
				},
			},
		},
	},
	{
		Name: "Notifications",
		Routes: []Route{
			{
				Method:  "GET",
				Path:    "/api/v1/notifications",
				Summary: "List platform-wide notifications with per-caller read state",
				Auth:    Auth{Required: true},
				Response: &Example{Body: `{
  "notifications": [
    {
      "id": "<uuid>",
      "image": "<icon-url>",
      "title": "Scheduled maintenance",
      "message": "The cluster will be unavailable Saturday 02:00–04:00 UTC.",
      "read": false,
      "createdAt": "2026-03-05T10:00:00Z"
    }
  ]
}`},
				Responses: []string{
					"`200` — array of notification objects",
				},
			},
			{
				Method:  "PUT",
				Path:    "/api/v1/notifications/{id}/read",
				Summary: "Mark a notification as read for the calling user",
				Auth:    Auth{Required: true},
				PathParams: []Param{
					{Name: "id", In: "path", Description: "Notification UUID"},
				},
				Responses: []string{
					"`204` — marked read",
					"`404` — notification not found",
				},
			},
		},
	},
	{
		Name: "Pins",
		Routes: []Route{
			{
				Method:   "GET",
				Path:     "/api/v1/pins",
				Summary:  "List service UIDs pinned by the calling user",
				Auth:     Auth{Required: true},
				Response: &Example{Body: `{ "pins": ["<uid-a>", "<uid-b>"] }`},
				Responses: []string{
					"`200` — array of pinned UIDs",
				},
			},
			{
				Method:  "PUT",
				Path:    "/api/v1/pins/{uid}",
				Summary: "Pin a service",
				Auth:    Auth{Required: true},
				PathParams: []Param{
					{Name: "uid", In: "path", Description: "Service UID to pin"},
				},
				Responses: []string{"`204` — pinned"},
			},
			{
				Method:  "DELETE",
				Path:    "/api/v1/pins/{uid}",
				Summary: "Unpin a service",
				Auth:    Auth{Required: true},
				PathParams: []Param{
					{Name: "uid", In: "path", Description: "Service UID to unpin"},
				},
				Responses: []string{"`204` — unpinned"},
			},
		},
	},
	{
		Name: "Caller Identity",
		Routes: []Route{
			{
				Method:  "GET",
				Path:    "/api/v1/caller-identity",
				Summary: "Return the identity decoded from the caller's JWT",
				Auth:    Auth{Required: false},
				Response: &Example{Body: `{
  "authenticated": true,
  "username": "alice",
  "email": "alice@example.com",
  "name": "Alice Smith",
  "groups": ["admin", "developers"]
}`},
				Responses: []string{
					"`200` — `authenticated` is `false` when no valid token is present",
				},
			},
		},
	},
	{
		Name: "WebSocket",
		Routes: []Route{
			{
				Method:  "GET",
				Path:    "/api/v1/ws",
				Summary: "Real-time service events (WebSocket upgrade)",
				Auth:    Auth{Required: true, Note: "JWT in `Authorization` header **or** `?token=<jwt>` query param"},
				Response: &Example{
					Description: "Server pushes JSON frames on every service change event.",
					Body: `{
  "type": "added" | "modified" | "deleted",
  "service": { /* ServiceView shape */ }
}`,
				},
				Notes: []string{
					"Events are published via Redis Pub/Sub (`nebari:events`) so all webapi replicas fan out every event to their local WebSocket clients.",
					"`modified` events are also emitted whenever a service's health status changes (e.g. `\"unknown\"` → `\"healthy\"`), allowing the frontend to update health badges in real time without polling.",
				},
			},
		},
	},
	{
		Name: "Health",
		Routes: []Route{
			{
				Method:   "GET",
				Path:     "/api/v1/health",
				Summary:  "Liveness / readiness probe — no auth required",
				Auth:     Auth{Required: false},
				Response: &Example{Body: `{ "status": "healthy" }`},
				Responses: []string{
					"`200` — always `{\"status\":\"healthy\"}` while the process is alive",
				},
			},
		},
	},
	{
		Name: "Admin",
		Routes: []Route{
			{
				Method:  "GET",
				Path:    "/api/v1/admin/access-requests",
				Summary: "List all access requests",
				Auth:    Auth{Required: true, Note: "admin group membership"},
				QueryParams: []Param{
					{Name: "status", In: "query", Description: "`pending` | `approved` | `denied` — omit for all"},
				},
				Responses: []string{
					"`200` — array of access-request objects",
					"`403` — caller is not a member of the admin group",
				},
			},
			{
				Method:      "PUT",
				Path:        "/api/v1/admin/access-requests/{id}",
				Summary:     "Approve or deny an access request",
				Description: "When approved and a Keycloak client is configured, the requesting user is added to the service's required Keycloak group automatically.",
				Auth:        Auth{Required: true, Note: "admin group membership"},
				PathParams: []Param{
					{Name: "id", In: "path", Description: "Access-request UUID"},
				},
				Request: &Example{Body: `{ "status": "approved" | "denied" }`},
				Responses: []string{
					"`200` — updated access-request object",
					"`404` — request not found",
					"`403` — caller is not a member of the admin group",
				},
			},
			{
				Method:  "POST",
				Path:    "/api/v1/admin/notifications",
				Summary: "Create a platform-wide notification",
				Auth:    Auth{Required: true, Note: "admin group membership"},
				Request: &Example{Body: `{
  "image": "<icon-url>",
  "title": "Maintenance window",
  "message": "Details here."
}`},
				Responses: []string{
					"`201` — notification created",
					"`403` — caller is not a member of the admin group",
				},
			},
		},
	},
}

// ─── Template ─────────────────────────────────────────────────────────────────

const tmplSrc = `<!--
  THIS FILE IS AUTO-GENERATED — do not edit by hand.
  Source:    tools/apidoc/main.go
  Regenerate: go generate ./internal/api/...
  Generated: {{ .Timestamp }}
-->

# Webapi — HTTP API Reference

Base path: ` + "`/api/v1/`" + `

Endpoints that require authentication expect a Keycloak-issued JWT in the
` + "`Authorization: Bearer <token>`" + ` header. When the server runs with
` + "`--enable-auth=false`" + ` all routes are open.
{{ range .Groups }}
---

## {{ .Name }}
{{ range .Routes }}
### ` + "`{{ .Method }}`" + ` ` + "`{{ .Path }}`" + `

{{ .Summary }}
{{ if .Description }}
{{ .Description }}
{{ end -}}
**Auth:** {{ .Auth }}
{{ if .PathParams }}
**Path parameters**

| Name | Description |
|---|---|
{{ range .PathParams -}}
| ` + "`{{ .Name }}`" + ` | {{ .Description }} |
{{ end -}}
{{ end -}}
{{ if .QueryParams }}
**Query parameters**

| Name | Description |
|---|---|
{{ range .QueryParams -}}
| ` + "`{{ .Name }}`" + ` | {{ .Description }} |
{{ end -}}
{{ end -}}
{{ if .Request }}
**Request body**{{ if .Request.Description }} — {{ .Request.Description }}{{ end }}

` + "```" + `json
{{ .Request.Body }}
` + "```" + `
{{ end -}}
{{ if .Responses }}
**Responses**
{{ range .Responses }}
- {{ . }}
{{- end }}
{{ end -}}
{{ if .Response }}{{ if .Response.Description }}
{{ .Response.Description }}
{{ end }}{{ if .Response.Body -}}
` + "```" + `json
{{ .Response.Body }}
` + "```" + `
{{ end -}}{{ end -}}
{{ if .Notes }}
> **Notes**
{{ range .Notes -}}
> - {{ . }}
{{ end }}
{{ end -}}
{{ end -}}
{{ end -}}
`

// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	tmpl, err := template.New("api").Parse(tmplSrc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "template parse error: %v\n", err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any{
		"Groups":    groups,
		"Timestamp": time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "template execute error: %v\n", err)
		os.Exit(1)
	}

	// When invoked via `go generate` the cwd is the package directory
	// (internal/api/). When run from the repo root it is the root itself.
	// We support both by walking up until we find go.mod.
	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot locate module root: %v\n", err)
		os.Exit(1)
	}

	out := filepath.Join(root, "docs", "api.md")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", out)
}

// findModuleRoot walks up from the current working directory until it finds a
// directory containing go.mod, which is the module root.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
