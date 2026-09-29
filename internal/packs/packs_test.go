// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package packs

import (
	"testing"

	"github.com/nebari-dev/nebari-landing/internal/cache"
)

func TestSplitChartLabel(t *testing.T) {
	cases := map[string][2]string{
		"nebari-lgtm-pack-0.2.0":                  {"nebari-lgtm-pack", "0.2.0"},
		"nebari-data-science-pack-0.1.0-alpha.11": {"nebari-data-science-pack", "0.1.0-alpha.11"},
		"harbor-pack-0.1.0":                       {"harbor-pack", "0.1.0"},
		"weird":                                   {"weird", ""},
		"":                                        {"", ""},
	}
	for in, want := range cases {
		n, v := SplitChartLabel(in)
		if n != want[0] || v != want[1] {
			t.Errorf("%q: got (%q,%q) want %v", in, n, v, want)
		}
	}
}

func svc(uid, name, ns string, labels, annos map[string]string) *cache.ServiceInfo {
	return &cache.ServiceInfo{UID: uid, Name: name, DisplayName: name, Namespace: ns, Visibility: "private",
		Labels: labels, Annotations: annos, Health: &cache.HealthStatus{Status: "healthy"}}
}

func TestBuild_JoinsByTrackingIDThenNamespace(t *testing.T) {
	apps := []ArgoApp{
		{Name: "lgtm-pack", Tier: TierPack, Namespace: "monitoring", Chart: "nebari-lgtm-pack", TargetRevision: "0.2.0", SyncStatus: "OutOfSync", HealthStatus: "Healthy"},
		{Name: "nebari-landingpage", Tier: TierPlatform, Namespace: "nebari-system", Path: "charts/nebari-landing", TargetRevision: "v0.1.5", SyncStatus: "Synced", HealthStatus: "Healthy"},
		{Name: "nebari-chat", Tier: TierPack, Namespace: "nebari-chat", Chart: "nebari-chat", TargetRevision: "0.0.26"},
	}
	services := []*cache.ServiceInfo{
		svc("s1", "grafana", "monitoring",
			map[string]string{"helm.sh/chart": "nebari-lgtm-pack-0.2.0", "app.kubernetes.io/version": "1.0.0", "app.kubernetes.io/instance": "lgtm-pack"},
			map[string]string{"argocd.argoproj.io/tracking-id": "lgtm-pack:reconcilers.nebari.dev/NebariApp:monitoring/grafana"}),
		svc("s2", "nebari-landing", "nebari-system",
			map[string]string{"helm.sh/chart": "nebari-landing-0.1.5", "app.kubernetes.io/version": "0.1.5"}, nil),
		svc("s3", "chat-frontend", "nebari-chat", map[string]string{"helm.sh/chart": "nebari-chat-0.0.26"}, nil),
		svc("s4", "chat-backend", "nebari-chat", map[string]string{"helm.sh/chart": "nebari-chat-0.0.26"}, nil),
		svc("s5", "orphan", "elsewhere", map[string]string{"app.kubernetes.io/instance": "mystery", "helm.sh/chart": "mystery-1.2.3"}, nil),
	}
	got := Build(apps, services)
	if len(got) != 4 {
		t.Fatalf("want 4 packs, got %d: %+v", len(got), got)
	}
	// packs first (alphabetical), then platform
	if got[0].Name != "lgtm-pack" || got[1].Name != "mystery" || got[2].Name != "nebari-chat" || got[3].Name != "nebari-landingpage" {
		t.Fatalf("order: %v %v %v %v", got[0].Name, got[1].Name, got[2].Name, got[3].Name)
	}
	lgtm := got[0]
	if len(lgtm.Services) != 1 || lgtm.Services[0].ID != "s1" || lgtm.ChartVersion != "0.2.0" || lgtm.AppVersion != "1.0.0" || lgtm.Argo == nil {
		t.Fatalf("lgtm: %+v", lgtm)
	}
	landing := got[3]
	if landing.Tier != TierPlatform || landing.ChartName != "nebari-landing" || landing.ChartVersion != "0.1.5" || len(landing.Services) != 1 {
		t.Fatalf("landing: %+v", landing)
	}
	if chat := got[2]; len(chat.Services) != 2 || chat.Services[0].Name != "chat-backend" {
		t.Fatalf("chat: %+v", chat)
	}
	if mystery := got[1]; mystery.Argo != nil || mystery.ChartVersion != "1.2.3" || mystery.Namespace != "elsewhere" {
		t.Fatalf("mystery: %+v", mystery)
	}
}

func TestBuild_WithoutArgo(t *testing.T) {
	services := []*cache.ServiceInfo{
		svc("s1", "grafana", "monitoring", map[string]string{"app.kubernetes.io/instance": "lgtm-pack", "helm.sh/chart": "nebari-lgtm-pack-0.2.0"}, nil),
		svc("s2", "hub", "jupyterhub", nil, nil),
	}
	got := Build(nil, services)
	if len(got) != 2 || got[0].Name != "jupyterhub" || got[1].Name != "lgtm-pack" || got[1].ChartName != "nebari-lgtm-pack" {
		t.Fatalf("got %+v", got)
	}
}
