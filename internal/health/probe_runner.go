package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nebari-dev/nebari-landing/internal/cache"
	"k8s.io/apimachinery/pkg/util/validation"
)

const maxProbeRunnerBodyBytes = 4096

// NewProbeRunnerHandler returns the HTTP handler used by the isolated
// health-probe runner deployment. It has no Kubernetes, Redis, or Keycloak
// clients; its pod-level NetworkPolicy is the enforcement point for health
// target egress.
func NewProbeRunnerHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/probe", handleProbeRunnerRequest)
	return mux
}

func handleProbeRunnerRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req remoteProbeRequest
	body := http.MaxBytesReader(w, r.Body, maxProbeRunnerBodyBytes)
	defer func() { _ = body.Close() }()
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		http.Error(w, "invalid probe request", http.StatusBadRequest)
		return
	}
	if err := validateProbeRunnerTarget(req.ProbeURL); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	result := performProbeRequest(r.Context(), "", req.ProbeURL, newProbeHTTPClient(timeout))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(remoteProbeResponse{
		Status:    result.status,
		Message:   result.message,
		CheckedAt: result.checkedAt,
	}); err != nil {
		log.Error(err, "Failed to encode probe runner response")
	}
}

func validateProbeRunnerTarget(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid probe URL")
	}
	if parsed.Scheme != "http" || parsed.Host == "" {
		return fmt.Errorf("probe URL must be an in-cluster HTTP service URL")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return fmt.Errorf("probe URL must include a numeric port")
	}
	hostParts := strings.Split(parsed.Hostname(), ".")
	if len(hostParts) != 2 && (len(hostParts) < 4 || hostParts[2] != "svc") {
		return fmt.Errorf("probe URL must use service.namespace DNS")
	}
	serviceName := hostParts[0]
	namespace := hostParts[1]
	if len(validation.IsDNS1035Label(serviceName)) != 0 || len(validation.IsDNS1123Label(namespace)) != 0 {
		return fmt.Errorf("probe URL must use valid service and namespace labels")
	}
	if cache.HealthCheckTargetProtected(namespace, serviceName, port) {
		return fmt.Errorf("probe target is protected")
	}
	return nil
}
