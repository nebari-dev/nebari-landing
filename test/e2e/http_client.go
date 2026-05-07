//go:build e2e
// +build e2e

// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// WebAPIClient wraps HTTP access to the webapi service
type WebAPIClient struct {
	BaseURL    string
	HTTPClient *http.Client
	BearerToken string
}

// NewWebAPIClient creates a client that talks directly to the webapi service
// without requiring port-forwarding. This is more reliable for e2e tests.
//
// It automatically detects if running in-cluster (CI) or out-of-cluster (local dev)
// and uses the appropriate service URL.
func NewWebAPIClient(config *rest.Config, namespace, serviceName string) (*WebAPIClient, error) {
	// Check if running in-cluster by attempting to detect service
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s/%s: %w", namespace, serviceName, err)
	}

	// Determine service URL based on environment
	// In CI (k3d, kind, etc), use cluster DNS
	// For local port-forwards, this could be extended to try localhost first
	var baseURL string
	
	// First, check if we're in-cluster by looking for service account token
	if _, err := rest.InClusterConfig(); err == nil {
		// In-cluster: use service DNS name
		baseURL = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
			serviceName, namespace, getServicePort(svc))
	} else {
		// Out-of-cluster: assume port-forward or NodePort
		// This could be enhanced to auto-detect or configure via env var
		baseURL = fmt.Sprintf("http://localhost:%d", getServicePort(svc))
	}

	return &WebAPIClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// NewRequest creates an HTTP request with bearer token if set
func (c *WebAPIClient) NewRequest(method, path string) (*http.Request, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	
	if c.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	}
	
	return req, nil
}

// Do executes an HTTP request
func (c *WebAPIClient) Do(req *http.Request) (*http.Response, error) {
	return c.HTTPClient.Do(req)
}

// Health checks the /api/v1/health endpoint
func (c *WebAPIClient) Health(ctx context.Context) error {
	req, err := c.NewRequest(http.MethodGet, "/api/v1/health")
	if err != nil {
		return err
	}
	
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}
	
	return nil
}

// getServicePort extracts the first port from a service
func getServicePort(svc *corev1.Service) int32 {
	if len(svc.Spec.Ports) > 0 {
		return svc.Spec.Ports[0].Port
	}
	return 8080 // default fallback
}
