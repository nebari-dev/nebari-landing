package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/nebari-dev/nebari-landing/internal/accessrequests"
	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	webkeycloak "github.com/nebari-dev/nebari-landing/internal/keycloak"
	"github.com/nebari-dev/nebari-landing/internal/notifications"
	"github.com/nebari-dev/nebari-landing/internal/pins"
	wshub "github.com/nebari-dev/nebari-landing/internal/websocket"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("api")

// Handler handles HTTP requests for the landing page API
type Handler struct {
	cache              *cache.ServiceCache
	jwtValidator       *auth.JWTValidator
	enableAuth         bool
	hub                *wshub.Hub
	pinStore           *pins.PinStore
	accessRequestStore *accessrequests.Store
	notificationStore  *notifications.Store
	// keycloakClient is used for admin operations (e.g. adding users to groups on approval).
	// When nil, Keycloak group membership is not updated automatically.
	keycloakClient *webkeycloak.Client
	// adminGroup is the Keycloak group name whose members may access admin-only endpoints.
	// Defaults to "admin" when not set.
	adminGroup string
	// allowedOrigins is the list of Origins permitted by the CORS middleware.
	// Use ["*"] (default) to allow all origins, or supply specific origins such
	// as ["https://nebari.example.com"] for stricter enforcement.
	// In production the browser always hits the same external hostname whether
	// it talks to the frontend or /api/*, so CORS is not triggered. This flag
	// is primarily useful for local Vite dev (localhost:5173 → localhost:8080).
	allowedOrigins []string
	// debugMode enables the GET /api/v1/debug endpoint which exposes request
	// headers, resolved auth claims, and per-visibility service counts to aid
	// in diagnosing auth and proxy-forwarding issues (similar to Keycloak's
	// KC_HOSTNAME_DEBUG mode). Never enable in production.
	debugMode bool
	// docsEnabled exposes the OpenAPI spec at /api/v1/docs/openapi.json and a
	// Scalar viewer at /api/v1/docs. Off by default — never enable in production.
	docsEnabled bool
	// claimsExtractor, when non-nil, replaces the JWT validation step.
	// Use WithClaimsExtractor in tests to inject synthetic claims without
	// needing a real Keycloak instance or signed token.
	claimsExtractor func(*http.Request) (*auth.Claims, bool)
}

// HandlerOption configures optional Handler fields.
type HandlerOption func(*Handler)

// WithAccessRequestStore attaches an access-request store to the handler.
func WithAccessRequestStore(s *accessrequests.Store) HandlerOption {
	return func(h *Handler) { h.accessRequestStore = s }
}

// WithAdminGroup sets the Keycloak group name that grants admin privileges.
// Defaults to "admin".
func WithAdminGroup(group string) HandlerOption {
	return func(h *Handler) { h.adminGroup = group }
}

// WithNotificationStore attaches a notification store to the handler.
// When set, GET /api/v1/notifications returns real data and
// PUT /api/v1/notifications/{id}/read becomes available.
func WithNotificationStore(s *notifications.Store) HandlerOption {
	return func(h *Handler) { h.notificationStore = s }
}

// WithKeycloakAdminClient attaches a Keycloak admin client to the handler.
// When set, approving an access request automatically adds the user to the
// service's required Keycloak groups. When nil (default), the status is
// updated in the store but no Keycloak group change is made.
func WithKeycloakAdminClient(c *webkeycloak.Client) HandlerOption {
	return func(h *Handler) { h.keycloakClient = c }
}

// WithAllowedOrigins sets the list of Origins the CORS middleware will accept.
// Pass ["*"] (the default) to allow all origins, or one or more explicit
// origins (e.g. ["https://nebari.example.com", "http://localhost:5173"]) to
// enforce a whitelist. Requests whose Origin header is not in the list will
// not receive Access-Control-Allow-Origin in the response.
func WithAllowedOrigins(origins []string) HandlerOption {
	return func(h *Handler) {
		if len(origins) > 0 {
			h.allowedOrigins = origins
		}
	}
}

// WithDebugMode enables the GET /api/v1/debug endpoint which dumps request
// headers, resolved JWT claims, and per-visibility service counts to aid in
// diagnosing auth and proxy-forwarding issues. Never enable in production.
func WithDebugMode() HandlerOption {
	return func(h *Handler) { h.debugMode = true }
}

// WithDocsEnabled exposes the OpenAPI spec at /api/v1/docs/openapi.json and a
// Scalar viewer at /api/v1/docs. Routes are not registered when disabled.
// Never enable in production — the spec leaks endpoint shape and parameter names.
func WithDocsEnabled() HandlerOption {
	return func(h *Handler) { h.docsEnabled = true }
}

// WithClaimsExtractor replaces the JWT validation step with a custom function.
// Intended for unit tests that need to simulate authenticated requests without
// a real Keycloak instance or signed JWT. The function receives the request and
// returns the synthetic claims plus an authenticated flag.
//
// Example (test-only):
//
//	h := NewHandler(sc, nil, true, nil, nil,
//	    WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
//	        return &auth.Claims{PreferredUsername: "alice", Groups: []string{"admin"}}, true
//	    }))
func WithClaimsExtractor(fn func(*http.Request) (*auth.Claims, bool)) HandlerOption {
	return func(h *Handler) { h.claimsExtractor = fn }
}

// NewHandler creates a new API handler.
// pinStore may be nil; when nil the /api/v1/pins endpoints return 501.
func NewHandler(serviceCache *cache.ServiceCache, jwtValidator *auth.JWTValidator, enableAuth bool, hub *wshub.Hub, pinStore *pins.PinStore, opts ...HandlerOption) *Handler {
	h := &Handler{
		cache:          serviceCache,
		jwtValidator:   jwtValidator,
		enableAuth:     enableAuth,
		hub:            hub,
		pinStore:       pinStore,
		adminGroup:     "admin",
		allowedOrigins: []string{"*"},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Routes returns the HTTP router for the API
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/v1/services", h.handleGetServices)
	mux.HandleFunc("/api/v1/services/", h.handleServicesSub) // /{id} and /{id}/request_access
	mux.HandleFunc("/api/v1/categories", h.handleGetCategories)
	mux.HandleFunc("/api/v1/health", h.handleHealth)
	mux.HandleFunc("/api/v1/notifications", h.handleGetNotifications)
	mux.HandleFunc("/api/v1/notifications/", h.handleNotificationSub) // /{id}/read

	// WebSocket — real-time service updates
	if h.hub != nil {
		mux.HandleFunc("/api/v1/ws", h.handleWS)
	}

	// Caller identity — returns JWT claims for the requesting user
	mux.HandleFunc("/api/v1/caller-identity", h.handleCallerIdentity)

	// Debug endpoint — only registered when debug mode is enabled.
	// Shows request headers, resolved JWT claims, and service visibility counts.
	// Use --debug / DEBUG_MODE=true to activate. Never enable in production.
	if h.debugMode {
		mux.HandleFunc("/api/v1/debug", h.handleDebug)
	}

	// OpenAPI docs — only registered when --enable-docs is set.
	// Serves the raw spec at /api/v1/docs/openapi.json and a Scalar HTML viewer
	// at /api/v1/docs. Never enable in production.
	if h.docsEnabled {
		mux.HandleFunc("/api/v1/docs/openapi.json", h.handleOpenAPISpec)
		mux.HandleFunc("/api/v1/docs", h.handleDocsViewer)
	}

	// User pins — requires authentication; 501 when no PinStore is configured
	mux.HandleFunc("/api/v1/pins", h.handleGetPins)
	mux.HandleFunc("/api/v1/pins/", h.handlePinByUID)

	// Admin routes — require caller to be a member of h.adminGroup
	mux.HandleFunc("/api/v1/admin/access-requests", h.handleAdminListAccessRequests)
	mux.HandleFunc("/api/v1/admin/access-requests/", h.handleAdminAccessRequestSub)
	mux.HandleFunc("/api/v1/admin/notifications", h.handleAdminCreateNotification)

	// Static content is served by the dedicated frontend pod; the webapi never
	// handles bare "/" requests. Return 404 for any unmatched root path so API
	// clients get a clear signal rather than an unexpected HTML page.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	return corsMiddleware(h.allowedOrigins)(mux)
}

// handleWS serves GET /api/v1/ws.
// It uses the same authentication gate as protected API endpoints before
// allowing the WebSocket protocol upgrade.
//
//	@Summary		WebSocket: real-time service + notification events
//	@Description	Upgrade to WebSocket. Server pushes JSON envelopes for service add/update/delete and new notifications. The same auth gate as protected REST endpoints applies before the upgrade is accepted.
//	@Tags			websocket
//	@Success		101	{string}	string	"Switching Protocols"
//	@Failure		401	{string}	string	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/ws [get]
func (h *Handler) handleWS(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAuth(w, r); !ok {
		return
	}
	h.hub.ServeWS(w, r)
}

// ServiceView is the client-facing representation of a service.
type ServiceView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Description string   `json:"description"`
	Category    []string `json:"category"`
	Pinned      bool     `json:"pinned"`
	Image       string   `json:"image"`
	URL         string   `json:"url"`
}

// ServiceResponse is the response format for GET /api/v1/services.
type ServiceResponse struct {
	Services []*ServiceView `json:"services"`
}

// toServiceView converts a cached ServiceInfo into the client-facing ServiceView.
func toServiceView(svc *cache.ServiceInfo, pinned bool) *ServiceView {
	name := svc.DisplayName
	if name == "" {
		name = svc.Name
	}
	status := "unknown"
	if svc.Health != nil {
		status = svc.Health.Status
	}
	category := []string{}
	if svc.Category != "" {
		category = []string{svc.Category}
	}
	return &ServiceView{
		ID:          svc.UID,
		Name:        name,
		Status:      status,
		Description: svc.Description,
		Category:    category,
		Pinned:      pinned,
		Image:       svc.Icon,
		URL:         svc.URL,
	}
}

// NotificationItem is a single notification as returned by GET /api/v1/notifications.
type NotificationItem struct {
	ID        string `json:"id"`
	Image     string `json:"image"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}

// CallerIdentityResponse is the response format for GET /api/v1/caller-identity.
// It reflects the identity of the caller as decoded from the JWT, or an
// unauthenticated sentinel when no valid token is present.
type CallerIdentityResponse struct {
	Authenticated bool     `json:"authenticated"`
	Username      string   `json:"username,omitempty"`
	Email         string   `json:"email,omitempty"`
	Name          string   `json:"name,omitempty"`
	Groups        []string `json:"groups,omitempty"`
}

// callerPinnedUIDs returns the set of UIDs pinned by the authenticated caller.
// Returns an empty map when pins are not configured or the caller is not authenticated.
func (h *Handler) callerPinnedUIDs(claims *auth.Claims, authenticated bool) map[string]bool {
	if h.pinStore == nil || !authenticated || claims == nil {
		return map[string]bool{}
	}
	username := claims.PreferredUsername
	if username == "" {
		username = claims.Subject
	}
	if username == "" {
		return map[string]bool{}
	}
	uids, err := h.pinStore.Get(username)
	if err != nil {
		log.Error(err, "Failed to read pins for caller", "user", username)
		return map[string]bool{}
	}
	m := make(map[string]bool, len(uids))
	for _, uid := range uids {
		m[uid] = true
	}
	return m
}

// handleGetServices serves GET /api/v1/services.
//
//	@Summary		List discoverable services
//	@Description	Returns the union of public services plus any private services the caller may access based on their JWT groups.
//	@Description	Anonymous callers see public services only; bearer-token callers additionally see private services they have access to. The "pinned" flag reflects the caller's own pin list (always false for anonymous).
//	@Tags			services
//	@Produce		json
//	@Success		200	{object}	ServiceResponse
//	@Failure		405	{string}	string	"Method not allowed"
//	@Security		BearerAuth
//	@Router			/services [get]
func (h *Handler) handleGetServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, authenticated := h.extractAndValidateJWT(r)
	pinnedUIDs := h.callerPinnedUIDs(claims, authenticated)

	allServices := h.cache.GetAll()
	views := make([]*ServiceView, 0, len(allServices))
	for _, service := range allServices {
		if !h.canAccessService(service, authenticated, claims) {
			continue
		}
		views = append(views, toServiceView(service, pinnedUIDs[service.UID]))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ServiceResponse{Services: views}); err != nil {
		log.Error(err, "Failed to encode response")
	}
}

// handleServicesSub dispatches requests under /api/v1/services/{id}[/sub-resource].
func (h *Handler) handleServicesSub(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/services/")
	// SplitN ensures at most two parts: the service ID and an optional sub-resource.
	parts := strings.SplitN(rest, "/", 2)
	serviceID := parts[0]
	if serviceID == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 {
		switch parts[1] {
		case "request_access":
			h.handleRequestAccess(w, r, serviceID)
		default:
			http.NotFound(w, r)
		}
		return
	}

	// GET /api/v1/services/{id}
	h.handleGetServiceByUID(w, r, serviceID)
}

// handleGetServiceByUID serves GET /api/v1/services/{id}.
//
//	@Summary		Get a service by UID
//	@Description	Returns the service if the caller may access it. Anonymous callers may only fetch public services; private services require a JWT whose groups intersect the service's required groups.
//	@Tags			services
//	@Produce		json
//	@Param			id	path		string	true	"Service UID"
//	@Success		200	{object}	ServiceView
//	@Failure		400	{string}	string	"Service ID is required"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		404	{string}	string	"Service not found"
//	@Failure		405	{string}	string	"Method not allowed"
//	@Security		BearerAuth
//	@Router			/services/{id} [get]
func (h *Handler) handleGetServiceByUID(w http.ResponseWriter, r *http.Request, serviceID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service := h.cache.Get(serviceID)
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	claims, authenticated := h.extractAndValidateJWT(r)
	if !h.canAccessService(service, authenticated, claims) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	pinnedUIDs := h.callerPinnedUIDs(claims, authenticated)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(toServiceView(service, pinnedUIDs[serviceID])); err != nil {
		log.Error(err, "Failed to encode service")
	}
}

// RequestAccessBody is the optional JSON body for POST /api/v1/services/{id}/request_access.
type RequestAccessBody struct {
	Message string `json:"message,omitempty"` // optional free-text note from the user
}

// handleRequestAccess serves POST /api/v1/services/{id}/request_access.
// Requires authentication. Returns 202 Accepted on success, 409 Conflict when
// a pending request already exists, 501 when the access-request store is not configured.
//
//	@Summary		Request access to a private service
//	@Description	Requires authentication. Creates a pending access request for the caller against the named service; an admin then approves or denies via /admin/access-requests/{id}/{approve|deny}.
//	@Tags			services
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Service UID"
//	@Param			body	body		RequestAccessBody	false	"Optional message from the requester"
//	@Success		202		{object}	accessrequests.AccessRequest
//	@Failure		400		{string}	string	"Bad request"
//	@Failure		401		{string}	string	"Unauthorized"
//	@Failure		404		{string}	string	"Service not found"
//	@Failure		405		{string}	string	"Method not allowed"
//	@Failure		409		{string}	string	"Pending request already exists"
//	@Failure		501		{string}	string	"Access request feature not configured"
//	@Security		BearerAuth
//	@Router			/services/{id}/request_access [post]
func (h *Handler) handleRequestAccess(w http.ResponseWriter, r *http.Request, serviceID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.accessRequestStore == nil {
		http.Error(w, "Access request feature not configured", http.StatusNotImplemented)
		return
	}

	claims, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	service := h.cache.Get(serviceID)
	if service == nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	var body RequestAccessBody
	// Body is optional — decode only when Content-Type is JSON and body is non-empty.
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

	username := claims.PreferredUsername
	if username == "" {
		username = claims.Subject
	}

	req, err := h.accessRequestStore.Create(
		service.UID,
		service.Name,
		username,
		claims.Email,
		body.Message,
	)
	if err != nil {
		if errors.Is(err, accessrequests.ErrDuplicatePending) {
			http.Error(w, "A pending access request already exists for this service", http.StatusConflict)
			return
		}
		log.Error(err, "Failed to create access request", "user", username, "serviceUID", service.UID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Info("Access request created", "id", req.ID, "user", username, "service", service.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(req); err != nil {
		log.Error(err, "Failed to encode access request response")
	}
}

// handleGetNotifications serves GET /api/v1/notifications.
// Returns notifications with per-caller read state when a notification store is
// configured; returns an empty list when the store is not configured.
//
//	@Summary		List notifications
//	@Description	Returns all platform notifications. When the caller is authenticated, each item's "read" flag reflects that user's per-notification read state; for anonymous callers all items are reported as unread.
//	@Tags			notifications
//	@Produce		json
//	@Success		200	{array}		NotificationItem
//	@Failure		405	{string}	string	"Method not allowed"
//	@Failure		500	{string}	string	"Internal server error"
//	@Security		BearerAuth
//	@Router			/notifications [get]
func (h *Handler) handleGetNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var items []NotificationItem

	if h.notificationStore != nil {
		notifs, err := h.notificationStore.List()
		if err != nil {
			log.Error(err, "Failed to list notifications")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Overlay per-user read state when the caller is authenticated.
		var readSet map[string]bool
		claims, ok := h.requireAuth(w, r)
		if ok && claims.PreferredUsername != "_anonymous" {
			readSet, _ = h.notificationStore.ReadSet(claims.PreferredUsername)
		}
		if readSet == nil {
			readSet = map[string]bool{}
		}

		items = make([]NotificationItem, 0, len(notifs))
		for _, n := range notifs {
			items = append(items, NotificationItem{
				ID:        n.ID,
				Image:     n.Image,
				Title:     n.Title,
				Message:   n.Message,
				Read:      readSet[n.ID],
				CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	} else {
		items = []NotificationItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"notifications": items,
	}); err != nil {
		log.Error(err, "Failed to encode notifications")
	}
}

// handleNotificationSub dispatches sub-routes under /api/v1/notifications/.
// Currently handles: PUT /api/v1/notifications/{id}/read
//
//	@Summary		Mark a notification as read
//	@Description	Marks the specified notification as read for the calling user. Idempotent — returns 204 even if already read.
//	@Tags			notifications
//	@Param			id	path	string	true	"Notification ID"
//	@Success		204
//	@Failure		400	{string}	string	"Bad request"
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		404	{string}	string	"Notification not found"
//	@Failure		405	{string}	string	"Method not allowed"
//	@Failure		501	{string}	string	"Notifications feature not configured"
//	@Security		BearerAuth
//	@Router			/notifications/{id}/read [put]
func (h *Handler) handleNotificationSub(w http.ResponseWriter, r *http.Request) {
	if h.notificationStore == nil {
		http.Error(w, "Notifications feature not configured", http.StatusNotImplemented)
		return
	}

	// Expect path: /api/v1/notifications/{id}/read
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/notifications/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		http.Error(w, "Invalid path: expected /api/v1/notifications/{id}/read", http.StatusBadRequest)
		return
	}
	notifID, action := parts[0], parts[1]

	if action != "read" {
		http.Error(w, "Unknown action — only 'read' is supported", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	userID := claims.PreferredUsername

	if err := h.notificationStore.MarkRead(userID, notifID); err != nil {
		if errors.Is(err, notifications.ErrNotFound) {
			http.Error(w, "Notification not found", http.StatusNotFound)
			return
		}
		log.Error(err, "Failed to mark notification as read", "user", userID, "notifID", notifID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Admin access-request handlers ---

// isAdmin returns true when the caller's JWT groups contain h.adminGroup.
func (h *Handler) isAdmin(claims *auth.Claims) bool {
	for _, g := range claims.Groups {
		if g == h.adminGroup {
			return true
		}
	}
	return false
}

// requireAdmin validates the JWT and checks admin-group membership.
// Writes an appropriate error and returns ok=false on failure.
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := h.requireAuth(w, r)
	if !ok {
		return nil, false
	}
	if !h.isAdmin(claims) {
		http.Error(w, "Forbidden: admin group required", http.StatusForbidden)
		return nil, false
	}
	return claims, true
}

// handleAdminListAccessRequests serves GET /api/v1/admin/access-requests.
// Accepts an optional ?status=pending|approved|denied query parameter.
//
//	@Summary		List access requests (admin)
//	@Description	Returns access requests across the cluster. Admin-only — caller's JWT must include the configured admin group.
//	@Tags			admin
//	@Produce		json
//	@Param			status	query		string	false	"Filter by status: pending, approved, or denied"
//	@Success		200		{array}		accessrequests.AccessRequest
//	@Failure		401		{string}	string	"Unauthorized"
//	@Failure		403		{string}	string	"Forbidden: admin group required"
//	@Failure		405		{string}	string	"Method not allowed"
//	@Failure		501		{string}	string	"Access request feature not configured"
//	@Security		BearerAuth
//	@Router			/admin/access-requests [get]
func (h *Handler) handleAdminListAccessRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.accessRequestStore == nil {
		http.Error(w, "Access request feature not configured", http.StatusNotImplemented)
		return
	}
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	var reqs []*accessrequests.AccessRequest
	var err error
	switch r.URL.Query().Get("status") {
	case "pending":
		reqs, err = h.accessRequestStore.ListPending()
	default:
		reqs, err = h.accessRequestStore.ListAll()
	}
	if err != nil {
		log.Error(err, "Failed to list access requests")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if reqs == nil {
		reqs = []*accessrequests.AccessRequest{}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"accessRequests": reqs}); err != nil {
		log.Error(err, "Failed to encode access requests")
	}
}

// handleAdminAccessRequestSub dispatches /api/v1/admin/access-requests/{id}/{action}.
//
//	@Summary		Approve or deny an access request (admin)
//	@Description	Action is encoded in the URL: PUT .../{id}/approve or .../{id}/deny. Approving also adds the user to the service's required Keycloak groups when a Keycloak admin client is configured.
//	@Tags			admin
//	@Produce		json
//	@Param			id		path		string	true	"Access request ID"
//	@Param			action	path		string	true	"approve | deny"
//	@Success		200		{object}	accessrequests.AccessRequest
//	@Failure		400		{string}	string	"Bad request"
//	@Failure		401		{string}	string	"Unauthorized"
//	@Failure		403		{string}	string	"Forbidden: admin group required"
//	@Failure		404		{string}	string	"Request not found"
//	@Failure		405		{string}	string	"Method not allowed"
//	@Failure		501		{string}	string	"Access request feature not configured"
//	@Security		BearerAuth
//	@Router			/admin/access-requests/{id}/{action} [put]
func (h *Handler) handleAdminAccessRequestSub(w http.ResponseWriter, r *http.Request) {
	if h.accessRequestStore == nil {
		http.Error(w, "Access request feature not configured", http.StatusNotImplemented)
		return
	}
	claims, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/access-requests/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "Path must be /api/v1/admin/access-requests/{id}/approve|deny", http.StatusBadRequest)
		return
	}
	id, action := parts[0], parts[1]

	adminUser := claims.PreferredUsername
	if adminUser == "" {
		adminUser = claims.Subject
	}

	var status accessrequests.Status
	switch action {
	case "approve":
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status = accessrequests.StatusApproved
	case "deny":
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status = accessrequests.StatusDenied
	default:
		http.NotFound(w, r)
		return
	}

	updated, err := h.accessRequestStore.UpdateStatus(id, status, adminUser)
	if err != nil {
		if errors.Is(err, accessrequests.ErrNotFound) {
			http.Error(w, "Access request not found", http.StatusNotFound)
			return
		}
		log.Error(err, "Failed to update access request", "id", id, "action", action)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Phase 2: when the request is approved, add the user to every Keycloak group
	// that the service requires. This makes the user visible in private services
	// immediately after approval without any manual Keycloak intervention.
	if status == accessrequests.StatusApproved {
		h.applyKeycloakGroupMembership(r.Context(), updated)
	}
	log.Info("Access request updated", "id", id, "status", status, "by", adminUser)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		log.Error(err, "Failed to encode access request")
	}
}

// applyKeycloakGroupMembership adds the approved user to every Keycloak group
// that the target service requires. It is called fire-and-forget style: errors
// are logged but do not affect the HTTP response (the store record was already
// updated successfully). When the keycloak client is not configured, a warning
// is logged reminding operators to set KEYCLOAK_ADMIN_USERNAME/PASSWORD.
func (h *Handler) applyKeycloakGroupMembership(ctx context.Context, req *accessrequests.AccessRequest) {
	if h.keycloakClient == nil {
		log.Info("Keycloak admin client not configured — skipping group membership update "+
			"(set KEYCLOAK_ADMIN_USERNAME and KEYCLOAK_ADMIN_PASSWORD to enable)",
			"user", req.UserID, "service", req.ServiceName)
		return
	}

	// Look up the service to discover its requiredGroups.
	service := h.cache.Get(req.ServiceUID)
	if service == nil {
		log.Info("Service no longer in cache — cannot determine required groups for Keycloak",
			"serviceUID", req.ServiceUID, "user", req.UserID)
		return
	}

	if len(service.RequiredGroups) == 0 {
		log.Info("Service has no requiredGroups — no Keycloak group update needed",
			"service", service.Name, "user", req.UserID)
		return
	}

	for _, groupName := range service.RequiredGroups {
		if err := h.keycloakClient.AddUserToGroup(ctx, req.UserID, groupName); err != nil {
			log.Error(err, "Failed to add user to Keycloak group — access request approved in store but group membership NOT updated",
				"user", req.UserID, "group", groupName, "service", service.Name)
		}
	}
}

// CreateNotificationBody is the request body for POST /api/v1/admin/notifications.
type CreateNotificationBody struct {
	Image   string `json:"image,omitempty"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// handleAdminCreateNotification serves POST /api/v1/admin/notifications.
// Creates a new platform-wide notification. Requires admin group membership.
//
//	@Summary		Create a notification (admin)
//	@Description	Creates a new platform-wide notification visible to all users via GET /notifications. Admin-only.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			body	body		CreateNotificationBody	true	"Notification payload"
//	@Success		201		{object}	notifications.Notification
//	@Failure		400		{string}	string	"Bad request"
//	@Failure		401		{string}	string	"Unauthorized"
//	@Failure		403		{string}	string	"Forbidden: admin group required"
//	@Failure		405		{string}	string	"Method not allowed"
//	@Failure		501		{string}	string	"Notifications feature not configured"
//	@Security		BearerAuth
//	@Router			/admin/notifications [post]
func (h *Handler) handleAdminCreateNotification(w http.ResponseWriter, r *http.Request) {
	if h.notificationStore == nil {
		http.Error(w, "Notifications feature not configured", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	claims, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	var body CreateNotificationBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if body.Title == "" || body.Message == "" {
		http.Error(w, "title and message are required", http.StatusBadRequest)
		return
	}

	n, err := h.notificationStore.Create(body.Image, body.Title, body.Message)
	if err != nil {
		log.Error(err, "Failed to create notification", "admin", claims.PreferredUsername)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Info("Notification created", "id", n.ID, "title", n.Title, "by", claims.PreferredUsername)
	if h.hub != nil {
		h.hub.PublishNotification(n)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(n); err != nil {
		log.Error(err, "Failed to encode notification")
	}
}

// handleGetCategories serves GET /api/v1/categories.
//
//	@Summary		List service categories
//	@Description	Returns the distinct set of category labels currently advertised by services in the cache. Public — no auth required.
//	@Tags			services
//	@Produce		json
//	@Success		200	{object}	map[string][]string
//	@Failure		405	{string}	string	"Method not allowed"
//	@Router			/categories [get]
func (h *Handler) handleGetCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	categories := h.cache.GetCategories()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"categories": categories,
	}); err != nil {
		log.Error(err, "Failed to encode categories")
	}
}

// HealthResponse is the response body for GET /api/v1/health.
type HealthResponse struct {
	Status string `json:"status" example:"healthy"`
}

// handleHealth serves GET /api/v1/health.
//
//	@Summary		Liveness probe
//	@Description	Returns {"status":"healthy"} as long as the server is up. Does not check downstream dependencies.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	HealthResponse
//	@Router			/health [get]
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	}); err != nil {
		log.Error(err, "Failed to encode health response")
	}
}

func (h *Handler) extractAndValidateJWT(r *http.Request) (*auth.Claims, bool) {
	// Test/debug hook: use the injected extractor when present.
	if h.claimsExtractor != nil {
		return h.claimsExtractor(r)
	}

	if !h.enableAuth || h.jwtValidator == nil {
		if h.debugMode {
			log.Info("[debug] JWT extraction skipped",
				"enableAuth", h.enableAuth,
				"validatorConfigured", h.jwtValidator != nil)
		}
		return nil, false
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		if h.debugMode {
			log.Info("[debug] No Authorization header on request", "path", r.URL.Path)
		}
		return nil, false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		log.Info("Invalid Authorization header format",
			"scheme", parts[0], "path", r.URL.Path)
		return nil, false
	}

	tokenString := parts[1]

	claims, err := h.jwtValidator.ValidateToken(tokenString)
	if err != nil {
		log.Info("JWT validation failed",
			"error", err.Error(),
			"path", r.URL.Path,
			"hint", "check KEYCLOAK_ISSUER_URL matches the 'iss' claim in the token")
		return nil, false
	}

	if h.debugMode {
		log.Info("[debug] JWT validated",
			"user", claims.PreferredUsername,
			"groups", claims.Groups,
			"issuer", claims.Issuer)
	}
	return claims, true
}

func (h *Handler) canAccessService(service *cache.ServiceInfo, authenticated bool, claims *auth.Claims) bool {
	switch service.Visibility {
	case "public":
		return true

	case "private":
		// "private" requires authentication. When requiredGroups is empty the
		// service is visible to any authenticated user; when groups are listed
		// the caller must be a member of at least one of them.
		if !authenticated {
			return false
		}
		return h.hasRequiredGroups(claims.Groups, service.RequiredGroups)

	default:
		// Unknown / legacy visibility values default to private semantics.
		return authenticated && h.hasRequiredGroups(claims.Groups, service.RequiredGroups)
	}
}

func (h *Handler) hasRequiredGroups(userGroups, requiredGroups []string) bool {
	if len(requiredGroups) == 0 {
		return true
	}

	for _, required := range requiredGroups {
		for _, userGroup := range userGroups {
			if userGroup == required {
				return true
			}
		}
	}

	return false
}

// handleCallerIdentity serves GET /api/v1/caller-identity.
// Returns the identity of the caller as decoded from the JWT.
// When no valid token is present, returns {"authenticated": false}.
//
//	@Summary		Get the caller's identity
//	@Description	Echoes the resolved JWT claims for the caller. Returns {"authenticated": false} when no valid token is present.
//	@Tags			identity
//	@Produce		json
//	@Success		200	{object}	CallerIdentityResponse
//	@Failure		405	{string}	string	"Method not allowed"
//	@Security		BearerAuth
//	@Router			/caller-identity [get]
func (h *Handler) handleCallerIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, authenticated := h.extractAndValidateJWT(r)

	var resp CallerIdentityResponse
	if authenticated {
		username := claims.PreferredUsername
		if username == "" {
			username = claims.Subject
		}
		resp = CallerIdentityResponse{
			Authenticated: true,
			Username:      username,
			Email:         claims.Email,
			Name:          claims.Name,
			Groups:        claims.Groups,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error(err, "Failed to encode caller-identity response")
	}
}

// PinsResponse is the response body for GET /api/v1/pins.
type PinsResponse struct {
	// Pins is the ordered list of pinned services (cached ServiceInfo snapshots).
	Pins []*cache.ServiceInfo `json:"pins"`
	// UIDs lists exactly which UIDs are stored, including those that are no longer
	// cached (e.g. the NebariApp was deleted).
	UIDs []string `json:"uids"`
}

// handleGetPins serves GET /api/v1/pins.
// Requires a valid JWT. Returns the caller's pinned services as full ServiceInfo
// objects, resolved from the live cache. Pins whose UIDs are no longer in the
// cache are included in UIDs but absent from Pins (graceful stale handling).
//
//	@Summary		List the caller's pins
//	@Description	Returns the caller's pinned services. UIDs is the raw stored list; Pins is the subset still resolvable in the live cache (so deleted services are gracefully filtered out).
//	@Tags			pins
//	@Produce		json
//	@Success		200	{object}	PinsResponse
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		405	{string}	string	"Method not allowed"
//	@Failure		500	{string}	string	"Internal server error"
//	@Failure		501	{string}	string	"Pins feature not configured"
//	@Security		BearerAuth
//	@Router			/pins [get]
func (h *Handler) handleGetPins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.pinStore == nil {
		http.Error(w, "Pins feature not configured", http.StatusNotImplemented)
		return
	}
	claims, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	uids, err := h.pinStore.Get(claims.PreferredUsername)
	if err != nil {
		log.Error(err, "Failed to read pins", "user", claims.PreferredUsername)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	svcs := make([]*cache.ServiceInfo, 0, len(uids))
	for _, uid := range uids {
		if svc := h.cache.Get(uid); svc != nil {
			svcs = append(svcs, svc)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(PinsResponse{Pins: svcs, UIDs: uids}); err != nil {
		log.Error(err, "Failed to encode pins response")
	}
}

// handlePinByUID serves PUT and DELETE /api/v1/pins/{uid}.
// PUT pins the service; DELETE unpins it. Both are idempotent.
// The {uid} segment is the NebariApp UID (UIDType string from status.serviceDiscovery).
//
//	@Summary		Pin or unpin a service
//	@Description	PUT pins the service; DELETE unpins. Both operations are idempotent. The UID is the NebariApp UID exposed at status.serviceDiscovery.
//	@Tags			pins
//	@Param			uid	path	string	true	"NebariApp UID"
//	@Success		204
//	@Failure		400	{string}	string	"UID is required"
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		405	{string}	string	"Method not allowed"
//	@Failure		500	{string}	string	"Internal server error"
//	@Failure		501	{string}	string	"Pins feature not configured"
//	@Security		BearerAuth
//	@Router			/pins/{uid} [put]
//	@Router			/pins/{uid} [delete]
func (h *Handler) handlePinByUID(w http.ResponseWriter, r *http.Request) {
	if h.pinStore == nil {
		http.Error(w, "Pins feature not configured", http.StatusNotImplemented)
		return
	}
	claims, ok := h.requireAuth(w, r)
	if !ok {
		return
	}
	uid := strings.TrimPrefix(r.URL.Path, "/api/v1/pins/")
	if uid == "" {
		http.Error(w, "UID is required: /api/v1/pins/{uid}", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		if err := h.pinStore.Pin(claims.PreferredUsername, uid); err != nil {
			log.Error(err, "Failed to pin service", "user", claims.PreferredUsername, "uid", uid)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := h.pinStore.Unpin(claims.PreferredUsername, uid); err != nil {
			log.Error(err, "Failed to unpin service", "user", claims.PreferredUsername, "uid", uid)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// requireAuth validates the JWT on r and returns the claims.
// On failure it writes an appropriate HTTP error and returns ok=false.
func (h *Handler) requireAuth(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	// Test/debug hook: claimsExtractor is a full replacement for JWT validation.
	// Check it before the enableAuth short-circuit so WithClaimsExtractor always
	// works in tests regardless of how enableAuth is set.
	if h.claimsExtractor != nil {
		claims, ok := h.claimsExtractor(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return nil, false
		}
		if claims.PreferredUsername == "" {
			claims.PreferredUsername = claims.Subject
		}
		return claims, true
	}

	if !h.enableAuth || h.jwtValidator == nil {
		// Auth disabled globally — return a synthetic claims with empty username
		// so that pin operations still work in dev/test mode (all stored under "").
		return &auth.Claims{PreferredUsername: "_anonymous"}, true
	}
	claims, ok := h.extractAndValidateJWT(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	// Identify the user by preferred_username; fall back to the JWT Subject (sub)
	// for Keycloak configurations that omit preferred_username from access tokens.
	if claims.PreferredUsername == "" {
		claims.PreferredUsername = claims.Subject
	}
	if claims.PreferredUsername == "" {
		http.Error(w, "JWT missing user identity claim (preferred_username or sub)", http.StatusUnauthorized)
		return nil, false
	}
	return claims, true
}

// DebugRequestInfo is the request section of the debug response.
type DebugRequestInfo struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
}

// DebugAuthInfo is the auth section of the debug response.
type DebugAuthInfo struct {
	Enabled             bool     `json:"enabled"`
	ValidatorConfigured bool     `json:"validator_configured"`
	Authenticated       bool     `json:"authenticated"`
	Username            string   `json:"username,omitempty"`
	Email               string   `json:"email,omitempty"`
	Groups              []string `json:"groups,omitempty"`
	ValidationError     string   `json:"validation_error,omitempty"`
}

// DebugServiceCounts is the service-cache section of the debug response.
type DebugServiceCounts struct {
	Total   int `json:"total_in_cache"`
	Public  int `json:"visible_public"`
	Private int `json:"visible_private"`
	Hidden  int `json:"hidden_to_caller"`
}

// DebugResponse is the full body returned by GET /api/v1/debug.
type DebugResponse struct {
	Request  DebugRequestInfo   `json:"request"`
	Auth     DebugAuthInfo      `json:"auth"`
	Services DebugServiceCounts `json:"services"`
}

// handleDebug serves GET /api/v1/debug.
// Only registered when the handler is created with WithDebugMode().
// Returns request headers (with Bearer tokens redacted), resolved JWT claims,
// and per-visibility service counts — similar to Keycloak's KC_HOSTNAME_DEBUG
// page, but as JSON so it's queryable from scripts and kubectl port-forward.
func (h *Handler) handleDebug(w http.ResponseWriter, r *http.Request) {
	// Sanitise headers: redact Bearer tokens, keep everything else.
	headers := make(map[string]string, len(r.Header))
	for k, vals := range r.Header {
		v := strings.Join(vals, ", ")
		if strings.EqualFold(k, "Authorization") {
			parts := strings.SplitN(v, " ", 2)
			if len(parts) == 2 {
				v = fmt.Sprintf("%s [REDACTED, %d bytes]", parts[0], len(parts[1]))
			}
		}
		headers[k] = v
	}

	// Resolve auth state.
	// When a claimsExtractor is configured (test / dev injection) or when the full
	// JWT validator is available, extractAndValidateJWT already handles everything.
	// For the debug response we also want to surface the validation *error* when
	// the real validator is present, so we run the two paths in parallel.
	claims, authenticated := h.extractAndValidateJWT(r)

	authInfo := DebugAuthInfo{
		Enabled:             h.enableAuth,
		ValidatorConfigured: h.jwtValidator != nil || h.claimsExtractor != nil,
	}

	if authenticated && claims != nil {
		authInfo.Authenticated = true
		authInfo.Username = claims.PreferredUsername
		authInfo.Email = claims.Email
		authInfo.Groups = claims.Groups
	} else {
		// Provide a human-readable reason why authentication did not succeed.
		switch {
		case !h.enableAuth:
			authInfo.ValidationError = "auth is disabled (ENABLE_AUTH=false)"
		case h.jwtValidator == nil && h.claimsExtractor == nil:
			authInfo.ValidationError = "JWT validator not configured (KEYCLOAK_URL missing?)"
		default:
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				authInfo.ValidationError = "no Authorization header present"
			} else {
				// Re-run validation to surface the actual error message.
				parts := strings.Split(authHeader, " ")
				if len(parts) != 2 || parts[0] != "Bearer" {
					authInfo.ValidationError = "Authorization header is not in 'Bearer <token>' format"
				} else if h.jwtValidator != nil {
					if _, err := h.jwtValidator.ValidateToken(parts[1]); err != nil {
						authInfo.ValidationError = err.Error()
					}
				}
			}
		}
	}

	// Count services per visibility for this caller.
	counts := DebugServiceCounts{}
	for _, svc := range h.cache.GetAll() {
		counts.Total++
		switch svc.Visibility {
		case "public":
			counts.Public++
		default: // "private" and any legacy values
			if h.canAccessService(svc, authenticated, claims) {
				counts.Private++
			} else {
				counts.Hidden++
			}
		}
	}

	resp := DebugResponse{
		Request: DebugRequestInfo{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: headers,
		},
		Auth:     authInfo,
		Services: counts,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error(err, "Failed to encode debug response")
	}
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	allowedSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowedSet[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				if _, ok := allowedSet[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					// Vary: Origin tells caches the response differs by origin
					w.Header().Add("Vary", "Origin")
				}
				// Origin not in whitelist → no ACAO header; browser blocks the response
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
