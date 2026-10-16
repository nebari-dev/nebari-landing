// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/nebari-dev/nebari-landing/internal/identitymigration"
	"github.com/redis/go-redis/v9"
)

func main() {
	var (
		mappingFile   string
		apply         bool
		redisAddr     string
		redisUsername string
		redisPassword string
		redisDB       int
	)

	flag.StringVar(&mappingFile, "mapping-file", "", "JSON file mapping legacy usernames to issuer+subject identities")
	flag.BoolVar(&apply, "apply", false, "Apply the migration. Without this flag the command only prints a dry-run plan.")
	flag.StringVar(&redisAddr, "redis-addr", envStr("REDIS_ADDR", "localhost:6379"), "Redis server address host:port (env: REDIS_ADDR)")
	flag.StringVar(&redisUsername, "redis-username", os.Getenv("REDIS_USERNAME"), "Redis ACL username (env: REDIS_USERNAME)")
	flag.StringVar(&redisPassword, "redis-password", os.Getenv("REDIS_PASSWORD"), "Redis password (env: REDIS_PASSWORD)")
	flag.IntVar(&redisDB, "redis-db", envInt("REDIS_DB", 0), "Redis database index (env: REDIS_DB)")
	flag.Parse()

	if mappingFile == "" {
		fmt.Fprintln(os.Stderr, "--mapping-file is required")
		os.Exit(2)
	}

	mappings, err := identitymigration.LoadMappings(mappingFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load mappings: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: redisUsername,
		Password: redisPassword,
		DB:       redisDB,
	})
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "connect to Redis: %v\n", err)
		os.Exit(1)
	}

	migrator := &identitymigration.Migrator{Redis: rdb}
	result, err := migrator.Plan(ctx, mappings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan migration: %v\n", err)
		os.Exit(1)
	}
	if apply && len(result.Collisions) == 0 {
		if err := migrator.Apply(ctx, result); err != nil {
			fmt.Fprintf(os.Stderr, "apply migration: %v\n", err)
			os.Exit(1)
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
	if len(result.Collisions) > 0 {
		os.Exit(2)
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	var v int
	if _, err := fmt.Sscanf(os.Getenv(key), "%d", &v); err == nil {
		return v
	}
	return fallback
}
