#!/usr/bin/env python3
"""
Local WebAPI integration test script for the Nebari landing page.

Runs a series of checks against the port-forwarded webapi and Keycloak:
  - Unauthenticated /api/v1/services  → only public bucket populated
  - Authenticated /api/v1/services    → public + authenticated buckets
  - /api/v1/categories
  - /api/v1/health
  - Single service lookup (authenticated)

Prerequisites:
    pip install requests

Port-forwards (run in separate terminals before this script):
    kubectl -n keycloak      port-forward svc/keycloak-keycloakx-http 8180:80
    kubectl -n nebari-system port-forward svc/webapi                  8090:8080

Also apply test NebariApps:
    make -f dev/Makefile test-apps

Usage:
    python dev/webapi_test.py
    python dev/webapi_test.py -u admin -p nebari-realm-admin
    python dev/webapi_test.py --keycloak-url http://localhost:8180/auth
    python dev/webapi_test.py --webapi-url http://localhost:8090 --path /api/v1/health
"""

import argparse
import getpass
import json
import sys
from typing import Optional

try:
    import requests
except ImportError:
    sys.exit("Missing dependency: pip install requests")

# ── Local defaults (port-forwarded) ───────────────────────────────────────────
KEYCLOAK_URL  = "http://localhost:8180/auth"
WEBAPI_URL    = "http://localhost:8090"
DEFAULT_REALM = "nebari"
DEFAULT_PATH  = "/api/v1/services"

PASS_MARK = "✅"
FAIL_MARK = "❌"
SKIP_MARK = "⚠️ "


# ── Auth helpers ───────────────────────────────────────────────────────────────

def get_token_password(
    keycloak_url: str,
    realm: str,
    username: str,
    password: str,
    client_id: str = "webapi",
    client_secret: Optional[str] = None,
) -> str:
    token_url = f"{keycloak_url}/realms/{realm}/protocol/openid-connect/token"
    data: dict = {
        "grant_type": "password",
        "client_id": client_id,
        "username": username,
        "password": password,
    }
    if client_secret:
        data["client_secret"] = client_secret
    resp = requests.post(token_url, data=data, timeout=10)
    resp.raise_for_status()
    return resp.json()["access_token"]


def call_api(base_url: str, path: str, token: Optional[str] = None) -> requests.Response:
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    url = base_url.rstrip("/") + path
    return requests.get(url, headers=headers, timeout=10)


def dump(obj: dict, indent: int = 2) -> str:
    return json.dumps(obj, indent=indent)


# ── Test runners ───────────────────────────────────────────────────────────────

def check(label: str, condition: bool, detail: str = "") -> bool:
    mark = PASS_MARK if condition else FAIL_MARK
    suffix = f"  ({detail})" if detail else ""
    print(f"  {mark}  {label}{suffix}")
    return condition


def section(title: str) -> None:
    print(f"\n{'─' * 60}")
    print(f"  {title}")
    print(f"{'─' * 60}")


def run_health_check(webapi_url: str) -> bool:
    section("GET /api/v1/health")
    try:
        r = call_api(webapi_url, "/api/v1/health")
        ok = check("HTTP 200", r.status_code == 200, f"got {r.status_code}")
        if ok:
            body = r.json()
            check("status=healthy", body.get("status") == "healthy", dump(body))
        return ok
    except Exception as e:
        check("Request succeeded", False, str(e))
        return False


def run_unauthenticated_services(webapi_url: str) -> dict:
    section("GET /api/v1/services  (unauthenticated)")
    try:
        r = call_api(webapi_url, "/api/v1/services")
        check("HTTP 200", r.status_code == 200, f"got {r.status_code}")
        body = r.json()
        services = body.get("services", {})
        public        = services.get("public", [])
        authenticated = services.get("authenticated", [])
        private       = services.get("private", [])
        user          = body.get("user")

        check("public bucket present",        isinstance(public, list))
        check("authenticated bucket empty",   len(authenticated) == 0,
              f"expected 0, got {len(authenticated)}")
        check("private bucket empty",         len(private) == 0,
              f"expected 0, got {len(private)}")
        check("user field absent",            user is None,
              "user should be omitted when unauthenticated")
        check("disabled-app not in public",
              not any(s.get("name") == "disabled-app" for s in public))

        print(f"\n  Public services ({len(public)}):")
        for s in public:
            print(f"    • {s.get('displayName')} [{s.get('category')}] (priority={s.get('priority')}) url={s.get('url')}")
        return body
    except Exception as e:
        check("Request succeeded", False, str(e))
        return {}


def run_authenticated_services(webapi_url: str, token: str, realm_admin_password: str) -> dict:
    section("GET /api/v1/services  (authenticated as admin)")
    try:
        r = call_api(webapi_url, "/api/v1/services", token)
        check("HTTP 200", r.status_code == 200, f"got {r.status_code}")
        body = r.json()
        services = body.get("services", {})
        public        = services.get("public", [])
        authenticated = services.get("authenticated", [])
        private       = services.get("private", [])
        user          = body.get("user", {})
        categories    = body.get("categories", [])

        check("public bucket present",            len(public) > 0)
        check("authenticated bucket present",     len(authenticated) > 0,
              f"got {len(authenticated)}")
        check("private bucket present (admin)",   len(private) > 0,
              f"got {len(private)} (admin group membership required)")
        check("user.authenticated=true",          user.get("authenticated") is True)
        check("user.username=admin",              user.get("username") == "admin",
              f"got '{user.get('username')}'")
        check("categories is list",               isinstance(categories, list))
        check("categories non-empty",             len(categories) > 0, str(categories))
        check("categories sorted",
              categories == sorted(categories), str(categories))

        print(f"\n  Public ({len(public)}):        ", [s['displayName'] for s in public])
        print(f"  Authenticated ({len(authenticated)}): ", [s['displayName'] for s in authenticated])
        print(f"  Private ({len(private)}):       ", [s['displayName'] for s in private])
        print(f"  Categories: {categories}")
        print(f"  User: {dump(user)}")
        return body
    except Exception as e:
        check("Request succeeded", False, str(e))
        return {}


def run_single_service(webapi_url: str, token: str, namespace: str, name: str) -> None:
    section(f"GET /api/v1/services/{namespace}/{name}  (single lookup)")
    try:
        r = call_api(webapi_url, f"/api/v1/services/{namespace}/{name}", token)
        check("HTTP 200", r.status_code == 200, f"got {r.status_code}")
        if r.status_code == 200:
            s = r.json()
            check("name matches",    s.get("name") == name)
            check("has url field",   bool(s.get("url")))
            check("has category",    bool(s.get("category")))
            check("has displayName", bool(s.get("displayName")))
            print(f"\n  {dump(s)}")
    except Exception as e:
        check("Request succeeded", False, str(e))


def run_categories(webapi_url: str) -> None:
    section("GET /api/v1/categories")
    try:
        r = call_api(webapi_url, "/api/v1/categories")
        check("HTTP 200", r.status_code == 200, f"got {r.status_code}")
        body = r.json()
        cats = body.get("categories", [])
        check("categories key present", "categories" in body)
        check("categories is list",     isinstance(cats, list))
        check("categories sorted",      cats == sorted(cats), str(cats))
        print(f"\n  {cats}")
    except Exception as e:
        check("Request succeeded", False, str(e))


def run_pins(webapi_url: str, token: str, uid: Optional[str]) -> None:
    section("Pins  (GET / PUT / DELETE)")
    if not uid:
        print(f"  {SKIP_MARK}  Skipping — no service UID available (no services in cache?)")
        return
    try:
        # PUT pin
        url = webapi_url.rstrip("/") + f"/api/v1/pins/{uid}"
        r = requests.put(url, headers={"Authorization": f"Bearer {token}"}, timeout=10)
        check(f"PUT /api/v1/pins/{uid} → 2xx", r.ok, f"got {r.status_code}")

        # GET pins
        r2 = call_api(webapi_url, "/api/v1/pins", token)
        check("GET /api/v1/pins → 200", r2.status_code == 200)
        pins_body = r2.json()
        check("uid in stored uids", uid in pins_body.get("uids", []))
        check("service in pins list",
              any(p.get("uid") == uid for p in pins_body.get("pins", [])))

        # DELETE pin
        r3 = requests.delete(url, headers={"Authorization": f"Bearer {token}"}, timeout=10)
        check(f"DELETE /api/v1/pins/{uid} → 2xx", r3.ok, f"got {r3.status_code}")
    except Exception as e:
        check("Pins request succeeded", False, str(e))


# ── CLI ────────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(description="Nebari WebAPI integration tests (local port-forward)")
    parser.add_argument("--keycloak-url", default=KEYCLOAK_URL,
                        help=f"Keycloak base URL with /auth (default: {KEYCLOAK_URL})")
    parser.add_argument("--webapi-url", default=WEBAPI_URL,
                        help=f"WebAPI base URL (default: {WEBAPI_URL})")
    parser.add_argument("--realm", default=DEFAULT_REALM,
                        help=f"Keycloak realm (default: {DEFAULT_REALM})")
    parser.add_argument("--path", default=None,
                        help="Run a single ad-hoc GET against this path (skips test suite)")
    parser.add_argument("-u", "--username", default="admin")
    parser.add_argument("-p", "--password", help="Keycloak password (prompted if omitted)")
    parser.add_argument("--client-id", default="webapi")
    parser.add_argument("--client-secret", default=None)
    args = parser.parse_args()

    # ── Ad-hoc single path mode ────────────────────────────────────────────────
    if args.path:
        password = args.password or getpass.getpass("Password: ")
        token = get_token_password(args.keycloak_url, args.realm,
                                   args.username, password,
                                   client_id=args.client_id,
                                   client_secret=args.client_secret)
        r = call_api(args.webapi_url, args.path, token)
        print(f"HTTP {r.status_code}")
        try:
            print(json.dumps(r.json(), indent=2))
        except ValueError:
            print(r.text)
        sys.exit(0 if r.ok else 1)

    # ── Full test suite ────────────────────────────────────────────────────────
    print("\n╔══════════════════════════════════════════════════════════════╗")
    print("║          Nebari WebAPI — Local Integration Tests             ║")
    print("╚══════════════════════════════════════════════════════════════╝")
    print(f"  WebAPI:    {args.webapi_url}")
    print(f"  Keycloak:  {args.keycloak_url}  (realm: {args.realm})")

    # 1. Health
    if not run_health_check(args.webapi_url):
        sys.exit("WebAPI health check failed — is the port-forward running?")

    # 2. Unauthenticated services
    anon_body = run_unauthenticated_services(args.webapi_url)

    # 3. Obtain token
    section("Keycloak authentication")
    password = args.password or getpass.getpass("Password (default realm admin): ")
    try:
        token = get_token_password(
            args.keycloak_url, args.realm, args.username, password,
            client_id=args.client_id, client_secret=args.client_secret,
        )
        check("Token obtained", True, f"user={args.username}")
    except requests.HTTPError as exc:
        check("Token obtained", False, f"{exc.response.status_code} {exc.response.text[:120]}")
        sys.exit(1)

    # 4. Authenticated services
    auth_body = run_authenticated_services(args.webapi_url, token, password)

    # 5. Categories
    run_categories(args.webapi_url)

    # 6. Single service lookup
    all_services = (
        auth_body.get("services", {}).get("public", []) +
        auth_body.get("services", {}).get("authenticated", [])
    )
    if all_services:
        first = all_services[0]
        run_single_service(args.webapi_url, token,
                           first.get("namespace", "nebari-system"),
                           first.get("name", ""))

    # 7. Pins
    uid = all_services[0].get("uid") if all_services else None
    run_pins(args.webapi_url, token, uid)

    print("\n" + "═" * 62)
    print("  Tests complete.")
    print("═" * 62 + "\n")


if __name__ == "__main__":
    main()
