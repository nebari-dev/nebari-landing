# -*- mode: Python -*-
# Tiltfile for nebari-landing local development.
#
# Sits on top of an existing `make -f dev/Makefile setup` cluster — Tilt
# does not bootstrap Keycloak, MetalLB, the operator, or the foundational
# stack. It only owns the webapi + frontend iteration loop, replacing the
# manual `make image-build install` cycle with a watch-rebuild-redeploy
# loop that reacts on every save.
#
# Prerequisites:
#   1. The local minikube cluster is running:
#          make -f dev/Makefile setup
#   2. Docker points at minikube's daemon so Tilt-built images are visible
#      to the cluster without a separate load step:
#          eval $(minikube --profile=nebari-local docker-env)
#   3. Tilt CLI installed: https://docs.tilt.dev/install.html
#
# Then:
#   tilt up    # opens the Tilt UI at http://localhost:10350
#   tilt down  # tears down everything Tilt created (leaves cluster intact)
#
# References:
#   https://docs.tilt.dev/api.html#api.docker_build
#   https://docs.tilt.dev/helm.html
#   https://github.com/nebari-dev/nebari-data-science-pack/blob/main/Tiltfile

# Image pulls / pod rollouts can be slow on first start. Default is 30 s.
update_settings(k8s_upsert_timeout_secs=600)

# Refuse to deploy anywhere except the local minikube context. Catches the
# common mistake of running `tilt up` with kubectl pointed at a real cluster.
# `make setup` provisions a cluster named `nebari-local`; the corresponding
# kubectl context has the same name.
allow_k8s_contexts('nebari-local')


# ── webapi ────────────────────────────────────────────────────────────────
# `only=` restricts the watched files inside the build context so that
# editing the chart, the frontend, or dev/ scripts does not trigger a Go
# rebuild. Paths are relative to the context (the repo root).
docker_build(
    'nebari-landing/webapi',
    context='.',
    dockerfile='Dockerfile',
    only=[
        'cmd',
        'internal',
        'go.mod',
        'go.sum',
    ],
)


# ── frontend ──────────────────────────────────────────────────────────────
# Build context is `frontend/` to match the Dockerfile's expectations.
# `only=` paths are relative to that context.
docker_build(
    'nebari-landing/nebari-landing',
    context='frontend',
    dockerfile='frontend/Dockerfile',
    only=[
        'src',
        'public',
        'index.html',
        'package.json',
        'package-lock.json',
        'tsconfig.json',
        'tsconfig.app.json',
        'tsconfig.node.json',
        'vite.config.ts',
        'eslint.config.js',
        'components.json',
    ],
)


# ── Render the chart ──────────────────────────────────────────────────────
# Same chart + same dev values as `make install`, so the manifests Tilt
# applies match what `helm install` would produce. Tilt detects the image
# references in the rendered manifests and substitutes its own immutable
# tags for the placeholder `:dev` so each rebuild rolls the deployment.
k8s_yaml(helm(
    'charts/nebari-landing',
    name='nebari-landing',
    namespace='nebari-system',
    values=['dev/chart-values.yaml'],
))


# ── Port-forwards + UI labels ─────────────────────────────────────────────
# The cluster already has MetalLB IPs (192.168.49.x) for these services;
# the localhost forwards below are convenience for the Tilt UI's "open in
# browser" buttons and for anyone who prefers localhost URLs.
k8s_resource(
    workload='nebari-landing-webapi',
    port_forwards=['8090:8080'],
    labels=['app'],
)
k8s_resource(
    workload='nebari-landing-frontend',
    port_forwards=['8080:8080'],
    labels=['app'],
)

# Redis comes from the bitnami subchart; group it so it doesn't clutter the
# top-level resource list in the UI.
k8s_resource(
    workload='nebari-landing-redis-master',
    labels=['infra'],
)
