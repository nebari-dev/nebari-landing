// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package packs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestCompareVersions(t *testing.T) {
	cases := [][3]string{
		{"0.2.0", "0.2.0", VersionCurrent},
		{"0.1.4", "0.2.0", VersionBehind},
		{"v0.3.0", "0.2.0", VersionAhead},
		{"", "0.2.0", VersionUnknown},
		{"main", "0.2.0", VersionUnknown},
	}
	for _, c := range cases {
		if got := CompareVersions(c[0], c[1]); got != c[2] {
			t.Errorf("%q vs %q: got %q want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestVersionSource_Latest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`apiVersion: v1
entries:
  nebari-lgtm-pack:
    - version: 0.3.0-rc.1
    - version: 0.2.1
    - version: 0.2.0
    - version: 0.1.0
  other:
    - version: 9.9.9
`))
	}))
	defer srv.Close()
	src := NewVersionSource()
	ctx := context.Background()

	if v, ok := src.Latest(ctx, srv.URL, "nebari-lgtm-pack", "0.2.0"); !ok || v != "0.2.1" {
		t.Fatalf("stable: %q %v", v, ok)
	}
	if v, ok := src.Latest(ctx, srv.URL, "nebari-lgtm-pack", "0.3.0-alpha.1"); !ok || v != "0.3.0-rc.1" {
		t.Fatalf("prerelease installed: %q %v", v, ok)
	}
	if _, ok := src.Latest(ctx, srv.URL, "missing", "1.0.0"); ok {
		t.Fatal("missing chart should be unknown")
	}
	if _, ok := src.Latest(ctx, "oci://quay.io/nebari/charts/x", "x", "1.0.0"); ok {
		t.Fatal("oci should be unknown")
	}
	if _, ok := src.Latest(ctx, srv.URL+"/nope", "nebari-lgtm-pack", "0.2.0"); ok {
		t.Fatal("bad repo should be unknown")
	}

	packs := []Pack{{Name: "lgtm-pack", ChartVersion: "0.2.0", Argo: &ArgoApp{RepoURL: srv.URL, Chart: "nebari-lgtm-pack"}}, {Name: "git", Argo: &ArgoApp{RepoURL: srv.URL, Path: "."}}}
	Annotate(ctx, src, packs)
	if packs[0].LatestVersion != "0.2.1" || packs[0].VersionStatus != VersionBehind || packs[1].VersionStatus != "" {
		t.Fatalf("annotate: %+v", packs)
	}
}

func TestParseIndex(t *testing.T) {
	src := `apiVersion: v1
entries:
  nebari-lgtm-pack:
  - apiVersion: v2
    appVersion: "1.0.0"
    created: "2026-09-01T00:00:00Z"
    name: nebari-lgtm-pack
    version: 0.2.0
    urls:
    - https://example.com/nebari-lgtm-pack-0.2.0.tgz
  - name: nebari-lgtm-pack
    version: "0.1.0"
  keycloakx:
    - name: keycloakx
      version: 7.1.6
      dependencies:
        - name: postgresql
          version: 12.0.0
generated: "2026-09-01T00:00:00Z"
`
	idx, err := parseIndex(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if got := idx.Entries["nebari-lgtm-pack"]; len(got) != 2 || got[0] != "0.2.0" || got[1] != "0.1.0" {
		t.Fatalf("lgtm: %v", got)
	}
	// Dependency versions nested under an entry are attributed to the chart
	// key; the newest real version still wins in Latest via semver max.
	if got := idx.Entries["keycloakx"]; len(got) == 0 || got[0] != "7.1.6" {
		t.Fatalf("keycloakx: %v", got)
	}
}
