# Design Document: Nebari Landing Page

**Status:** Draft **Author:** @viniciusdc **Created:** 2026-01-12 **Last Updated:** 2026-02-24

## Update Summary (2026-02-24)

This design has been updated to reflect the new authentication architecture:

- **Client-Side OIDC:** Frontend handles OIDC flow directly using `enforceAtGateway: false`
- **Public Landing Page:** No gateway authentication; users browse public services anonymously
- **User-Initiated Sign-In:** "Sign In" button triggers OIDC flow (not automatic redirect)
- **Group-Based Visibility:** Services use `visibility: private` with `requiredGroups` for access control
- **Keycloak Integration:** Leverages upcoming `keycloakConfig` section in AuthConfig for declarative group management
- **JWT Groups Claim:** Groups appear as flat list `["admin", "users"]` (not `["/admin"]`) for easy filtering

**Key Architecture Changes:**
- Landing page is a **public-facing** React SPA with optional authentication
- Backend validates JWTs and filters services by user's Keycloak groups
- PKCE flow for public client (no client_secret in browser)
- Separate frontend and backend deployments (can be in different repos)

## Table of Contents

1. [Background](#background)
2. [Goals](#goals)
3. [Non-Goals](#non-goals)
4. [Proposed Design](#proposed-design)
5. [Detailed Design](#detailed-design)
6. [API Specification](#api-specification)
7. [Alternatives Considered](#alternatives-considered)
8. [Security Considerations](#security-considerations)
9. [Testing Strategy](#testing-strategy)
10. [Rollout Plan](#rollout-plan)
11. [Open Questions](#open-questions)



## Background

The Nebari ecosystem deploys various services on top of Kubernetes clusters. Currently, there is no centralized way for
users to discover and access these services. Users must know the specific URLs for each service or navigate through
multiple documentation sources.

The nebari-operator already provides a framework for service onboarding via the `NebariApp` Custom Resource, which
handles routing (HTTPRoute), TLS (cert-manager), and SSO (Envoy Gateway). This proposal extends that system to include a
landing page that automatically displays all registered services.

### Current State

- Services are deployed via Helm/ArgoCD
- Each service creates a `NebariApp` CR for routing/TLS/auth configuration
- No unified service discovery or landing page exists
- Users must bookmark individual service URLs

### Problem Statement

Users need a single entry point to:
1. Discover all available services in the platform
2. Access services without memorizing individual URLs
3. See the health status of services at a glance
4. Filter and search through available services



## Goals

1. **Service Discovery**: Provide a central landing page that displays all services registered via `NebariApp` CRs
2. **Real-time Updates**: Automatically update the landing page when services are added, modified, or removed
3. **Health Visibility**: Display health status for services that expose health endpoints
4. **Self-Service**: Services register themselves by including landing page metadata in their `NebariApp` CR
5. **GitOps Compatible**: No manual registration required; everything is declarative via CRDs



## Non-Goals

1. **Service Management**: The landing page is read-only; it does not manage or configure services
2. **Authentication Portal**: This is not a replacement for Keycloak or identity provider UIs
3. **Monitoring Dashboard**: This is not a replacement for Grafana or similar monitoring tools
4. **API Gateway**: The landing page does not proxy requests to services



## Proposed Design

### Architecture Overview

```
┌─────────────────┐     creates      ┌─────────────────┐
│  Helm Chart /   │ ───────────────► │   NebariApp CR  │
│  ArgoCD App     │                  │  (CRD instance) │
└─────────────────┘                  └────────┬────────┘
                                              │
                    ┌─────────────────────────┼─────────────────────────┐
                    │                         │                         │
                    ▼                         ▼                         ▼
           ┌───────────────┐         ┌───────────────┐         ┌───────────────┐
           │nebari-operator│         │nebari-operator│         │nebari-operator│
           │ (Routing)     │         │ (TLS/Auth)    │         │  (Landing)    │
           └───────────────┘         └───────────────┘         └───────┬───────┘
                    │                         │                         │
                    ▼                         ▼                    Watches CRs
           ┌───────────────┐         ┌───────────────┐                 │
           │  HTTPRoute    │         │  Certificate  │                 │
           │  Gateway      │         │  Keycloak     │◄────────────────┘
           └───────────────┘         │  Client       │
                                     └───────────────┘
                                              │
                    ┌─────────────────────────┴─────────────────────────┐
                    │                                                   │
                    ▼                                                   ▼
           ┌────────────────┐                                  ┌────────────────┐
           │ Go API Server  │                                  │  React SPA     │
           │ (Backend)      │◄─────────HTTP REST/WebSocket────│  (Frontend)    │
           │                │                                  │                │
           │ - Watch CRs    │                                  │ - Public page  │
           │ - JWT validate │                                  │ - Sign in btn  │
           │ - Filter by    │                                  │ - OIDC client  │
           │   groups       │                                  │ - PKCE flow    │
           └────────────────┘                                  └────────┬───────┘
                                                                        │
                                                                        │
                    ┌───────────────────────────────────────────────────┘
                    │ User clicks "Sign In"
                    ▼
           ┌─────────────────┐
           │   Keycloak      │
           │   (OIDC IdP)    │
           │                 │
           │ - Login form    │
           │ - Issues JWT    │
           │ - Groups claim  │
           └─────────────────┘
```

**Data Flow:**

1. **Service Creation:** Admin deploys app → creates NebariApp CR with `landingPage` config
2. **Operator Reconciliation:**
   - Routing reconciler creates HTTPRoute + TLS cert
   - Auth reconciler creates Keycloak client (if `auth.enabled: true`)
3. **Landing Page Backend:**
   - Watches NebariApp CRs via Kubernetes informer
   - Maintains in-memory cache of services
   - Validates JWT tokens from frontend requests
4. **Frontend (Unauthenticated):**
   - Loads public landing page
   - Shows `visibility: "public"` services only
   - Displays "Sign In" button
5. **Frontend (User Auth Flow):**
   - User clicks "Sign In"
   - OIDC PKCE flow redirects to Keycloak
   - User authenticates
   - Keycloak redirects back with auth code
   - Frontend exchanges code for JWT (access_token)
6. **Frontend (Authenticated):**
   - Stores JWT in sessionStorage
   - Makes API calls with `Authorization: Bearer <token>`
   - Backend validates JWT, extracts groups claim
   - Backend returns filtered services based on visibility + groups
   - Frontend displays personalized service list

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Deployment Model | Separate from operator | Independent scaling, clearer separation of concerns, can restart without affecting operator |
| Frontend Stack | React 19 SPA with TypeScript | Rich interactivity, type safety, large ecosystem |
| UI Components | react-uswds (USWDS 3.0) | Section 508 accessibility, consistent design, pre-built accessible components |
| Real-time Updates | WebSocket | Low latency updates without polling, efficient for multiple clients |
| Health Checks | Server-side | Centralized checking avoids CORS issues, reduces client load |
| Data Source | Watch NebariApp CRs | Native Kubernetes pattern, no additional data store needed |

### Component Summary

1. **NebariApp CRD Extension**: Add `landingPage` field to existing NebariApp spec
2. **Go API Server**: Watches NebariApp resources, performs health checks, serves REST API
3. **React Frontend**: SPA that displays services with filtering, search, and real-time updates
4. **Kubernetes Manifests**: Deployment, Service, RBAC, and self-registration NebariApp



## Detailed Design

### 1. NebariApp CRD Extension

Extend the `NebariApp` spec with landing page metadata:

```yaml
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: jupyterhub
  namespace: nebari
spec:
  # Existing fields (routing, TLS, auth)
  hostname: jupyterhub.example.com
  tls:
    enabled: true
  auth:
    enabled: true

  # NEW: Landing page configuration
  landingPage:
    enabled: true                    # Whether to show on landing page (default: false)
    displayName: "JupyterHub"        # Human-readable name (required if enabled)
    description: "Interactive computing environment for data science"
    icon: "jupyter"                  # Icon identifier (see Icon System below)
    category: "Development"          # Grouping category
    priority: 10                     # Sort order within category (lower = higher)
    externalUrl: ""                  # Override URL (default: derived from hostname)
    # NOTE: visibility and requiredGroups are computed from spec.auth
    # - auth.enabled=false → visibility="public"
    # - auth.enabled=true, auth.groups=[] → visibility="private" (any authenticated user)
    # - auth.enabled=true, auth.groups=[...] → visibility="private", requiredGroups=auth.groups
    healthCheck:
      enabled: true                  # Enable health checking (default: false)
      path: "/health"                # Health endpoint path
      intervalSeconds: 30            # Check interval (default: 30, min: 10, max: 300)
      timeoutSeconds: 5              # Request timeout (default: 5)
```

**Go Type Definition:**

```go
// LandingPageConfig defines how a service appears on the landing page
type LandingPageConfig struct {
    // Enabled determines if this service appears on the landing page
    // +kubebuilder:default=false
    Enabled bool `json:"enabled"`

    // DisplayName is the human-readable name shown on the landing page
    // +kubebuilder:validation:MaxLength=64
    DisplayName string `json:"displayName"`

    // Description provides additional context about the service
    // +kubebuilder:validation:MaxLength=256
    // +optional
    Description string `json:"description,omitempty"`

    // Icon is an identifier for the service icon (e.g., "jupyter", "grafana")
    // or a URL to a custom icon
    // +optional
    Icon string `json:"icon,omitempty"`

    // Category groups related services together
    // +optional
    Category string `json:"category,omitempty"`

    // Priority determines sort order within a category (lower = higher priority)
    // +kubebuilder:default=100
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Maximum=1000
    // +optional
    Priority int `json:"priority,omitempty"`

    // ExternalUrl overrides the default URL derived from hostname
    // +optional
    ExternalUrl string `json:"externalUrl,omitempty"`

    // Visibility controls who can see this service on the landing page
    // - "public": Visible to everyone (unauthenticated users included)
    // - "authenticated": Visible to any authenticated user (default)
    // - "private": Visible only to users in requiredGroups
    // +kubebuilder:validation:Enum=public;authenticated;private
    // +kubebuilder:default=authenticated
    // +optional
    Visibility string `json:"visibility,omitempty"`

    // RequiredGroups specifies Keycloak groups required to see/access this service.
    // Only used when visibility is "private".
    // Groups are checked from the user's JWT claims (groups field).
    // Example: ["data-science", "admin"]
    // +optional
    RequiredGroups []string `json:"requiredGroups,omitempty"`

    // HealthCheck configures health status monitoring
    // +optional
    HealthCheck *HealthCheckConfig `json:"healthCheck,omitempty"`
}

// HealthCheckConfig defines health check parameters
type HealthCheckConfig struct {
    // Enabled determines if health checks are performed
    // +kubebuilder:default=false
    Enabled bool `json:"enabled"`

    // Path is the HTTP path to check (e.g., "/health", "/healthz")
    // +kubebuilder:default="/health"
    Path string `json:"path"`

    // IntervalSeconds is how often to check health
    // +kubebuilder:default=30
    // +kubebuilder:validation:Minimum=10
    // +kubebuilder:validation:Maximum=300
    IntervalSeconds int `json:"intervalSeconds,omitempty"`

    // TimeoutSeconds is the request timeout
    // +kubebuilder:default=5
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:validation:Maximum=30
    TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}
```

### 2. Go API Server

**Location:** `cmd/landingpage/` and `internal/landingpage/`

**Responsibilities:**
- Watch NebariApp resources across all namespaces
- Maintain in-memory cache of services with landing page enabled
- Perform periodic health checks for services with health checking enabled
- Serve REST API for the React frontend
- Serve the built React SPA static files
- Broadcast updates via WebSocket

**API Endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/services` | List all registered services |
| GET | `/api/v1/services/{namespace}/{name}` | Get single service details |
| GET | `/api/v1/categories` | List unique categories |
| GET | `/api/v1/health` | Landing page health check |
| WS | `/api/v1/ws` | WebSocket for real-time updates |
| GET | `/*` | Serve React SPA (catch-all) |

**Key Components:**

```
internal/landingpage/
├── server.go           # HTTP server setup, routing
├── api/
│   ├── handlers.go     # REST endpoint handlers
│   └── types.go        # API request/response types
├── watcher/
│   └── watcher.go      # NebariApp informer, cache management
├── health/
│   └── checker.go      # Health check scheduler and executor
└── websocket/
    └── hub.go          # WebSocket connection management, broadcasting
```

**Kubernetes Informer Pattern:**

The API server uses the standard client-go informer pattern for data synchronization:

1. **On Startup (LIST):** The informer performs a LIST operation to fetch all existing `NebariApp` resources across all
   namespaces, populating the in-memory cache with services that have `landingPage.enabled: true`.

2. **Ongoing (WATCH):** After the initial list, the informer establishes a persistent WATCH connection to the Kubernetes
   API server, receiving real-time events for any `NebariApp` changes.

3. **Event Handling:** The watcher processes three event types:
   - `ADDED` - New NebariApp created; add to cache if landing page enabled
   - `MODIFIED` - NebariApp updated; update cache entry or add/remove based on enabled flag
   - `DELETED` - NebariApp removed; remove from cache

```go
// Simplified informer setup
informer := cache.NewSharedIndexInformer(
    &cache.ListWatch{
        ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
            return client.NebariApps("").List(ctx, opts)  // LIST all namespaces
        },
        WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
            return client.NebariApps("").Watch(ctx, opts)  // WATCH all namespaces
        },
    },
    &v1.NebariApp{},
    resyncPeriod,
    cache.Indexers{},
)

informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc:    s.onNebariAppAdded,
    UpdateFunc: s.onNebariAppUpdated,
    DeleteFunc: s.onNebariAppDeleted,
})
```

This pattern provides:
- **Consistency:** The informer handles reconnection and resyncs automatically
- **Efficiency:** Only deltas are transmitted after the initial list
- **Reliability:** Built-in retry logic and exponential backoff
- **No Polling:** The server reacts to events rather than polling the API

**Health Check Implementation:**

- Health checks run in a dedicated goroutine pool
- Results are cached and updated asynchronously
- Failed checks are retried with exponential backoff
- Status transitions (healthy → unhealthy) are broadcast via WebSocket

**JWT Validation and Group-Based Filtering:**

The backend validates JWTs and filters services based on visibility and user groups:

```go
// internal/landingpage/auth/jwt.go
package auth

import (
    "context"
    "crypto/rsa"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/golang-jwt/jwt/v5"
)

type JWTValidator struct {
    keycloakURL string
    realm       string
    publicKey   *rsa.PublicKey
}

type Claims struct {
    jwt.RegisteredClaims
    Email  string   `json:"email"`
    Name   string   `json:"name"`
    Groups []string `json:"groups"`  // ["data-science", "users"]
}

func NewJWTValidator(keycloakURL, realm string) (*JWTValidator, error) {
    v := &JWTValidator{
        keycloakURL: keycloakURL,
        realm:       realm,
    }

    // Fetch Keycloak public key for JWT verification
    if err := v.fetchPublicKey(); err != nil {
        return nil, err
    }

    return v, nil
}

func (v *JWTValidator) fetchPublicKey() error {
    // GET https://keycloak.example.com/realms/main/protocol/openid-connect/certs
    certsURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs",
        v.keycloakURL, v.realm)

    resp, err := http.Get(certsURL)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    var jwks struct {
        Keys []struct {
            Kty string   `json:"kty"`
            Kid string   `json:"kid"`
            Use string   `json:"use"`
            N   string   `json:"n"`
            E   string   `json:"e"`
        } `json:"keys"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
        return err
    }

    // Parse first RSA key (in production, match by 'kid' from token header)
    // ... RSA public key parsing logic ...

    return nil
}

func (v *JWTValidator) ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        // Verify signing algorithm
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return v.publicKey, nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, fmt.Errorf("invalid token")
}
```

```go
// internal/landingpage/api/handlers.go
package api

import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/nebari-dev/nebari-landing-backend/internal/landingpage/auth"
)

type Handler struct {
    cache        *ServiceCache
    jwtValidator *auth.JWTValidator
}

type ServiceResponse struct {
    Services struct {
        Public        []ServiceInfo `json:"public"`
        Authenticated []ServiceInfo `json:"authenticated"`
        Private       []ServiceInfo `json:"private"`
    } `json:"services"`
    Categories []string   `json:"categories"`
    User       *UserInfo  `json:"user,omitempty"`
}

type UserInfo struct {
    Authenticated bool     `json:"authenticated"`
    Username      string   `json:"username,omitempty"`
    Email         string   `json:"email,omitempty"`
    Groups        []string `json:"groups,omitempty"`
}

func (h *Handler) GetServices(w http.ResponseWriter, r *http.Request) {
    response := ServiceResponse{
        Categories: h.cache.GetCategories(),
    }

    // Extract and validate JWT if present
    claims, authenticated := h.extractAndValidateJWT(r)

    // Get all services from cache
    allServices := h.cache.GetAll()

    // Filter by visibility
    for _, service := range allServices {
        switch service.Visibility {
        case "public":
            response.Services.Public = append(response.Services.Public, service)

        case "authenticated":
            if authenticated {
                response.Services.Authenticated = append(response.Services.Authenticated, service)
            }

        case "private":
            if authenticated && h.hasRequiredGroups(claims.Groups, service.RequiredGroups) {
                response.Services.Private = append(response.Services.Private, service)
            }
        }
    }

    // Include user info if authenticated
    if authenticated {
        response.User = &UserInfo{
            Authenticated: true,
            Username:      claims.Subject,
            Email:         claims.Email,
            Groups:        claims.Groups,
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (h *Handler) extractAndValidateJWT(r *http.Request) (*auth.Claims, bool) {
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return nil, false
    }

    // Extract Bearer token
    parts := strings.Split(authHeader, " ")
    if len(parts) != 2 || parts[0] != "Bearer" {
        return nil, false
    }

    tokenString := parts[1]

    // Validate JWT
    claims, err := h.jwtValidator.ValidateToken(tokenString)
    if err != nil {
        return nil, false
    }

    return claims, true
}

func (h *Handler) hasRequiredGroups(userGroups, requiredGroups []string) bool {
    if len(requiredGroups) == 0 {
        return true  // No groups required
    }

    // User must be in at least one required group (OR logic)
    for _, required := range requiredGroups {
        for _, userGroup := range userGroups {
            if userGroup == required {
                return true
            }
        }
    }

    return false
}
```

```go
// internal/landingpage/cache/service_cache.go
package cache

import (
    "sync"

    appsv1 "github.com/nebari-dev/nebari-operator/api/v1"
)

type ServiceInfo struct {
    UID            string   `json:"uid"`
    Name           string   `json:"name"`
    Namespace      string   `json:"namespace"`
    DisplayName    string   `json:"displayName"`
    Description    string   `json:"description"`
    URL            string   `json:"url"`
    Icon           string   `json:"icon"`
    Category       string   `json:"category"`
    Priority       int      `json:"priority"`
    Visibility     string   `json:"visibility"`
    RequiredGroups []string `json:"requiredGroups,omitempty"`
    Health         *HealthStatus `json:"health,omitempty"`
}

type ServiceCache struct {
    mu       sync.RWMutex
    services map[string]ServiceInfo  // keyed by UID
}

func NewServiceCache() *ServiceCache {
    return &ServiceCache{
        services: make(map[string]ServiceInfo),
    }
}

func (c *ServiceCache) Add(nebariApp *appsv1.NebariApp) {
    if nebariApp.Spec.LandingPage == nil || !nebariApp.Spec.LandingPage.Enabled {
        return
    }

    uid := string(nebariApp.UID)

    service := ServiceInfo{
        UID:         uid,
        Name:        nebariApp.Name,
        Namespace:   nebariApp.Namespace,
        DisplayName: nebariApp.Spec.LandingPage.DisplayName,
        Description: nebariApp.Spec.LandingPage.Description,
        Icon:        nebariApp.Spec.LandingPage.Icon,
        Category:    nebariApp.Spec.LandingPage.Category,
        Priority:    nebariApp.Spec.LandingPage.Priority,
        Visibility:  nebariApp.Spec.LandingPage.Visibility,
        RequiredGroups: nebariApp.Spec.LandingPage.RequiredGroups,
        URL:         buildURL(nebariApp),
    }

    c.mu.Lock()
    defer c.mu.Unlock()
    c.services[uid] = service
}

func (c *ServiceCache) Remove(uid string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.services, uid)
}

func (c *ServiceCache) GetAll() []ServiceInfo {
    c.mu.RLock()
    defer c.mu.RUnlock()

    services := make([]ServiceInfo, 0, len(c.services))
    for _, service := range c.services {
        services = append(services, service)
    }
    return services
}

func buildURL(nebariApp *appsv1.NebariApp) string {
    if nebariApp.Spec.LandingPage.ExternalUrl != "" {
        return nebariApp.Spec.LandingPage.ExternalUrl
    }

    // Default: https://<hostname>
    return "https://" + nebariApp.Spec.Hostname
}
```

### 3. React Frontend

**Location:** `web/`

**Tech Stack:**
- React 19 with TypeScript
- Vite for build tooling
- [react-uswds](https://github.com/trussworks/react-uswds) (USWDS 3.0 components) for UI components and styling
- React Query (TanStack Query) for data fetching and caching
- Native WebSocket API for real-time updates

**Why USWDS:**
- Section 508 accessibility compliance built-in
- Consistent, professional design system
- Pre-built accessible components (Card, Grid, Search, Tag, Alert)
- Well-maintained React implementation by Trussworks

**Component Hierarchy:**

```
App
├── Layout
│   ├── Header (logo, search bar)
│   └── Main
│       ├── CategoryFilter (filter chips)
│       └── ServiceGrid
│           └── ServiceCard (repeated)
│               ├── Icon
│               ├── Title/Description
│               ├── HealthBadge
│               └── Link
```

**Key Features:**

1. **Service Grid**: Responsive grid layout showing service cards
2. **Category Filtering**: Filter services by category (chips/tabs)
3. **Search**: Client-side filtering by name/description
4. **Health Status**: Visual indicators (green/red/gray badges)
5. **Real-time Updates**: Services appear/disappear without page reload
6. **Responsive Design**: Works on desktop and mobile

**State Management:**

```typescript
// React Query handles server state
const { data: services, isLoading } = useQuery({
  queryKey: ['services'],
  queryFn: fetchServices,
});

// WebSocket updates invalidate query cache
useWebSocket('/api/v1/ws', {
  onMessage: (event) => {
    const update = JSON.parse(event.data);
    queryClient.invalidateQueries(['services']);
  },
});
```

**Frontend Authentication Implementation:**

The landing page uses **react-oidc-context** for client-side OIDC with PKCE:

```typescript
// src/config/oidc.ts
import { AuthProviderProps } from 'react-oidc-context';

export const oidcConfig: AuthProviderProps = {
  authority: 'https://keycloak.example.com/realms/main',
  client_id: 'landing-page',
  redirect_uri: window.location.origin + '/callback',
  response_type: 'code',  // Authorization Code Flow
  scope: 'openid profile email groups',  // Include groups claim

  // PKCE for public clients (no client_secret)
  // react-oidc-context enables this automatically for public clients

  // Automatic token refresh
  automaticSilentRenew: true,

  // Store tokens in sessionStorage (more secure than localStorage)
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
};
```

```tsx
// src/App.tsx
import { AuthProvider } from 'react-oidc-context';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { oidcConfig } from './config/oidc';
import { LandingPage } from './pages/LandingPage';

const queryClient = new QueryClient();

function App() {
  return (
    <AuthProvider {...oidcConfig}>
      <QueryClientProvider client={queryClient}>
        <LandingPage />
      </QueryClientProvider>
    </AuthProvider>
  );
}
```

```tsx
// src/components/Navbar.tsx
import { useAuth } from 'react-oidc-context';

export function Navbar() {
  const auth = useAuth();

  if (auth.isLoading) {
    return <div>Loading...</div>;
  }

  return (
    <nav className="usa-nav">
      <Logo />

      {auth.isAuthenticated ? (
        <div className="usa-nav__primary">
          <span>Welcome, {auth.user?.profile.name}</span>
          <button
            className="usa-button usa-button--secondary"
            onClick={() => auth.removeUser()}
          >
            Sign Out
          </button>
        </div>
      ) : (
        <button
          className="usa-button"
          onClick={() => auth.signinRedirect()}
        >
          Sign In
        </button>
      )}
    </nav>
  );
}
```

```tsx
// src/pages/LandingPage.tsx
import { useAuth } from 'react-oidc-context';
import { useServices } from '../hooks/useServices';
import { ServiceGrid } from '../components/ServiceGrid';

export function LandingPage() {
  const auth = useAuth();
  const { data, isLoading } = useServices(
    auth.user?.access_token
  );

  if (isLoading) {
    return <LoadingSpinner />;
  }

  return (
    <main className="usa-section">
      <Navbar />

      {/* Public Services - Always Visible */}
      {data?.services.public.length > 0 && (
        <Section title="Platform Services">
          <ServiceGrid services={data.services.public} />
        </Section>
      )}

      {/* Authenticated Content */}
      {auth.isAuthenticated ? (
        <>
          {data?.services.authenticated.length > 0 && (
            <Section title="Your Services">
              <ServiceGrid services={data.services.authenticated} />
            </Section>
          )}

          {data?.services.private.length > 0 && (
            <Section title="Team Services">
              <ServiceGrid services={data.services.private} />
            </Section>
          )}
        </>
      ) : (
        <div className="usa-alert usa-alert--info">
          <div className="usa-alert__body">
            <h4 className="usa-alert__heading">Sign in to see more</h4>
            <p>
              Authenticate to access personalized services and team resources.
            </p>
            <button
              className="usa-button"
              onClick={() => auth.signinRedirect()}
            >
              Sign In
            </button>
          </div>
        </div>
      )}
    </main>
  );
}
```

```typescript
// src/hooks/useServices.ts
import { useQuery } from '@tanstack/react-query';

export function useServices(accessToken?: string) {
  return useQuery({
    queryKey: ['services', accessToken],
    queryFn: async () => {
      const headers: HeadersInit = {
        'Content-Type': 'application/json',
      };

      // Add Authorization header if authenticated
      if (accessToken) {
        headers['Authorization'] = `Bearer ${accessToken}`;
      }

      const response = await fetch('/api/v1/services', { headers });

      if (!response.ok) {
        throw new Error('Failed to fetch services');
      }

      return response.json();
    },
    // Refetch every 30 seconds to catch new services
    refetchInterval: 30000,
  });
}
```

```tsx
// src/pages/CallbackPage.tsx
import { useEffect } from 'react';
import { useAuth } from 'react-oidc-context';
import { useNavigate } from 'react-router-dom';

export function CallbackPage() {
  const auth = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    // Handle redirect from Keycloak
    if (auth.isAuthenticated) {
      // Redirect to home page after successful authentication
      navigate('/');
    }
  }, [auth.isAuthenticated, navigate]);

  return (
    <div className="usa-section">
      <h1>Completing authentication...</h1>
    </div>
  );
}
```

### 4. Icon System

The landing page supports both built-in icons and custom URLs:

**Built-in Icons** (stored in frontend assets):
- `jupyter` - JupyterHub/JupyterLab
- `grafana` - Grafana
- `prometheus` - Prometheus
- `keycloak` - Keycloak
- `argocd` - ArgoCD
- `default` - Generic application icon

**Custom Icons:**
- Provide a full URL in the `icon` field
- Supports PNG, SVG, or any web-accessible image

### 5. Deployment

**Kubernetes Resources:**

```yaml
# ServiceAccount
apiVersion: v1
kind: ServiceAccount
metadata:
  name: landing-page
  namespace: nebari-operator-system

---
# ClusterRole (read-only access to NebariApp resources)
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: landing-page
rules:
- apiGroups: ["reconcilers.nebari.dev"]
  resources: ["nebariapps"]
  verbs: ["get", "list", "watch"]

---
# Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: landing-page
  namespace: nebari-operator-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: landing-page
  template:
    spec:
      serviceAccountName: landing-page
      containers:
      - name: landing-page
        image: ghcr.io/nebari-dev/nebari-landing-page:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 128Mi
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080

---
# Service
apiVersion: v1
kind: Service
metadata:
  name: landing-page
  namespace: nebari-operator-system
spec:
  selector:
    app: landing-page
  ports:
  - port: 80
    targetPort: 8080
```

**Self-Registration:**

The landing page registers itself via NebariApp with client-side OIDC:

```yaml
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: landing-page
  namespace: nebari-operator-system
spec:
  hostname: nebari.example.com  # Root domain
  service:
    name: landing-page-frontend
    port: 80
  routing:
    tls:
      enabled: true
  auth:
    enabled: true
    enforceAtGateway: false  # Frontend handles OIDC itself (no Envoy auth)
    provider: keycloak
    # Operator provisions Keycloak client but doesn't enforce at gateway
    keycloakConfig:
      clientType: public  # Public client (uses PKCE, no client_secret in browser)
      redirectUris:
        - "https://nebari.example.com/callback"
        - "https://nebari.example.com/*"
      webOrigins:
        - "https://nebari.example.com"
  landingPage:
    enabled: true
    displayName: "Home"
    description: "Service directory and platform portal"
    icon: "home"
    category: "Platform"
    priority: 0
    visibility: "public"  # Landing page itself is public
```



## API Specification

### GET /api/v1/services

Returns services visible to the requesting user. Accepts optional `Authorization: Bearer <token>` header.

**Filtering Logic:**
- **No auth header:** Returns only `visibility: "public"` services
- **Valid JWT:** Returns public + authenticated + private services (if user in requiredGroups)

**Request Headers:**
```
GET /api/v1/services HTTP/1.1
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...  # Optional
```

**Response:**

```json
{
  "services": {
    "public": [
      {
        "uid": "abc-123-def-456",
        "name": "documentation",
        "namespace": "platform",
        "displayName": "Documentation",
        "description": "Platform user guides",
        "url": "https://docs.example.com",
        "icon": "book",
        "category": "Platform",
        "priority": 5,
        "visibility": "public",
        "health": {
          "status": "healthy",
          "lastCheck": "2026-02-24T10:30:00Z",
          "message": null
        }
      }
    ],
    "authenticated": [
      {
        "uid": "xyz-789-ghi-012",
        "name": "grafana",
        "namespace": "monitoring",
        "displayName": "Grafana",
        "description": "Metrics and dashboards",
        "url": "https://grafana.example.com",
        "icon": "grafana",
        "category": "Monitoring",
        "priority": 20,
        "visibility": "private",  // Computed from spec.auth.enabled=true, auth.groups=[]
        "health": {
          "status": "healthy",
          "lastCheck": "2026-02-24T10:31:00Z",
          "message": null
        }
      }
    ],
    "private": [
      {
        "uid": "def-456-jkl-345",
        "name": "jupyterhub",
        "namespace": "data-science",
        "displayName": "JupyterHub",
        "description": "Interactive computing environment",
        "url": "https://jupyterhub.example.com",
        "icon": "jupyter",
        "category": "Development",
        "priority": 10,
        "visibility": "private",
        "requiredGroups": ["data-science", "admin"],
        "health": {
          "status": "healthy",
          "lastCheck": "2026-02-24T10:30:00Z",
          "message": null
        }
      }
    ]
  },
  "categories": ["Development", "Monitoring", "Platform"],
  "user": {
    "authenticated": true,
    "username": "alice",
    "groups": ["data-science", "users"]
  }
}
```

### WebSocket /api/v1/ws

Broadcasts service updates in real-time.

**Message Format:**

```json
{
  "type": "added" | "modified" | "deleted",
  "service": { /* Service object */ }
}
```



## Integration with AuthConfig Enhancements

The landing page design leverages upcoming enhancements to the `AuthConfig` CRD:

### enforceAtGateway Flag

The `enforceAtGateway` field allows applications to handle OIDC authentication client-side while still having the operator provision Keycloak resources:

```yaml
auth:
  enabled: true
  enforceAtGateway: false  # No Envoy SecurityPolicy created
  provider: keycloak
```

**How it works:**
- Operator creates Keycloak client (client ID, client secret, redirect URIs)
- Operator stores credentials in Kubernetes Secret
- Operator does **NOT** create Envoy Gateway SecurityPolicy
- Application (landing page frontend) handles OIDC flow directly
- No automatic redirects to Keycloak at gateway level

**Benefits for Landing Page:**
- Public access without authentication
- User-initiated sign-in via button click
- Full control over OIDC flow (PKCE, scopes, token refresh)
- Can show public services to anonymous users

### keycloakConfig Section

The `keycloakConfig` section enables declarative management of Keycloak resources:

```yaml
auth:
  enabled: true
  enforceAtGateway: false
  keycloakConfig:
    clientType: public  # public (PKCE) or confidential (client_secret)
    redirectUris:
      - "https://nebari.example.com/callback"
      - "https://nebari.example.com/*"
    webOrigins:
      - "https://nebari.example.com"
    groups:
      - name: "data-science"
        description: "Data science team"
        members: ["alice@example.com", "bob@example.com"]
      - name: "platform-admin"
        description: "Platform administrators"
        members: ["admin@example.com"]
    protocolMappers:
      - name: "group-membership"
        protocol: "openid-connect"
        protocolMapper: "oidc-group-membership-mapper"
        config:
          "full.path": "false"  # Groups as ["admin"] not ["/admin"]
          "claim.name": "groups"
```

**Landing Page Usage:**
- Operator creates Keycloak groups for each service with `requiredGroups`
- Groups appear in JWT as flat list: `["data-science", "users"]`
- Backend filters services based on JWT groups claim
- No additional Keycloak API calls needed at runtime

### Group-Based Access Control Flow

```
1. Service App defines requiredGroups
   ─────────────────────────────────────
   apiVersion: reconcilers.nebari.dev/v1
   kind: NebariApp
   metadata:
     name: jupyterhub
   spec:
     landingPage:
       enabled: true
       visibility: "private"
       requiredGroups: ["data-science"]

2. Operator syncs to Keycloak (future enhancement)
   ─────────────────────────────────────
   - Creates "data-science" group if not exists
   - Or validates group exists

3. Admin assigns users to groups (via Keycloak UI or keycloakConfig)
   ─────────────────────────────────────
   User alice@example.com → groups: ["data-science", "users"]

4. User authenticates, JWT contains groups claim
   ─────────────────────────────────────
   {
     "sub": "alice@example.com",
     "groups": ["data-science", "users"],
     "email": "alice@example.com"
   }

5. Backend filters services by groups
   ─────────────────────────────────────
   Service: jupyterhub (requiredGroups: ["data-science"])
   User groups: ["data-science", "users"]
   Match: YES → service visible to alice
```

### Public Client with PKCE

For the landing page frontend (browser-based), we use a **public client** with PKCE:

**Why Public Client:**
- No client_secret embedded in JavaScript (security risk)
- PKCE (Proof Key for Code Exchange) prevents authorization code interception
- Standard for SPAs and mobile apps

**PKCE Flow:**
```
1. Frontend generates code_verifier (random string)
2. Frontend creates code_challenge = base64(sha256(code_verifier))
3. Frontend redirects to Keycloak with code_challenge
4. User authenticates
5. Keycloak redirects back with authorization code
6. Frontend exchanges code + code_verifier for tokens
7. Keycloak validates code_verifier matches code_challenge
8. Returns access_token, id_token, refresh_token
```

**Operator Configuration:**
```yaml
keycloakConfig:
  clientType: public
  pkceEnabled: true  # Default for public clients
```

## Alternatives Considered

### 1. Embed Landing Page in Operator

**Pros:**
- Single deployment
- Shared Kubernetes client

**Cons:**
- Couples UI lifecycle to operator
- Harder to scale independently
- Operator restarts affect landing page availability

**Decision:** Rejected - separation of concerns is more important

### 2. Server-Side Rendering (Go templates + htmx)

**Pros:**
- No JavaScript build step
- Simpler deployment
- Lower client-side complexity

**Cons:**
- Less interactive UI
- Harder to implement complex filtering/search
- Team less familiar with htmx

**Decision:** Rejected - React provides better UX for this use case

### 3. Static Site Generator

**Pros:**
- Simple hosting
- No server needed

**Cons:**
- No real-time updates
- Requires regeneration on changes
- Adds complexity to deployment pipeline

**Decision:** Rejected - real-time updates are a key requirement

### 4. ConfigMap-Based Registration

**Pros:**
- No CRD changes needed
- Works without operator

**Cons:**
- Separate from routing/TLS configuration
- Potential for drift between ConfigMap and NebariApp
- Less declarative

**Decision:** Rejected - extending NebariApp provides a unified configuration point



## Security Considerations

### Authentication & Authorization

**Landing Page Access:**
- The landing page frontend is **public** (no gateway enforcement)
- Users can browse without authentication
- "Sign In" button initiates OIDC flow for personalized view

**Service Visibility Access Control:**

Visibility and group access for the landing page are **automatically computed from `spec.auth`** to ensure consistency:

- **`auth.enabled = false`** → Service appears as "public" on landing page (no authentication required)
- **`auth.enabled = true, auth.groups = []`** → Service appears as "private" (any authenticated user can see it)
- **`auth.enabled = true, auth.groups = [...]`** → Service appears as "private" with group restrictions (only users in specified groups can see it)

This ensures landing page visibility exactly matches service access control, eliminating configuration redundancy.

**Group Validation Logic (in webapi):**
```go
// Backend validates groups from JWT claims
func (h *Handler) canAccessService(service *cache.ServiceInfo, authenticated bool, claims *auth.Claims) bool {
    if service.Visibility == "public" {
        return true
    }
    // visibility: "private" (computed from auth)
    if !authenticated {
        return false
    }
    if len(service.RequiredGroups) == 0 {
        return true  // Any authenticated user
    }
    // visibility == "private": check groups
    for _, requiredGroup := range service.RequiredGroups {
        for _, userGroup := range userGroups {
            if userGroup == requiredGroup {
                return true
            }
        }
    }
    return false
}
```

**Keycloak Integration:**
- Operator provisions Keycloak client via `auth.keycloakConfig`
- Groups are managed declaratively via `auth.keycloakConfig.groups`
- JWT `/groups` claim contains user's group memberships (as `["admin", "users"]` not `["/admin"]`)
- Backend validates groups from JWT without additional Keycloak queries

**API Server Permissions:**
- Read-only access to NebariApp resources
- No write operations performed
- No privileged Kubernetes operations

### Network Policies

- Landing page only needs egress to:
  - Kubernetes API server (for watching NebariApps)
  - Service health endpoints (for health checks)
- Ingress only from the gateway

### Health Check Security

- Health checks are performed server-side to avoid exposing internal endpoints
- Only the configured health path is accessed
- Timeouts prevent hanging connections

### Container Security

- Non-root user
- Read-only filesystem
- Dropped capabilities
- Restricted pod security standard



## Testing Strategy

### Unit Tests

- API handler logic
- Health check scheduling
- Service filtering and sorting
- React component rendering

### Integration Tests

- NebariApp watcher correctly updates cache
- WebSocket broadcasts on changes
- API returns correct data format

### End-to-End Tests

```bash
# 1. Deploy landing page to Kind cluster
make deploy-landingpage

# 2. Create test NebariApp
kubectl apply -f - <<EOF
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: test-service
  namespace: default
spec:
  hostname: test.example.com
  landingPage:
    enabled: true
    displayName: "Test Service"
    category: "Testing"
EOF

# 3. Verify service appears in API
curl http://localhost:8080/api/v1/services | jq '.services[] | select(.name=="test-service")'

# 4. Delete NebariApp and verify removal
kubectl delete nebariapp test-service
curl http://localhost:8080/api/v1/services | jq '.services | length'
```



## Rollout Plan

### Phase 1: CRD Extension
1. Add `landingPage` field to NebariApp types
2. Run `make manifests` to regenerate CRD
3. Update existing NebariApp samples with landing page examples
4. Release new CRD version

### Phase 2: Backend Implementation
1. Implement Go API server
2. Add unit and integration tests
3. Create Dockerfile
4. Add CI workflow

### Phase 3: Frontend Implementation
1. Scaffold React application
2. Implement components
3. Add component tests
4. Integrate with backend

### Phase 4: Deployment & Documentation
1. Create Kubernetes manifests
2. Add to Kustomize overlays
3. Write user documentation
4. Create sample NebariApps for common services

### Phase 5: Production Rollout
1. Deploy to staging environment
2. Validate with test services
3. Deploy to production
4. Monitor and iterate



## Open Questions

1. **Branding**: Should the landing page support custom branding (logo, colors, title)?
   - Consider ConfigMap-based theming configuration

2. **Service Grouping**: Should we support nested categories or tags in addition to single category?
   - Tags could enable multi-dimensional filtering (e.g., "python", "GPU", "production")

3. **Favorites**: Should users be able to "favorite" services for quick access?
   - Requires backend storage (ConfigMap per user? Database? Browser localStorage?)

4. **Announcements**: Should the landing page support system-wide announcements/banners?
   - Platform-admin could post maintenance windows, new feature announcements

5. **External Services**: Should we support registering services that don't have NebariApp CRs (e.g., external SaaS tools)?
   - Could add `externalServices` ConfigMap for manual registration

6. **Group Hierarchy**: Should `requiredGroups` support OR vs AND logic?
   - Current: ANY group matches (OR logic)
   - Alternative: ALL groups required (AND logic)
   - Or support expressions: `requiredGroups: ["(admin OR data-science) AND users"]`

7. **User Preferences**: Should authenticated users be able to customize their view?
   - Hide categories, reorder services, change grid/list view
   - Store preferences in browser localStorage or backend ConfigMap



## Appendix

### Example NebariApp Configurations

**Public Service (Documentation):**
```yaml
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: documentation
  namespace: platform
spec:
  hostname: docs.example.com
  service:
    name: docs-service
    port: 80
  routing:
    tls:
      enabled: true
  auth:
    enabled: false  # No authentication required
  landingPage:
    enabled: true
    displayName: "Documentation"
    description: "Platform user guides and API reference"
    icon: "book"
    category: "Platform"
    priority: 1
    visibility: "public"  # Visible to everyone (no auth required)
```

**Authenticated Service (Grafana):**
```yaml
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: grafana
  namespace: monitoring
spec:
  hostname: grafana.example.com
  service:
    name: grafana
    port: 3000
  routing:
    tls:
      enabled: true
  auth:
    enabled: true
    enforceAtGateway: false  # Grafana handles OIDC itself
    provider: keycloak
    keycloakConfig:
      clientType: confidential
      redirectUris:
        - "https://grafana.example.com/login/generic_oauth"
      protocolMappers:
        - name: "group-membership"
          protocol: "openid-connect"
          protocolMapper: "oidc-group-membership-mapper"
          config:
            "full.path": "false"
            "claim.name": "groups"
  landingPage:
    enabled: true
    displayName: "Grafana"
    description: "Metrics, dashboards, and alerting"
    icon: "grafana"
    category: "Monitoring"
    priority: 10
    # visibility computed from spec.auth (auth.enabled=true, auth.groups=[])
    # → visibility="private", requiredGroups=[]
    healthCheck:
      enabled: true
      path: "/api/health"
      intervalSeconds: 30
```

**Private Service (JupyterHub - Data Science Team Only):**
```yaml
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: jupyterhub
  namespace: data-science
spec:
  hostname: jupyter.example.com
  service:
    name: jupyterhub
    port: 8000
  routing:
    tls:
      enabled: true
  auth:
    enabled: true
    enforceAtGateway: true  # Enforce auth at gateway
    provider: keycloak
    keycloakConfig:
      groups:
        - name: "data-science"
          description: "Data science team members"
          members:
            - "alice@example.com"
            - "bob@example.com"
        - name: "data-science-admin"
          description: "Data science administrators"
          members:
            - "admin@example.com"
  landingPage:
    enabled: true
    displayName: "JupyterHub"
    description: "Interactive computing for data science"
    icon: "jupyter"
    category: "Development"
    priority: 5
    visibility: "private"  # Only visible to users in requiredGroups
    requiredGroups:
      - "data-science"
      - "data-science-admin"  # User must be in at least one of these
    healthCheck:
      enabled: true
      path: "/hub/health"
```

**Admin-Only Service (Kubernetes Dashboard):**
```yaml
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: kubernetes-dashboard
  namespace: kube-system
spec:
  hostname: k8s-dashboard.example.com
  service:
    name: kubernetes-dashboard
    port: 443
  routing:
    tls:
      enabled: true
  auth:
    enabled: true
    enforceAtGateway: true
    provider: keycloak
    keycloakConfig:
      groups:
        - name: "platform-admin"
          description: "Platform administrators"
          members:
            - "admin@example.com"
  landingPage:
    enabled: true
    displayName: "Kubernetes Dashboard"
    description: "Cluster management and operations"
    icon: "kubernetes"
    category: "Platform"
    priority: 100  # Lower priority (appears last)
    visibility: "private"
    requiredGroups:
      - "platform-admin"  # Only platform admins can see
    healthCheck:
      enabled: false  # Dashboard doesn't have a health endpoint
```

**Landing Page Itself:**
```yaml
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
metadata:
  name: landing-page
  namespace: nebari-operator-system
spec:
  hostname: nebari.example.com
  service:
    name: landing-page-frontend
    port: 80
  routing:
    tls:
      enabled: true
  auth:
    enabled: true
    enforceAtGateway: false  # Frontend handles OIDC client-side with PKCE
    provider: keycloak
    keycloakConfig:
      clientType: public  # Public client (no client_secret)
      redirectUris:
        - "https://nebari.example.com/callback"
        - "https://nebari.example.com/*"
      webOrigins:
        - "https://nebari.example.com"
  landingPage:
    enabled: true
    displayName: "Home"
    description: "Nebari platform portal"
    icon: "home"
    category: "Platform"
    priority: 0  # Always first
    visibility: "public"  # Landing page is public (shows Sign In button)
```

### File Structure

```
nebari-operator/
├── api/v1/
│   ├── groupversion_info.go
│   ├── nebariapp_types.go           # NebariApp CRD with landingPage field
│   └── zz_generated.deepcopy.go
├── cmd/
│   ├── main.go                      # Operator entry point
│   └── landingpage/
│       └── main.go                  # Landing page entry point
├── internal/
│   ├── controller/
│   │   └── nebariapp_controller.go
│   └── landingpage/
│       ├── server.go
│       ├── api/
│       │   ├── handlers.go
│       │   └── types.go
│       ├── watcher/
│       │   └── watcher.go
│       ├── health/
│       │   └── checker.go
│       └── websocket/
│           └── hub.go
├── web/
│   ├── package.json
│   ├── vite.config.ts
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   ├── api/
│   │   ├── components/
│   │   └── hooks/
│   └── dist/
├── config/
│   ├── crd/bases/
│   │   └── reconcilers.nebari.dev_nebariapps.yaml
│   ├── landingpage/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── serviceaccount.yaml
│   │   ├── rbac.yaml
│   │   └── kustomization.yaml
│   └── samples/
│       └── landingpage-nebariapp.yaml
├── Dockerfile
├── Dockerfile.landingpage
└── Makefile
```

### References

- [nebari-operator README](../../README.md)
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [React 19 Documentation](https://react.dev/)
- [react-uswds (Trussworks)](https://github.com/trussworks/react-uswds)
- [react-uswds Storybook](https://trussworks.github.io/react-uswds/)
- [USWDS Design System](https://designsystem.digital.gov/)
- [USWDS Components](https://designsystem.digital.gov/components/overview/)
- [React Query Documentation](https://tanstack.com/query/latest)
- [Vite Documentation](https://vitejs.dev/)
