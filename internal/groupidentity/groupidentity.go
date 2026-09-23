// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package groupidentity normalizes Keycloak group identities for authorization.
//
// Landing authorization uses Keycloak group full paths, not leaf names. A
// configured group and a token claim must both carry full paths such as
// "/division-a/research" to match.
package groupidentity

import "strings"

// RequiredPath canonicalizes a configured group reference. Bare leaf names are
// rejected so operators must make an explicit migration decision when moving
// from leaf-name groups to full Keycloak paths.
func RequiredPath(group string) (string, bool) {
	group = strings.TrimSpace(group)
	if group == "" || !strings.HasPrefix(group, "/") {
		return "", false
	}
	parts := cleanParts(group)
	if len(parts) == 0 {
		return "", false
	}
	return "/" + strings.Join(parts, "/"), true
}

// RequiredPaths canonicalizes a slice of full-path group references, dropping
// invalid entries and preserving first-seen order.
func RequiredPaths(groups []string) []string {
	seen := make(map[string]struct{}, len(groups))
	paths := make([]string, 0, len(groups))
	for _, group := range groups {
		path, ok := RequiredPath(group)
		if !ok {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

// InvalidRequiredPaths returns non-empty configured group references that are
// not valid explicit full paths.
func InvalidRequiredPaths(groups []string) []string {
	seen := make(map[string]struct{}, len(groups))
	invalid := make([]string, 0)
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := RequiredPath(group); ok {
			continue
		}
		if _, exists := seen[group]; exists {
			continue
		}
		seen[group] = struct{}{}
		invalid = append(invalid, group)
	}
	return invalid
}

// TokenPath canonicalizes a group value from a JWT claim. It rejects leaf-only
// values because Keycloak can emit the same leaf for distinct subgroup paths.
func TokenPath(group string) (string, bool) {
	group = strings.TrimSpace(group)
	if !strings.HasPrefix(group, "/") {
		return "", false
	}
	return RequiredPath(group)
}

// HasIntersection reports whether any JWT group path equals any configured
// required group path.
func HasIntersection(userGroups, requiredGroups []string) bool {
	if len(requiredGroups) == 0 {
		return true
	}

	required := make(map[string]struct{}, len(requiredGroups))
	for _, group := range requiredGroups {
		path, ok := RequiredPath(group)
		if ok {
			required[path] = struct{}{}
		}
	}
	if len(required) == 0 {
		return false
	}

	for _, group := range userGroups {
		path, ok := TokenPath(group)
		if !ok {
			continue
		}
		if _, exists := required[path]; exists {
			return true
		}
	}
	return false
}

// Leaf returns the final path segment of a canonical group path.
func Leaf(groupPath string) string {
	parts := cleanParts(groupPath)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// IsRootPath reports whether groupPath identifies a top-level Keycloak group.
func IsRootPath(groupPath string) bool {
	parts := cleanParts(groupPath)
	return len(parts) == 1
}

func cleanParts(group string) []string {
	raw := strings.Split(group, "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
