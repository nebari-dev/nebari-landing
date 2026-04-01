// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	wshub "github.com/nebari-dev/nebari-landing/internal/websocket"
	"github.com/redis/go-redis/v9"
)

func newAPIWSHub(t *testing.T) *wshub.Hub {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = rdb.Close() })
	return wshub.NewHub(ctx, rdb)
}

func TestHandleWS_Unauthorized_WhenAuthFails(t *testing.T) {
	hub := newAPIWSHub(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, hub, nil,
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return nil, false
		}),
	)

	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/ws"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected websocket dial to fail without auth")
	}
	if resp == nil {
		t.Fatal("expected HTTP response on failed handshake")
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHandleWS_Upgrades_WhenAuthPasses(t *testing.T) {
	hub := newAPIWSHub(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, hub, nil,
		WithClaimsExtractor(func(r *http.Request) (*auth.Claims, bool) {
			if r.Header.Get("Authorization") == "Bearer test-token" {
				return &auth.Claims{PreferredUsername: "alice"}, true
			}
			return nil, false
		}),
	)

	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/ws"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test-token")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			t.Fatalf("expected websocket upgrade, got status %d and error %v", resp.StatusCode, err)
		}
		t.Fatalf("expected websocket upgrade, got error %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Give ServeWS a moment to register and then close cleanly.
	time.Sleep(20 * time.Millisecond)
}
