// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package identitymigration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/nebari-dev/nebari-landing/internal/useridentity"
	"github.com/redis/go-redis/v9"
)

func newMigrator(t *testing.T) (*Migrator, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &Migrator{Redis: rdb}, rdb
}

func TestPlanDetectsSetTargetCollision(t *testing.T) {
	ctx := context.Background()
	migrator, rdb := newMigrator(t)
	target := useridentity.StableID("https://keycloak.example/realms/main", "subject-1")
	if err := rdb.SAdd(ctx, pinsPrefix+"alice", "svc-1").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.SAdd(ctx, pinsPrefix+target, "svc-2").Err(); err != nil {
		t.Fatal(err)
	}

	result, err := migrator.Plan(ctx, []Mapping{{
		Username: "alice",
		Issuer:   "https://keycloak.example/realms/main",
		Subject:  "subject-1",
	}})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(result.Collisions) == 0 {
		t.Fatalf("expected collision, got %+v", result)
	}
	if result.Collisions[0].Kind != "pins" {
		t.Fatalf("expected pins collision, got %+v", result.Collisions)
	}
	if err := migrator.Apply(ctx, result); !errors.Is(err, ErrCollisions) {
		t.Fatalf("Apply should refuse collision plan, got %v", err)
	}
}

func TestPlanDetectsDuplicateStableTarget(t *testing.T) {
	ctx := context.Background()
	migrator, _ := newMigrator(t)

	result, err := migrator.Plan(ctx, []Mapping{
		{Username: "alice", Issuer: "https://keycloak.example/realms/main", Subject: "subject-1"},
		{Username: "alice-old", Issuer: "https://keycloak.example/realms/main", Subject: "subject-1"},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(result.Collisions) == 0 {
		t.Fatalf("expected duplicate target collision, got %+v", result)
	}
}

func TestPlanDetectsExistingActiveAccessTarget(t *testing.T) {
	ctx := context.Background()
	migrator, rdb := newMigrator(t)
	issuer := "https://keycloak.example/realms/main"
	target := useridentity.StableID(issuer, "subject-1")
	requestedAt := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

	if err := rdb.HSet(ctx, arHashPrefix+"req-1",
		"id", "req-1",
		"serviceUID", "svc-1",
		"serviceName", "grafana",
		"userID", "alice",
		"status", "approved",
		"requestedAt", requestedAt.Format(time.RFC3339Nano),
	).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.ZAdd(ctx, arAllKey, redis.Z{Score: float64(requestedAt.UnixMilli()), Member: "req-1"}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, arActivePrefix+"alice:svc-1", "req-1", time.Hour).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, arActivePrefix+target+":svc-1", "req-1", time.Hour).Err(); err != nil {
		t.Fatal(err)
	}

	result, err := migrator.Plan(ctx, []Mapping{{
		Username: "alice",
		Issuer:   issuer,
		Subject:  "subject-1",
	}})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(result.Collisions) == 0 {
		t.Fatalf("expected active access collision, got %+v", result)
	}
	if result.Collisions[0].Kind != "access_request_active" {
		t.Fatalf("expected active access collision, got %+v", result.Collisions)
	}
	if err := migrator.Apply(ctx, result); !errors.Is(err, ErrCollisions) {
		t.Fatalf("Apply should refuse collision plan, got %v", err)
	}
}

func TestApplyMigratesDurableUserState(t *testing.T) {
	ctx := context.Background()
	migrator, rdb := newMigrator(t)
	issuer := "https://keycloak.example/realms/main"
	userTarget := useridentity.StableID(issuer, "subject-1")
	adminTarget := useridentity.StableID(issuer, "admin-subject")

	if err := rdb.SAdd(ctx, pinsPrefix+"alice", "svc-1").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.SAdd(ctx, readsPrefix+"alice", "notif-1").Err(); err != nil {
		t.Fatal(err)
	}

	requestedAt := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	if err := rdb.HSet(ctx, arHashPrefix+"req-1",
		"id", "req-1",
		"serviceUID", "svc-1",
		"serviceName", "grafana",
		"userID", "alice",
		"userEmail", "alice@example.com",
		"status", "pending",
		"requestedAt", requestedAt.Format(time.RFC3339Nano),
	).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.HSet(ctx, arHashPrefix+"req-2",
		"id", "req-2",
		"serviceUID", "svc-2",
		"serviceName", "mlflow",
		"userID", "bob",
		"status", "approved",
		"resolvedBy", "admin",
		"requestedAt", requestedAt.Add(time.Minute).Format(time.RFC3339Nano),
	).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.HSet(ctx, arHashPrefix+"req-3",
		"id", "req-3",
		"serviceUID", "svc-3",
		"serviceName", "jupyterlab",
		"userID", "alice",
		"status", "approved",
		"requestedAt", requestedAt.Add(2*time.Minute).Format(time.RFC3339Nano),
	).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.ZAdd(ctx, arAllKey,
		redis.Z{Score: float64(requestedAt.UnixMilli()), Member: "req-1"},
		redis.Z{Score: float64(requestedAt.Add(time.Minute).UnixMilli()), Member: "req-2"},
		redis.Z{Score: float64(requestedAt.Add(2 * time.Minute).UnixMilli()), Member: "req-3"},
	).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.ZAdd(ctx, arUserPrefix+"alice", redis.Z{Score: float64(requestedAt.UnixMilli()), Member: "req-1"}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.ZAdd(ctx, arUserPrefix+"alice", redis.Z{Score: float64(requestedAt.Add(2 * time.Minute).UnixMilli()), Member: "req-3"}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, arDedupPrefix+"alice:svc-1", "req-1", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, arActivePrefix+"alice:svc-3", "req-3", time.Hour).Err(); err != nil {
		t.Fatal(err)
	}

	result, err := migrator.Plan(ctx, []Mapping{
		{Username: "alice", Issuer: issuer, Subject: "subject-1", DisplayUsername: "alice-renamed"},
		{Username: "admin", Issuer: issuer, Subject: "admin-subject"},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(result.Collisions) > 0 {
		t.Fatalf("unexpected collisions: %+v", result.Collisions)
	}
	if err := migrator.Apply(ctx, result); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if exists, _ := rdb.Exists(ctx, pinsPrefix+"alice").Result(); exists != 0 {
		t.Fatal("legacy pins key should be removed")
	}
	if members, _ := rdb.SMembers(ctx, pinsPrefix+userTarget).Result(); len(members) != 1 || members[0] != "svc-1" {
		t.Fatalf("expected migrated pins, got %v", members)
	}
	if exists, _ := rdb.Exists(ctx, readsPrefix+"alice").Result(); exists != 0 {
		t.Fatal("legacy reads key should be removed")
	}
	if members, _ := rdb.SMembers(ctx, readsPrefix+userTarget).Result(); len(members) != 1 || members[0] != "notif-1" {
		t.Fatalf("expected migrated reads, got %v", members)
	}

	fields, err := rdb.HGetAll(ctx, arHashPrefix+"req-1").Result()
	if err != nil {
		t.Fatal(err)
	}
	if fields["userID"] != userTarget || fields["userIssuer"] != issuer || fields["userSubject"] != "subject-1" || fields["username"] != "alice-renamed" {
		t.Fatalf("request user identity not migrated: %+v", fields)
	}
	if score, err := rdb.ZScore(ctx, arUserPrefix+userTarget, "req-1").Result(); err != nil || score != float64(requestedAt.UnixMilli()) {
		t.Fatalf("request user index not migrated: score=%v err=%v", score, err)
	}
	if exists, _ := rdb.Exists(ctx, arDedupPrefix+"alice:svc-1").Result(); exists != 0 {
		t.Fatal("legacy dedup key should be removed")
	}
	if val, err := rdb.Get(ctx, arDedupPrefix+userTarget+":svc-1").Result(); err != nil || val != "req-1" {
		t.Fatalf("dedup key not migrated: val=%q err=%v", val, err)
	}
	if exists, _ := rdb.Exists(ctx, arActivePrefix+"alice:svc-3").Result(); exists != 0 {
		t.Fatal("legacy active access key should be removed")
	}
	if val, err := rdb.Get(ctx, arActivePrefix+userTarget+":svc-3").Result(); err != nil || val != "req-3" {
		t.Fatalf("active access key not migrated: val=%q err=%v", val, err)
	}

	fields, err = rdb.HGetAll(ctx, arHashPrefix+"req-2").Result()
	if err != nil {
		t.Fatal(err)
	}
	if fields["resolvedBy"] != adminTarget || fields["resolvedByIssuer"] != issuer || fields["resolvedBySubject"] != "admin-subject" || fields["resolvedByUsername"] != "admin" {
		t.Fatalf("resolver identity not migrated: %+v", fields)
	}
}
