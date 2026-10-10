#!/usr/bin/env bash
# Renders the chart with the static-services settings and checks the
# STATIC_SERVICES env the webapi receives (see the nebari-landing.staticServices
# helper). Requires helm and jq; run after `helm dependency build`.
set -euo pipefail

chart="$(cd "$(dirname "$0")/.." && pwd)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# Prints the decoded STATIC_SERVICES JSON, or nothing when the env is absent.
static_services() {
  helm template nebari-landing "$chart" --namespace nebari-system "$@" \
    | grep -A1 'name: STATIC_SERVICES' \
    | sed -n 's/^ *value: //p' \
    | jq -r .
}

# Expects rendering to fail with a message containing $1.
expect_render_error() {
  local want="$1"
  shift
  local out
  if out=$(helm template nebari-landing "$chart" --namespace nebari-system "$@" 2>&1); then
    fail "expected render to fail with \"$want\""
  fi
  grep -qF "$want" <<<"$out" || fail "render error does not mention \"$want\": $out"
}

toggle=(
  --set webapi.keycloak.landingPage.enabled=true
  --set frontend.keycloak.url=https://keycloak.example.com//
  --set webapi.keycloak.url=http://keycloak.keycloak.svc:8080/
)

# Default install: nothing rendered.
[ -z "$(static_services)" ] || fail "STATIC_SERVICES rendered with default values"

# Toggle on: one Keycloak entry derived from the Keycloak settings.
got=$(static_services "${toggle[@]}")
jq -e '
  length == 1 and .[0] == {
    id: "keycloak",
    displayName: "Keycloak",
    description: "Identity and access administration",
    url: "https://keycloak.example.com/admin/nebari/console/",
    category: "Platform",
    priority: 90,
    visibility: "private",
    requiredGroups: ["/keycloak-admins"],
    healthCheck: {url: "http://keycloak.keycloak.svc:8080/realms/nebari"}
  }' <<<"$got" >/dev/null || fail "unexpected Keycloak entry: $got"

# Custom entries are kept alongside the Keycloak entry.
got=$(static_services "${toggle[@]}" \
  --set 'webapi.staticServices[0].id=grafana-admin' \
  --set 'webapi.staticServices[0].url=https://grafana.example.com/admin')
jq -e '[.[].id] == ["grafana-admin", "keycloak"]' <<<"$got" >/dev/null \
  || fail "custom entry not merged: $got"

# Misconfigurations fail at render time instead of crash-looping the webapi or
# widening the card's audience.
expect_render_error "frontend.keycloak.url is required" \
  --set webapi.keycloak.landingPage.enabled=true
expect_render_error 'must not define id "keycloak"' "${toggle[@]}" \
  --set 'webapi.staticServices[0].id=keycloak' \
  --set 'webapi.staticServices[0].url=https://keycloak.example.com'
expect_render_error "requiredGroups must not be empty" "${toggle[@]}" \
  --set 'webapi.keycloak.landingPage.requiredGroups=null'

echo "static services render assertions passed"
