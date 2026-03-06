//go:build e2e
// +build e2e

// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nebari-dev/nebari-landing/test/utils"
)

// nebariAppGVK is the GroupVersionKind for NebariApp resources.
// The CRD must already be installed in the cluster before running e2e tests.
var nebariAppGVK = schema.GroupVersionKind{
	Group:   "reconcilers.nebari.dev",
	Version: "v1",
	Kind:    "NebariApp",
}

// envOrDefault returns the value of the named environment variable, or def if unset.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// e2e configuration — every coordinate can be overridden via an env var so the
// suite runs against Kind, minikube, or any other cluster without source edits.
//
//	E2E_NAMESPACE               target namespace (default: nebari-system)
//	E2E_HELM_RELEASE            Helm release name (default: nebari-landing)
//	E2E_HELM_CHART              path to the chart (default: charts/nebari-landing)
//	E2E_HELM_VALUES             values override file (default: dev/chart-values.yaml)
//	E2E_WEBAPI_DEPLOYMENT       webapi Deployment name override (default: <release>-webapi)
//	E2E_WEBAPI_SERVICE          webapi Service name override (default: <release>-webapi)
//	E2E_KEYCLOAK_NAMESPACE      namespace where Keycloak runs (default: keycloak)
//	E2E_KEYCLOAK_SERVICE        Keycloak Service name (default: keycloak-keycloakx-http)
//	E2E_KEYCLOAK_REALM          Keycloak realm (default: nebari)
//	E2E_KEYCLOAK_ADMIN_USER     realm admin username (default: admin)
//	E2E_KEYCLOAK_ADMIN_PASSWORD realm admin password (default: nebari-realm-admin)
var (
	e2eNamespace = envOrDefault("E2E_NAMESPACE", "nebari-system")

	// Helm-based deployment coordinates.
	helmRelease = envOrDefault("E2E_HELM_RELEASE", "nebari-landing")
	helmChart   = envOrDefault("E2E_HELM_CHART", "charts/nebari-landing")
	helmValues  = envOrDefault("E2E_HELM_VALUES", "dev/chart-values.yaml")

	// Names follow the chart convention: <release>-webapi.
	e2eWebapiDeployment = func() string {
		if v := os.Getenv("E2E_WEBAPI_DEPLOYMENT"); v != "" {
			return v
		}
		return helmRelease + "-webapi"
	}()
	e2eWebapiService = func() string {
		if v := os.Getenv("E2E_WEBAPI_SERVICE"); v != "" {
			return v
		}
		return helmRelease + "-webapi"
	}()

	kcNamespace     = envOrDefault("E2E_KEYCLOAK_NAMESPACE", "keycloak")
	kcService       = envOrDefault("E2E_KEYCLOAK_SERVICE", "keycloak-keycloakx-http")
	kcRealm         = envOrDefault("E2E_KEYCLOAK_REALM", "nebari")
	kcAdminUser     = envOrDefault("E2E_KEYCLOAK_ADMIN_USER", "admin")
	kcAdminPassword = envOrDefault("E2E_KEYCLOAK_ADMIN_PASSWORD", "nebari-realm-admin")
)

// VeryLongTimeout is used for slow cluster operations.
const VeryLongTimeout = 5 * time.Minute

// newNebariApp creates an unstructured NebariApp with a landing-page config.
// No api/v1 import is needed — the resource is built from raw field maps.
func newNebariApp(name, namespace, hostname, visibility string, priority int) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(nebariAppGVK)
	u.SetName(name)
	u.SetNamespace(namespace)
	_ = unstructured.SetNestedMap(u.Object, map[string]interface{}{
		"hostname": hostname,
		"service": map[string]interface{}{
			"name": "test-service",
			"port": int64(8080),
		},
		"landingPage": map[string]interface{}{
			"enabled":     true,
			"displayName": fmt.Sprintf("Test Service %s", name),
			"description": fmt.Sprintf("E2E test resource with visibility=%s", visibility),
			"category":    "Testing",
			"visibility":  visibility,
			"priority":    int64(priority),
		},
	}, "spec")
	return u
}

var _ = Describe("Webapi – Service Discovery", Ordered, func() {
	var (
		ctx           = context.Background()
		namespace     = e2eNamespace
		testAppName   = "test-svc-api-app"
		keycloakPFCmd *exec.Cmd
	)

	BeforeAll(func() {
		By("Ensuring the nebari-system namespace exists")
		cmd := exec.Command("kubectl", "create", "namespace", namespace,
			"--dry-run=client", "-o", "yaml")
		nsYAML, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to generate namespace YAML")
		applyNs := exec.Command("kubectl", "apply", "-f", "-")
		applyNs.Stdin = strings.NewReader(nsYAML)
		_, err = utils.Run(applyNs)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply namespace %s", namespace)

		By("Starting Keycloak port-forward to discover issuer URL")
		keycloakPFCmd = exec.Command("kubectl", "port-forward",
			"-n", kcNamespace, fmt.Sprintf("svc/%s", kcService),
			"18090:80")
		Expect(keycloakPFCmd.Start()).NotTo(HaveOccurred(), "keycloak port-forward should start")
		var keycloakIssuer string
		Eventually(func() error {
			resp, err := http.Get(fmt.Sprintf("http://localhost:18090/auth/realms/%s/.well-known/openid-configuration", kcRealm))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("keycloak not ready: status %d", resp.StatusCode)
			}
			var disc struct {
				Issuer string `json:"issuer"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
				return fmt.Errorf("failed to decode OIDC discovery: %w", err)
			}
			if disc.Issuer == "" {
				return fmt.Errorf("OIDC discovery returned empty issuer")
			}
			// issuer looks like "http://<host>/auth/realms/<realm>";
			// strip the realm suffix to get the base URL for KEYCLOAK_ISSUER_URL.
			keycloakIssuer = strings.TrimSuffix(disc.Issuer, fmt.Sprintf("/realms/%s", kcRealm))
			return nil
		}, 30*time.Second, time.Second).Should(Succeed(),
			"keycloak should be reachable via port-forward")

		if !useExistingCluster {
			By("Installing nebari-landing Helm chart (webapi + Redis, no frontend)")
			// Split webapiImage into repo and tag for --set overrides.
			imgRepo, imgTag, found := strings.Cut(webapiImage, ":")
			if !found {
				imgTag = "latest"
			}
			cmd = exec.Command("helm", "upgrade", "--install", helmRelease, helmChart,
				"--namespace", namespace,
				"--create-namespace",
				"-f", helmValues,
				"--set", "frontend.enabled=false",
				"--set", "httpRoute.enabled=false",
				"--set", "nebariApp.enabled=false",
				"--set", fmt.Sprintf("webapi.image.repository=%s", imgRepo),
				"--set", fmt.Sprintf("webapi.image.tag=%s", imgTag),
				"--set", "webapi.image.pullPolicy=Never",
			)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "helm upgrade --install should succeed")
		}

		By("Patching webapi deployment image to configured image")
		// Always override the image so the deployment uses the locally built
		// version that matches this codebase.  For existing clusters this
		// corrects drift; for freshly installed charts it is a no-op when
		// the helm --set already matched.
		setImg := exec.Command("kubectl", "set", "image",
			fmt.Sprintf("deployment/%s", e2eWebapiDeployment),
			"-n", namespace,
			fmt.Sprintf("webapi=%s", webapiImage))
		_, err = utils.Run(setImg)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch webapi container image")

		By("Patching webapi deployment with discovered KEYCLOAK_ISSUER_URL")
		// The issuer in tokens is set by KC_HOSTNAME_URL (e.g. http://<minikube-lb-ip>/auth).
		// The Helm chart value webapi.keycloak.issuerUrl may point to an in-cluster
		// URL ≠ the token issuer.  Patch the live deployment so the JWT validator
		// accepts tokens from this cluster regardless of the values file.
		setEnv := exec.Command("kubectl", "set", "env",
			fmt.Sprintf("deployment/%s", e2eWebapiDeployment),
			"-n", namespace,
			fmt.Sprintf("KEYCLOAK_ISSUER_URL=%s", keycloakIssuer))
		_, err = utils.Run(setEnv)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch KEYCLOAK_ISSUER_URL on webapi deployment")

		By("Waiting for webapi deployment to become ready")
		rollout := exec.Command("kubectl", "rollout", "status",
			fmt.Sprintf("deployment/%s", e2eWebapiDeployment),
			"-n", namespace, "--timeout=2m")
		_, err = utils.Run(rollout)
		Expect(err).NotTo(HaveOccurred(), "webapi deployment should become ready")
	})

	AfterAll(func() {
		By("Deleting test NebariApp resources")
		for _, name := range []string{testAppName, "test-auth-visibility"} {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(nebariAppGVK)
			u.SetName(name)
			u.SetNamespace(namespace)
			// Strip operator finalizers before deletion so cleanup is not blocked
			// when the operator cannot reconcile (e.g. missing cluster dependencies).
			existing := u.DeepCopy()
			if getErr := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing); getErr == nil {
				existing.SetFinalizers(nil)
				_ = k8sClient.Update(ctx, existing)
			}
			_ = k8sClient.Delete(ctx, u)
		}

		By("Stopping Keycloak port-forward")
		if keycloakPFCmd != nil && keycloakPFCmd.Process != nil {
			_ = keycloakPFCmd.Process.Kill()
		}

		if !useExistingCluster {
			By("Uninstalling nebari-landing Helm release")
			cmd := exec.Command("helm", "uninstall", helmRelease, "--namespace", namespace, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		}
	})

	Context("Service Discovery", func() {
		It("should expose a Service object", func() {
			svc := &corev1.Service{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      e2eWebapiService,
				Namespace: namespace,
			}, svc)
			Expect(err).NotTo(HaveOccurred(), "webapi Service should exist")
			Expect(svc.Spec.Ports).NotTo(BeEmpty(), "Service should have ports defined")
		})

		It("should return public services without authentication", func() {
			By("Creating a public-visibility NebariApp")
			testApp := newNebariApp(testAppName, namespace,
				fmt.Sprintf("%s.nebari.test", testAppName), "public", 99)
			err := k8sClient.Create(ctx, testApp)
			Expect(err).NotTo(HaveOccurred(), "Should create test NebariApp")
			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, testApp)
			})

			By("Port-forwarding to webapi")
			pfCmd := exec.Command("kubectl", "port-forward",
				"-n", namespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18082:8080")
			Expect(pfCmd.Start()).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = pfCmd.Process.Kill() })

			By("Waiting for webapi port-forward to be ready")
			Eventually(func() error {
				resp, err := http.Get("http://localhost:18082/api/v1/health")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 30*time.Second, time.Second).Should(Succeed())

			By("Waiting for watcher to process the NebariApp")
			time.Sleep(5 * time.Second)

			By("Calling GET /api/v1/services without credentials")
			resp, err := http.Get("http://localhost:18082/api/v1/services")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var result ServiceListResponse
			Expect(json.NewDecoder(resp.Body).Decode(&result)).To(Succeed())
			Expect(serviceNames(result)).To(ContainElement("Test Service "+testAppName),
				"Public services must appear when unauthenticated")
		})

		It("should filter services based on visibility", func() {
			By("Acquiring a JWT from Keycloak (host-header override for issuer match)")
			// The port-forward reaches Keycloak at localhost:18090, but the Host header
			// must match the in-cluster hostname so the token issuer ("iss") equals
			// KEYCLOAK_URL/realms/<realm>, which is what the webapi validates.
			tokenForm := url.Values{
				"client_id":  {"admin-cli"},
				"username":   {kcAdminUser},
				"password":   {kcAdminPassword},
				"grant_type": {"password"},
				"scope":      {"openid profile"},
			}
			tokenReq, err := http.NewRequest(http.MethodPost,
				fmt.Sprintf("http://localhost:18090/auth/realms/%s/protocol/openid-connect/token", kcRealm),
				strings.NewReader(tokenForm.Encode()))
			Expect(err).NotTo(HaveOccurred())
			tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			tokenReq.Host = fmt.Sprintf("%s.%s.svc.cluster.local", kcService, kcNamespace)
			tokenResp, err := http.DefaultClient.Do(tokenReq)
			Expect(err).NotTo(HaveOccurred(), "Should be able to request token from Keycloak")
			defer tokenResp.Body.Close()
			Expect(tokenResp.StatusCode).To(Equal(http.StatusOK),
				fmt.Sprintf("Keycloak token request must succeed (realm=%s, user=%s)", kcRealm, kcAdminUser))
			var tokenData struct {
				AccessToken string `json:"access_token"`
			}
			Expect(json.NewDecoder(tokenResp.Body).Decode(&tokenData)).To(Succeed())
			Expect(tokenData.AccessToken).NotTo(BeEmpty(), "JWT must be non-empty")

			By("Creating a NebariApp with authenticated visibility")
			authApp := newNebariApp("test-auth-visibility", namespace,
				"test-auth-visibility.nebari.test", "authenticated", 50)
			Expect(k8sClient.Create(ctx, authApp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, authApp) })

			By("Port-forwarding to webapi (port 18081)")
			pfCmd := exec.Command("kubectl", "port-forward",
				"-n", namespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18081:8080")
			Expect(pfCmd.Start()).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = pfCmd.Process.Kill() })

			By("Waiting for port-forward to be ready")
			Eventually(func() error {
				resp, err := http.Get("http://localhost:18081/api/v1/health")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 30*time.Second, time.Second).Should(Succeed())

			By("Waiting for watcher to process the NebariApp")
			time.Sleep(5 * time.Second)

			By("Calling /api/v1/services without a token — auth services must be hidden")
			resp, err := http.Get("http://localhost:18081/api/v1/services")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			var unauthResult ServiceListResponse
			Expect(json.NewDecoder(resp.Body).Decode(&unauthResult)).To(Succeed())
			Expect(serviceNames(unauthResult)).NotTo(ContainElement("Test Service test-auth-visibility"),
				"Authenticated services must not appear without a token")

			By("Calling /api/v1/services with a valid JWT — auth services must appear")
			req, err := http.NewRequest(http.MethodGet, "http://localhost:18081/api/v1/services", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
			authResp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer authResp.Body.Close()
			Expect(authResp.StatusCode).To(Equal(http.StatusOK))
			var authResult ServiceListResponse
			Expect(json.NewDecoder(authResp.Body).Decode(&authResult)).To(Succeed())
			Expect(serviceNames(authResult)).To(ContainElement("Test Service test-auth-visibility"),
				"The authenticated-visibility NebariApp must appear when a valid JWT is presented")
		})
	})

	Context("Health Checks", func() {
		It("should report healthy status", func() {
			By("Port-forwarding to webapi (port 18080)")
			pfCmd := exec.Command("kubectl", "port-forward",
				"-n", namespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18080:8080")
			Expect(pfCmd.Start()).NotTo(HaveOccurred(), "port-forward should start")
			DeferCleanup(func() { _ = pfCmd.Process.Kill() })

			By("Waiting for health endpoint to respond")
			Eventually(func() error {
				resp, err := http.Get("http://localhost:18080/api/v1/health")
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("unexpected status %d", resp.StatusCode)
				}
				return nil
			}, 30*time.Second, time.Second).Should(Succeed(), "health endpoint should return 200")

			By("Verifying response body contains status field")
			resp, err := http.Get("http://localhost:18080/api/v1/health")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(ContainSubstring(`"status"`),
				"health response must contain a 'status' field")
		})
	})
})

// ── helpers ──────────────────────────────────────────────────────────────────

// ServiceListResponse matches the flat API response for GET /api/v1/services.
type ServiceListResponse struct {
	Services []map[string]interface{} `json:"services"`
}

// serviceNames extracts the "name" key from each service map.
func serviceNames(r ServiceListResponse) []string {
	names := make([]string, 0, len(r.Services))
	for _, s := range r.Services {
		if n, ok := s["name"].(string); ok {
			names = append(names, n)
		}
	}
	return names
}

