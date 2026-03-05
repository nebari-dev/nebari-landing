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
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		namespace     = "nebari-system"
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

		By("Applying webapi manifest (deploy/manifest.yaml)")
		cmd = exec.Command("kubectl", "apply", "-f", "deploy/manifest.yaml")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to apply deploy/manifest.yaml")

		By("Waiting for webapi deployment to become ready")
		rollout := exec.Command("kubectl", "rollout", "status", "deployment/webapi",
			"-n", namespace, "--timeout=2m")
		_, err = utils.Run(rollout)
		Expect(err).NotTo(HaveOccurred(), "webapi deployment should become ready")

		By("Starting Keycloak port-forward for auth tests")
		keycloakPFCmd = exec.Command("kubectl", "port-forward",
			"-n", "keycloak", "svc/keycloak-keycloakx-http",
			"18090:80")
		Expect(keycloakPFCmd.Start()).NotTo(HaveOccurred(), "keycloak port-forward should start")
		Eventually(func() error {
			resp, err := http.Get("http://localhost:18090/auth/realms/nebari/.well-known/openid-configuration")
			if err != nil {
				return err
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("keycloak not ready: status %d", resp.StatusCode)
			}
			return nil
		}, 30*time.Second, time.Second).Should(Succeed(),
			"keycloak should be reachable via port-forward")
	})

	AfterAll(func() {
		By("Deleting test NebariApp resources")
		for _, name := range []string{testAppName, "test-auth-visibility"} {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(nebariAppGVK)
			u.SetName(name)
			u.SetNamespace(namespace)
			_ = k8sClient.Delete(ctx, u)
		}

		By("Stopping Keycloak port-forward")
		if keycloakPFCmd != nil && keycloakPFCmd.Process != nil {
			_ = keycloakPFCmd.Process.Kill()
		}

		By("Removing webapi manifests")
		cmd := exec.Command("kubectl", "delete", "-f", "deploy/manifest.yaml", "--ignore-not-found")
		_, _ = utils.Run(cmd)
	})

	Context("Service Discovery", func() {
		It("should expose a Service object", func() {
			svc := &corev1.Service{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      "webapi",
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
				"-n", namespace, "svc/webapi", "18082:8080")
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
			Expect(serviceNames(result)).To(ContainElement(testAppName),
				"Public services must appear when unauthenticated")
		})

		It("should filter services based on visibility", func() {
			By("Acquiring a JWT from Keycloak (host-header override for issuer match)")
			// The port-forward reaches Keycloak at localhost:18090, but the Host header
			// must match the in-cluster hostname so the token issuer ("iss") equals
			// KEYCLOAK_URL/realms/nebari, which is what the webapi validates.
			tokenForm := url.Values{
				"client_id":  {"admin-cli"},
				"username":   {"admin"},
				"password":   {"nebari-admin"},
				"grant_type": {"password"},
				"scope":      {"openid profile"},
			}
			tokenReq, err := http.NewRequest(http.MethodPost,
				"http://localhost:18090/auth/realms/nebari/protocol/openid-connect/token",
				strings.NewReader(tokenForm.Encode()))
			Expect(err).NotTo(HaveOccurred())
			tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			tokenReq.Host = "keycloak-keycloakx-http.keycloak.svc.cluster.local"
			tokenResp, err := http.DefaultClient.Do(tokenReq)
			Expect(err).NotTo(HaveOccurred(), "Should be able to request token from Keycloak")
			defer tokenResp.Body.Close()
			Expect(tokenResp.StatusCode).To(Equal(http.StatusOK),
				"Keycloak token request must succeed (realm=nebari, user=admin/nebari-admin)")
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
				"-n", namespace, "svc/webapi", "18081:8080")
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
			Expect(serviceNames(unauthResult)).NotTo(ContainElement("test-auth-visibility"),
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
			Expect(serviceNames(authResult)).To(ContainElement("test-auth-visibility"),
				"The authenticated-visibility NebariApp must appear when a valid JWT is presented")
		})
	})

	Context("Health Checks", func() {
		It("should report healthy status", func() {
			By("Port-forwarding to webapi (port 18080)")
			pfCmd := exec.Command("kubectl", "port-forward",
				"-n", namespace, "svc/webapi", "18080:8080")
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

	Context("Frontend Serving", func() {
		It("should serve static frontend files", func() {
			Skip("Covered by unit tests; requires ingress/port-forward setup")
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

// makeAuthenticatedRequest issues a GET to endpoint, optionally adding a Bearer token.
func makeAuthenticatedRequest(endpoint, token string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return (&http.Client{Timeout: 10 * time.Second}).Do(req)
}

// getServiceList fetches and decodes the unauthenticated service list.
func getServiceList(endpoint string) (*ServiceListResponse, error) {
	resp, err := makeAuthenticatedRequest(endpoint, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}
	var result ServiceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ServiceListUser is kept for completeness; identity is now returned by
// GET /api/v1/caller-identity.
type ServiceListUser struct {
	Authenticated bool     `json:"authenticated"`
	Username      string   `json:"username,omitempty"`
	Email         string   `json:"email,omitempty"`
	Name          string   `json:"name,omitempty"`
	Groups        []string `json:"groups,omitempty"`
}

// _ are compile-time checks for unused exported helpers.
var (
	_ = makeAuthenticatedRequest
	_ = getServiceList
	_ = ServiceListUser{}
	_ = metav1.ObjectMeta{}
)
