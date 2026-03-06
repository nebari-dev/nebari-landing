//go:build e2e
// +build e2e

// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nebari-dev/nebari-landing/test/utils"
)

var (
	// USE_EXISTING_CLUSTER=true — skip cluster creation and image loading.
	useExistingCluster = os.Getenv("USE_EXISTING_CLUSTER") == "true"

	// SKIP_DOCKER_BUILD=true — assume the image is already built/loaded.
	skipDockerBuild = os.Getenv("SKIP_DOCKER_BUILD") == "true"

	// CLUSTER_TYPE controls how images are loaded when not using an existing
	// cluster.  Supported values: "kind" (default), "minikube".
	clusterType = func() string {
		if v := os.Getenv("CLUSTER_TYPE"); v != "" {
			return v
		}
		return "kind"
	}()

	// MINIKUBE_PROFILE — minikube profile to target when CLUSTER_TYPE=minikube.
	minikubeProfile = func() string {
		if v := os.Getenv("MINIKUBE_PROFILE"); v != "" {
			return v
		}
		return "minikube"
	}()

	// WEBAPI_IMG — image to deploy.  Defaults to the dev tag.
	webapiImage = func() string {
		if v := os.Getenv("WEBAPI_IMG"); v != "" {
			return v
		}
		return "nebari-landing/webapi:dev"
	}()

	// KIND_CLUSTER — name of the Kind cluster when CLUSTER_TYPE=kind and not
	// using an existing cluster.
	kindCluster = func() string {
		if v := os.Getenv("KIND_CLUSTER"); v != "" {
			return v
		}
		return "nebari-operator-dev"
	}()

	isKindClusterCreated bool

	// k8sClient is available to all e2e specs.
	// NOTE: The scheme only carries core k8s types.  NebariApp resources are
	// created via unstructured.Unstructured — no operator/api import needed.
	k8sClient client.Client
)

// TestE2E is the Ginkgo test runner registered for `go test -tags=e2e`.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting nebari-landing e2e test suite\n")
	RunSpecs(t, "nebari-landing e2e suite")
}

var _ = BeforeSuite(func() {
	if !useExistingCluster {
		if clusterType == "kind" {
			By("creating Kind cluster")
			cmd := exec.Command("kind", "create", "cluster",
				"--name", kindCluster,
				"--wait", "120s")
			_, err := utils.Run(cmd)
			if err == nil {
				isKindClusterCreated = true
			}
			ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create Kind cluster")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "CLUSTER_TYPE=%s — skipping cluster creation (must already exist)\n", clusterType)
		}
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "USE_EXISTING_CLUSTER=true — using current kubeconfig context\n")
	}

	if !skipDockerBuild && !useExistingCluster {
		By("building the webapi image")
		cmd := exec.Command("docker", "build", "-t", webapiImage, ".")
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build webapi image")

		By(fmt.Sprintf("loading webapi image into %s cluster", clusterType))
		ExpectWithOffset(1, utils.LoadImageToCluster(webapiImage, clusterType, minikubeProfile)).
			To(Succeed(), "Failed to load webapi image into cluster")
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping docker build/image load\n")
	}

	// Prerequisites that must already exist in the cluster:
	//   - NebariApp CRDs (install via: make install in nebari-operator)
	//   - nebari-system namespace
	// The e2e test itself deploys / tears down the webapi Deployment.

	By("setting up k8s client")
	scheme := runtime.NewScheme()
	ExpectWithOffset(1, clientgoscheme.AddToScheme(scheme)).To(Succeed())
	cfg := ctrl.GetConfigOrDie()
	var err error
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create k8s client")
})

var _ = AfterSuite(func() {
	if isKindClusterCreated {
		By("deleting Kind cluster")
		cmd := exec.Command("kind", "delete", "cluster", "--name", kindCluster)
		if _, err := utils.Run(cmd); err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "warning: failed to delete Kind cluster: %v\n", err)
		}
	}
})
