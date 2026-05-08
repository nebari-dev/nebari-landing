//go:build e2e
// +build e2e

// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bufio"
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
//	E2E_KEYCLOAK_ADMIN_USER     realm admin username — must be in the "admin" group (default: admin)
//	E2E_KEYCLOAK_ADMIN_PASSWORD realm admin password (default: nebari-realm-admin)
//	E2E_TEST_USER               unprivileged test-user for regular-user flows (default: test-user)
//	E2E_TEST_USER_PASSWORD      password for E2E_TEST_USER (default: test-user)
//	E2E_OIDC_CLIENT_ID          OIDC client ID for token acquisition (default: nebari-system-nebari-landing)
//	E2E_OIDC_CLIENT_SECRET      OIDC client secret written by the operator to nebari-landing-oidc-client
//
// Token acquisition uses the operator-provisioned confidential client
// (nebari-system-nebari-landing) rather than admin-cli.  Keycloak 26+ sets
// client.use.lightweight.access.token.enabled=true on admin-cli by default,
// stripping all identity claims (sub, preferred_username) from its access
// tokens — which causes 401s on identity-keyed endpoints like /api/v1/pins.
// The confidential client carries no such flag and issues full-claims tokens.
//
// directAccessGrantsEnabled is patched on by the e2e CI workflow (not set by
// the operator, which correctly leaves it false in production).
// TODO: remove the CI kcadm patch once nebari-operator supports
//   spec.auth.keycloakConfig.directAccessGrantsEnabled.
//   Tracking: https://github.com/nebari-dev/nebari-operator/issues/TBD
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
	// E2E_KEYCLOAK_PORT is the service port that the keycloakx chart exposes.
	// keycloakx v7+ sets service.httpPort=8080 so the service listens on 8080,
	// not the HTTP default 80.  Override to 80 only for older local deployments.
	kcPort          = envOrDefault("E2E_KEYCLOAK_PORT", "8080")
	kcRealm         = envOrDefault("E2E_KEYCLOAK_REALM", "nebari")
	// kcAdminUser is the realm admin — must be in the "admin" Keycloak group.
	// Used only for tests that exercise admin-gated endpoints.
	kcAdminUser     = envOrDefault("E2E_KEYCLOAK_ADMIN_USER", "admin")
	kcAdminPassword = envOrDefault("E2E_KEYCLOAK_ADMIN_PASSWORD", "nebari-realm-admin")
	// kcTestUser is an unprivileged user for regular-user flows (pins, access
	// requests from the requester side, notification reads, etc.).
	kcTestUser     = envOrDefault("E2E_TEST_USER", "test-user")
	kcTestPassword = envOrDefault("E2E_TEST_USER_PASSWORD", "test-user")

	// oidcClientID / oidcClientSecret are used for password-grant token
	// acquisition in the e2e tests.  We use the operator-provisioned confidential
	// client rather than admin-cli because Keycloak 26+ enables
	// client.use.lightweight.access.token.enabled on admin-cli by default,
	// which strips sub and preferred_username from access tokens.
	oidcClientID     = envOrDefault("E2E_OIDC_CLIENT_ID", "nebari-system-nebari-landing")
	oidcClientSecret = os.Getenv("E2E_OIDC_CLIENT_SECRET")
)

// VeryLongTimeout is used for slow cluster operations.
const VeryLongTimeout = 5 * time.Minute

// startPortForwardAndWait establishes a kubectl port-forward and waits for it to be ready.
// It retries the entire port-forward setup if the tunnel fails to establish within the timeout.
//
// The retry logic is at the port-forward level (not HTTP client level) to properly handle
// the case where kubectl port-forward is slow to establish the tunnel. When the tunnel is
// mid-handshake, TCP connections succeed but hang forever waiting for the upstream proxy.
//
// Returns the running command which the caller should clean up with DeferCleanup or Process.Kill.
func startPortForwardAndWait(namespace, target, ports string) *exec.Cmd {
	var cmd *exec.Cmd
	Eventually(func() error {
		// Kill any previous attempt
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}

		// Start a new port-forward with pod-running-timeout for fast failure
		cmd = exec.Command("kubectl", "port-forward",
			"-n", namespace,
			target,
			ports,
			"--pod-running-timeout=30s")

		// Capture both stdout and stderr - kubectl port-forward outputs to stdout
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start port-forward: %w", err)
		}

		// Wait for "Forwarding from..." in stdout or stderr, which signals the tunnel is ready
		ready := make(chan error, 1)
		
		// Monitor stdout
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "Forwarding from") {
					ready <- nil
					return
				}
			}
			if err := scanner.Err(); err != nil {
				ready <- fmt.Errorf("stdout scan error: %w", err)
			}
		}()
		
		// Monitor stderr for errors
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "Forwarding from") {
					ready <- nil
					return
				}
				// Check for error messages
				if strings.Contains(line, "error") || strings.Contains(line, "Error") {
					ready <- fmt.Errorf("port-forward error: %s", line)
					return
				}
			}
			if err := scanner.Err(); err != nil {
				ready <- fmt.Errorf("stderr scan error: %w", err)
			}
		}()

		// Wait for ready signal or timeout (increased to handle slow pod startup)
		select {
		case err := <-ready:
			return err
		case <-time.After(30 * time.Second):
			return fmt.Errorf("port-forward did not become ready within 30s")
		}
	}, 120*time.Second, 5*time.Second).Should(Succeed(), "port-forward should become ready")

	return cmd
}

// acquireToken obtains an access token from Keycloak via the resource-owner
// password grant using the operator-provisioned confidential client.
//
// The Keycloak port-forward to localhost:18090 must already be running when
// this is called (the parent BeforeAll starts it via startPortForwardAndWait).
// The Host header is set to the in-cluster service name so the token issuer
// matches KEYCLOAK_ISSUER_URL that the webapi was patched with.
func acquireToken(username, password string) string {
	GinkgoHelper()
	tokenForm := url.Values{
		"client_id":     {oidcClientID},
		"client_secret": {oidcClientSecret},
		"username":      {username},
		"password":      {password},
		"grant_type":    {"password"},
		"scope":         {"openid profile"},
	}
	tokenReq, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://localhost:18090/realms/%s/protocol/openid-connect/token", kcRealm),
		strings.NewReader(tokenForm.Encode()))
	Expect(err).NotTo(HaveOccurred())
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Host = fmt.Sprintf("%s.%s.svc.cluster.local", kcService, kcNamespace)
	tokenResp, err := http.DefaultClient.Do(tokenReq)
	Expect(err).NotTo(HaveOccurred())
	defer tokenResp.Body.Close()
	Expect(tokenResp.StatusCode).To(Equal(http.StatusOK),
		fmt.Sprintf("Keycloak token request must succeed (realm=%s, user=%s)", kcRealm, username))
	var td struct {
		AccessToken string `json:"access_token"`
	}
	Expect(json.NewDecoder(tokenResp.Body).Decode(&td)).To(Succeed())
	Expect(td.AccessToken).NotTo(BeEmpty())
	return td.AccessToken
}

// newNebariApp creates an unstructured NebariApp with a landing-page config.
// No api/v1 import is needed — the resource is built from raw field maps.
//
// The visibility parameter controls access:
//   - "public" → no auth block (service is visible without authentication)
//   - any other value (e.g. "private", "authenticated") → spec.auth.enabled=true
//     (service requires authentication; visibility computed from spec.auth by the controller)
func newNebariApp(name, namespace, hostname, visibility string, priority int) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(nebariAppGVK)
	u.SetName(name)
	u.SetNamespace(namespace)

	spec := map[string]interface{}{
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
			"priority":    int64(priority),
		},
	}

	// spec.landingPage no longer carries visibility; access control is expressed
	// via spec.auth. The operator's controller derives visibility/requiredGroups
	// from spec.auth and writes them to status.serviceDiscovery.
	if visibility != "public" {
		spec["auth"] = map[string]interface{}{
			"enabled": true,
		}
	}

	_ = unstructured.SetNestedMap(u.Object, spec, "spec")
	return u
}

// newTestService creates a minimal Kubernetes Service for testing NebariApp discovery.
// The NebariApp CRDs created by tests point to spec.service.name="test-service",
// so we need an actual K8s Service to exist for the webapi watcher to report them.
func newTestService(name, namespace string, port int) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	})
	u.SetName(name)
	u.SetNamespace(namespace)

	spec := map[string]interface{}{
		"type": "ClusterIP",
		"ports": []interface{}{
			map[string]interface{}{
				"name":       "http",
				"port":       int64(port),
				"targetPort": int64(port),
				"protocol":   "TCP",
			},
		},
		"selector": map[string]interface{}{
			"app": "test-dummy",
		},
	}

	_ = unstructured.SetNestedMap(u.Object, spec, "spec")
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

		By("Creating shared backing Service for all test NebariApps")
		sharedTestSvc := newTestService("test-service", e2eNamespace, 8080)
		Expect(k8sClient.Create(ctx, sharedTestSvc)).To(Succeed(), "should create shared test-service")
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, sharedTestSvc) })

		By("Starting Keycloak port-forward to discover issuer URL")
		keycloakPFCmd = startPortForwardAndWait(kcNamespace, fmt.Sprintf("svc/%s", kcService), fmt.Sprintf("18090:%s", kcPort))

		By("Discovering Keycloak issuer URL")
		var keycloakIssuer string
		resp, err := http.Get(fmt.Sprintf("http://localhost:18090/realms/%s/.well-known/openid-configuration", kcRealm))
		Expect(err).NotTo(HaveOccurred(), "should fetch OIDC discovery document")
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK), "OIDC discovery should return 200")
		var disc struct {
			Issuer string `json:"issuer"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&disc)).To(Succeed(), "should decode OIDC discovery")
		Expect(disc.Issuer).NotTo(BeEmpty(), "OIDC discovery issuer should not be empty")
		// issuer looks like "http://<host>/realms/<realm>";
		// strip the realm suffix to get the base URL for KEYCLOAK_ISSUER_URL.
		keycloakIssuer = strings.TrimSuffix(disc.Issuer, fmt.Sprintf("/realms/%s", kcRealm))

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

		By("Patching webapi deployment with discovered KEYCLOAK_ISSUER_URL and ENABLE_DOCS=true")
		// The issuer in tokens is set by KC_HOSTNAME_URL (e.g. http://<minikube-lb-ip>).
		// Keycloak 17+ no longer uses /auth as a context root.
		// The Helm chart value webapi.keycloak.issuerUrl may point to an in-cluster
		// URL ≠ the token issuer.  Patch the live deployment so the JWT validator
		// accepts tokens from this cluster regardless of the values file.
		// ENABLE_DOCS=true exposes the OpenAPI 3.1 spec at /api/v1/docs/openapi.json
		// and the Scalar viewer at /api/v1/docs so the suite can assert on them.
		setEnv := exec.Command("kubectl", "set", "env",
			fmt.Sprintf("deployment/%s", e2eWebapiDeployment),
			"-n", namespace,
			fmt.Sprintf("KEYCLOAK_ISSUER_URL=%s", keycloakIssuer),
			"ENABLE_DOCS=true")
		_, err = utils.Run(setEnv)
		Expect(err).NotTo(HaveOccurred(), "Failed to patch webapi deployment env vars")

		By("Waiting for webapi deployment to become ready")
		rollout := exec.Command("kubectl", "rollout", "status",
			fmt.Sprintf("deployment/%s", e2eWebapiDeployment),
			"-n", namespace, "--timeout=2m")
		_, err = utils.Run(rollout)
		Expect(err).NotTo(HaveOccurred(), "webapi deployment should become ready")
	})

	AfterAll(func() {
		By("Deleting test NebariApp resources")
		// Sub-context NebariApps (test-pin-app, test-ar-app) also live under the
		// outer Describe — strip finalizers here too so a partial run leaves no
		// Terminating-but-stuck CRs behind for the next run to trip on.
		for _, name := range []string{testAppName, "test-auth-visibility", "test-pin-app", "test-ar-app"} {
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
			pfCmd := startPortForwardAndWait(namespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18082:8080")
			DeferCleanup(func() { _ = pfCmd.Process.Kill() })

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
			tokenData := struct{ AccessToken string }{}
			tokenData.AccessToken = acquireToken(kcTestUser, kcTestPassword)

			By("Creating a NebariApp with authenticated visibility")
			authApp := newNebariApp("test-auth-visibility", namespace,
				"test-auth-visibility.nebari.test", "authenticated", 50)
			Expect(k8sClient.Create(ctx, authApp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, authApp) })

			By("Port-forwarding to webapi (port 18081)")
			pfCmd := startPortForwardAndWait(namespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18081:8080")
			DeferCleanup(func() { _ = pfCmd.Process.Kill() })

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
			pfCmd := startPortForwardAndWait(namespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18080:8080")
			DeferCleanup(func() { _ = pfCmd.Process.Kill() })

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

	Context("OpenAPI docs", func() {
		// Asserts the --enable-docs / ENABLE_DOCS flag plumbing end-to-end:
		// the chart must wire it into the deployment, the deployment must restart
		// with the env var, and the binary must register the routes when set.
		// BeforeAll above patches the webapi deployment with ENABLE_DOCS=true.
		//
		// SKIPPED on the existing-cluster path until #62 lands: ArgoCD's
		// self-heal reverts the BeforeAll's `kubectl set image` patch back to
		// the published image, so the binary actually answering on the cluster
		// has no docs routes and these probes always 404.  Once the gitops-
		// based image override from #62 is in main, remove the Skip below.
		// Until then, the gating + content is covered by httptest unit tests
		// in internal/api/openapi_test.go.
		var (
			pfCmd  *exec.Cmd
			docsBase = "http://localhost:18081"
		)
		BeforeAll(func() {
			if useExistingCluster {
				Skip("docs e2e disabled on existing-cluster path until ArgoCD image-override fix from #62 lands; httptest unit tests in internal/api/openapi_test.go cover the feature")
			}
			By("Port-forwarding to webapi on :18081 for the docs probe")
			pfCmd = exec.Command("kubectl", "port-forward",
				"-n", namespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18081:8080")
			Expect(pfCmd.Start()).NotTo(HaveOccurred(), "port-forward should start")
			DeferCleanup(func() {
				if pfCmd != nil && pfCmd.Process != nil {
					_ = pfCmd.Process.Kill()
				}
			})

			// The set-env patch above triggers a rollout; wait until /api/v1/health
			// answers on the new pod before probing the docs routes.
			Eventually(func() error {
				resp, err := http.Get(docsBase + "/api/v1/health")
				if err != nil {
					return err
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("health %d", resp.StatusCode)
				}
				return nil
			}, 60*time.Second, time.Second).Should(Succeed(),
				"webapi must be ready before docs probe")
		})

		It("should serve a valid OpenAPI 3.x spec at /api/v1/docs/openapi.json", func() {
			resp, err := http.Get(docsBase + "/api/v1/docs/openapi.json")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK),
				"openapi.json should return 200 when ENABLE_DOCS=true")
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))

			var spec map[string]interface{}
			Expect(json.NewDecoder(resp.Body).Decode(&spec)).To(Succeed(),
				"response must be valid JSON")
			version, _ := spec["openapi"].(string)
			Expect(version).To(HavePrefix("3."),
				"spec must declare OpenAPI 3.x, got %q", version)
			_, hasPaths := spec["paths"].(map[string]interface{})
			Expect(hasPaths).To(BeTrue(), "spec must include a 'paths' object")
		})

		It("should serve the Scalar viewer HTML at /api/v1/docs", func() {
			resp, err := http.Get(docsBase + "/api/v1/docs")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Content-Type")).To(HavePrefix("text/html"),
				"viewer must declare HTML content type")
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(ContainSubstring("/api/v1/docs/openapi.json"),
				"viewer HTML must reference the spec endpoint")
		})

		It("should reject non-GET methods on docs routes with 405", func() {
			req, err := http.NewRequest(http.MethodPost, docsBase+"/api/v1/docs/openapi.json", nil)
			Expect(err).NotTo(HaveOccurred())
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
		})
	})

	// Pins — e2e coverage for GET/PUT/DELETE /api/v1/pins against a live cluster.
	Context("Pins", func() {
		var (
			webapiBase  string
			pfCmd       *exec.Cmd
			bearerToken string
			serviceUID  string
			pinAppName  = "test-pin-app"
		)

		BeforeAll(func() {
			By("Acquiring a JWT from Keycloak (test-user — unprivileged)")
			bearerToken = acquireToken(kcTestUser, kcTestPassword)

			By("Creating a public NebariApp for pinning")
			pinApp := newNebariApp(pinAppName, e2eNamespace,
				fmt.Sprintf("%s.nebari.test", pinAppName), "public", 88)
			Expect(k8sClient.Create(context.Background(), pinApp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), pinApp) })

			By("Port-forwarding to webapi on :18084")
			pfCmd = exec.Command("kubectl", "port-forward",
				"-n", e2eNamespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18084:8080")
			Expect(pfCmd.Start()).NotTo(HaveOccurred())
			webapiBase = "http://localhost:18084"

			Eventually(func() error {
				resp, err := http.Get(webapiBase + "/api/v1/health")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 30*time.Second, time.Second).Should(Succeed())

			By("Waiting for watcher to process the NebariApp and extracting service UID")
			Eventually(func() error {
				req, err := http.NewRequest(http.MethodGet, webapiBase+"/api/v1/services", nil)
				if err != nil {
					return err
				}
				req.Header.Set("Authorization", "Bearer "+bearerToken)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				var result ServiceListResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					return err
				}
				for _, svc := range result.Services {
					if name, _ := svc["name"].(string); name == "Test Service "+pinAppName {
						if uid, _ := svc["id"].(string); uid != "" {
							serviceUID = uid
							return nil
						}
					}
				}
				return fmt.Errorf("service %q not yet visible", pinAppName)
			}, VeryLongTimeout, 2*time.Second).Should(Succeed(),
				"NebariApp should appear in services list")
		})

		AfterAll(func() {
			if pfCmd != nil && pfCmd.Process != nil {
				_ = pfCmd.Process.Kill()
			}
		})

		It("should return an empty pin list for a new user", func() {
			req, err := http.NewRequest(http.MethodGet, webapiBase+"/api/v1/pins", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+bearerToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			var body struct {
				Pins []interface{} `json:"pins"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			Expect(body.Pins).To(BeEmpty(), "new user should have no pins")
		})

		It("should pin a service", func() {
			req, err := http.NewRequest(http.MethodPut,
				webapiBase+"/api/v1/pins/"+serviceUID, nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+bearerToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent),
				"PUT /api/v1/pins/{uid} must return 204")
		})

		It("should return the pinned service in GET /api/v1/pins", func() {
			req, err := http.NewRequest(http.MethodGet, webapiBase+"/api/v1/pins", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+bearerToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			var body struct {
				Pins []map[string]interface{} `json:"pins"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			Expect(body.Pins).To(HaveLen(1), "one service should be pinned")
			Expect(body.Pins[0]["uid"]).To(Equal(serviceUID))
		})

		It("should unpin a service", func() {
			req, err := http.NewRequest(http.MethodDelete,
				webapiBase+"/api/v1/pins/"+serviceUID, nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+bearerToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent),
				"DELETE /api/v1/pins/{uid} must return 204")

			By("Verifying the pin list is empty again")
			req2, err := http.NewRequest(http.MethodGet, webapiBase+"/api/v1/pins", nil)
			Expect(err).NotTo(HaveOccurred())
			req2.Header.Set("Authorization", "Bearer "+bearerToken)
			resp2, err := http.DefaultClient.Do(req2)
			Expect(err).NotTo(HaveOccurred())
			defer resp2.Body.Close()
			var body struct {
				Pins []interface{} `json:"pins"`
			}
			Expect(json.NewDecoder(resp2.Body).Decode(&body)).To(Succeed())
			Expect(body.Pins).To(BeEmpty(), "pin list should be empty after unpin")
		})
	})

	// Notifications — e2e coverage for create / list / mark-read against a live cluster.
	Context("Notifications", func() {
		var (
			webapiBase     string
			pfCmd          *exec.Cmd
			// userToken is for list/mark-read — regular authenticated user.
			userToken      string
			// adminToken is for POST /api/v1/admin/notifications — requires admin group.
			adminToken     string
			notificationID string
		)

		BeforeAll(func() {
			By("Acquiring tokens from Keycloak")
			userToken  = acquireToken(kcTestUser, kcTestPassword)
			adminToken = acquireToken(kcAdminUser, kcAdminPassword)

			By("Port-forwarding to webapi on :18085")
			pfCmd = exec.Command("kubectl", "port-forward",
				"-n", e2eNamespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18085:8080")
			Expect(pfCmd.Start()).NotTo(HaveOccurred())
			webapiBase = "http://localhost:18085"

			Eventually(func() error {
				resp, err := http.Get(webapiBase + "/api/v1/health")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 30*time.Second, time.Second).Should(Succeed())
		})

		AfterAll(func() {
			if pfCmd != nil && pfCmd.Process != nil {
				_ = pfCmd.Process.Kill()
			}
		})

		It("should reject non-admin notification creation with 403", func() {
			body := `{"title":"E2E Negative","message":"should not persist"}`
			req, err := http.NewRequest(http.MethodPost,
				webapiBase+"/api/v1/admin/notifications",
				strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden),
				"non-admin POST /api/v1/admin/notifications must be 403 (requireAdmin)")
		})

		It("should create a notification as admin", func() {
			body := `{"title":"E2E Test Notification","message":"Created during e2e test"}`
			req, err := http.NewRequest(http.MethodPost,
				webapiBase+"/api/v1/admin/notifications",
				strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusCreated),
				"POST /api/v1/admin/notifications must return 201")
			var notif struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&notif)).To(Succeed())
			Expect(notif.ID).NotTo(BeEmpty())
			Expect(notif.Title).To(Equal("E2E Test Notification"))
			notificationID = notif.ID
		})

		It("should list notifications and include the created one", func() {
			req, err := http.NewRequest(http.MethodGet,
				webapiBase+"/api/v1/notifications", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+userToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			var body struct {
				Notifications []map[string]interface{} `json:"notifications"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			ids := make([]string, 0, len(body.Notifications))
			for _, n := range body.Notifications {
				if id, ok := n["id"].(string); ok {
					ids = append(ids, id)
				}
			}
			Expect(ids).To(ContainElement(notificationID),
				"notification list must include the newly created notification")
		})

		It("should mark the notification as read", func() {
			req, err := http.NewRequest(http.MethodPut,
				webapiBase+"/api/v1/notifications/"+notificationID+"/read", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+userToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent),
				"PUT /api/v1/notifications/{id}/read must return 204")

			By("Verifying the notification is marked read in the list")
			req2, err := http.NewRequest(http.MethodGet,
				webapiBase+"/api/v1/notifications", nil)
			Expect(err).NotTo(HaveOccurred())
			req2.Header.Set("Authorization", "Bearer "+userToken)
			resp2, err := http.DefaultClient.Do(req2)
			Expect(err).NotTo(HaveOccurred())
			defer resp2.Body.Close()
			var body struct {
				Notifications []map[string]interface{} `json:"notifications"`
			}
			Expect(json.NewDecoder(resp2.Body).Decode(&body)).To(Succeed())
			for _, n := range body.Notifications {
				if id, _ := n["id"].(string); id == notificationID {
					Expect(n["read"]).To(BeTrue(),
						"notification should be marked read for the same user")
					return
				}
			}
			Fail("notification " + notificationID + " not found in list after marking read")
		})
	})

	// Access Requests — e2e coverage for request / admin-list / approve flow.
	// userToken (test-user) submits the request; adminToken (admin group member)
	// lists and approves it.
	Context("Access Requests", func() {
		var (
			webapiBase  string
			pfCmd       *exec.Cmd
			// userToken is for the requester side (any authenticated user).
			userToken   string
			// adminToken is for admin-gated endpoints (requires "admin" group).
			adminToken    string
			serviceUID    string
			requestID     string
			denyRequestID string
			arAppName     = "test-ar-app"
		)

		BeforeAll(func() {
			By("Acquiring tokens from Keycloak")
			userToken  = acquireToken(kcTestUser, kcTestPassword)
			adminToken = acquireToken(kcAdminUser, kcAdminPassword)

			By("Creating a NebariApp to request access to")
			arApp := newNebariApp(arAppName, e2eNamespace,
				fmt.Sprintf("%s.nebari.test", arAppName), "authenticated", 77)
			Expect(k8sClient.Create(context.Background(), arApp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), arApp) })

			By("Port-forwarding to webapi on :18086")
			pfCmd = exec.Command("kubectl", "port-forward",
				"-n", e2eNamespace, fmt.Sprintf("svc/%s", e2eWebapiService), "18086:8080")
			Expect(pfCmd.Start()).NotTo(HaveOccurred())
			webapiBase = "http://localhost:18086"

			Eventually(func() error {
				resp, err := http.Get(webapiBase + "/api/v1/health")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 30*time.Second, time.Second).Should(Succeed())

			By("Waiting for watcher to surface the NebariApp and extracting service UID")
			Eventually(func() error {
				req, err := http.NewRequest(http.MethodGet, webapiBase+"/api/v1/services", nil)
				if err != nil {
					return err
				}
				req.Header.Set("Authorization", "Bearer "+userToken)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				var result ServiceListResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					return err
				}
				for _, svc := range result.Services {
					if name, _ := svc["name"].(string); name == "Test Service "+arAppName {
						if uid, _ := svc["id"].(string); uid != "" {
							serviceUID = uid
							return nil
						}
					}
				}
				return fmt.Errorf("service %q not yet visible", arAppName)
			}, VeryLongTimeout, 2*time.Second).Should(Succeed())
		})

		AfterAll(func() {
			if pfCmd != nil && pfCmd.Process != nil {
				_ = pfCmd.Process.Kill()
			}
		})

		It("should reject unauthenticated access requests with 401", func() {
			resp, err := http.Post(
				webapiBase+"/api/v1/services/"+serviceUID+"/request_access",
				"application/json", nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("should create an access request for an authenticated user", func() {
			// Body field must match RequestAccessBody.Message (json:"message")
			// declared in internal/api/handlers.go. The decoder doesn't reject
			// unknown fields, so a wrong key (e.g. "reason") would silently
			// persist an empty message and the test would still see 202.
			body := `{"message":"need access for e2e test"}`
			req, err := http.NewRequest(http.MethodPost,
				webapiBase+"/api/v1/services/"+serviceUID+"/request_access",
				strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted),
				"POST /api/v1/services/{uid}/request_access must return 202")
			var ar struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&ar)).To(Succeed())
			Expect(ar.ID).NotTo(BeEmpty())
			Expect(ar.Status).To(Equal("pending"))
			requestID = ar.ID
		})

		It("should reject non-admin GET /admin/access-requests with 403", func() {
			req, err := http.NewRequest(http.MethodGet,
				webapiBase+"/api/v1/admin/access-requests", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+userToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden),
				"non-admin GET /api/v1/admin/access-requests must be 403 (requireAdmin)")
		})

		It("should list the access request as admin", func() {
			req, err := http.NewRequest(http.MethodGet,
				webapiBase+"/api/v1/admin/access-requests", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+adminToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK),
				"GET /api/v1/admin/access-requests must return 200 (requires admin group)")
			var body struct {
				AccessRequests []map[string]interface{} `json:"accessRequests"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			// Find the request we just created and deep-assert the persisted
			// message field. This is the gate against the schema-mismatch class
			// of bug: the handler's JSON decoder doesn't DisallowUnknownFields,
			// so a wrong body key (e.g. "reason" vs "message") would still
			// return 202 with an empty message — only this assertion catches it.
			var found map[string]interface{}
			for _, r := range body.AccessRequests {
				if id, _ := r["id"].(string); id == requestID {
					found = r
					break
				}
			}
			Expect(found).NotTo(BeNil(), "created access request %s not in admin list", requestID)
			Expect(found["message"]).To(Equal("need access for e2e test"),
				"persisted message must match request body — empty value indicates the body key didn't match RequestAccessBody.Message")
		})

		It("should reject non-admin PUT .../approve with 403", func() {
			req, err := http.NewRequest(http.MethodPut,
				webapiBase+"/api/v1/admin/access-requests/"+requestID+"/approve", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+userToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusForbidden),
				"non-admin PUT /api/v1/admin/access-requests/{id}/approve must be 403 (requireAdmin)")
		})

		It("should approve the access request as admin", func() {
			req, err := http.NewRequest(http.MethodPut,
				webapiBase+"/api/v1/admin/access-requests/"+requestID+"/approve", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+adminToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK),
				"PUT /api/v1/admin/access-requests/{id}/approve must return 200")
			var ar struct {
				Status string `json:"status"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&ar)).To(Succeed())
			Expect(ar.Status).To(Equal("approved"))
		})

		It("should create a second access request for the deny path", func() {
			// A second AR is needed because the first was just approved; once
			// resolved, an AR can't be denied. Same user/service is fine since
			// ErrDuplicatePending only triggers while a request is still pending.
			body := `{"message":"second request to be denied"}`
			req, err := http.NewRequest(http.MethodPost,
				webapiBase+"/api/v1/services/"+serviceUID+"/request_access",
				strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+userToken)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
			var ar struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&ar)).To(Succeed())
			Expect(ar.ID).NotTo(BeEmpty())
			Expect(ar.Status).To(Equal("pending"))
			denyRequestID = ar.ID
		})

		It("should deny the access request as admin", func() {
			req, err := http.NewRequest(http.MethodPut,
				webapiBase+"/api/v1/admin/access-requests/"+denyRequestID+"/deny", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+adminToken)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK),
				"PUT /api/v1/admin/access-requests/{id}/deny must return 200")
			var ar struct {
				Status string `json:"status"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&ar)).To(Succeed())
			Expect(ar.Status).To(Equal("denied"))
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
