#!/usr/bin/env bash
# .github/scripts/setup-keycloak-e2e.sh
#
# TEMPORARY WORKAROUND - patches a running Keycloak so the e2e suite can run.
# This script is meant to be removed once the underlying gaps in nebari-operator
# (and the e2e environment bootstrap) are closed.  See "Removal plan" below.
#
# Why this exists
# ---------------
# 1. The nebari-operator provisions the `nebari-system-nebari-landing`
#    confidential client with `directAccessGrantsEnabled=false`, which is the
#    correct default for production.  The e2e suite, however, needs a headless
#    password-grant flow to acquire real user tokens (with `sub`, `groups`,
#    `preferred_username`, etc.) for the pin / notification / access-request
#    contexts.  We patch the flag on via `kcadm` here.
#
#    We deliberately do NOT use the built-in `admin-cli` client for token
#    acquisition: Keycloak 26+ ships `admin-cli` with
#    `client.use.lightweight.access.token.enabled=true`, which strips identity
#    claims (`sub`, `preferred_username`) from its access tokens and breaks
#    every identity-keyed endpoint (e.g. `/api/v1/pins`).
#
# 2. The e2e suite expects a couple of realm fixtures that aren't part of the
#    operator-provisioned defaults:
#      - an unprivileged `test-user` for regular-user flows
#      - an `admin` group with the realm `admin` user as a member
#        (the webapi's admin-gated endpoints check group membership)
#
# Removal plan
# ------------
# * Once nebari-operator exposes `spec.auth.keycloakConfig.directAccessGrantsEnabled`
#   (or an equivalent), the NebariApp CR can declare the flag and the operator
#   reconciles it - the kcadm patch in step (1) goes away entirely.
#   Tracking: https://github.com/nebari-dev/nebari-operator/issues/TBD
# * The fixtures in step (2) should ideally be expressed via NebariApp users /
#   groups, or via a dedicated dev-only Keycloak seed manifest, rather than
#   being created imperatively from CI.
#
# Usage
# -----
#   CI:
#     .github/scripts/setup-keycloak-e2e.sh --env-file "$GITHUB_ENV"
#
#   Local (kind / k3d / minikube - any cluster running the operator-deployed
#   keycloak + nebari-landing):
#     .github/scripts/setup-keycloak-e2e.sh
#     set -a; source ./e2e.env; set +a
#     go test ./test/e2e/... -tags=e2e -v -timeout 10m
#
# Requirements
# ------------
#   - kubectl pointed at the target cluster (current-context)
#   - python3 in PATH (used to parse small JSON blobs from kcadm)
#   - The `keycloak` and `nebari-system` namespaces with their default
#     resources (operator-provisioned)
#
# The script is idempotent - re-running on an already-configured cluster is
# safe.  Existing user / group / membership are detected and preserved; the
# OIDC client patch is unconditional but a no-op when the flag is already true.

set -euo pipefail

ENV_FILE="./e2e.env"
KEYCLOAK_NAMESPACE="${E2E_KEYCLOAK_NAMESPACE:-keycloak}"
NEBARI_NAMESPACE="${E2E_NEBARI_NAMESPACE:-nebari-system}"
REALM="${E2E_KEYCLOAK_REALM:-nebari}"
OIDC_CLIENT_ID="${E2E_OIDC_CLIENT_ID:-nebari-system-nebari-landing}"
WEBAPI_AUDIENCE="${E2E_KEYCLOAK_AUDIENCE:-nebari-landingpage}"
TEST_USER="${E2E_TEST_USER:-test-user}"
TEST_USER_PASS="${E2E_TEST_USER_PASSWORD:-test-user}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      ENV_FILE="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,/^set -e/p' "$0" | sed 's/^# \{0,1\}//' | sed '$d'
      exit 0 ;;
    *)
      echo "unknown arg: $1" >&2
      exit 2 ;;
  esac
done

# Logs go to stderr so stdout stays clean for Actions workflow commands.
log()     { printf '  %s\n' "$*" >&2; }
section() { printf '\n>> %s\n' "$*" >&2; }

# Emit a workflow command on stdout when running under GitHub Actions.
mask() {
  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    printf '::add-mask::%s\n' "$1"
  fi
}

emit_env() { printf '%s=%s\n' "$1" "$2" >> "$ENV_FILE"; }

kc() {
  kubectl exec -n "$KEYCLOAK_NAMESPACE" "$KEYCLOAK_POD" -- \
    /opt/keycloak/bin/kcadm.sh "$@"
}

# kcadm queries return JSON arrays; pull the `id` of the first element.
first_id() {
  python3 -c \
    "import sys, json; arr = json.load(sys.stdin); print(arr[0]['id'] if arr else '')"
}

section "Preflight"
command -v kubectl >/dev/null || { echo "kubectl not found" >&2; exit 1; }
command -v python3 >/dev/null || { echo "python3 not found" >&2; exit 1; }
log "kubectl context: $(kubectl config current-context 2>/dev/null || echo '<none>')"

KEYCLOAK_POD=$(kubectl -n "$KEYCLOAK_NAMESPACE" get pod \
  -l app.kubernetes.io/name=keycloakx \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
[[ -n "$KEYCLOAK_POD" ]] || {
  echo "no keycloakx pod found in namespace '$KEYCLOAK_NAMESPACE'" >&2
  exit 1
}
log "keycloak pod: $KEYCLOAK_POD"

section "Reading credentials"
ADMIN_PASS=$(kubectl -n "$KEYCLOAK_NAMESPACE" get secret keycloak-admin-credentials \
  -o jsonpath='{.data.admin-password}' | base64 -d)
CLIENT_SECRET=$(kubectl -n "$NEBARI_NAMESPACE" get secret nebari-landing-oidc-client \
  -o jsonpath='{.data.client-secret}' | base64 -d)
mask "$CLIENT_SECRET"
emit_env "E2E_OIDC_CLIENT_SECRET" "$CLIENT_SECRET"
log "fetched master-realm admin password and OIDC client secret"

section "Authenticating kcadm against the master realm"
kc config credentials \
  --server http://localhost:8080 --realm master \
  --user admin --password "$ADMIN_PASS" >/dev/null
log "authenticated"

section "Patch ${OIDC_CLIENT_ID} (directAccessGrantsEnabled=true)"
CLIENT_UUID=$(kc get clients -r "$REALM" \
  -q "clientId=${OIDC_CLIENT_ID}" --fields id | first_id)
[[ -n "$CLIENT_UUID" ]] || {
  echo "client '$OIDC_CLIENT_ID' not found in realm '$REALM'" >&2
  exit 1
}
kc update "clients/${CLIENT_UUID}" -r "$REALM" \
  -s directAccessGrantsEnabled=true >/dev/null
log "directAccessGrantsEnabled=true"

section "Ensure ${OIDC_CLIENT_ID} access tokens include audience '${WEBAPI_AUDIENCE}'"
AUDIENCE_MAPPER=$(kc get "clients/${CLIENT_UUID}/protocol-mappers/models" -r "$REALM" \
  | python3 -c \
    "import sys, json; print(next((m.get('id','') for m in json.load(sys.stdin) if m.get('name') == 'landing-page-api-audience'), ''))")
if [[ -z "$AUDIENCE_MAPPER" ]]; then
  kc create "clients/${CLIENT_UUID}/protocol-mappers/models" -r "$REALM" \
    -s name=landing-page-api-audience \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-audience-mapper \
    -s "config.\"included.custom.audience\"=${WEBAPI_AUDIENCE}" \
    -s 'config."id.token.claim"=false' \
    -s 'config."access.token.claim"=true' >/dev/null
  log "audience mapper added"
else
  kc update "clients/${CLIENT_UUID}/protocol-mappers/models/${AUDIENCE_MAPPER}" -r "$REALM" \
    -s name=landing-page-api-audience \
    -s protocol=openid-connect \
    -s protocolMapper=oidc-audience-mapper \
    -s "config.\"included.custom.audience\"=${WEBAPI_AUDIENCE}" \
    -s 'config."id.token.claim"=false' \
    -s 'config."access.token.claim"=true' >/dev/null
  log "audience mapper updated ($AUDIENCE_MAPPER)"
fi

section "Ensure user '${TEST_USER}' exists"
TEST_USER_ID=$(kc get users -r "$REALM" -q "username=${TEST_USER}" --fields id | first_id)
if [[ -z "$TEST_USER_ID" ]]; then
  kc create users -r "$REALM" \
    -s "username=${TEST_USER}" -s enabled=true \
    -s "email=${TEST_USER}@nebari.test" \
    -s firstName=Test -s lastName=User >/dev/null
  TEST_USER_ID=$(kc get users -r "$REALM" -q "username=${TEST_USER}" --fields id | first_id)
  log "created ($TEST_USER_ID)"
else
  log "already exists ($TEST_USER_ID)"
fi
kc set-password -r "$REALM" --userid "$TEST_USER_ID" --new-password "$TEST_USER_PASS" >/dev/null
log "password set"

section "Ensure 'admin' group exists with realm admin as a member"
GROUP_ID=$(kc get groups -r "$REALM" -q search=admin --fields id,name \
  | python3 -c "import sys, json; print(next((g['id'] for g in json.load(sys.stdin) if g.get('name')=='admin'), ''))")
if [[ -z "$GROUP_ID" ]]; then
  GROUP_ID=$(kc create groups -r "$REALM" -s name=admin -i)
  log "group created ($GROUP_ID)"
else
  log "group already exists ($GROUP_ID)"
fi

REALM_ADMIN_ID=$(kc get users -r "$REALM" -q username=admin --fields id | first_id)
[[ -n "$REALM_ADMIN_ID" ]] || {
  echo "no realm admin user 'admin' found in realm '$REALM'" >&2
  exit 1
}

# PUT /admin/realms/{realm}/users/{id}/groups/{gid} is idempotent -
# Keycloak returns 204 whether or not the membership already existed.
kc update "users/${REALM_ADMIN_ID}/groups/${GROUP_ID}" -r "$REALM" \
  -s realm="$REALM" -s "userId=${REALM_ADMIN_ID}" -s "groupId=${GROUP_ID}" --no-merge >/dev/null
log "realm admin in 'admin' group"

section "Wrote env file: $ENV_FILE"
log "next: set -a; source $ENV_FILE; set +a; go test -tags=e2e ./test/e2e/..."
