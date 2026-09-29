// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package packs derives the "software pack" view for the admin area. A pack
// is an ArgoCD Application labeled app.kubernetes.io/part-of=nebari-packs
// (platform components carry nebari-foundational). Its landing-page services
// are the NebariApps whose ArgoCD tracking-id names the Application, or that
// live in its destination namespace. When ArgoCD is not readable the view
// degrades to what the NebariApp Helm labels alone can tell.
package packs

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nebari-dev/nebari-landing/internal/cache"
)

const (
	labelPartOf   = "app.kubernetes.io/part-of"
	labelInstance = "app.kubernetes.io/instance"
	labelVersion  = "app.kubernetes.io/version"
	labelChart    = "helm.sh/chart"
	annoTracking  = "argocd.argoproj.io/tracking-id"

	TierPack     = "pack"
	TierPlatform = "platform"
)

// ArgoApp is the slice of an ArgoCD Application the packs view needs.
type ArgoApp struct {
	Name           string    `json:"name"`
	Tier           string    `json:"tier"`
	Namespace      string    `json:"namespace"`
	RepoURL        string    `json:"repoURL"`
	Chart          string    `json:"chart,omitempty"`
	Path           string    `json:"path,omitempty"`
	TargetRevision string    `json:"targetRevision"`
	SyncStatus     string    `json:"syncStatus"`
	HealthStatus   string    `json:"healthStatus"`
	Revision       string    `json:"revision,omitempty"`
	LastSyncPhase  string    `json:"lastSyncPhase,omitempty"`
	LastSyncAt     time.Time `json:"lastSyncAt"`
	ReconciledAt   time.Time `json:"reconciledAt"`
	AutoSync       bool      `json:"autoSync"`
	Images         []string  `json:"images"`
	ResourceCount  int       `json:"resourceCount"`
}

// PackService is a landing-page service attributed to a pack.
type PackService struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	Namespace      string   `json:"namespace"`
	URL            string   `json:"url"`
	Visibility     string   `json:"visibility"`
	RequiredGroups []string `json:"requiredGroups"`
	HealthStatus   string   `json:"healthStatus"`
}

// Pack is one row of the Software Packs view.
type Pack struct {
	Name         string   `json:"name"`
	Tier         string   `json:"tier"`
	Namespace    string   `json:"namespace"`
	ChartName    string   `json:"chartName,omitempty"`
	ChartVersion string   `json:"chartVersion,omitempty"`
	AppVersion   string   `json:"appVersion,omitempty"`
	Argo         *ArgoApp `json:"argo,omitempty"`
	// LatestVersion is the newest version published in the pack's Helm
	// repository; VersionStatus is current | behind | ahead | unknown.
	LatestVersion string        `json:"latestVersion,omitempty"`
	VersionStatus string        `json:"versionStatus"`
	Services      []PackService `json:"services"`
}

// Lister reads ArgoCD Applications with a controller-runtime client. Results
// are cached briefly and refreshed by one caller at a time: Applications are
// large objects and several admin views ask for them at once.
type Lister struct {
	kube client.Client
	ttl  time.Duration

	mu        sync.Mutex
	fetchedAt time.Time
	apps      []ArgoApp
	err       error
}

// NewLister returns a Lister over the given client. A nil client yields a
// lister that always reports ArgoCD as unavailable.
func NewLister(kube client.Client) *Lister { return &Lister{kube: kube, ttl: 20 * time.Second} }

var applicationListGVK = schema.GroupVersionKind{
	Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationList",
}

// ListArgoApps returns every ArgoCD Application in the cluster that carries a
// Nebari part-of label. Errors (CRD missing, RBAC) are returned as-is so the
// caller can degrade.
func (l *Lister) ListArgoApps(ctx context.Context) ([]ArgoApp, error) {
	if l == nil || l.kube == nil {
		return nil, errNoClient
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Since(l.fetchedAt) < l.ttl {
		return l.apps, l.err
	}
	apps, err := l.fetch(ctx)
	l.fetchedAt = time.Now()
	l.apps, l.err = apps, err
	return apps, err
}

func (l *Lister) fetch(ctx context.Context) ([]ArgoApp, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(applicationListGVK)
	if err := l.kube.List(ctx, list); err != nil {
		return nil, err
	}
	out := make([]ArgoApp, 0, len(list.Items))
	for i := range list.Items {
		if a, ok := fromUnstructured(&list.Items[i]); ok {
			out = append(out, a)
		}
	}
	return out, nil
}

type noClientError struct{}

func (noClientError) Error() string { return "no kubernetes client configured" }

var errNoClient error = noClientError{}

func fromUnstructured(u *unstructured.Unstructured) (ArgoApp, bool) {
	tier := tierFor(u.GetLabels()[labelPartOf])
	if tier == "" {
		return ArgoApp{}, false
	}
	a := ArgoApp{Name: u.GetName(), Tier: tier, Images: []string{}}
	a.Namespace, _, _ = unstructured.NestedString(u.Object, "spec", "destination", "namespace")
	a.RepoURL, _, _ = unstructured.NestedString(u.Object, "spec", "source", "repoURL")
	a.Chart, _, _ = unstructured.NestedString(u.Object, "spec", "source", "chart")
	a.Path, _, _ = unstructured.NestedString(u.Object, "spec", "source", "path")
	a.TargetRevision, _, _ = unstructured.NestedString(u.Object, "spec", "source", "targetRevision")
	if a.RepoURL == "" {
		// Multi-source apps: take the first source.
		if sources, found, _ := unstructured.NestedSlice(u.Object, "spec", "sources"); found && len(sources) > 0 {
			if first, ok := sources[0].(map[string]interface{}); ok {
				a.RepoURL, _, _ = unstructured.NestedString(first, "repoURL")
				a.Chart, _, _ = unstructured.NestedString(first, "chart")
				a.Path, _, _ = unstructured.NestedString(first, "path")
				a.TargetRevision, _, _ = unstructured.NestedString(first, "targetRevision")
			}
		}
	}
	_, a.AutoSync, _ = unstructured.NestedMap(u.Object, "spec", "syncPolicy", "automated")
	a.SyncStatus, _, _ = unstructured.NestedString(u.Object, "status", "sync", "status")
	a.Revision, _, _ = unstructured.NestedString(u.Object, "status", "sync", "revision")
	a.HealthStatus, _, _ = unstructured.NestedString(u.Object, "status", "health", "status")
	a.LastSyncPhase, _, _ = unstructured.NestedString(u.Object, "status", "operationState", "phase")
	if ts, _, _ := unstructured.NestedString(u.Object, "status", "operationState", "finishedAt"); ts != "" {
		a.LastSyncAt, _ = time.Parse(time.RFC3339, ts)
	}
	if ts, _, _ := unstructured.NestedString(u.Object, "status", "reconciledAt"); ts != "" {
		a.ReconciledAt, _ = time.Parse(time.RFC3339, ts)
	}
	if imgs, _, _ := unstructured.NestedStringSlice(u.Object, "status", "summary", "images"); len(imgs) > 0 {
		a.Images = imgs
	}
	if res, found, _ := unstructured.NestedSlice(u.Object, "status", "resources"); found {
		a.ResourceCount = len(res)
	}
	if a.SyncStatus == "" {
		a.SyncStatus = "Unknown"
	}
	if a.HealthStatus == "" {
		a.HealthStatus = "Unknown"
	}
	return a, true
}

func tierFor(partOf string) string {
	switch partOf {
	case "nebari-packs":
		return TierPack
	case "nebari-foundational":
		return TierPlatform
	}
	return ""
}

var chartVersionRe = regexp.MustCompile(`-(\d+\.\d+\.\d+[0-9A-Za-z.+-]*)$`)

// SplitChartLabel splits a helm.sh/chart label ("nebari-lgtm-pack-0.2.0") into
// chart name and version.
func SplitChartLabel(label string) (name, version string) {
	if m := chartVersionRe.FindStringSubmatchIndex(label); m != nil {
		return label[:m[2]-1], label[m[2]:m[3]]
	}
	return label, ""
}

// trackingApp extracts the Application name from an ArgoCD tracking-id
// annotation ("<app>:<group>/<Kind>:<ns>/<name>").
func trackingApp(anno string) string {
	if i := strings.Index(anno, ":"); i > 0 {
		return anno[:i]
	}
	return ""
}

func toPackService(s *cache.ServiceInfo) PackService {
	groups := s.RequiredGroups
	if groups == nil {
		groups = []string{}
	}
	vis := s.Visibility
	if vis == "" {
		vis = "private"
	}
	health := "unknown"
	if s.Health != nil && s.Health.Status != "" {
		health = s.Health.Status
	}
	return PackService{
		ID: s.UID, Name: s.Name, DisplayName: s.DisplayName, Namespace: s.Namespace, URL: s.URL,
		Visibility: vis, RequiredGroups: groups, HealthStatus: health,
	}
}

// Build joins ArgoCD Applications (possibly nil when unavailable) with the
// landing-page services into pack rows, sorted packs first then by name.
func Build(apps []ArgoApp, services []*cache.ServiceInfo) []Pack {
	byName := map[string]*Pack{}
	order := []string{}
	add := func(p Pack) *Pack {
		byName[p.Name] = &p
		order = append(order, p.Name)
		return &p
	}
	for i := range apps {
		a := apps[i]
		p := Pack{Name: a.Name, Tier: a.Tier, Namespace: a.Namespace, Argo: &a, Services: []PackService{}}
		p.ChartName = a.Chart
		if a.Chart != "" {
			p.ChartVersion = a.TargetRevision
		}
		add(p)
	}

	byNamespace := map[string]string{}
	for _, a := range apps {
		if a.Namespace != "" {
			if _, dup := byNamespace[a.Namespace]; !dup {
				byNamespace[a.Namespace] = a.Name
			}
		}
	}

	for _, s := range services {
		target := ""
		if s.Annotations != nil {
			if app := trackingApp(s.Annotations[annoTracking]); app != "" {
				if _, ok := byName[app]; ok {
					target = app
				}
			}
		}
		if target == "" {
			if app, ok := byNamespace[s.Namespace]; ok {
				target = app
			}
		}
		if target == "" {
			// No ArgoCD match: fall back to the Helm release instance.
			instance := ""
			if s.Labels != nil {
				instance = s.Labels[labelInstance]
			}
			if instance == "" {
				instance = s.Namespace
			}
			if _, ok := byName[instance]; !ok {
				add(Pack{Name: instance, Tier: TierPack, Namespace: s.Namespace, Services: []PackService{}})
			}
			target = instance
		}
		p := byName[target]
		p.Services = append(p.Services, toPackService(s))
		if s.Labels != nil {
			if p.ChartName == "" || p.ChartVersion == "" {
				name, version := SplitChartLabel(s.Labels[labelChart])
				if p.ChartName == "" {
					p.ChartName = name
				}
				if p.ChartVersion == "" {
					p.ChartVersion = version
				}
			}
			if p.AppVersion == "" {
				p.AppVersion = s.Labels[labelVersion]
			}
		}
	}

	out := make([]Pack, 0, len(order))
	for _, n := range order {
		p := byName[n]
		p.VersionStatus = VersionUnknown
		sort.Slice(p.Services, func(i, j int) bool { return p.Services[i].DisplayName < p.Services[j].DisplayName })
		out = append(out, *p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier == TierPack
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Annotate fills LatestVersion / VersionStatus on each pack using src. A nil
// source leaves every pack unknown.
func Annotate(ctx context.Context, src *VersionSource, packs []Pack) {
	if src == nil {
		return
	}
	for i := range packs {
		p := &packs[i]
		if p.Argo == nil || p.Argo.Chart == "" {
			continue
		}
		if latest, ok := src.Latest(ctx, p.Argo.RepoURL, p.Argo.Chart, p.ChartVersion); ok {
			p.LatestVersion = latest
			p.VersionStatus = CompareVersions(p.ChartVersion, latest)
		}
	}
}
