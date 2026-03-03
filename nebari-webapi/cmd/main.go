package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/accessrequests"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/api"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/auth"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/cache"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/health"
	webkeycloak "github.com/nebari-dev/nebari-landing/nebari-webapi/internal/keycloak"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/notifications"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/pins"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/watcher"
	wshub "github.com/nebari-dev/nebari-landing/nebari-webapi/internal/websocket"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
}

func main() {
	var (
		port                 int
		keycloakURL          string
		keycloakRealm        string
		enableAuth           bool
		healthInterval       int
		pinsDBPath           string
		accessRequestsDBPath string
		adminGroup           string
		notificationsDBPath  string
	)

	// Flags fall back to environment variables so the binary works naturally when
	// deployed as a Kubernetes Pod using env: in the Deployment manifest.
	// Precedence: CLI flag > environment variable > built-in default.
	flag.IntVar(&port, "port", envInt("PORT", 8080),
		"Port to listen on (env: PORT)")
	// Note: controller-runtime registers --kubeconfig in its own init(); use ctrl.GetConfig() below.
	flag.StringVar(&keycloakURL, "keycloak-url", os.Getenv("KEYCLOAK_URL"),
		"Keycloak base URL for JWK fetching, e.g. http://keycloak-internal:8080/auth (env: KEYCLOAK_URL)")
	flag.StringVar(&keycloakRealm, "keycloak-realm", envStr("KEYCLOAK_REALM", "main"),
		"Keycloak realm name (env: KEYCLOAK_REALM)")
	flag.BoolVar(&enableAuth, "enable-auth", envBool("ENABLE_AUTH", false),
		"Enable JWT authentication and authorization (env: ENABLE_AUTH)")
	flag.IntVar(&healthInterval, "health-interval", envInt("HEALTH_INTERVAL", 30),
		"Health check interval in seconds (env: HEALTH_INTERVAL)")
	flag.StringVar(&pinsDBPath, "pins-db", envStr("PINS_DB_PATH", "/data/pins.db"),
		"Path to the bbolt database file for user pin storage (env: PINS_DB_PATH)")
	flag.StringVar(&accessRequestsDBPath, "access-requests-db", envStr("ACCESS_REQUESTS_DB_PATH", "/data/access_requests.db"),
		"Path to the bbolt database file for access request storage (env: ACCESS_REQUESTS_DB_PATH)")
	flag.StringVar(&adminGroup, "admin-group", envStr("ADMIN_GROUP", "admin"),
		"Keycloak group whose members may access admin endpoints (env: ADMIN_GROUP)")
	flag.StringVar(&notificationsDBPath, "notifications-db", envStr("NOTIFICATIONS_DB_PATH", "/data/notifications.db"),
		"Path to the bbolt database file for notifications (env: NOTIFICATIONS_DB_PATH)")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("Starting Nebari Landing Page API Server",
		"port", port,
		"authEnabled", enableAuth,
		"healthInterval", healthInterval,
	)

	config, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "Failed to get kubeconfig")
		os.Exit(1)
	}

	// Build a k8s client for cross-namespace secret reads (Keycloak admin creds).
	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "Failed to create Kubernetes client")
		os.Exit(1)
	}

	serviceCache := cache.NewServiceCache()
	hub := wshub.NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nebariAppWatcher, err := watcher.NewNebariAppWatcher(config, scheme, serviceCache)
	if err != nil {
		setupLog.Error(err, "Failed to create NebariApp watcher")
		os.Exit(1)
	}
	nebariAppWatcher.SetPublisher(hub)

	go func() {
		if err := nebariAppWatcher.Start(ctx); err != nil {
			setupLog.Error(err, "Failed to start NebariApp watcher")
			os.Exit(1)
		}
	}()

	setupLog.Info("Waiting for cache to sync...")
	if !nebariAppWatcher.WaitForCacheSync(ctx) {
		setupLog.Error(nil, "Failed to sync cache")
		os.Exit(1)
	}
	setupLog.Info("Cache synced successfully")

	var jwtValidator *auth.JWTValidator
	if enableAuth {
		if keycloakURL == "" {
			setupLog.Error(nil, "keycloak-url is required when auth is enabled")
			os.Exit(1)
		}
		jwtValidator, err = auth.NewJWTValidator(keycloakURL, keycloakRealm)
		if err != nil {
			setupLog.Error(err, "Failed to create JWT validator")
			os.Exit(1)
		}
		// KEYCLOAK_ISSUER_URL lets operators keep KEYCLOAK_URL pointing at the
		// internal cluster address (fast, no TLS) while validating the `iss`
		// claim against the external public URL that Keycloak embeds in tokens.
		if issuerURL := os.Getenv("KEYCLOAK_ISSUER_URL"); issuerURL != "" {
			jwtValidator.SetIssuerURL(issuerURL)
			setupLog.Info("JWT issuer URL overridden", "issuerURL", issuerURL)
		}
		setupLog.Info("JWT validation enabled", "keycloakURL", keycloakURL, "realm", keycloakRealm)
	} else {
		setupLog.Info("JWT validation disabled - all requests will be treated as unauthenticated")
	}

	healthChecker := health.NewHealthChecker(serviceCache, time.Duration(healthInterval)*time.Second)
	go healthChecker.Start(ctx)

	// Open the pin store (bbolt). The database file is created if it doesn't exist.
	// A nil store disables the /api/v1/pins endpoints gracefully.
	var pinStore *pins.PinStore
	if pinsDBPath != "" {
		ps, err := pins.NewPinStore(pinsDBPath)
		if err != nil {
			setupLog.Error(err, "Failed to open pin store", "path", pinsDBPath)
			os.Exit(1)
		}
		pinStore = ps
		setupLog.Info("Pin store opened", "path", pinsDBPath)
	} else {
		setupLog.Info("PINS_DB_PATH is empty — pin endpoints disabled")
	}

	// Open the access request store. A nil store returns 501 on the request-access endpoints.
	var accessRequestStore *accessrequests.Store
	if accessRequestsDBPath != "" {
		ars, err := accessrequests.NewStore(accessRequestsDBPath)
		if err != nil {
			setupLog.Error(err, "Failed to open access request store", "path", accessRequestsDBPath)
			os.Exit(1)
		}
		accessRequestStore = ars
		setupLog.Info("Access request store opened", "path", accessRequestsDBPath)
	} else {
		setupLog.Info("ACCESS_REQUESTS_DB_PATH is empty — access request endpoints disabled")
	}

	// Open the notification store. A nil store returns an empty list on GET and 501 on sub-routes.
	var notificationStore *notifications.Store
	if notificationsDBPath != "" {
		ns, err := notifications.NewStore(notificationsDBPath)
		if err != nil {
			setupLog.Error(err, "Failed to open notification store", "path", notificationsDBPath)
			os.Exit(1)
		}
		notificationStore = ns
		setupLog.Info("Notification store opened", "path", notificationsDBPath)
	} else {
		setupLog.Info("NOTIFICATIONS_DB_PATH is empty — notifications endpoints return empty list")
	}

	// Build Keycloak admin client from the same env vars the operator uses.
	// Supports cross-namespace secret lookup via KEYCLOAK_ADMIN_SECRET_NAME +
	// KEYCLOAK_ADMIN_SECRET_NAMESPACE (identical to the operator); falls back to
	// KEYCLOAK_ADMIN_USERNAME / KEYCLOAK_ADMIN_PASSWORD if no secret is named.
	// Non-fatal: when creds are absent the approve endpoint still updates the
	// store record; it just skips the Keycloak group-membership step and warns.
	var keycloakAdminClient *webkeycloak.Client
	if kc, err := webkeycloak.NewFromEnvWithK8sClient(ctx, k8sClient); err != nil {
		setupLog.Info("Keycloak admin client not configured — group membership will not be updated on approval",
			"hint", "set KEYCLOAK_ADMIN_SECRET_NAME or KEYCLOAK_ADMIN_USERNAME/PASSWORD")
	} else {
		keycloakAdminClient = kc
		setupLog.Info("Keycloak admin client configured",
			"url", os.Getenv("KEYCLOAK_URL"),
			"realm", os.Getenv("KEYCLOAK_REALM"))
	}

	handler := api.NewHandler(serviceCache, jwtValidator, enableAuth, hub, pinStore,
		api.WithAccessRequestStore(accessRequestStore),
		api.WithAdminGroup(adminGroup),
		api.WithNotificationStore(notificationStore),
		api.WithKeycloakAdminClient(keycloakAdminClient),
	)

	mux := handler.Routes()

	server := &http.Server{
		Addr:        fmt.Sprintf(":%d", port),
		Handler:     mux,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout must be 0 when WebSocket connections are active — a non-zero
		// value causes the http.Server to cancel upgraded connections after the timeout
		// fires, disconnecting all WS clients even when the connection is healthy.
		// Per-frame write deadlines are enforced inside the Hub.Broadcast instead.
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		setupLog.Info("Starting HTTP server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			setupLog.Error(err, "HTTP server failed")
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	setupLog.Info("Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		setupLog.Error(err, "Server shutdown failed")
	}

	if pinStore != nil {
		if err := pinStore.Close(); err != nil {
			setupLog.Error(err, "Failed to close pin store")
		}
	}
	if accessRequestStore != nil {
		if err := accessRequestStore.Close(); err != nil {
			setupLog.Error(err, "Failed to close access request store")
		}
	}
	if notificationStore != nil {
		if err := notificationStore.Close(); err != nil {
			setupLog.Error(err, "Failed to close notification store")
		}
	}

	setupLog.Info("Server stopped")
}

// envStr returns the value of the named environment variable, or def if unset/empty.
func envStr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// envInt returns the int value of the named environment variable, or def if unset/invalid.
func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// envBool returns the bool value of the named environment variable, or def if unset/invalid.
// Accepts "1", "true", "yes" (case-insensitive) as true.
func envBool(name string, def bool) bool {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
