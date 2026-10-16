// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package identitymigration migrates legacy Redis user-state keys from mutable
// usernames to stable issuer+subject identity keys.
package identitymigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nebari-dev/nebari-landing/internal/accessrequests"
	"github.com/nebari-dev/nebari-landing/internal/useridentity"
	"github.com/redis/go-redis/v9"
)

const (
	pinsPrefix     = "nebari:pins:"
	readsPrefix    = "nebari:reads:"
	arHashPrefix   = "nebari:ar:"
	arAllKey       = "nebari:ar:all"
	arUserPrefix   = "nebari:ar:user:"
	arDedupPrefix  = "nebari:ar:dedup:"
	arActivePrefix = "nebari:ar:active:"
)

// ErrCollisions is returned by Apply when the plan contains collisions.
var ErrCollisions = errors.New("identity migration collisions detected")

// Mapping binds one legacy username key to the immutable Keycloak issuer and
// subject that should own the migrated state.
type Mapping struct {
	Username        string `json:"username"`
	Issuer          string `json:"issuer"`
	Subject         string `json:"subject"`
	DisplayUsername string `json:"displayUsername,omitempty"`
}

func (m Mapping) normalized() Mapping {
	m.Username = strings.TrimSpace(m.Username)
	m.Issuer = strings.TrimSpace(m.Issuer)
	m.Subject = strings.TrimSpace(m.Subject)
	m.DisplayUsername = strings.TrimSpace(m.DisplayUsername)
	if m.DisplayUsername == "" {
		m.DisplayUsername = m.Username
	}
	return m
}

// KeyMove records a legacy Redis key that can be renamed.
type KeyMove struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Target string `json:"target"`
	Count  int64  `json:"count"`
}

// AccessRequestRewrite records the hash/index/dedup migration for one access request.
type AccessRequestRewrite struct {
	ID               string `json:"id"`
	ServiceUID       string `json:"serviceUID,omitempty"`
	OldUserID        string `json:"oldUserID,omitempty"`
	NewUserID        string `json:"newUserID,omitempty"`
	UserIssuer       string `json:"userIssuer,omitempty"`
	UserSubject      string `json:"userSubject,omitempty"`
	Username         string `json:"username,omitempty"`
	OldResolvedBy    string `json:"oldResolvedBy,omitempty"`
	NewResolvedBy    string `json:"newResolvedBy,omitempty"`
	ResolverIssuer   string `json:"resolverIssuer,omitempty"`
	ResolverSubject  string `json:"resolverSubject,omitempty"`
	ResolverUsername string `json:"resolverUsername,omitempty"`
	Pending          bool   `json:"pending,omitempty"`
}

// Collision reports a condition that must be resolved before rewriting keys.
type Collision struct {
	Kind   string `json:"kind"`
	Source string `json:"source,omitempty"`
	Target string `json:"target,omitempty"`
	Detail string `json:"detail"`
}

// Result is the dry-run plan and, after Apply, the applied migration summary.
type Result struct {
	DryRun         bool                   `json:"dryRun"`
	Applied        bool                   `json:"applied"`
	Mappings       int                    `json:"mappings"`
	KeyMoves       []KeyMove              `json:"keyMoves,omitempty"`
	AccessRequests []AccessRequestRewrite `json:"accessRequests,omitempty"`
	Collisions     []Collision            `json:"collisions,omitempty"`
}

// Migrator plans and applies Redis identity-key migrations.
type Migrator struct {
	Redis *redis.Client
}

// LoadMappings reads a JSON array of Mapping values from path.
func LoadMappings(path string) ([]Mapping, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var mappings []Mapping
	if err := json.Unmarshal(b, &mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

// Plan inspects Redis and returns the migration work plus any collisions.
func (m *Migrator) Plan(ctx context.Context, input []Mapping) (*Result, error) {
	mappings, err := normalizeMappings(input)
	if err != nil {
		return nil, err
	}
	result := &Result{DryRun: true, Mappings: len(mappings)}
	byUsername := make(map[string]Mapping, len(mappings))
	byTarget := make(map[string][]string)
	for _, mapping := range mappings {
		targetID := useridentity.StableID(mapping.Issuer, mapping.Subject)
		byUsername[mapping.Username] = mapping
		byTarget[targetID] = append(byTarget[targetID], mapping.Username)
	}

	targets := sortedKeys(byTarget)
	for _, target := range targets {
		sources := byTarget[target]
		if len(sources) > 1 {
			result.Collisions = append(result.Collisions, Collision{
				Kind:   "mapping",
				Target: target,
				Detail: fmt.Sprintf("multiple legacy usernames map to the same stable identity: %s", strings.Join(sources, ", ")),
			})
		}
	}

	usernames := sortedKeys(byUsername)
	for _, username := range usernames {
		mapping := byUsername[username]
		targetID := useridentity.StableID(mapping.Issuer, mapping.Subject)
		if err := m.planSetMove(ctx, result, "pins", pinsPrefix+username, pinsPrefix+targetID); err != nil {
			return nil, err
		}
		if err := m.planSetMove(ctx, result, "notification_reads", readsPrefix+username, readsPrefix+targetID); err != nil {
			return nil, err
		}
	}

	if err := m.planAccessRequests(ctx, result, byUsername); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeMappings(input []Mapping) ([]Mapping, error) {
	if len(input) == 0 {
		return nil, errors.New("at least one identity mapping is required")
	}
	out := make([]Mapping, 0, len(input))
	seen := map[string]struct{}{}
	for i, mapping := range input {
		mapping = mapping.normalized()
		targetID := useridentity.StableID(mapping.Issuer, mapping.Subject)
		switch {
		case mapping.Username == "":
			return nil, fmt.Errorf("mapping %d missing username", i)
		case mapping.Issuer == "":
			return nil, fmt.Errorf("mapping %d for username %q missing issuer", i, mapping.Username)
		case mapping.Subject == "":
			return nil, fmt.Errorf("mapping %d for username %q missing subject", i, mapping.Username)
		case targetID == "":
			return nil, fmt.Errorf("mapping %d for username %q produced an empty target ID", i, mapping.Username)
		}
		if _, ok := seen[mapping.Username]; ok {
			return nil, fmt.Errorf("duplicate mapping for username %q", mapping.Username)
		}
		seen[mapping.Username] = struct{}{}
		out = append(out, mapping)
	}
	return out, nil
}

func (m *Migrator) planSetMove(ctx context.Context, result *Result, kind, source, target string) error {
	if source == target {
		return nil
	}
	sourceCount, err := m.Redis.SCard(ctx, source).Result()
	if err != nil {
		return fmt.Errorf("inspect %s source %q: %w", kind, source, err)
	}
	if sourceCount == 0 {
		return nil
	}
	targetCount, err := m.Redis.SCard(ctx, target).Result()
	if err != nil {
		return fmt.Errorf("inspect %s target %q: %w", kind, target, err)
	}
	if targetCount > 0 {
		result.Collisions = append(result.Collisions, Collision{
			Kind:   kind,
			Source: source,
			Target: target,
			Detail: "target key already exists; merge or delete it before applying migration",
		})
		return nil
	}
	result.KeyMoves = append(result.KeyMoves, KeyMove{
		Kind:   kind,
		Source: source,
		Target: target,
		Count:  sourceCount,
	})
	return nil
}

func (m *Migrator) planAccessRequests(ctx context.Context, result *Result, byUsername map[string]Mapping) error {
	ids, err := m.Redis.ZRange(ctx, arAllKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("list access requests: %w", err)
	}
	for _, id := range ids {
		fields, err := m.Redis.HGetAll(ctx, arHashPrefix+id).Result()
		if err != nil {
			return fmt.Errorf("read access request %q: %w", id, err)
		}
		if len(fields) == 0 {
			continue
		}
		rewrite := AccessRequestRewrite{
			ID:         id,
			ServiceUID: fields["serviceUID"],
			Pending:    fields["status"] == string(accessrequests.StatusPending),
		}
		if mapping, ok := byUsername[fields["userID"]]; ok {
			if subject := fields["userSubject"]; subject != "" && subject != mapping.Subject {
				result.Collisions = append(result.Collisions, Collision{
					Kind:   "access_request_user",
					Source: arHashPrefix + id,
					Detail: fmt.Sprintf("stored userSubject %q does not match mapping subject %q", subject, mapping.Subject),
				})
			} else {
				rewrite.OldUserID = fields["userID"]
				rewrite.NewUserID = useridentity.StableID(mapping.Issuer, mapping.Subject)
				rewrite.UserIssuer = mapping.Issuer
				rewrite.UserSubject = mapping.Subject
				rewrite.Username = mapping.DisplayUsername
				if rewrite.Pending {
					if err := m.planDedupMove(ctx, result, id, rewrite.OldUserID, rewrite.NewUserID, rewrite.ServiceUID); err != nil {
						return err
					}
				}
				if err := m.planActiveMove(ctx, result, id, rewrite.OldUserID, rewrite.NewUserID, rewrite.ServiceUID); err != nil {
					return err
				}
			}
		}
		if mapping, ok := byUsername[fields["resolvedBy"]]; ok {
			if subject := fields["resolvedBySubject"]; subject != "" && subject != mapping.Subject {
				result.Collisions = append(result.Collisions, Collision{
					Kind:   "access_request_resolver",
					Source: arHashPrefix + id,
					Detail: fmt.Sprintf("stored resolvedBySubject %q does not match mapping subject %q", subject, mapping.Subject),
				})
			} else {
				rewrite.OldResolvedBy = fields["resolvedBy"]
				rewrite.NewResolvedBy = useridentity.StableID(mapping.Issuer, mapping.Subject)
				rewrite.ResolverIssuer = mapping.Issuer
				rewrite.ResolverSubject = mapping.Subject
				rewrite.ResolverUsername = mapping.DisplayUsername
			}
		}
		if rewrite.NewUserID != "" || rewrite.NewResolvedBy != "" {
			result.AccessRequests = append(result.AccessRequests, rewrite)
		}
	}
	return nil
}

func (m *Migrator) planDedupMove(ctx context.Context, result *Result, requestID, oldUserID, newUserID, serviceUID string) error {
	if oldUserID == "" || newUserID == "" || serviceUID == "" {
		return nil
	}
	source := arDedupPrefix + oldUserID + ":" + serviceUID
	target := arDedupPrefix + newUserID + ":" + serviceUID
	if source == target {
		return nil
	}
	sourceValue, err := m.Redis.Get(ctx, source).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect dedup source %q: %w", source, err)
	}
	targetValue, err := m.Redis.Get(ctx, target).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("inspect dedup target %q: %w", target, err)
	}
	if err == nil && targetValue != sourceValue {
		result.Collisions = append(result.Collisions, Collision{
			Kind:   "access_request_dedup",
			Source: source,
			Target: target,
			Detail: fmt.Sprintf("target dedup key already points to request %q, source points to %q", targetValue, sourceValue),
		})
		return nil
	}
	if sourceValue != requestID {
		result.Collisions = append(result.Collisions, Collision{
			Kind:   "access_request_dedup",
			Source: source,
			Detail: fmt.Sprintf("source dedup key points to request %q, expected %q", sourceValue, requestID),
		})
	}
	return nil
}

func (m *Migrator) planActiveMove(ctx context.Context, result *Result, requestID, oldUserID, newUserID, serviceUID string) error {
	if oldUserID == "" || newUserID == "" || serviceUID == "" {
		return nil
	}
	source := arActivePrefix + oldUserID + ":" + serviceUID
	target := arActivePrefix + newUserID + ":" + serviceUID
	if source == target {
		return nil
	}
	sourceValue, err := m.Redis.Get(ctx, source).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect active access source %q: %w", source, err)
	}
	if sourceValue != requestID {
		return nil
	}
	targetValue, err := m.Redis.Get(ctx, target).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("inspect active access target %q: %w", target, err)
	}
	if err == nil {
		result.Collisions = append(result.Collisions, Collision{
			Kind:   "access_request_active",
			Source: source,
			Target: target,
			Detail: fmt.Sprintf("target active key already points to request %q, source points to %q", targetValue, sourceValue),
		})
		return nil
	}
	result.KeyMoves = append(result.KeyMoves, KeyMove{
		Kind:   "access_request_active",
		Source: source,
		Target: target,
		Count:  1,
	})
	return nil
}

// Apply rewrites Redis according to a collision-free plan.
func (m *Migrator) Apply(ctx context.Context, result *Result) error {
	if result == nil {
		return errors.New("migration result is nil")
	}
	if len(result.Collisions) > 0 {
		return ErrCollisions
	}
	for _, move := range result.KeyMoves {
		if err := m.applyKeyMove(ctx, move); err != nil {
			return err
		}
	}
	for _, rewrite := range result.AccessRequests {
		if err := m.applyAccessRequestRewrite(ctx, rewrite); err != nil {
			return err
		}
	}
	result.DryRun = false
	result.Applied = true
	return nil
}

func (m *Migrator) applyKeyMove(ctx context.Context, move KeyMove) error {
	exists, err := m.Redis.Exists(ctx, move.Source).Result()
	if err != nil {
		return fmt.Errorf("check source key %q: %w", move.Source, err)
	}
	if exists == 0 || move.Source == move.Target {
		return nil
	}
	targetExists, err := m.Redis.Exists(ctx, move.Target).Result()
	if err != nil {
		return fmt.Errorf("check target key %q: %w", move.Target, err)
	}
	if targetExists != 0 {
		return fmt.Errorf("%w: target key %q now exists", ErrCollisions, move.Target)
	}
	if err := m.Redis.Rename(ctx, move.Source, move.Target).Err(); err != nil {
		return fmt.Errorf("rename %s key %q to %q: %w", move.Kind, move.Source, move.Target, err)
	}
	return nil
}

func (m *Migrator) applyAccessRequestRewrite(ctx context.Context, rewrite AccessRequestRewrite) error {
	hashKey := arHashPrefix + rewrite.ID
	fields, err := m.Redis.HGetAll(ctx, hashKey).Result()
	if err != nil {
		return fmt.Errorf("read access request %q: %w", rewrite.ID, err)
	}
	if len(fields) == 0 {
		return nil
	}
	if rewrite.NewUserID != "" {
		if err := m.moveUserIndex(ctx, rewrite, fields["requestedAt"]); err != nil {
			return err
		}
		if err := m.moveDedup(ctx, rewrite); err != nil {
			return err
		}
	}

	_, err = m.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		if rewrite.NewUserID != "" {
			pipe.HSet(ctx, hashKey,
				"userID", rewrite.NewUserID,
				"userIssuer", rewrite.UserIssuer,
				"userSubject", rewrite.UserSubject,
				"username", rewrite.Username,
			)
		}
		if rewrite.NewResolvedBy != "" {
			pipe.HSet(ctx, hashKey,
				"resolvedBy", rewrite.NewResolvedBy,
				"resolvedByIssuer", rewrite.ResolverIssuer,
				"resolvedBySubject", rewrite.ResolverSubject,
				"resolvedByUsername", rewrite.ResolverUsername,
			)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("rewrite access request %q: %w", rewrite.ID, err)
	}
	return nil
}

func (m *Migrator) moveUserIndex(ctx context.Context, rewrite AccessRequestRewrite, requestedAt string) error {
	score, err := m.requestScore(ctx, rewrite.ID, rewrite.OldUserID, requestedAt)
	if err != nil {
		return err
	}
	_, err = m.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZRem(ctx, arUserPrefix+rewrite.OldUserID, rewrite.ID)
		pipe.ZAdd(ctx, arUserPrefix+rewrite.NewUserID, redis.Z{Score: score, Member: rewrite.ID})
		return nil
	})
	if err != nil {
		return fmt.Errorf("move access-request user index for %q: %w", rewrite.ID, err)
	}
	return nil
}

func (m *Migrator) requestScore(ctx context.Context, id, oldUserID, requestedAt string) (float64, error) {
	score, err := m.Redis.ZScore(ctx, arUserPrefix+oldUserID, id).Result()
	if err == nil {
		return score, nil
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, fmt.Errorf("read access-request index score for %q: %w", id, err)
	}
	t, err := time.Parse(time.RFC3339Nano, requestedAt)
	if err != nil {
		return 0, fmt.Errorf("parse requestedAt for %q: %w", id, err)
	}
	return float64(t.UnixMilli()), nil
}

func (m *Migrator) moveDedup(ctx context.Context, rewrite AccessRequestRewrite) error {
	if !rewrite.Pending || rewrite.OldUserID == "" || rewrite.NewUserID == "" || rewrite.ServiceUID == "" {
		return nil
	}
	source := arDedupPrefix + rewrite.OldUserID + ":" + rewrite.ServiceUID
	target := arDedupPrefix + rewrite.NewUserID + ":" + rewrite.ServiceUID
	value, err := m.Redis.Get(ctx, source).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read dedup key %q: %w", source, err)
	}
	targetValue, err := m.Redis.Get(ctx, target).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("read dedup target %q: %w", target, err)
	}
	if err == nil && targetValue != value {
		return fmt.Errorf("%w: target dedup key %q now points to %q", ErrCollisions, target, targetValue)
	}
	_, err = m.Redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, target, value, 0)
		pipe.Del(ctx, source)
		return nil
	})
	if err != nil {
		return fmt.Errorf("move dedup key %q to %q: %w", source, target, err)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
