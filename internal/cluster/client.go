// Package cluster provides methods for querying cluster-level metadata,
// node capacity, and resource utilization directly from the Kubernetes API.
// No external dependencies (Prometheus, metrics-server) are required.
package cluster

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client wraps a Kubernetes API client and discovery client to expose
// cluster-level information for the landing-page overview endpoints.
type Client struct {
	k8s         client.Client
	disc        *discovery.DiscoveryClient
	clusterName string
	minNodes    int
}

// New creates a new cluster Client.
//
//   - clusterName is the human-readable name shown in GET /api/v1/cluster/info
//     (set via the --cluster-name flag / CLUSTER_NAME env var).
//   - minNodes is reported as-is in GET /api/v1/cluster/nodes; set to 0 when
//     not configured (omitted from the response when 0).
func New(cfg *rest.Config, k8s client.Client, clusterName string, minNodes int) (*Client, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		k8s:         k8s,
		disc:        dc,
		clusterName: clusterName,
		minNodes:    minNodes,
	}, nil
}
