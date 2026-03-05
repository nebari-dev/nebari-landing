<!--
  THIS FILE IS AUTO-GENERATED — do not edit by hand.
  Source:    tools/apidoc/main.go
  Regenerate: go generate ./internal/api/...
  Generated: 2026-03-05T17:25:33Z
-->

# Webapi — HTTP API Reference

Base path: `/api/v1/`

Endpoints that require authentication expect a Keycloak-issued JWT in the
`Authorization: Bearer <token>` header. When the server runs with
`--enable-auth=false` all routes are open.

---

## Services

### `GET` `/api/v1/services`

List services
**Auth:** None / optional — passes through when `--enable-auth=false`

**Responses**

- `200` — array of service objects
```json
{
  "services": [
    {
      "id": "<uid>",
      "name": "JupyterHub",
      "status": "healthy",
      "description": "Interactive notebooks",
      "category": ["compute"],
      "pinned": false,
      "image": "<icon-url>",
      "url": "https://..."
    }
  ]
}
```

> **Notes**
> - Services whose `spec.landingPage.visibility` is `private` are filtered to members of the required Keycloak groups.
> - The `pinned` field reflects the calling user's pin state; always `false` for unauthenticated callers.


### `GET` `/api/v1/services/{id}`

Get a service by UID
**Auth:** None / optional — passes through when `--enable-auth=false`

**Path parameters**

| Name | Description |
|---|---|
| `id` | Service UID (`NebariApp.metadata.uid`) |

**Responses**

- `200` — service object (same shape as list item)
- `404` — not found or not visible to the caller

### `POST` `/api/v1/services/{id}/request_access`

Submit an access request for a service
**Auth:** Required — `Authorization: Bearer <token>`

**Path parameters**

| Name | Description |
|---|---|
| `id` | Service UID |

**Request body** — All fields optional.

```json
{ "message": "I need access for research purposes." }
```

**Responses**

- `202` — request created
- `409` — a pending request from this user for this service already exists
- `501` — access-request store not configured

---

## Categories

### `GET` `/api/v1/categories`

List categories derived from visible services
**Auth:** None / optional — passes through when `--enable-auth=false`

**Responses**

- `200` — distinct category strings
```json
{ "categories": ["compute", "data", "monitoring"] }
```

---

## Notifications

### `GET` `/api/v1/notifications`

List platform-wide notifications with per-caller read state
**Auth:** Required — `Authorization: Bearer <token>`

**Responses**

- `200` — array of notification objects
```json
{
  "notifications": [
    {
      "id": "<uuid>",
      "image": "<icon-url>",
      "title": "Scheduled maintenance",
      "message": "The cluster will be unavailable Saturday 02:00–04:00 UTC.",
      "read": false,
      "createdAt": "2026-03-05T10:00:00Z"
    }
  ]
}
```

### `PUT` `/api/v1/notifications/{id}/read`

Mark a notification as read for the calling user
**Auth:** Required — `Authorization: Bearer <token>`

**Path parameters**

| Name | Description |
|---|---|
| `id` | Notification UUID |

**Responses**

- `204` — marked read
- `404` — notification not found

---

## Pins

### `GET` `/api/v1/pins`

List service UIDs pinned by the calling user
**Auth:** Required — `Authorization: Bearer <token>`

**Responses**

- `200` — array of pinned UIDs
```json
{ "pins": ["<uid-a>", "<uid-b>"] }
```

### `PUT` `/api/v1/pins/{uid}`

Pin a service
**Auth:** Required — `Authorization: Bearer <token>`

**Path parameters**

| Name | Description |
|---|---|
| `uid` | Service UID to pin |

**Responses**

- `204` — pinned

### `DELETE` `/api/v1/pins/{uid}`

Unpin a service
**Auth:** Required — `Authorization: Bearer <token>`

**Path parameters**

| Name | Description |
|---|---|
| `uid` | Service UID to unpin |

**Responses**

- `204` — unpinned

---

## Caller Identity

### `GET` `/api/v1/caller-identity`

Return the identity decoded from the caller's JWT
**Auth:** None / optional — passes through when `--enable-auth=false`

**Responses**

- `200` — `authenticated` is `false` when no valid token is present
```json
{
  "authenticated": true,
  "username": "alice",
  "email": "alice@example.com",
  "name": "Alice Smith",
  "groups": ["admin", "developers"]
}
```

---

## WebSocket

### `GET` `/api/v1/ws`

Real-time service events (WebSocket upgrade)
**Auth:** Required — `Authorization: Bearer <token>` + JWT in `Authorization` header **or** `?token=<jwt>` query param

Server pushes JSON frames on every service change event.
```json
{
  "type": "added" | "modified" | "deleted",
  "service": { /* ServiceView shape */ }
}
```

> **Notes**
> - Events are published via Redis Pub/Sub (`nebari:events`) so all webapi replicas fan out every event to their local WebSocket clients.


---

## Health

### `GET` `/api/v1/health`

Liveness / readiness probe — no auth required
**Auth:** None / optional — passes through when `--enable-auth=false`

**Responses**

- `200` — always `{"status":"ok"}` while the process is alive
```json
{ "status": "ok" }
```

---

## Admin

### `GET` `/api/v1/admin/access-requests`

List all access requests
**Auth:** Required — `Authorization: Bearer <token>` + admin group membership

**Query parameters**

| Name | Description |
|---|---|
| `status` | `pending` | `approved` | `denied` — omit for all |

**Responses**

- `200` — array of access-request objects
- `403` — caller is not a member of the admin group

### `PUT` `/api/v1/admin/access-requests/{id}`

Approve or deny an access request

When approved and a Keycloak client is configured, the requesting user is added to the service's required Keycloak group automatically.
**Auth:** Required — `Authorization: Bearer <token>` + admin group membership

**Path parameters**

| Name | Description |
|---|---|
| `id` | Access-request UUID |

**Request body**

```json
{ "status": "approved" | "denied" }
```

**Responses**

- `200` — updated access-request object
- `404` — request not found
- `403` — caller is not a member of the admin group

### `POST` `/api/v1/admin/notifications`

Create a platform-wide notification
**Auth:** Required — `Authorization: Bearer <token>` + admin group membership

**Request body**

```json
{
  "image": "<icon-url>",
  "title": "Maintenance window",
  "message": "Details here."
}
```

**Responses**

- `201` — notification created
- `403` — caller is not a member of the admin group
