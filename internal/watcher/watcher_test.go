package watcher

import (
	"context"
	"testing"

	landingcache "github.com/nebari-dev/nebari-landing/internal/cache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWatcherToAppAllowsCrossNamespaceHealthCheckWithReferenceGrant(t *testing.T) {
	w := newTestWatcher(referenceGrant("team-a", "shared-api"))
	app := w.toApp(context.Background(), nebariApp("uid-allow", "app", "team-a", "shared-api", "platform"))

	if !app.HealthCheckCrossNamespaceAllowed {
		t.Fatal("expected ReferenceGrant to allow cross-namespace health check")
	}

	c := landingcache.NewServiceCache()
	c.Add(app)
	svc := c.Get("uid-allow")
	if svc == nil || svc.HealthCheckConfig == nil {
		t.Fatalf("expected cross-namespace health check config, got %+v", svc)
	}
	if got, want := svc.HealthCheckConfig.ProbeURL, "http://shared-api.platform:8080/ready"; got != want {
		t.Fatalf("ProbeURL = %q, want %q", got, want)
	}
}

func TestWatcherToAppDeniesCrossNamespaceHealthCheckWithoutReferenceGrant(t *testing.T) {
	w := newTestWatcher()
	app := w.toApp(context.Background(), nebariApp("uid-deny", "app", "team-a", "shared-api", "platform"))

	if app.HealthCheckCrossNamespaceAllowed {
		t.Fatal("expected missing ReferenceGrant to deny cross-namespace health check")
	}

	c := landingcache.NewServiceCache()
	c.Add(app)
	svc := c.Get("uid-deny")
	if svc == nil {
		t.Fatal("expected service to remain visible")
	}
	if svc.HealthCheckConfig != nil {
		t.Fatalf("expected denied cross-namespace health check config, got %+v", svc.HealthCheckConfig)
	}
}

func TestReferenceGrantAllowsSpecificService(t *testing.T) {
	grant := referenceGrant("team-a", "shared-api")

	if !referenceGrantAllows(grant, "team-a", "shared-api") {
		t.Fatal("expected ReferenceGrant to allow matching source namespace and service")
	}
}

func TestReferenceGrantAllowsAllServicesWhenNameOmitted(t *testing.T) {
	grant := referenceGrant("team-a", "")

	if !referenceGrantAllows(grant, "team-a", "shared-api") {
		t.Fatal("expected unnamed ReferenceGrant target to allow any Service name")
	}
}

func TestReferenceGrantRejectsMismatches(t *testing.T) {
	for _, tt := range []struct {
		name            string
		sourceNamespace string
		serviceName     string
	}{
		{name: "source namespace", sourceNamespace: "team-b", serviceName: "shared-api"},
		{name: "service name", sourceNamespace: "team-a", serviceName: "other-api"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if referenceGrantAllows(referenceGrant("team-a", "shared-api"), tt.sourceNamespace, tt.serviceName) {
				t.Fatal("expected ReferenceGrant mismatch to be rejected")
			}
		})
	}
}

func newTestWatcher(objects ...*unstructured.Unstructured) *NebariAppWatcher {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(referenceGrantGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(referenceGrantListGVK, &unstructured.UnstructuredList{})

	runtimeObjects := make([]runtime.Object, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}

	return &NebariAppWatcher{
		client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(runtimeObjects...).
			Build(),
	}
}

func nebariApp(uid, name, namespace, serviceName, serviceNamespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "reconcilers.nebari.dev/v1",
		"kind":       "NebariApp",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"uid":       uid,
		},
		"spec": map[string]interface{}{
			"hostname": name + ".example.com",
			"service": map[string]interface{}{
				"name":      serviceName,
				"namespace": serviceNamespace,
				"port":      int64(8080),
			},
			"landingPage": map[string]interface{}{
				"enabled":     true,
				"displayName": name,
				"healthCheck": map[string]interface{}{
					"enabled": true,
					"path":    "/ready",
				},
			},
		},
	}}
}

func referenceGrant(sourceNamespace, serviceName string) *unstructured.Unstructured {
	to := map[string]interface{}{
		"group": "",
		"kind":  "Service",
	}
	if serviceName != "" {
		to["name"] = serviceName
	}

	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1beta1",
		"kind":       "ReferenceGrant",
		"metadata": map[string]interface{}{
			"name":      "allow-health-check",
			"namespace": "platform",
		},
		"spec": map[string]interface{}{
			"from": []interface{}{
				map[string]interface{}{
					"group":     nebariAppGVK.Group,
					"kind":      nebariAppGVK.Kind,
					"namespace": sourceNamespace,
				},
			},
			"to": []interface{}{to},
		},
	}}
}
