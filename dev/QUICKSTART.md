# Nebari Landing — Local Dev Quick-Start

This guide explains the local development environment: what it sets up, why each piece exists, and how to work with it
day-to-day.



## Prerequisites

| Tool | Minimum version | Notes |
|------|-----------------|-------|
| `docker` | 24+ | Used as the minikube driver |
| `kubectl` | 1.28+ | Cluster interaction |
| `helm` | 3.14+ | Installs Keycloak and PostgreSQL |
| `python3` | 3.10+ | Integration test script only |
| `minikube` | any | Auto-downloaded to `.bin/` if absent |

All Make targets are run from the **repository root**:

```sh
make -f dev/Makefile <target>
# or add an alias:
alias mk='make -f dev/Makefile'
```



## What gets set up

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│  Host machine                                                                   │
│                                                                                 │
│  Browser → http://192.168.49.102/    (landing page + oauth2-proxy)             │
│  curl    → http://192.168.49.101:8080/api/v1/services  (webapi)               │
│  browser → http://192.168.49.100/auth/admin            (Keycloak UI)           │
│                                                                                 │
│               minikube cluster (docker driver, 4 CPU / 8 GB)                  │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                                                                         │   │
│  │  MetalLB (L2, 192.168.49.100–150)                                      │   │
│  │                                                                         │   │
│  │  namespace: keycloak                                                    │   │
│  │  ┌──────────────────────────────────┐                                   │   │
│  │  │  PostgreSQL (chart)              │                                   │   │
│  │  │  Keycloak (keycloakx chart)      │ ← 192.168.49.100:80              │   │
│  │  │    realm: nebari                  │                                   │   │
│  │  │    clients: webapi, nebari-landingpage │                                │   │
│  │  │    users:   admin (group: admin)  │                                   │   │
│  │  └──────────────────────────────────┘                                   │   │
│  │                                                                         │   │
│  │  namespace: nebari-system  (label: nebari.dev/managed=true)            │   │
│  │  ┌──────────────────────────────────┐                                   │   │
│  │  │  webapi (nebari-operator image)  │ ← 192.168.49.101:8080            │   │
│  │  │    validates JWTs from Keycloak  │                                   │   │
│  │  │    reads NebariApp CRs via watch │                                   │   │
│  │  └──────────────────────────────────┘                                   │   │
│  │  ┌──────────────────────────────────┐                                   │   │
│  │  │  nebari-landingpage pod          │ ← 192.168.49.102:80              │   │
│  │  │  ┌───────────────┐ ┌──────────┐  │                                   │   │
│  │  │  │ oauth2-proxy  │→│  nginx   │  │                                   │   │
│  │  │  │  :4180        │ │  :8080   │  │                                   │   │
│  │  │  └───────────────┘ └──────────┘  │                                   │   │
│  │  └──────────────────────────────────┘                                   │   │
│  │  NebariApp CRs: docs, jupyterhub, grafana, admin-panel, disabled-app   │   │
│  │                                                                         │   │
│  │  namespace: nebari-operator-system                                      │   │
│  │  ┌──────────────────────────────────┐                                   │   │
│  │  │  nebari-operator (controller)    │                                   │   │
│  │  │    reconciles NebariApp CRs      │                                   │   │
│  │  └──────────────────────────────────┘                                   │   │
│  │                                                                         │   │
│  │  namespace: cert-manager  (required by operator)                       │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key design decisions

**MetalLB instead of port-forwards** MetalLB assigns real IPs from `192.168.49.100–150` (inside minikube's docker bridge
subnet). All three IPs are reachable from the host _and_ from pods, so the JWT `iss` claim
(`http://192.168.49.100/auth/…`) is verifiable by both the browser and the webapi without any routing tricks.

**oauth2-proxy as a sidecar** The landing page container (nginx) doesn't know about authentication — that's
OAuth2-proxy's job. oauth2-proxy runs on port 4180 next to nginx (port 8080) in the same pod. The LoadBalancer points at
port 4180. On every authenticated request, oauth2-proxy injects `Authorization: Bearer <token>` as an upstream header.
nginx exposes `GET /auth-token` which returns `{"access_token": "<token>"}` from that header so the SPA can use it for
webapi calls.

**NebariApp-driven Keycloak client provisioning** The `nebari-landingpage` NebariApp CR declares
`spec.auth.enabled: true` and `spec.auth.provisionClient: true`. When the nebari-operator is fully operational (requires
`envoy-gateway-system`), it reads this spec and creates a Secret named `nebari-landingpage-oidc-client` with keys
`client-id` and `client-secret`. oauth2-proxy reads its client credentials from that Secret — no hardcoded values in the
pod spec.

_Dev fallback:_ Until the operator's TLS/Envoy step is unblocked, `make frontend-client` runs a kcadm Job to create the
Keycloak client and then writes the same Secret manually so the oauth2-proxy sidecar starts cleanly.

**NebariApp CRD → webapi cache** The nebari-operator watches `NebariApp` custom resources in `nebari-system` and
populates the webapi's in-memory service cache. The webapi then serves that cache at `GET /api/v1/services`, bucketed by
`visibility: public | authenticated | private`. Test CRs are in `dev/manifests/test-nebariapps.yaml`.



## Running on WSL2 (Windows)

MetalLB uses L2/ARP to advertise the `192.168.49.x` IP pool. In default WSL2
networking, that ARP traffic never crosses the Hyper-V VM boundary, so the
MetalLB IPs are unreachable from the Windows host browser.

There are three ways to fix this, in order of recommendation:

---

### Option A — WSL2 mirrored networking (Windows 11 24H2+, recommended)

Mirrored mode bridges the WSL2 VM NIC to the Windows host network stack.
However, the minikube docker bridge (`192.168.49.0/24`) is an *internal* Linux
bridge — it is not automatically exposed to Windows by mirrored mode alone.
You also need `minikube tunnel`, which injects host routes into the Linux kernel;
in mirrored mode those routes propagate to the Windows routing table, making the
MetalLB IPs reachable from the Windows browser.

**One-time Windows setup:**

1. Check your Windows build: `winver` → must show build ≥ 26100 (24H2).

2. Create or edit `%USERPROFILE%\.wslconfig`:

   ```ini
   [wsl2]
   networkingMode=mirrored

   [experimental]
   # Required for mirrored mode to propagate internal routes to the Windows host.
   hostAddressLoopback=true
   ```

3. Restart WSL2 from PowerShell (Admin):

   ```powershell
   wsl --shutdown
   # wait ~2 s, then reopen your WSL terminal
   ```

**dev session setup (run once per session):**

```sh
# 1. Full cluster bootstrap
make -f dev/Makefile setup

# 2. Inject routes so MetalLB IPs reach Windows (keep this running)
make -f dev/Makefile minikube-tunnel
```

`minikube tunnel` must stay running for the duration of the session. It may
prompt for `sudo` (it modifies the kernel routing table). If it does, run it
in a separate terminal directly:

```sh
sudo minikube tunnel -p nebari-local
```

Note on IP pool: the `metallb` target auto-detects the subnet from `minikube ip`
at runtime (it no longer uses the hardcoded `dev/manifests/metallb/config.yaml`).
The pool will always be `.100–.150` within whatever subnet minikube assigned.
With the docker driver and the `nebari-local` profile name, minikube consistently
uses `192.168.49.0/24`, so the IPs in this guide stay valid. If you rename the
cluster via `CLUSTER_NAME=…`, the pool re-calculates automatically.

**Verify connectivity from Windows** (after `minikube tunnel` is running):

```powershell
# From PowerShell on Windows:
curl http://192.168.49.100/auth/admin
# Should return a redirect (302) — not a timeout
```

If you still get a timeout, add a Windows Defender Firewall inbound rule allowing
TCP traffic from `192.168.49.0/24`.

---

### Option B — Docker Desktop (any Windows version)

Docker Desktop routes container networks through `vpnkit`, making the minikube
docker bridge accessible from Windows regardless of WSL2 networking mode.

1. Install [Docker Desktop for Windows](https://docs.docker.com/desktop/install/windows-install/).
2. In Docker Desktop → Settings → Resources → WSL Integration: enable your WSL2 distro.
3. **Run `make setup` from a Windows terminal (PowerShell/CMD)**, not from WSL2 —
   minikube must target the Windows Docker daemon, not the WSL2 one.
   ```powershell
   cd \path\to\nebari-landing
   make -f dev/Makefile setup
   ```

   Alternatively, from WSL2 point Docker at the Windows socket:
   ```sh
   export DOCKER_HOST=npipe:////./pipe/docker_engine
   make -f dev/Makefile setup
   ```

---

### Option C — port-forward mode (any Windows version, no extra software)

If neither option above is available, use the WSL-specific targets that skip
MetalLB entirely and expose services via `kubectl port-forward` on `localhost`.
WSL2 automatically forwards `localhost:<PORT>` to `127.0.0.1` on the Windows host.

```sh
make -f dev/Makefile wsl-setup
```

**Service URLs in port-forward mode:**

| Service | URL |
|---------|-----|
| Landing page | `http://localhost:8080/` |
| WebAPI | `http://localhost:8090/api/v1/` |
| Keycloak admin | `http://localhost:8180/auth/admin` |

The port-forwards run in the background. Restart them at any time with:

```sh
make -f dev/Makefile port-forward     # start / restart
make -f dev/Makefile stop-port-forward # stop
```

**How it differs from the MetalLB setup** (see `dev/keycloak/values-wsl.yaml`
and `dev/manifests/nebari-landingpage/overlays/wsl/`):
- Keycloak `KC_HOSTNAME_URL=http://localhost:8180/auth` (the JWT `iss` matches
  the Windows-visible URL).
- `KC_HOSTNAME_BACKCHANNEL_DYNAMIC=true` — pods fetch Keycloak discovery via the
  cluster-internal service name; the returned `token_endpoint` / `jwks_uri` use
  that same internal hostname so back-channel calls never go through the forward.
- `proxy.mode=none` — `kubectl port-forward` injects no `X-Forwarded-*` headers;
  `xforwarded` mode (the default) would cause the admin console to hang.
- oauth2-proxy `--oidc-issuer-url` points to the cluster-internal Keycloak
  service; `--skip-oidc-issuer-verification` suppresses the issuer-URL mismatch.

---

## Quick-start (all-in-one)

```sh
make -f dev/Makefile setup
```

This runs the full sequence:
1. Downloads minikube if absent
2. Creates/starts the `nebari-local` minikube cluster (4 CPU, 8 GB)
3. Builds the Docker image and loads it into minikube
4. Installs cert-manager + selfsigned ClusterIssuer
5. Enables MetalLB and configures the IP pool
6. Installs PostgreSQL + Keycloak, creates the `nebari` realm, users, and groups
7. Creates the `webapi` public OIDC client
8. Creates the `nebari-landingpage` confidential OIDC client + writes Secret `nebari-landingpage-oidc-client` (dev
   fallback for operator provisioning)
9. Deploys the nebari-operator + webapi
10. Deploys the landing page with oauth2-proxy sidecar
11. Prints the info summary

After completion you can open `http://192.168.49.102/` in a browser — you'll be redirected to Keycloak login and then
back to the landing page.



## Service URLs (no port-forwarding needed)

| Service | URL | Credentials |
|---------|-----|-------------|
| Landing page | `http://192.168.49.102/` | log in as `admin` / `nebari-realm-admin` |
| WebAPI | `http://192.168.49.101:8080/api/v1/` | — |
| Keycloak admin | `http://192.168.49.100/auth/admin` | `admin` / `nebari-admin-secret` |



## Day-to-day workflow

### Rebuild the frontend after source changes

```sh
make -f dev/Makefile image-build install
```

`image-build` rebuilds the Docker image (nginx + React build) and loads it into minikube. `install` re-applies the
kustomize overlay and triggers a rolling restart.

### Run the integration tests

```sh
python3 dev/webapi_test.py \
  --webapi-url   http://192.168.49.101:8080 \
  --keycloak-url http://192.168.49.100/auth \
  -u admin -p nebari-realm-admin
```

Tests cover: health, unauthenticated services, authenticated services (all three visibility buckets), categories,
single-service lookup, and pins CRUD.

### Apply or change NebariApp test fixtures

```sh
# Edit dev/manifests/test-nebariapps.yaml, then:
make -f dev/Makefile test-apps
```

The webapi reconciles within seconds (watch the logs with
`kubectl logs -n nebari-system -l app.kubernetes.io/component=webapi -f`).

### Recreate Keycloak clients from scratch

```sh
make -f dev/Makefile keycloak-client      # webapi public client
make -f dev/Makefile frontend-client      # nebari-landingpage confidential client + k8s Secret
```

The `frontend-client` target does two things:
1. Runs a kcadm Job to create the `nebari-landingpage` Keycloak client with the groups mapper and redirect URI.
2. Writes a k8s Secret `nebari-landingpage-oidc-client` (keys: `client-id`, `client-secret`) that mirrors what the
   operator would create when `spec.auth.provisionClient: true`.

### Tear down and restart

```sh
make -f dev/Makefile uninstall            # remove app resources (keep cluster)
make -f dev/Makefile cluster-delete       # destroy the minikube cluster entirely
```



## Directory layout

```
dev/
├── QUICKSTART.md                   ← you are here
├── Dockerfile                      ← multi-stage: node build → nginx:alpine
├── nginx.conf                      ← non-root nginx; exposes /auth-token for SPA
├── Makefile                        ← all automation targets
├── webapi_test.py                  ← integration test suite
│
├── keycloak/
│   ├── values.yaml                 ← Helm values for keycloakx chart
│   │                                  (LoadBalancer, KC_HOSTNAME_URL)
│   ├── values-wsl.yaml             ← Overlay for WSL port-forward mode
│   │                                  (NodePort, localhost URLs, proxy.mode=none)
│   ├── postgresql-values.yaml      ← Helm values for PostgreSQL chart
│   ├── realm-setup-job.yaml        ← Job: creates realm, users, groups via kcadm
│   ├── webapi-client-job.yaml      ← Job: creates 'webapi' public OIDC client
│   ├── frontend-client-job.yaml    ← Job: dev fallback — creates 'nebari-landingpage'
│   │                                  confidential OIDC client (used by oauth2-proxy).
│   │                                  Normally the operator provisions this via NebariApp spec.auth
│   └── nebari-realm.json           ← exported realm snapshot (reference only)
│
└── manifests/
    ├── cert-manager/
    │   └── selfsigned-clusterissuer.yaml
    │
    ├── metallb/
    │   └── config.yaml             ← IP pool 192.168.49.100-150 (ConfigMap, v0.9)
    │
    ├── test-nebariapps.yaml        ← Sample NebariApp CRs loaded into webapi cache
    │
    ├── nebari-landingpage/
    │   ├── base/                   ← Deployment + Service + ServiceAccount + NebariApp CR
    │   └── overlays/
    │       ├── dev/                ← MetalLB LoadBalancer overlay (default)
    │       │   ├── kustomization.yaml
    │       │   ├── deployment-patch.yaml  ← Adds oauth2-proxy sidecar container
    │       │   ├── service-patch.yaml     ← Changes service to LoadBalancer (LB_IP_LANDING)
    │       │   └── oauth2proxy-secret.yaml
    │       └── wsl/                ← NodePort + localhost overlay (make wsl-setup)
    │           ├── kustomization.yaml
    │           ├── deployment-patch.yaml  ← oauth2-proxy with cluster-internal KC URL
    │           ├── service-patch.yaml     ← NodePort 30080
    │           └── oauth2proxy-secret.yaml
    │
    ├── nebari-operator/
    │   ├── operator/               ← Pulls from nebari-operator repo (kustomize remote)
    │   ├── webapi/                 ← Pulls webapi manifest; patches env + service type (LoadBalancer)
    │   └── webapi-wsl/             ← Same but NodePort + localhost OIDC URLs (make wsl-setup)
    │
    └── argocd/                     ← ArgoCD Application CRs (for GitOps; use envsubst)
```



## Authentication flow (end-to-end)

```
Browser                oauth2-proxy          Keycloak
  │                      (4180)             (192.168.49.100)
  │─── GET / ──────────►│
  │                      │─── 302 ──────────────────────────────────────►│
  │◄── 302 (to Keycloak login) ─────────────────────────────────────────│
  │─── POST /login ─────────────────────────────────────────────────────►│
  │◄── 302 (to /oauth2/callback?code=...) ─────────────────────────────│
  │─── GET /oauth2/callback ───────────►│
  │                      │─── token exchange ──────────────────────────►│
  │                      │◄── access_token + id_token ─────────────────│
  │◄── 302 / + session cookie ──────────│
  │─── GET / (with cookie) ────────────►│
  │                      │─── GET localhost:8080/ (+ Authorization: Bearer <at>)
  │                      │              nginx
  │◄── 200 HTML ────────────────────────────────────────────────────────│

SPA (in browser)
  │─── GET /auth-token ─────────────────────────────────────────────────►nginx
  │◄── {"access_token": "<at>"} ────────────────────────────────────────│
  │─── GET /api/v1/services (Authorization: Bearer <at>) ──────────────►webapi
  │◄── {services: {public:[...], authenticated:[...], private:[...]}} ──│
```



## Keycloak test configuration

| Thing | Value |
|-------|-------|
| Realm | `nebari` |
| Admin user | `admin` / `nebari-realm-admin` |
| Admin group | `admin` (gives access to `visibility: private` services) |
| OIDC issuer | `http://192.168.49.100/auth/realms/nebari` |
| `webapi` client | public, direct grants — used by the test script |
| `nebari-landingpage` client | confidential, standard flow — used by oauth2-proxy |
| Client Secret (dev) | `nebari-frontend-dev-secret` (in k8s Secret `nebari-landingpage-oidc-client`) |
| Groups claim | `groups` mapper attached to both clients |
| Operator Secret | `nebari-landingpage-oidc-client` — keys `client-id`, `client-secret` |



## Troubleshooting

### Pod is `CrashLoopBackOff`

```sh
kubectl get pods -n nebari-system
kubectl logs -n nebari-system <pod> -c oauth2-proxy   # or -c nebari-landingpage
kubectl logs -n nebari-system <pod> -c nebari-landingpage
```

### webapi returns `public: 0 services`

The operator and webapi watch NebariApp CRs. Re-apply them:

```sh
make -f dev/Makefile test-apps
kubectl logs -n nebari-system -l app.kubernetes.io/component=webapi --tail=30
```

### JWT validation fails (`invalid issuer`)

The `KEYCLOAK_ISSUER_URL` in the webapi must be the _base_ URL — the webapi appends `/realms/<realm>` itself. Check:

```sh
kubectl get deploy webapi -n nebari-system \
  -o jsonpath='{.spec.template.spec.containers[0].env}' | python3 -m json.tool \
  | grep -A1 KEYCLOAK_ISSUER_URL
# Should be: "http://192.168.49.100/auth"  (no /realms/nebari suffix)
```

The JWT `iss` claim must exactly match what Keycloak's OIDC discovery returns. Check Keycloak's discovery:

```sh
curl -s http://192.168.49.100/auth/realms/nebari/.well-known/openid-configuration \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['issuer'])"
# Should be: http://192.168.49.100/auth/realms/nebari
```

### MetalLB services stuck in `<pending>`

```sh
kubectl get configmap config -n metallb-system -o yaml
kubectl get pods -n metallb-system
```

MetalLB v0.9.6 (minikube addon) uses a ConfigMap — not the newer CRD API. The pool is in
`dev/manifests/metallb/config.yaml`.

### A kcadm Job is stuck / not completing

All kcadm jobs connect to Keycloak on its cluster-internal service, which now listens on **port 80** (not 8080) after
the Helm upgrade to LoadBalancer. The URL used is: `http://keycloak-keycloakx-http.keycloak.svc.cluster.local/auth`

If you need to run a job again:

```sh
kubectl delete job <job-name> -n keycloak --ignore-not-found
kubectl apply -f dev/keycloak/<job-file>.yaml
kubectl wait --for=condition=complete job/<job-name> -n keycloak --timeout=5m
kubectl logs -n keycloak -l job-name=<job-name>
```

### Full reset

```sh
make -f dev/Makefile cluster-delete   # wipe the cluster
make -f dev/Makefile setup            # rebuild from scratch (~10 min)
```
