package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// NodesResponse is the JSON body returned by GET /api/v1/cluster/nodes.
type NodesResponse struct {
	Nodes     NodesCapacity `json:"nodes"`
	NodeTypes NodeTypeCount `json:"node_types"`
}

// NodesCapacity holds aggregate node counts.
type NodesCapacity struct {
	Total     int `json:"total"`
	InUse     int `json:"in_use"`
	Available int `json:"available"`
	Min       int `json:"min,omitempty"`
}

// NodeTypeCount breaks down nodes by workload type.
type NodeTypeCount struct {
	CPU int `json:"cpu"`
	GPU int `json:"gpu"`
}

// Nodes returns cluster node capacity information.
//
// "in_use" is defined as nodes that have at least one non-DaemonSet pod in
// Running phase scheduled on them. DaemonSet pods are excluded because every
// schedulable node runs them, so they do not indicate workload pressure.
// "gpu" nodes are those with nvidia.com/gpu capacity > 0.
func (c *Client) Nodes(ctx context.Context) (*NodesResponse, error) {
	var nodeList corev1.NodeList
	if err := c.k8s.List(ctx, &nodeList); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	var podList corev1.PodList
	if err := c.k8s.List(ctx, &podList); err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	// Build the set of node names that have at least one Running workload pod.
	nodeHasWorkload := make(map[string]bool, len(nodeList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if isDaemonSetPod(pod) {
			continue
		}
		if pod.Spec.NodeName != "" {
			nodeHasWorkload[pod.Spec.NodeName] = true
		}
	}

	var resp NodesResponse
	resp.Nodes.Min = c.minNodes

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		resp.Nodes.Total++

		// GPU node: nvidia.com/gpu capacity resource present and non-zero.
		if q, ok := node.Status.Capacity["nvidia.com/gpu"]; ok && !q.IsZero() {
			resp.NodeTypes.GPU++
		} else {
			resp.NodeTypes.CPU++
		}

		if nodeHasWorkload[node.Name] {
			resp.Nodes.InUse++
		}
	}

	resp.Nodes.Available = resp.Nodes.Total - resp.Nodes.InUse
	return &resp, nil
}

// isDaemonSetPod returns true when the pod is owned by a DaemonSet controller.
func isDaemonSetPod(pod *corev1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}
