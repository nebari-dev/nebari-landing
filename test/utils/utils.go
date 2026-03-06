// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package utils provides helpers for running shell commands in e2e tests.
package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,staticcheck
)

// Run executes the provided command rooted at the project directory and returns
// its combined stdout+stderr output. Any non-zero exit code is wrapped as an error.
func Run(cmd *exec.Cmd) (string, error) {
	dir, err := GetProjectDir()
	if err != nil {
		return "", fmt.Errorf("could not determine project dir: %w", err)
	}
	cmd.Dir = dir
	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir %q: %v\n", cmd.Dir, err)
	}
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with %q: %w", command, string(output), err)
	}
	return string(output), nil
}

// GetProjectDir returns the directory containing the go.mod file (the module root).
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Walk up until we find go.mod
	dir := wd
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir, nil
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd, nil
}

// GetNonEmptyLines returns lines from output that are not blank.
func GetNonEmptyLines(output string) []string {
	var res []string
	scanner := bufio.NewScanner(bytes.NewBufferString(output))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			res = append(res, line)
		}
	}
	return res
}

// LoadImageToKindCluster loads a container image into the default (or
// KIND_CLUSTER env) Kind cluster.
func LoadImageToKindCluster(name string) error {
	cluster := os.Getenv("KIND_CLUSTER")
	if cluster == "" {
		cluster = "nebari-operator-dev"
	}
	cmd := exec.Command("kind", "load", "docker-image", name, "--name", cluster)
	_, err := Run(cmd)
	return err
}

// LoadImageToCluster loads a container image into the appropriate cluster based
// on clusterType ("kind" or "minikube").  profile is used as the minikube
// profile name when clusterType is "minikube".
func LoadImageToCluster(name, clusterType, profile string) error {
	switch clusterType {
	case "minikube":
		cmd := exec.Command("minikube", "-p", profile, "image", "load", name)
		_, err := Run(cmd)
		return err
	default: // "kind"
		return LoadImageToKindCluster(name)
	}
}
