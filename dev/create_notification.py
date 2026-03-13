#!/usr/bin/env python3
"""
Minimal example: create a Nebari platform notification via the webapi.

Usage:
    python3 dev/create_notification.py
    python3 dev/create_notification.py --title "Maintenance" --message "Down at 22:00 UTC"

Requirements:
    pip install requests
"""

import argparse
import json
import sys

import requests

# ── defaults (match dev/Makefile) ──────────────────────────────────────────────
KC_URL = "http://localhost:8180"
KC_REALM = "nebari"
KC_CLIENT = "webapi"  # must be the client with the groups mapper
USERNAME = "admin"
PASSWORD = "nebari-realm-admin"
WEBAPI_URL = "http://localhost:8090"
# ───────────────────────────────────────────────────────────────────────────────


def get_token(kc_url: str, realm: str, client: str, username: str, password: str) -> str:
    url = f"{kc_url}/realms/{realm}/protocol/openid-connect/token"
    resp = requests.post(
        url,
        data={
            "grant_type": "password",
            "client_id": client,
            "username": username,
            "password": password,
        },
        timeout=10,
    )
    resp.raise_for_status()
    return resp.json()["access_token"]


def create_notification(webapi_url: str, token: str, title: str, message: str, image: str = "") -> dict:
    url = f"{webapi_url}/api/v1/admin/notifications"
    body = {"title": title, "message": message}
    if image:
        body["image"] = image
    resp = requests.post(
        url,
        json=body,
        headers={
            "Authorization": f"Bearer {token}",
        },
        timeout=10,
    )
    resp.raise_for_status()
    return resp.json()


def main():
    parser = argparse.ArgumentParser(description="Create a Nebari platform notification")
    parser.add_argument("--kc-url", default=KC_URL, help="Keycloak base URL")
    parser.add_argument("--realm", default=KC_REALM, help="Keycloak realm name")
    parser.add_argument("--client", default=KC_CLIENT, help="Keycloak client_id (needs groups mapper)")
    parser.add_argument("--username", default=USERNAME, help="Realm user")
    parser.add_argument("--password", default=PASSWORD, help="Realm user password")
    parser.add_argument("--webapi", default=WEBAPI_URL, help="webapi base URL")
    parser.add_argument("--title", default="Test notification", help="Notification title")
    parser.add_argument("--message", default="This is a test notification.", help="Notification body")
    parser.add_argument("--image", default="", help="Optional image URL")
    args = parser.parse_args()

    print("→ Fetching token from Keycloak...")
    try:
        token = get_token(args.kc_url, args.realm, args.client, args.username, args.password)
    except requests.HTTPError as e:
        print(f"  ERROR: {e.response.status_code} {e.response.text}", file=sys.stderr)
        sys.exit(1)
    print(f"  OK (token length: {len(token)})")

    print(f"→ Creating notification '{args.title}'...")
    try:
        notif = create_notification(args.webapi, token, args.title, args.message, args.image)
    except requests.HTTPError as e:
        print(f"  ERROR: {e.response.status_code} {e.response.text}", file=sys.stderr)
        sys.exit(1)

    print("  Created:")
    print(json.dumps(notif, indent=2))


if __name__ == "__main__":
    main()
