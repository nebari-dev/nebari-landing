package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// ResourcesResponse is the JSON body returned by GET /api/v1/cluster/resources.
type ResourcesResponse struct {
	Resources ResourceUtilization `json:"resources"`
	// Note is a human-readable explanation of how the percentages are computed,
	// to avoid confusion with actual real-time CPU/memory usage metrics.
	Note string `json:"note"`
}

// ResourceUtilization holds cluster-wide resource request percentages.
type ResourceUtilization struct {
	// CPUPercent is the fraction of total allocatable CPU requested by all
	// non-terminal pods, expressed as an integer percentage (0-100+).
	CPUPercent int `json:"cpu_percent"`
	// MemoryPercent is the fraction of total allocatable memory requested.
	MemoryPercent int `json:"memory_percent"`
}

// Resources returns cluster-wide resource utilization computed from pod
// resource requests versus node allocatable capacity.
//
// These are request-based percentages, not actual runtime usage. Values above
// 100 indicate over-commitment (requests exceed allocatable capacity).
func (c *Client) Resources(ctx context.Context) (*ResourcesResponse, error) {
	var nodeList corev1.NodeList
	if err := c.k8s.List(ctx, &nodeList); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	var podList corev1.PodList
	if err := c.k8s.List(ctx, &podList); err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	// Sum allocatable capacity across all nodes.
	var totalCPUMillis, totalMemBytes int64
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		totalCPUMillis += node.Status.Allocatable.Cpu().MilliValue()
		totalMemBytes += node.Status.Allocatable.Memory().Value()
	}

	// Sum resource requests across all non-terminal pods.
	// Init containers run sequentially; we add the heaviest one per pod
	// (conservative: reflects the worst-case scheduling pressure).
	var reqCPUMillis, reqMemBytes int64
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		for j := range pod.Spec.Containers {
			reqCPUMillis += pod.Spec.Containers[j].Resources.Requests.Cpu().MilliValue()
			reqMemBytes += pod.Spec.Containers[j].Resources.Requests.Memory().Value()
		}
		// Include the heaviest init container (they run sequentially, not concurrently).
		var initCPU, initMem int64
		for j := range pod.Spec.InitContainers {
			if v := pod.Spec.InitContainers[j].Resources.Requests.Cpu().MilliValue(); v > initCPU {
				initCPU = v
			}
			if v := pod.Spec.InitContainers[j].Resources.Requests.Memory().Value(); v > initMem {
				initMem = v
			}
		}
		reqCPUMillis += initCPU
		reqMemBytes += initMem
	}

	var resp ResourcesResponse
	resp.Note = "Percentages reflect pod resource requests versus node allocatable capacity, not real-time usage."
	if totalCPUMillis > 0 {
		resp.Resources.CPUPercent = int(reqCPUMillis * 100 / totalCPUMillis)
	}
	if totalMemBytes > 0 {
		resp.Resources.MemoryPercent = int(reqMemBytes * 100 / totalMemBytes)
	}
	return &resp, nil
}
