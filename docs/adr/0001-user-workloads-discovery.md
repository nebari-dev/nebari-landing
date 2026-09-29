# ADR-0001: User-Workload Discovery

## Status

Proposed

## Date

2026-05-09

## Context

A NIC installation runs many software packs side by side —
`nebari-data-science-pack` (JupyterHub + jhub-apps), the LLM serving
pack, a Dask Gateway pack, an Argo Workflows pack, future packs around
inference and capabilities. Each pack manages its own user-spawned
workloads: notebook servers, named-server apps, Dask clusters, workflow
runs, model inference sessions.

As the number of installed packs grows, **awareness of what is running
on the user's behalf across all of them becomes a real problem**. A
typical user in a day might:

- Open a JupyterLab session in the morning, leave it idling.
- Spawn a Dask cluster with a few large worker nodes for a one-off
  computation, then close the browser tab.
- Submit an Argo workflow that runs for hours.
- Launch a jhub-app or coding-agent that holds compute as long as
  it lives.

Each of those costs cluster resources — sometimes substantially: GPU
nodes for model serving, large memory profiles for Dask schedulers,
persistent storage for long-running labs. Many of the tools have *some*
form of auto-management: JupyterHub's idle culler, Dask Gateway's idle
timeout, KubeRay's job-completion teardown. But auto-management is
conservative by design — it won't stop a JupyterLab with an open
notebook that still has activity, and it won't kill a Dask cluster
mid-task. **The user has to make the decision**, and they can only
make it if they know they have something running.

[Issue #52](https://github.com/nebari-dev/nebari-landing/issues/52),
raised by @kcpevey, framed this directly: when a user leaves a workload
running and comes back later, there's no signal anywhere on the
landing page that they have a resource consuming cluster cost. As NIC
adds more packs, the gap grows — every new pack-managed user workload
is one more thing the user might leave behind.

### Why the landing page

The landing page is already the user's entry point to every pack in the
installation — JupyterHub, MLflow, the Dask dashboard, Ray serve,
whatever else is deployed. Surfacing "you have *X* running right now"
alongside the existing service cards is a small extension to that role:
the user sees the things they can launch *and* the things they have
already launched, in the same place.

### What we explicitly do not want to change

**Pack responsibility for its own child resources stays where it is.**
Each pack owns the full lifecycle of its user-spawned workloads — spawn,
monitor, cull, terminate. The landing page does **not** take any of
that over. It does not stop workloads, does not negotiate with the
pack's controller, does not become a generic Kubernetes resource
manager. It provides *read-only awareness*; the user clicks back to the
pack's own UI to act.

This is the load-bearing constraint that shapes everything below: we
need **cross-pack visibility without crossing pack boundaries**. The
operator and the landing-page webapi must be able to *aggregate* what
packs already publish, without *knowing* the internals of any specific
pack.

### How we got to the chosen design

[Issue #68](https://github.com/nebari-dev/nebari-landing/issues/68)
proposed an initial implementation: child resources carry a fixed
label (`app.kubernetes.io/part-of: <NebariApp name>`), the webapi runs
a single Pod informer filtered on that label, JupyterHub is the pilot.

A multi-pack spike on a local kind cluster (`nebari-data-science-pack`
with JHub + jhub-apps, a Dask Gateway pack, a KubeRay spike, plus a
survey of every `*-pack` repo in `nebari-dev`) falsified those
assumptions:

- **No universal discriminator label.** z2jh stamps
  `app.kubernetes.io/instance`, not `part-of`. Dask Gateway uses
  vendor-namespaced labels. KubeRay uses
  `app.kubernetes.io/created-by`. A single fixed label is not a
  workable contract.
- **The "user-spawned thing" is per-controller-shaped.** JHub user
  pods are direct children; Dask Gateway's per-user object is the
  `DaskCluster` CR (its scheduler and worker pods are implementation
  detail under it); Argo's is the `Workflow` CR. Discovery has to
  target *the manager workload*, not its sub-pods.
- **Per-user attribution lives in a different field per controller.**
  JHub: label and annotation at the same key (escaped on label,
  unescaped on annotation). Dask Gateway: `spec.username`. Argo:
  label only, DNS-friendlified. KubeRay: nothing. A single
  attribution mechanism doesn't fit.
- **Encoding mismatches exist.** KubeSpawner escapes usernames
  (`labels-test` → `labels-2dtest`); Argo DNS-friendlifies them
  (`john@x.com` → `john.at.x.com`). Encoding logic in the operator or
  webapi would be a permanent maintenance treadmill across every
  controller's quirks — exactly the kind of pack-internal knowledge the
  constraint above forbids.
- **Ray has no upstream user-identity propagation** (token auth only;
  no OIDC/SSO; no creator metadata on RayJob), so it's outside the
  scope of this contract in v1.

The original architecture in #68 — single Pod informer, fixed label
selector, JHub-only filter — cannot accommodate this reality without
either baking pack-specific knowledge into the webapi (which crosses the
boundary we just ruled out) or shipping a partial solution that breaks
the moment a second pack lands. A new contract is needed.

## Decision Drivers

- **Pack authoring should not require K8s expertise.** Pack authors
  write one `include` line per controller; mechanical work (selectors,
  attribution paths) lives in pack-local helpers maintained by pack
  maintainers.
- **No operator-side or webapi-side encoding logic.** No named
  transforms, no escape functions, no smart-detection of identity
  encodings. Permanent commitments to pack-specific knowledge violate
  contract-independence.
- **The operator stays a contract aggregator.** No informers on
  user-workload kinds, no cluster queries on behalf of consumers, no
  validating webhook.
- **Spec is public, status is internal.** What packs write is
  permanent. What the operator publishes for the webapi can evolve.
- **Graceful degradation everywhere.** Non-conformance is silent;
  reconcile loops are never affected.
- **Multi-pack, multi-kind from day one.** Adding a new pack with a
  novel `kind` must require no operator changes and no webapi code
  changes — only RBAC plumbing and a pack-side helper.
- **Worker-level / sub-pod discovery is out of scope.** Users think in
  terms of managers; per-worker rows would be noise.

## Considered Options

1. **Fixed label discriminator** (`app.kubernetes.io/part-of`) — keep
   #68's original assumption.
2. **Named transforms in the operator/webapi** — handle encoding
   mismatches by maintaining a registry of `kubespawner-escape`,
   `argo-dns-friendly`, etc.
3. **Auto-detection of identity encodings** — webapi tries known
   transforms on the caller identity until one matches.
4. **fromName template DSL** with operator-side regex compilation —
   pack writes `jupyter-{user}`, operator compiles a regex, webapi
   list-and-filters by name.
5. **Discriminated-union attribution + optional selector, no
   encoding logic** — pack chooses where the user identity lives
   (`fromLabel` / `fromAnnotation` / `fromSpec`); webapi compares
   literally against `claims.preferred_username`; pack-side `selector`
   discriminates multi-shape variants.

## Decision Outcome

Chosen option: **Option 5 — Discriminated-union attribution + optional
selector, no encoding logic**, because it is the only option that
simultaneously: (a) keeps the operator and webapi free of pack-specific
knowledge, (b) handles all real-world per-user controllers surveyed
(JHub, Dask Gateway, Argo Workflows), (c) supports multi-shape packs
natively, and (d) leaves room for future controllers without requiring
operator or webapi changes.

### Consequences

**Good:**

- No operator-side or webapi-side encoding logic. Every transformation,
  every pack-specific bit of knowledge, stays in the pack's own chart
  helpers.
- Multi-kind, multi-pack discovery from day one. Adding a new pack with
  a novel `kind` requires zero operator or webapi code changes — only
  RBAC plumbing.
- Multi-shape support is native: `data-science-pack`'s two Pod-shaped
  variants (default JLab vs jhub-apps) become two `userWorkloads[]`
  entries with different selectors.
- Status surface is internal: operator and webapi can co-evolve
  `status.serviceDiscovery` without breaking pack contracts.
- Pack authoring stays high-level: one `include` per controller; K8s
  primitives (selectors, JSONPath, label keys) live inside helper
  definitions maintained by pack maintainers.

**Bad:**

- `fromAnnotation` cannot be list-filtered; the webapi must do a
  per-object GET. JHub helpers use it for correctness (the matching
  label carries the escaped form). Sub-second at the operating scale
  (50–200 users, 100–1000 manager workloads); an informer cache is the
  obvious optimisation for larger scales.
- Multi-shape packs must declare a `selector` with `matchExpressions`
  (`NotIn [""]` for JHub named servers). Less elegant than a templated
  DSL, but honest about K8s primitives.
- Identity mismatches are silent. If `claims.preferred_username`
  doesn't match the value a pack stores (escape rules, encoding), the
  user sees fewer workloads with no error. Mitigated by surfacing the
  raw extracted `Owner` in the admin view.
- RBAC aggregation must be set up correctly across packs (see Design
  §RBAC).

## Options Detail

### Option 1: Fixed label discriminator (`app.kubernetes.io/part-of`)

#68's original plan: rely on a single label stamped by every spawning
controller, single Pod informer with a fixed selector.

**Pros:**

- Simple to implement; one informer, one label selector.
- Already partially worked for JHub.

**Cons:**

- **Falsified by the spike.** z2jh, Dask Gateway, KubeRay, and Argo
  Workflows all use different labels (or none). There is no universal
  discriminator.
- Worker-level granularity. Doesn't account for "the user-managing
  workload is sometimes a CR (DaskCluster), not a Pod."
- Forces every pack to either match the convention (unrealistic
  upstream) or be unsupported.

### Option 2: Named transforms in the operator/webapi

Operator/webapi maintains a registry of known transforms
(`kubespawner-escape`, `argo-dns-friendly`, ...) and applies them as
declared by the pack.

**Pros:**

- Lets the contract use a `fromLabel` mode even when the controller
  stores an escaped value.
- Lets packs declare their encoding declaratively.

**Cons:**

- **Permanent maintenance treadmill.** Every new controller adds a new
  named transform the operator/webapi must implement and support
  forever (principle 1).
- Encoding knowledge is pack-specific (kubespawner's hex escape, argo's
  `.at.` substitution); this violates the contract-independence
  principle.
- Adds a permanent surface for marginal gain: most cases compare
  cleanly when packs pick the right attribution source.

### Option 3: Auto-detection of identity encodings

Webapi tries known transforms against the caller identity until one
matches against the stored value.

**Pros:**

- Pack authors don't have to think about encoding.
- Operator side stays untouched.

**Cons:**

- **Same maintenance burden as Option 2**, just hidden behind a
  heuristic. The transform registry must still exist somewhere.
- **Encodings can collide** between controllers, producing false
  positives.
- **Debugging gets opaque.** "User A doesn't see their workloads" —
  was it a label miss, a transform miss, or an annotation miss?
- Performance: N transforms × M objects at list time.

### Option 4: fromName template DSL

Pack declares a name template (`jupyter-{user}`); operator compiles
anchored regex; webapi list-and-filters by name match.

**Pros:**

- Reads naturally for JHub-like packs whose names carry user identity.
- Cleaner multi-shape discrimination (`jupyter-{user}` vs
  `jupyter-{user}--{server}`) than `matchExpressions`.

**Cons:**

- **Names are opaque for most CRs** (Dask Gateway random hashes,
  KubeRay generic prefixes). The template approach falls back to
  other modes for those, so it doesn't replace the other modes.
- **Requires operator-side regex parsing and template-syntax
  decisions** (anchoring, capture semantics, escape rules) — new
  permanent surface.
- **Still needs transforms** to compare captured values against caller
  identity for escaped names.
- Marginal benefit: JHub multi-shape readability vs operator-side
  surface — not worth the trade.

### Option 5: Discriminated-union attribution + optional selector, no encoding (Chosen)

Pack declares `user` as a discriminated union — exactly one of
`fromLabel`, `fromAnnotation`, `fromSpec`. Optional `selector` (standard
`metav1.LabelSelector`) handles multi-shape discrimination. No
transforms anywhere.

**Pros:**

- All real-world per-user controllers surveyed (JHub default JLab,
  jhub-apps, Dask Gateway, Argo Workflows) map cleanly to one of the
  three modes plus a possible `selector`.
- Zero operator-side or webapi-side encoding logic.
- Multi-kind / multi-pack from day one.
- Status format is internal — operator/webapi can co-evolve without
  breaking packs.
- Pack-author surface stays at one `include` line; helper-author
  surface is K8s primitives only.

**Cons:**

- `fromAnnotation` is per-object GET (not list-filterable).
- Multi-shape packs write `matchExpressions: NotIn [""]` explicitly.
- Identity-encoding mismatches are silent; admin view surfaces the raw
  `Owner` so deployment-config drift is visible.

## Design

The remainder of this ADR documents the contract, operator
responsibilities, webapi code changes, pack authoring, and spike
findings.

### Contract schema (what packs write)

Packs declare discovery on the NebariApp CRD under
`spec.landingPage.userWorkloads`. Full surface:

```yaml
apiVersion: reconcilers.nebari.dev/v1
kind: NebariApp
spec:
  # ... existing NebariApp fields (hostname, service, auth, routing) ...
  landingPage:
    enabled: true                             # toggle the entire feature
    # ... existing landingPage fields (displayName, description, icon, ...) ...
    userWorkloads:                            # optional list; empty/missing = opt-out
      - kind: Pod                             # required — GVR shorthand or fully-qualified
        user:                                 # required — discriminated union
          # exactly one of:
          fromLabel:      hub.jupyter.org/username
          # fromAnnotation: hub.jupyter.org/username
          # fromSpec:     .spec.username
        # selector is a full metav1.LabelSelector. Optional; required for
        # multi-shape packs that need to discriminate same-kind variants.
        selector:
          matchLabels: { ... }
          matchExpressions: [ ... ]
        displayKindAs: Notebook server        # optional UI hint
```

Four fields per entry, three optional. No transform field — packs pick
attribution sources that compare directly to caller identity.

#### Field semantics

**`userWorkloads[]`** — a list of kind-specific discovery directives
for *user-managing* workloads only. Empty list or missing field is the
opt-out; webapi will not query anything for this NebariApp. Packs whose
workloads have no per-user identity should omit the entry — aggregate
visibility of non-user workloads is not in scope.

**`kind`** *(required)* — the Kubernetes kind to discover. Either a
shorthand (`Pod`, `Job`) or a fully-qualified GVR
(`daskclusters.gateway.dask.org`, `workflows.argoproj.io`). The webapi
verifies RBAC at runtime; the operator does not pre-check. Required
even when `selector` is set, because the operator has no pre-knowledge
of custom resources.

**`user`** *(required)* — discriminated union, exactly one of:

- **`fromLabel: <key>`** — read `.metadata.labels[<key>]`. Cheap to
  filter at list time; visible in `kubectl` output. Used when the
  controller stamps the user as a label *and* the value is directly
  comparable to caller identity (Argo Workflows'
  `creator-preferred-username`).
- **`fromAnnotation: <key>`** — read `.metadata.annotations[<key>]`.
  Used when the canonical comparable value lives on an annotation;
  the webapi must do a per-object GET. Used by JHub helpers because
  KubeSpawner's annotation carries the unescaped username.
- **`fromSpec: <jsonpath>`** — read a value at a JSONPath under
  `.spec`. Used when the controller stamps the user identity into
  spec rather than metadata. Required for Dask Gateway.

The webapi compares the value extracted via `user` *literally* against
`claims.preferred_username`. **No transformation is applied on either
side.**

**`selector`** *(optional)* — standard `metav1.LabelSelector`. The
webapi ANDs `selector` with the implicit namespace scope when listing.
Required for multi-shape packs.

**`displayKindAs`** *(optional)* — free-form short string used in the
UI as the row label. Falls back to `kind` verbatim when omitted.

#### Validation (CEL on the CRD)

- `kind`: required, valid Kubernetes kind name.
- `user`: required for every `userWorkloads[]` entry.
- `user`: exactly one of `fromLabel` / `fromAnnotation` / `fromSpec`.
- `selector.matchLabels`: if present, every key must be a valid label key.

No validating webhook.

#### Status surface (what the webapi consumes)

The operator copies spec entries to status with the implicit namespace
scope added:

```yaml
status:
  serviceDiscovery:
    namespace: <NebariApp.metadata.namespace>
    userWorkloads:
      - kind: Pod
        selector:
          matchLabels:
            app.kubernetes.io/managed-by: kubespawner
            hub.jupyter.org/servername: ""
        user:
          fromAnnotation: hub.jupyter.org/username
        displayKindAs: Notebook server
```

The status format is the operator/webapi internal contract; it can
evolve without breaking packs.

### Operator responsibilities

The operator's responsibilities:

1. Add types: `UserWorkloadConfig` under `LandingPageConfig`;
   `UserWorkloadStatus` under `ServiceDiscoveryStatus`.
2. Reconciler change: copy `spec.landingPage.userWorkloads` to
   `status.serviceDiscovery.userWorkloads`. Pure pass-through; only
   added field is `status.serviceDiscovery.namespace` mirrored from
   the NebariApp's metadata.
3. CEL validation on the CRD (rules above).
4. API reference + design doc updates.

What the operator does **not** do:

- Run informers on user-workload kinds.
- List, watch, or query any kind on behalf of consumers.
- Pre-check RBAC for declared kinds.
- Apply or invert any encoding / escape / transform.
- Filter children by user identity or by any attribute.
- Enforce per-pack auth.
- Run a validating webhook.

### Code changes in this repo (`nebari-landing`)

The shape from #68 is preserved — kube-cache backs a watcher that fills
a cache, the Hub fans events to clients via Redis. What changes is
*what the watcher watches* and *what the cache stores*.

#### Cache types (`internal/cache/`)

Replace #68's single `child_cache.go` with a generalised structure
keyed by NebariApp UID + (kind, object UID). The object is no longer
always `Pod`.

```go
package cache

import (
    "sync"
    "time"
)

// ChildResource is the typed projection of one user-managing workload.
type ChildResource struct {
    UID         string    `json:"uid"`
    Kind        string    `json:"kind"`        // GVR from userWorkloads[].kind
    DisplayKind string    `json:"displayKind"` // displayKindAs or kind verbatim
    Name        string    `json:"name"`
    Namespace   string    `json:"namespace"`
    StartedAt   time.Time `json:"startedAt"`   // .metadata.creationTimestamp; pods may use status.startTime
    Phase       string    `json:"phase,omitempty"` // Pod.status.phase or CR conditions
    URL         string    `json:"url,omitempty"`
    Owner       string    `json:"-"`           // raw extracted user id; never JSON-emitted
}

// ChildCache: keyed by parent NebariApp UID, then by child object UID.
type ChildCache struct {
    mu       sync.RWMutex
    children map[string]map[string]*ChildResource
}
```

Same separation rationale as #68: keep child churn out of the service
hot path.

#### Watcher (`internal/watcher/`)

Replace #68's hardcoded Pod-informer plan with a *dynamic
per-NebariApp-entry informer manager*.

- On every `ServiceCache` add/update event, read
  `status.serviceDiscovery.userWorkloads[]`.
- For each entry, ensure an informer is running with:
  - GVR from `kind` (resolved via the rest mapper)
  - Namespace from `status.serviceDiscovery.namespace`
  - LabelSelector from the entry's `selector` (if any)
  - Event handler that extracts `user` per the entry's `user.fromX` rule
- On `ServiceCache` delete, tear down all informers for that NebariApp UID.
- On entry change (selector edit, kind addition/removal), reconcile
  informers — close the ones no longer declared, start the new ones.

User-extraction is a small switch on the `user` discriminated union:

```go
func extractOwner(obj *unstructured.Unstructured, user UserWorkloadAttribution) string {
    switch {
    case user.FromLabel != "":
        return obj.GetLabels()[user.FromLabel]
    case user.FromAnnotation != "":
        return obj.GetAnnotations()[user.FromAnnotation]
    case user.FromSpec != "":
        // JSONPath into obj.Object using e.g. k8s.io/client-go/util/jsonpath
        return jsonPath(obj.Object, user.FromSpec)
    }
    return ""
}
```

No transforms applied. If `Owner == ""`, the object is cached but
invisible to non-admin callers.

The existing controller-runtime cache (`internal/watcher/watcher.go`)
already runs as a process; the child watcher participates in the same
manager so informers share a single client/informer factory.

#### Hub event types (`internal/websocket/`)

Three new event types (`child.added`, `child.modified`,
`child.deleted`), envelope includes `kind` and `displayKind`:

```go
type ChildEvent struct {
    Type         EventType      `json:"type"`
    NebariAppUID string         `json:"nebariAppUid"`
    Child        *ChildResource `json:"child"`
}
```

Redis Pub/Sub fan-out unchanged.

#### REST handler (`internal/api/`)

`ServiceView` gains `Children []ChildResource` populated from
`ChildCache.Get(uid)`. Filter rule:

| Caller | What appears in `Children` |
|---|---|
| Anonymous | empty list |
| Authenticated user | only entries where `Owner == claims.PreferredUsername` (literal compare) |
| Member of admin group | full list, with `Owner` exposed per row |

Filter happens *after* the existing `canAccessService` gate.

#### RBAC

The webapi ServiceAccount needs `get`/`list`/`watch` on every kind any
deployed pack declares. Two options:

- **Static expansion** — chart ships a ClusterRole pre-declaring known
  kinds. Simple, but every new kind requires a chart bump.
- **Aggregated ClusterRole** (proposed for v1) — webapi's ClusterRole
  carries
  `aggregationRule.clusterRoleSelectors[].matchLabels:
  rbac.nebari.dev/aggregate-to-landing-watcher: "true"`. Each pack
  ships its own Role with that label granting list/watch on its
  declared kinds. New packs are self-contained; webapi picks up the
  union automatically.

Aggregation is recommended. Worth piloting early.

#### Frontend (`frontend/src/`)

- `components/ServiceGridCard.tsx` — expand affordance when
  `service.children.length > 0`, listing per-child rows: name,
  "running" pill (mapped from `Phase`), duration since `startedAt`,
  link if `URL` set.
- WebSocket client subscribes to `child.*` events alongside service
  events; patches the per-service children list in place.
- Row label is `displayKind` (not `kind`). Multi-shape packs produce
  multiple rows with different `displayKind`s; the UI groups by
  `displayKind`.

External-link icon on the card's primary URL action — small but worth
doing in the same iteration.

### Pack authoring

Packs declare discovery in their own `templates/nebariapp.yaml` by
including a named template defined in the same chart's
`templates/_helpers.tpl`:

```yaml
# templates/nebariapp.yaml (excerpt)
spec:
  landingPage:
    enabled: true
    userWorkloads:
      {{- include "my-pack.userWorkloads.jhubDefaultServers" . | nindent 6 }}
```

The named template lives in the pack's own helpers — pack-local, not in
any central library chart. Pack maintainers absorb the K8s-mechanical
work once per controller. The template repo
(`nebari-software-pack-template`) ships one *generic example* showing
the shape; per-controller wiring is each pack's responsibility.

#### Example helper shapes (per surveyed controller)

**JHub default JupyterLab servers** (one per user):

```yaml
{{- define "my-pack.userWorkloads.jhubDefaultServers" -}}
- kind: Pod
  selector:
    matchLabels:
      app.kubernetes.io/managed-by: kubespawner
      hub.jupyter.org/servername: ""
  user:
    fromAnnotation: hub.jupyter.org/username
  displayKindAs: Notebook server
{{- end }}
```

`fromAnnotation` because KubeSpawner stamps the unescaped username on
the annotation (the matching label is escaped). Per-object GET; cost is
sub-second at the operating scale.

**JHub named servers** (jhub-apps, vscode, pi-coding-agent's `pi`):

```yaml
{{- define "my-pack.userWorkloads.jhubNamedServers" -}}
- kind: Pod
  selector:
    matchLabels:
      app.kubernetes.io/managed-by: kubespawner
    matchExpressions:
      - key: hub.jupyter.org/servername
        operator: NotIn
        values: [""]
  user:
    fromAnnotation: hub.jupyter.org/username
  displayKindAs: App
{{- end }}
```

For packs surfacing a specific named server (`pi-coding-agent`'s `pi`),
narrow the selector to `matchLabels: { hub.jupyter.org/servername: pi }`.

**Dask Gateway per-user DaskClusters**:

```yaml
{{- define "my-pack.userWorkloads.daskGateway" -}}
- kind: daskclusters.gateway.dask.org
  user:
    fromSpec: .spec.username
  displayKindAs: Dask cluster
{{- end }}
```

**Argo Workflows** (creator labels stamped by SSO-authenticated Argo Server):

```yaml
{{- define "my-pack.userWorkloads.argoWorkflow" -}}
- kind: workflows.argoproj.io
  user:
    fromLabel: workflows.argoproj.io/creator-preferred-username
  displayKindAs: Workflow
{{- end }}
```

Direct comparison works when the deployment's `preferred_username` is
already DNS-friendly (the common case in Keycloak with
username-as-username). Deployments using email-as-username need a
separate DNS-friendly claim exposed via Keycloak.

**Ray is intentionally absent.** Upstream Ray has no user-identity
propagation; the current Nebari Ray direction
(`nebari-rayserve-pack`) is a shared `RayService` that doesn't use
this contract. A future per-user Ray pack would have to invent its own
user-stamping convention, at which point the existing `user.fromX`
modes already cover whatever shape gets picked.

### Migration from #68

Issue #68's *Architecture* and *Backend changes* sections are
superseded by this ADR. The frontend section in #68 stays largely
valid; the `displayKind` grouping is small. Once this ADR is accepted,
#68 should be edited to point at it as the current spec.

### Spike findings (rationale)

This section captures *why* the schema is shaped the way it is. Future
contributors who want to understand the constraints before proposing
changes should read it.

#### What was tested

Local kind cluster with:

- `nebari-data-science-pack` (JHub + jhub-apps).
- A wrong-shape Dask spike that wrapped the Dask Kubernetes Operator
  directly and rendered a chart-managed DaskCluster — falsified an
  early assumption.
- A correct Dask Gateway spike that wrapped `dask-gateway` and
  simulated user submission via the Gateway REST API.
- A KubeRay spike that deployed `kuberay-operator` plus a
  chart-managed RayCluster.
- JHub user-spawn end-to-end via the full Keycloak OIDC flow
  (gateway-level oauth2-proxy → Keycloak login → callback → JHub
  session → spawn).
- Survey of every `*-pack` repo in `nebari-dev` for per-user
  attribution patterns.

#### Why these specific design choices

| Decision | Driving finding |
|---|---|
| `user` as discriminated union (label / annotation / spec) | Different controllers stamp attribution in different fields; no single mechanism works |
| `selector` mandatory for multi-shape, optional otherwise | `matchExpressions` needed for jhub-apps; instance-only too coarse for everything |
| `displayKindAs` kept even for single-shape | Multi-shape packs (`data-science-pack`) need it; kind-fallback covers the rest |
| No transforms | Permanent operator/webapi commitment to encoding knowledge violates principles |
| No `fromName` mode | Was useful as a vehicle for transforms + multi-shape disambiguation; both subsumed by `fromX + selector` once transforms are dropped |
| No central library chart | No actual reuse — jhub-apps wiring is `data-science-pack`-specific; pack-local helpers match the existing template-repo convention |
| Operator pass-through only | No need for richer logic; status is internal so future evolution doesn't break packs |
| Worker pods out of scope | Users think in terms of managers; per-worker rows would be noise |

### Implementation order

1. **Operator-side**: types, reconciler pass-through, CEL. Lands in
   `nebari-operator` first.
2. **RBAC pattern decision** (aggregation vs static). Documented here
   and in the chart change in this repo.
3. **`internal/cache/child_cache.go`** + tests.
4. **`internal/watcher/child_watcher.go`**: starts with a single
   hardcoded informer for Pods (JHub pilot), then generalised to
   per-entry dynamic informers as the second pack lands.
5. **Hub envelope + REST handler updates**.
6. **Frontend grouping + WebSocket subscription**.

Step 4's "start hardcoded, generalise later" lets us ship JHub
discovery on this contract before a second pack exists, without baking
JHub assumptions back into the watcher.

### Open questions

- **Which OIDC claim to compare against.** v1 compares against
  `claims.preferred_username` literally. For tools that escape
  (KubeSpawner) or DNS-friendlify (Argo) the value, the comparable
  form must already match. Deployments either use a directly-comparable
  username scheme or Keycloak exposes a custom claim. Deployment-config
  decision, not an operator-design one.
- **Cap on `userWorkloads[]` length.** Proposed: 16.
- **Finer granularity within JHub named servers.** jhub-apps doesn't
  stamp a self-discriminator label; jhub-apps and vscode-server look
  the same at the K8s level. Current contract groups them under one
  "App" row. Finer granularity would require upstream change to
  jhub-apps adding `nebari.dev/app-kind` or similar.
- **Notes on Ray.** Upstream Ray has no user-identity propagation.
  `nebari-rayserve-pack`'s user-traceability gap is worth flagging as
  an issue on that pack repo, separately.
- **Query optimisation, post-v1.** Webapi-side informer cache keyed by
  `(kind, namespace, extracted-user)` would make per-caller lookups
  O(1). Doesn't affect the contract.
- **Generic shape example in template repo.** Whether the discovery
  shape example lives in an existing example
  (`wrap-existing-chart`) or a dedicated discovery example. Style call.

## Links

- [nebari-dev/nebari-landing#68](https://github.com/nebari-dev/nebari-landing/issues/68) — implementation tracking issue (this ADR supersedes its Architecture + Backend sections)
- [nebari-dev/nebari-landing#52](https://github.com/nebari-dev/nebari-landing/issues/52) — original UX request
- [Argo Workflows creator labels](https://argo-workflows.readthedocs.io/en/latest/workflow-creator/)
- [Kubernetes Recommended Labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/)
- [Helm chart best practices — labels](https://helm.sh/docs/chart_best_practices/labels/)
- [Helm library charts](https://helm.sh/docs/topics/library_charts/)
- [Aggregated ClusterRoles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#aggregated-clusterroles)
- [MADR — Markdown Any Decision Records](https://adr.github.io/madr/)
