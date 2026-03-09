package watcher

import (
	"context"
	"fmt"
	"time"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
	landingcache "github.com/nebari-dev/nebari-landing/internal/cache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	cachepkg "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// nebariAppGVK is the GroupVersionKind for the NebariApp CRD.
// Using the GVK directly (instead of a registered scheme type) means this
// package has zero dependency on the nebari-operator/api module, making the
// webapi binary fully self-contained and movable to its own repository.
var nebariAppGVK = schema.GroupVersionKind{
	Group:   "reconcilers.nebari.dev",
	Version: "v1",
	Kind:    "NebariApp",
}

var log = ctrl.Log.WithName("watcher")

// Publisher receives service change events. *websocket.Hub satisfies this interface.
type Publisher interface {
	Publish(eventType string, service *landingcache.ServiceInfo)
}

// NebariAppWatcher watches NebariApp resources and updates the service cache
type NebariAppWatcher struct {
	cache       *landingcache.ServiceCache
	publisher   Publisher // optional; may be nil
	kubeCache   cachepkg.Cache
	client      client.Client
	syncedCh    chan struct{}
	cacheSynced bool
}

// NewNebariAppWatcher creates a new NebariApp watcher
func NewNebariAppWatcher(config *rest.Config, scheme *runtime.Scheme, serviceCache *landingcache.ServiceCache) (*NebariAppWatcher, error) {
	kubeCache, err := cachepkg.New(config, cachepkg.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	kubeClient, err := client.New(config, client.Options{
		Scheme: scheme,
		Cache: &client.CacheOptions{
			Reader: kubeCache,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &NebariAppWatcher{
		cache:       serviceCache,
		kubeCache:   kubeCache,
		client:      kubeClient,
		syncedCh:    make(chan struct{}),
		cacheSynced: false,
	}, nil
}

// SetPublisher attaches an event publisher (e.g. a WebSocket hub) that is
// notified whenever services are added, updated, or deleted.
func (w *NebariAppWatcher) SetPublisher(p Publisher) {
	w.publisher = p
}

// Start starts watching NebariApp resources
func (w *NebariAppWatcher) Start(ctx context.Context) error {
	log.Info("Starting NebariApp watcher")

	go func() {
		if err := w.kubeCache.Start(ctx); err != nil {
			log.Error(err, "Failed to start cache")
		}
	}()

	if !w.kubeCache.WaitForCacheSync(ctx) {
		return fmt.Errorf("failed to sync cache")
	}

	log.Info("Cache synced, fetching initial NebariApp resources")

	if err := w.syncInitial(ctx); err != nil {
		return fmt.Errorf("failed to sync initial resources: %w", err)
	}

	close(w.syncedCh)
	w.cacheSynced = true

	log.Info("Initial sync complete, starting watch loop")
	return w.watch(ctx)
}

func (w *NebariAppWatcher) syncInitial(ctx context.Context) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   nebariAppGVK.Group,
		Version: nebariAppGVK.Version,
		Kind:    nebariAppGVK.Kind + "List",
	})
	if err := w.client.List(ctx, list); err != nil {
		return fmt.Errorf("failed to list NebariApps: %w", err)
	}

	log.Info("Found NebariApp resources", "count", len(list.Items))

	for i := range list.Items {
		u := &list.Items[i]
		if lpEnabled(u) {
			displayName, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "displayName")
			log.Info("Adding service to cache",
				"name", u.GetName(),
				"namespace", u.GetNamespace(),
				"displayName", displayName,
			)
			w.cache.Add(toApp(u))
		}
	}

	return nil
}

func (w *NebariAppWatcher) watch(ctx context.Context) error {
	informer, err := w.kubeCache.GetInformerForKind(ctx, nebariAppGVK)
	if err != nil {
		return fmt.Errorf("failed to get informer: %w", err)
	}

	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.onAdd,
		UpdateFunc: w.onUpdate,
		DeleteFunc: w.onDelete,
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	<-ctx.Done()
	return nil
}

// asUnstructured casts obj to *unstructured.Unstructured, handling tombstones.
func asUnstructured(obj interface{}) (*unstructured.Unstructured, bool) {
	u, ok := obj.(*unstructured.Unstructured)
	if ok {
		return u, true
	}
	// The informer may deliver a DeletedFinalStateUnknown tombstone on delete.
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if !ok {
		return nil, false
	}
	u, ok = tombstone.Obj.(*unstructured.Unstructured)
	return u, ok
}

func (w *NebariAppWatcher) onAdd(obj interface{}) {
	u, ok := asUnstructured(obj)
	if !ok {
		log.Error(nil, "Failed to cast object to Unstructured", "type", fmt.Sprintf("%T", obj))
		return
	}

	if lpEnabled(u) {
		displayName, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "displayName")
		log.Info("Service added",
			"name", u.GetName(),
			"namespace", u.GetNamespace(),
			"displayName", displayName,
		)
		w.cache.Add(toApp(u))
		if w.publisher != nil {
			w.publisher.Publish("added", w.cache.Get(string(u.GetUID())))
		}
	}
}

func (w *NebariAppWatcher) onUpdate(_, newObj interface{}) {
	u, ok := asUnstructured(newObj)
	if !ok {
		log.Error(nil, "Failed to cast object to Unstructured", "type", fmt.Sprintf("%T", newObj))
		return
	}

	uid := string(u.GetUID())

	if lpEnabled(u) {
		displayName, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "displayName")
		log.Info("Service updated",
			"name", u.GetName(),
			"namespace", u.GetNamespace(),
			"displayName", displayName,
		)
		w.cache.Add(toApp(u))
		if w.publisher != nil {
			w.publisher.Publish("modified", w.cache.Get(uid))
		}
	} else {
		log.Info("Service removed (landing page disabled)",
			"name", u.GetName(),
			"namespace", u.GetNamespace(),
		)
		svc := w.cache.Get(uid)
		w.cache.Remove(uid)
		if w.publisher != nil && svc != nil {
			w.publisher.Publish("deleted", svc)
		}
	}
}

func (w *NebariAppWatcher) onDelete(obj interface{}) {
	u, ok := asUnstructured(obj)
	if !ok {
		log.Error(nil, "Failed to cast object to Unstructured", "type", fmt.Sprintf("%T", obj))
		return
	}

	uid := string(u.GetUID())
	log.Info("Service deleted",
		"name", u.GetName(),
		"namespace", u.GetNamespace(),
	)
	svc := w.cache.Get(uid)
	w.cache.Remove(uid)
	if w.publisher != nil && svc != nil {
		w.publisher.Publish("deleted", svc)
	}
}

// WaitForCacheSync waits for the cache to be synced
func (w *NebariAppWatcher) WaitForCacheSync(ctx context.Context) bool {
	select {
	case <-w.syncedCh:
		return true
	case <-ctx.Done():
		return false
	case <-time.After(30 * time.Second):
		return false
	}
}

// lpEnabled reports whether the NebariApp has landingPage.enabled == true.
func lpEnabled(u *unstructured.Unstructured) bool {
	enabled, _, _ := unstructured.NestedBool(u.Object, "spec", "landingPage", "enabled")
	return enabled
}

// toApp converts an unstructured NebariApp object to the internal sdapp.App
// domain model. It mirrors the field access that the typed version performed,
// using the JSON field names from the CRD schema.
//
// Field priority: status.serviceDiscovery (controller-computed, authoritative)
// overrides spec.landingPage (user-provided) for display fields, exactly as the
// typed version did.
func toApp(u *unstructured.Unstructured) *sdapp.App {
	hostname, _, _ := unstructured.NestedString(u.Object, "spec", "hostname")
	serviceName, _, _ := unstructured.NestedString(u.Object, "spec", "service", "name")
	servicePort, _, _ := unstructured.NestedInt64(u.Object, "spec", "service", "port")
	a := &sdapp.App{
		UID:         string(u.GetUID()),
		Name:        u.GetName(),
		Namespace:   u.GetNamespace(),
		Hostname:    hostname,
		TLSEnabled:  tlsEnabled(u),
		ServiceName: serviceName,
		ServicePort: int(servicePort),
	}

	if !lpEnabled(u) {
		return a
	}

	displayName, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "displayName")
	description, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "description")
	icon, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "icon")
	category, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "category")
	visibility, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "visibility")
	externalURL, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "externalUrl")
	requiredGroups, _, _ := unstructured.NestedStringSlice(u.Object, "spec", "landingPage", "requiredGroups")

	priority := 100
	if p, found, _ := unstructured.NestedInt64(u.Object, "spec", "landingPage", "priority"); found {
		priority = int(p)
	}

	// Read health check configuration.
	hcEnabled, _, _ := unstructured.NestedBool(u.Object, "spec", "landingPage", "healthCheck", "enabled")
	hcPath, _, _ := unstructured.NestedString(u.Object, "spec", "landingPage", "healthCheck", "path")
	hcInterval, _, _ := unstructured.NestedInt64(u.Object, "spec", "landingPage", "healthCheck", "intervalSeconds")
	hcTimeout, _, _ := unstructured.NestedInt64(u.Object, "spec", "landingPage", "healthCheck", "timeoutSeconds")
	var healthCheck *sdapp.HealthCheck
	if hcEnabled {
		healthCheck = &sdapp.HealthCheck{
			Enabled:         true,
			Path:            hcPath,
			IntervalSeconds: int(hcInterval),
			TimeoutSeconds:  int(hcTimeout),
		}
	}

	page := &sdapp.LandingPage{
		Enabled:        true,
		DisplayName:    displayName,
		Description:    description,
		Icon:           icon,
		Category:       category,
		Priority:       priority,
		Visibility:     visibility,
		RequiredGroups: requiredGroups,
		ExternalURL:    externalURL,
		HealthCheck:    healthCheck,
	}

	// Prefer status.serviceDiscovery when the controller has written it.
	if sdEnabled, _, _ := unstructured.NestedBool(u.Object, "status", "serviceDiscovery", "enabled"); sdEnabled {
		if v, _, _ := unstructured.NestedString(u.Object, "status", "serviceDiscovery", "displayName"); v != "" {
			page.DisplayName = v
		}
		if v, _, _ := unstructured.NestedString(u.Object, "status", "serviceDiscovery", "description"); v != "" {
			page.Description = v
		}
		if v, _, _ := unstructured.NestedString(u.Object, "status", "serviceDiscovery", "url"); v != "" {
			page.ExternalURL = v
		}
		if v, _, _ := unstructured.NestedString(u.Object, "status", "serviceDiscovery", "icon"); v != "" {
			page.Icon = v
		}
		if v, _, _ := unstructured.NestedString(u.Object, "status", "serviceDiscovery", "category"); v != "" {
			page.Category = v
		}
		if v, found, _ := unstructured.NestedInt64(u.Object, "status", "serviceDiscovery", "priority"); found && v != 0 {
			page.Priority = int(v)
		}
		if v, _, _ := unstructured.NestedString(u.Object, "status", "serviceDiscovery", "visibility"); v != "" {
			page.Visibility = v
		}
		if v, found, _ := unstructured.NestedStringSlice(u.Object, "status", "serviceDiscovery", "requiredGroups"); found {
			page.RequiredGroups = v
		}
	}

	a.LandingPage = page
	return a
}

// tlsEnabled reports whether TLS termination is active.
// Defaults to true when spec.routing or spec.routing.tls is absent.
func tlsEnabled(u *unstructured.Unstructured) bool {
	enabled, found, _ := unstructured.NestedBool(u.Object, "spec", "routing", "tls", "enabled")
	if !found {
		return true // default: TLS on
	}
	return enabled
}
