// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package useridentity contains helpers for deriving durable user identity keys
// from immutable JWT claims.
package useridentity

import "encoding/base64"

// StableID returns the Redis-safe durable user key for an issuer and subject.
func StableID(issuer, subject string) string {
	if issuer == "" || subject == "" {
		return ""
	}
	enc := base64.RawURLEncoding
	return "iss:" + enc.EncodeToString([]byte(issuer)) + ":sub:" + enc.EncodeToString([]byte(subject))
}
