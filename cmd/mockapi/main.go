// Copyright 2026, OpenTeams. Apache-2.0
// mockapi: lightweight dev server with same HTTP API as the real webapi.
// No Kubernetes required — reads NebariApp YAML from disk, keeps pins in-memory,
// decodes JWT payloads without signature validation.
//
// Usage:
//
//	SERVICES_FILE=dev/manifests/test-nebariapps.yaml PORT=8090 go run ./cmd/mockapi
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ─── NebariApp YAML types ─────────────────────────────────────────────────────

type nebariApp struct {
	Kind     string       `yaml:"kind"`
	Metadata appMeta      `yaml:"metadata"`
	Spec     appSpec      `yaml:"spec"`
}

type appMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type appSpec struct {
	Hostname    string      `yaml:"hostname"`
	LandingPage landingPage `yaml:"landingPage"`
}

type landingPage struct {
	Enabled        bool     `yaml:"enabled"`
	DisplayName    string   `yaml:"displayName"`
	Description    string   `yaml:"description"`
	Icon           string   `yaml:"icon"`
	Category       string   `yaml:"category"`
	Priority       int      `yaml:"priority"`
	Visibility     string   `yaml:"visibility"`
	RequiredGroups []string `yaml:"requiredGroups"`
}

// ─── Service ──────────────────────────────────────────────────────────────────

type svcInfo struct {
	UID            string
	Name           string
	DisplayName    string
	Description    string
	URL            string
	Icon           string
	Category       string
	Priority       int
	Visibility     string
	RequiredGroups []string
}

// ─── Wire types (same JSON shape as real webapi) ─────────────────────────────

type svcView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Description string   `json:"description"`
	Category    []string `json:"category"`
	Pinned      bool     `json:"pinned"`
	Image       string   `json:"image"`
	URL         string   `json:"url"`
}

type svcResponse struct {
	Services []*svcView `json:"services"`
}

type callerID struct {
	Authenticated bool     `json:"authenticated"`
	Username      string   `json:"username,omitempty"`
	Email         string   `json:"email,omitempty"`
	Name          string   `json:"name,omitempty"`
	Groups        []string `json:"groups,omitempty"`
}

type pinsResp struct {
	Pins []*svcInfo `json:"pins"`
	UIDs []string   `json:"uids"`
}

// ─── JWT payload decoding (no signature validation) ──────────────────────────

type claims struct {
	Sub               string   `json:"sub"`
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
}

func parseJWT(token string) *claims {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var c claims
	if err := json.Unmarshal(b, &c); err != nil {
		return nil
	}
	if c.PreferredUsername == "" {
		c.PreferredUsername = c.Sub
	}
	return &c
}

func bearerClaims(r *http.Request) *claims {
	h := r.Header.Get("Authorization")
	if h == "" {
		return nil
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil
	}
	return parseJWT(parts[1])
}

// ─── In-memory pin store ──────────────────────────────────────────────────────

type pins struct {
	mu   sync.Mutex
	data map[string]map[string]bool
}

func newPins() *pins { return &pins{data: make(map[string]map[string]bool)} }

func (p *pins) get(user string) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.data[user]))
	for uid := range p.data[user] {
		out = append(out, uid)
	}
	return out
}

func (p *pins) add(user, uid string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.data[user] == nil {
		p.data[user] = make(map[string]bool)
	}
	p.data[user][uid] = true
}

func (p *pins) remove(user, uid string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.data[user], uid)
}

// ─── Access control ───────────────────────────────────────────────────────────

func canAccess(svc *svcInfo, c *claims) bool {
	switch svc.Visibility {
	case "public":
		return true
	case "private":
		if c == nil {
			return false
		}
		return hasGroups(c.Groups, svc.RequiredGroups)
	default: // "authenticated" or anything else
		return c != nil
	}
}

func hasGroups(user, required []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, r := range required {
		for _, g := range user {
			if g == r {
				return true
			}
		}
	}
	return false
}

// ─── Server ───────────────────────────────────────────────────────────────────

type srv struct {
	svcs  []*svcInfo
	byUID map[string]*svcInfo
	pins  *pins
}

func (s *srv) view(svc *svcInfo, pinned bool) *svcView {
	name := svc.DisplayName
	if name == "" {
		name = svc.Name
	}
	cat := []string{}
	if svc.Category != "" {
		cat = []string{svc.Category}
	}
	return &svcView{
		ID:          svc.UID,
		Name:        name,
		Status:      "healthy",
		Description: svc.Description,
		Category:    cat,
		Pinned:      pinned,
		Image:       svc.Icon,
		URL:         svc.URL,
	}
}

func (s *srv) pinnedSet(user string) map[string]bool {
	m := map[string]bool{}
	for _, uid := range s.pins.get(user) {
		m[uid] = true
	}
	return m
}

func (s *srv) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/services", s.listServices)
	mux.HandleFunc("/api/v1/services/", s.servicesSub)
	mux.HandleFunc("/api/v1/categories", s.getCategories)
	mux.HandleFunc("/api/v1/health", s.health)
	mux.HandleFunc("/api/v1/notifications", s.notifications)
	mux.HandleFunc("/api/v1/notifications/", s.notifSub)
	mux.HandleFunc("/api/v1/caller-identity", s.callerIdentity)
	mux.HandleFunc("/api/v1/pins", s.listPins)
	mux.HandleFunc("/api/v1/pins/", s.pinByUID)
	mux.HandleFunc("/api/v1/admin/access-requests", s.adminAccessRequests)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	return cors(mux)
}

func (s *srv) listServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c := bearerClaims(r)
	pinned := map[string]bool{}
	if c != nil {
		pinned = s.pinnedSet(c.PreferredUsername)
	}
	views := make([]*svcView, 0)
	for _, svc := range s.svcs {
		if canAccess(svc, c) {
			views = append(views, s.view(svc, pinned[svc.UID]))
		}
	}
	writeJSON(w, svcResponse{Services: views})
}

func (s *srv) servicesSub(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/services/")
	parts := strings.SplitN(rest, "/", 2)
	uid := parts[0]
	if uid == "" {
		http.Error(w, "uid required", http.StatusBadRequest)
		return
	}
	if len(parts) == 2 && parts[1] == "request_access" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c := bearerClaims(r)
		if c == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if s.byUID[uid] == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "mock-req", "status": "pending"})
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	svc := s.byUID[uid]
	if svc == nil {
		http.NotFound(w, r)
		return
	}
	c := bearerClaims(r)
	if !canAccess(svc, c) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	pinned := map[string]bool{}
	if c != nil {
		pinned = s.pinnedSet(c.PreferredUsername)
	}
	writeJSON(w, s.view(svc, pinned[uid]))
}

func (s *srv) getCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	seen := map[string]bool{}
	cats := []string{}
	for _, svc := range s.svcs {
		if svc.Category != "" && !seen[svc.Category] {
			seen[svc.Category] = true
			cats = append(cats, svc.Category)
		}
	}
	sort.Strings(cats)
	writeJSON(w, map[string]interface{}{"categories": cats})
}

func (s *srv) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "healthy"})
}

func (s *srv) notifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"notifications": []interface{}{}})
}

func (s *srv) notifSub(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.NotFound(w, r)
}

func (s *srv) callerIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c := bearerClaims(r)
	if c == nil {
		writeJSON(w, callerID{Authenticated: false})
		return
	}
	writeJSON(w, callerID{
		Authenticated: true,
		Username:      c.PreferredUsername,
		Email:         c.Email,
		Name:          c.Name,
		Groups:        c.Groups,
	})
}

func (s *srv) listPins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c := bearerClaims(r)
	if c == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	uids := s.pins.get(c.PreferredUsername)
	svcs := make([]*svcInfo, 0, len(uids))
	for _, uid := range uids {
		if svc := s.byUID[uid]; svc != nil {
			svcs = append(svcs, svc)
		}
	}
	writeJSON(w, pinsResp{Pins: svcs, UIDs: uids})
}

func (s *srv) pinByUID(w http.ResponseWriter, r *http.Request) {
	c := bearerClaims(r)
	if c == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	uid := strings.TrimPrefix(r.URL.Path, "/api/v1/pins/")
	if uid == "" {
		http.Error(w, "uid required", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		s.pins.add(c.PreferredUsername, uid)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		s.pins.remove(c.PreferredUsername, uid)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *srv) adminAccessRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c := bearerClaims(r)
	if c == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !hasGroups(c.Groups, []string{"admin"}) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, map[string]interface{}{"accessRequests": []interface{}{}})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── YAML loading ─────────────────────────────────────────────────────────────

func loadServices(path string) ([]*svcInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	var out []*svcInfo
	for {
		var app nebariApp
		if err := decoder.Decode(&app); err != nil {
			if err == io.EOF {
				break
			}
			slog.Warn("skipping unparseable YAML document", "err", err)
			continue
		}
		if app.Kind != "NebariApp" || !app.Spec.LandingPage.Enabled {
			continue
		}
		lp := app.Spec.LandingPage
		priority := lp.Priority
		if priority == 0 {
			priority = 100
		}
		visibility := lp.Visibility
		if visibility == "" {
			visibility = "private"
		}
		out = append(out, &svcInfo{
			UID:            app.Metadata.Namespace + "/" + app.Metadata.Name,
			Name:           app.Metadata.Name,
			DisplayName:    lp.DisplayName,
			Description:    lp.Description,
			URL:            "http://" + app.Spec.Hostname,
			Icon:           lp.Icon,
			Category:       lp.Category,
			Priority:       priority,
			Visibility:     visibility,
			RequiredGroups: lp.RequiredGroups,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	servicesFile := os.Getenv("SERVICES_FILE")
	if servicesFile == "" {
		servicesFile = "dev/manifests/test-nebariapps.yaml"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	svcs, err := loadServices(servicesFile)
	if err != nil {
		slog.Error("load services", "file", servicesFile, "err", err)
		os.Exit(1)
	}
	slog.Info("loaded services", "count", len(svcs), "file", servicesFile)

	byUID := make(map[string]*svcInfo, len(svcs))
	for _, svc := range svcs {
		byUID[svc.UID] = svc
		slog.Info("service", "uid", svc.UID, "name", svc.DisplayName, "visibility", svc.Visibility)
	}

	server := &srv{svcs: svcs, byUID: byUID, pins: newPins()}
	addr := ":" + port
	slog.Info("mockapi listening", "addr", addr)
	if err := http.ListenAndServe(addr, server.routes()); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}
