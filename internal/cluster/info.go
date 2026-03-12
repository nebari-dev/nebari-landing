package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InfoResponse is the JSON body returned by GET /api/v1/cluster/info.
type InfoResponse struct {
	Cluster ClusterMeta `json:"cluster"`
}

// ClusterMeta holds cluster-level metadata shown in the footer bar.
type ClusterMeta struct {
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Region     string `json:"region"`
	K8sVersion string `json:"k8s_version"`
	// LastDeploy is the RFC3339 timestamp of the most recent completed ArgoCD
	// operation across all Application CRs. Empty when ArgoCD is not installed
	// or no operations have completed.
	LastDeploy string `json:"last_deploy,omitempty"`
	Status     string `json:"status"`
}

// Info returns cluster metadata aggregated from the Kubernetes API and
// ArgoCD Application CRs (when available).
func (c *Client) Info(ctx context.Context) (*InfoResponse, error) {
	// Kubernetes server version via the discovery API.
	sv, err := c.disc.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("server version: %w", err)
	}

	// Sample one node to detect the cloud provider and region.
	var nodeList corev1.NodeList
	if err := c.k8s.List(ctx, &nodeList, &client.ListOptions{Limit: 3}); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	provider, region := "unknown", "unknown"
	if len(nodeList.Items) > 0 {
		provider, region = detectProviderRegion(&nodeList.Items[0])
	}

	// Best-effort: pull the most recent ArgoCD deploy timestamp.
	lastDeploy := latestArgoCDDeploy(ctx, c.k8s)

	return &InfoResponse{
		Cluster: ClusterMeta{
			Name:       c.clusterName,
			Provider:   provider,
			Region:     region,
			K8sVersion: sv.GitVersion,
			LastDeploy: lastDeploy,
			Status:     "active",
		},
	}, nil
}

// detectProviderRegion inspects a node's labels and providerID to infer the
// cloud provider and geographic region.
func detectProviderRegion(node *corev1.Node) (provider, region string) {
	// Standard topology labels (CSI / cloud-provider-X).
	region = node.Labels["topology.kubernetes.io/region"]
	if region == "" {
		// Legacy beta label kept for older clusters.
		region = node.Labels["failure-domain.beta.kubernetes.io/region"]
	}
	if region == "" {
		region = "unknown"
	}

	pid := node.Spec.ProviderID
	switch {
	case strings.HasPrefix(pid, "aws://"),
		node.Labels["eks.amazonaws.com/nodegroup"] != "",
		node.Labels["alpha.eksctl.io/cluster-name"] != "":
		return "AWS", region
	case strings.HasPrefix(pid, "gce://"),
		node.Labels["cloud.google.com/gke-nodepool"] != "":
		return "GCP", region
	case strings.HasPrefix(pid, "azure://"),
		node.Labels["kubernetes.azure.com/agentpool"] != "":
		return "Azure", region
	case node.Labels["node.k3s.io/instance-type"] != "",
		strings.HasPrefix(pid, "k3s://"):
		return "k3s", region
	default:
		return "on-prem", region
	}
}

// argoCDAppListGVK is the GroupVersionKind for an ArgoCD ApplicationList.
var argoCDAppListGVK = schema.GroupVersionKind{
	Group:   "argoproj.io",
	Version: "v1alpha1",
	Kind:    "ApplicationList",
}

// latestArgoCDDeploy queries ArgoCD Application CRs and returns the RFC3339
// timestamp of the most recently completed sync operation. Returns "" when
// ArgoCD is not installed, RBAC denies access, or no completed ops are found.
func latestArgoCDDeploy(ctx context.Context, k8s client.Client) string {
	appList := &unstructured.UnstructuredList{}
	appList.SetGroupVersionKind(argoCDAppListGVK)

	// List across all namespaces; RBAC will restrict as configured.
	if err := k8s.List(ctx, appList); err != nil {
		// ArgoCD not installed or no RBAC access — silently skip.
		return ""
	}

	var latest time.Time
	for i := range appList.Items {
		ts, found, err := unstructured.NestedString(
			appList.Items[i].Object,
			"status", "operationState", "finishedAt",
		)
		if err != nil || !found || ts == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		if t.After(latest) {
			latest = t
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.UTC().Format(time.RFC3339)
}
