package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nebari-dev/nebari-landing/internal/accessrequests"
	"github.com/nebari-dev/nebari-landing/internal/api"
	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/nebari-dev/nebari-landing/internal/health"
	webkeycloak "github.com/nebari-dev/nebari-landing/internal/keycloak"
	"github.com/nebari-dev/nebari-landing/internal/notifications"
	"github.com/nebari-dev/nebari-landing/internal/pins"
	"github.com/nebari-dev/nebari-landing/internal/watcher"
	wshub "github.com/nebari-dev/nebari-landing/internal/websocket"
)

// Build-time version info — injected by GoReleaser via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	// Register NebariApp as unstructured so the controller-runtime cache can build
	// informers without importing the nebari-operator API package.
	// The watcher uses unstructured objects internally; this registration only
	// tells the scheme which Go type to use for the GVK.
	nebariGV := schema.GroupVersion{Group: "reconcilers.nebari.dev", Version: "v1"}
	scheme.AddKnownTypeWithName(nebariGV.WithKind("NebariApp"), &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(nebariGV.WithKind("NebariAppList"), &unstructured.UnstructuredList{})
}

func main() {
	var (
		port           int
		keycloakURL    string
		keycloakRealm  string
		enableAuth     bool
		debugMode      bool
		healthInterval int
		adminGroup     string
		redisAddr      string
		redisUsername  string
		redisPassword  string
		redisDB        int
		allowedOrigins string
		notifStartup   bool
		notifLifecycle bool
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
	flag.StringVar(&adminGroup, "admin-group", envStr("ADMIN_GROUP", "admin"),
		"Keycloak group whose members may access admin endpoints (env: ADMIN_GROUP)")
	flag.StringVar(&redisAddr, "redis-addr", envStr("REDIS_ADDR", "localhost:6379"),
		"Redis server address host:port (env: REDIS_ADDR)")
	flag.StringVar(&redisUsername, "redis-username", os.Getenv("REDIS_USERNAME"),
		"Redis ACL username for Redis 6+ auth (env: REDIS_USERNAME, empty = default user)")
	flag.StringVar(&redisPassword, "redis-password", os.Getenv("REDIS_PASSWORD"),
		"Redis password, if any (env: REDIS_PASSWORD)")
	flag.IntVar(&redisDB, "redis-db", envInt("REDIS_DB", 0),
		"Redis database index (env: REDIS_DB)")
	flag.StringVar(&allowedOrigins, "allowed-origins", envStr("ALLOWED_ORIGINS", "*"),
		"Comma-separated list of allowed CORS origins, or * for all (env: ALLOWED_ORIGINS)")
	flag.BoolVar(&debugMode, "debug", envBool("DEBUG_MODE", false),
		"Enable /api/v1/debug endpoint and verbose JWT logging. Never use in production (env: DEBUG_MODE)")
	flag.BoolVar(&notifStartup, "notifications-startup", envBool("NOTIFICATIONS_STARTUP", true),
		"Post a welcome/feedback notification on every startup (env: NOTIFICATIONS_STARTUP)")
	flag.BoolVar(&notifLifecycle, "notifications-lifecycle", envBool("NOTIFICATIONS_LIFECYCLE", true),
		"Auto-post notifications for service lifecycle events: added, removed, back online (env: NOTIFICATIONS_LIFECYCLE)")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	setupLog.Info("Starting Nebari Landing Page API Server",
		"version", version,
		"commit", commit,
		"date", date,
		"port", port,
		"authEnabled", enableAuth,
		"debugMode", debugMode,
		"healthInterval", healthInterval,
		"allowedOrigins", allowedOrigins,
		"notificationsStartup", notifStartup,
		"notificationsLifecycle", notifLifecycle,
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serviceCache := cache.NewServiceCache()

	// Build Redis client — shared by all stores and the WebSocket hub.
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: redisUsername,
		Password: redisPassword,
		DB:       redisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		setupLog.Error(err, "Failed to connect to Redis", "addr", redisAddr)
		os.Exit(1)
	}
	setupLog.Info("Redis connected", "addr", redisAddr, "db", redisDB)
	defer func() {
		if err := rdb.Close(); err != nil {
			setupLog.Error(err, "failed to close Redis connection")
		}
	}()

	hub := wshub.NewHub(ctx, rdb)

	// Build Redis-backed stores early so they can be wired into the watcher
	// and health checker before those components start.
	pinStore := pins.NewPinStore(rdb)
	setupLog.Info("Pin store ready (Redis)")

	accessRequestStore := accessrequests.NewStore(rdb)
	setupLog.Info("Access request store ready (Redis)")

	notificationStore := notifications.NewStore(rdb)
	setupLog.Info("Notification store ready (Redis)")

	nebariAppWatcher, err := watcher.NewNebariAppWatcher(config, scheme, serviceCache)
	if err != nil {
		setupLog.Error(err, "Failed to create NebariApp watcher")
		os.Exit(1)
	}
	nebariAppWatcher.SetPublisher(hub)
	if notifLifecycle {
		nebariAppWatcher.SetNotificationStore(notificationStore)
		nebariAppWatcher.SetNotificationPublisher(hub)
	}

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

	// Post a one-time welcome/feedback notification on every startup (opt-out via --notifications-startup=false).
	if notifStartup {
		if n, err := notificationStore.Create(
			"https://github.com/nebari-dev/nebari-design/blob/main/symbol/Nebari-Symbol.svg?raw=true",
			"Welcome to Nebari!",
			"User feedback is welcomed! We value your input to improve Nebari.",
		); err != nil {
			setupLog.Error(err, "Failed to post startup notification")
		} else {
			hub.PublishNotification(n)
			setupLog.Info("Startup notification posted")
		}
	}

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
	healthChecker.SetPublisher(hub)
	if notifLifecycle {
		healthChecker.SetNotificationStore(notificationStore)
		healthChecker.SetNotificationPublisher(hub)
	}
	go healthChecker.Start(ctx)

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

	handlerOpts := []api.HandlerOption{
		api.WithAccessRequestStore(accessRequestStore),
		api.WithAdminGroup(adminGroup),
		api.WithNotificationStore(notificationStore),
		api.WithKeycloakAdminClient(keycloakAdminClient),
		api.WithAllowedOrigins(splitOrigins(allowedOrigins)),
	}
	if debugMode {
		setupLog.Info("Debug mode enabled — GET /api/v1/debug is active; do not use in production")
		handlerOpts = append(handlerOpts, api.WithDebugMode())
	}

	handler := api.NewHandler(serviceCache, jwtValidator, enableAuth, hub, pinStore, handlerOpts...)

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

// splitOrigins splits a comma-separated ALLOWED_ORIGINS string into a trimmed slice.
// "*" is returned as a single-element slice so the CORS middleware recognises it
// as "allow all".
func splitOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}
