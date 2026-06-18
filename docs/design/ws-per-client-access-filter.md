# WebSocket per-client access filter (#95)

## Problem

`GET /api/v1/services` filters response items per-caller via
`h.canAccessService(service, authenticated, claims)`. The WebSocket
`/api/v1/ws` does not. The hub fan-outs every `ServiceEvent` to every
connected client, so a user who is not in a private service's
`requiredGroups` still receives `added` / `modified` / `deleted` frames
for that service and the SPA renders the card. Symptom: Grafana,
Superset (and any other private service) appear in the launchpad a
moment after initial load even though the REST snapshot excluded them.

## Goal

WS broadcasts honor the same `canAccessService` policy as REST,
on a per-connected-client basis.

Scope is `ServiceEvent` only. `NotificationEvent` filtering is deferred
to a follow-up (see _Out of scope_).

## Current state

### Auth surfaces in front of the API

`handleListServices` and `handleGetService` both call

```go
claims, authenticated, err := h.extractAndValidateJWT(r)
```

then apply

```go
if !h.canAccessService(service, authenticated, claims) { … }
```

so the policy is already centralized on `*Handler`. We do not need
to move it.

### WS auth has two paths, neither propagates claims to the hub

`handleWS` (`internal/api/handlers.go:244-272`):

1. **Bearer header.** Calls `h.requireAuth(w, r)` but discards claims
   with `_, ok := …`. Claims are validated and then thrown away.
2. **Ticket query param.** Calls `h.wsTicketStore.Redeem(ctx, ticket)`,
   which returns only an error. The ticket store deliberately stripped
   identity (`internal/wsticket/store.go:22-25`):

   > "Stored value is empty — the existence of the key is the only
   > signal callers care about. We previously stored the issuing user's
   > username but no caller read it back, so it has been removed."

`handleWSTicket` (`internal/api/handlers.go:294-…`) does have the
issuer's claims (it ran `requireAuth` on line 303) but uses them only
for logging. They never reach the ticket value.

### Hub has no per-client state

`internal/websocket/hub.go:78`:

```go
clients map[*websocket.Conn]struct{}
```

`broadcast([]byte)` writes the same bytes to every connection. There
is nowhere to hang claims and nothing to evaluate.

## Approach

Two corrections, both small:

1. **Stop discarding claims in the WS auth chain.** The Bearer path
   already extracts them; the ticket path extracts them at issuance
   and can carry them through to redemption. Both call sites pass the
   resulting claims into a new `Hub.ServeWS` signature.
2. **Give the hub per-client state and a policy hook.** `Client`
   struct holds `conn`, `claims`, `authenticated`. The hub takes a
   filter function at construction (`func(svc, authed, claims) bool`),
   and `app.go` wires `handler.CanAccessService` (export it) in.

Why inject the predicate rather than move it into a shared package:
the handler already owns the policy and uses it for the REST paths.
Moving it would change two call sites for no behavioral benefit;
injection keeps the handler the single authority and avoids a new
package. (See _Alternatives considered_.)

## File-by-file plan

### `internal/wsticket/store.go`

Re-add identity. Replace the empty stored value with a JSON-encoded
struct that carries the subset the hub needs:

```go
type TicketClaims struct {
    Subject           string   `json:"sub"`
    PreferredUsername string   `json:"preferred_username,omitempty"`
    Groups            []string `json:"groups,omitempty"`
}
```

- `Issue(ctx context.Context, tc TicketClaims) (string, error)` —
  marshals `tc` to JSON, stores it under `wsticket:<id>` with the
  existing 30 s TTL.
- `Redeem(ctx context.Context, ticket string) (TicketClaims, error)` —
  `GETDEL`, unmarshal, return.

Update the package comment to explain why identity is stored now.

### `internal/api/handlers.go`

Three small edits:

1. Export the policy:

   ```go
   func (h *Handler) CanAccessService(svc *cache.ServiceInfo,
       authenticated bool, claims *auth.Claims) bool
   ```

   (Lowercase `canAccessService` callers in this file stay
   internal — rename them with `gopls` or `gorename`.)

2. `handleWSTicket`: build a `TicketClaims` from the `claims` that
   `requireAuth` already returned, pass it to `Issue`.

3. `handleWS`:
   - Bearer path: keep the claims (`claims, ok := h.requireAuth(...)`).
   - Ticket path: `tc, err := h.wsTicketStore.Redeem(ctx, ticket)`,
     translate `TicketClaims` to `*auth.Claims` (or directly to whatever
     shape the hub wants — see _Open questions_).
   - Call `h.hub.ServeWS(w, r, claims, authenticated)`.

### `internal/websocket/hub.go`

- New unexported `client` struct:

  ```go
  type client struct {
      conn          *websocket.Conn
      claims        *auth.Claims
      authenticated bool
  }
  ```

- `Hub.clients` becomes `map[*client]struct{}`.
- `Hub` gains a field `filter func(*cache.ServiceInfo, bool, *auth.Claims) bool`.
- `NewHub` takes that filter as a new argument.
- `ServeWS(w, r, claims, authenticated)` registers a `*client`.
- `broadcast` splits into two paths:
  - `broadcastService(*ServiceEvent)` — iterate clients, evaluate
    `filter` per client, send only to entitled clients.
  - `broadcastNotification(*NotificationEvent)` — fan-out as today.
- The Redis subscribe loop now unmarshals once to discriminate event
  type before fanning out (`json.RawMessage` peek on `type`), so the
  service vs notification routing decision lives in the subscriber, not
  the publisher.

`drop` and `ClientCount` adjust to the new map type but stay otherwise
unchanged.

### `internal/app/app.go`

One line: pass `handler.CanAccessService` into `NewHub`.

### Tests

- `internal/wsticket/store_test.go`: update existing tests for new
  `Issue`/`Redeem` signatures; add a roundtrip test that asserts the
  claims marshaled in come back out.
- `internal/websocket/hub_test.go`: update existing tests for the new
  `NewHub` and `ServeWS` signatures; add a focused test:
  - Two clients connect: client A has `groups=["admin"]`, client B has
    `groups=["data-science"]`.
  - A private service with `requiredGroups=["admin"]` is published.
  - Assert client A receives the frame; client B does not.
- `internal/api/ws_ticket_test.go` and `ws_auth_test.go`: update for
  the propagated-claims signature; add a regression covering "browser
  flow: POST /ws-ticket → upgrade with ?ticket=<id> → broadcast only
  reaches the entitled connection."

## Alternatives considered

### A. Move `canAccessService` into a new `internal/access` package

Original sketch in this conversation. Cleaner separation in the
abstract, but it changes two call sites (handler + hub) instead of
one and adds a package whose only contents are the two existing
methods. Rejected for being more code with no behavior gain.

### B. Filter at the source (watcher publishes per-client)

Issue #95 mentions this as Option 2. Watcher would need access to
the live client list and the filter — the hub already has both.
Putting it at the watcher inverts the fan-out responsibility.
Rejected.

### C. Embed identity in the ticket itself (signed JWT-style)

Re-implements JWT, adds key management. The Redis-backed store with
TTL already gives us single-use and expiry; storing claims in the
value is the smaller delta.

## Out of scope

- **Notification filtering.** Issue #95 explicitly defers it: "Worth
  a second look once this lands." The hub's `broadcastNotification`
  path stays fan-out-to-all in this PR. The split-by-event-type
  factoring done here makes a follow-up trivial.
- **Frontend de-dup against the REST snapshot.** The SPA likely already
  de-dups by service UID; this PR doesn't change that surface.
- **Caching the filter result per (service, claims-fingerprint).**
  Premature; the filter is a hashmap-membership check, microseconds.

## Open questions

1. **`auth.Claims` vs `TicketClaims`.** The hub's filter takes
   `*auth.Claims`. The ticket carries a subset. We can either (a) make
   the hub accept the subset directly, or (b) translate at the WS
   handler so the hub always sees `*auth.Claims`. (b) is consistent
   with the REST path; (a) avoids a translation step. Leaning (b).

2. **Filter signature.** Pass `func(*cache.ServiceInfo, bool, *auth.Claims) bool`
   into `NewHub` directly, or define an interface (`type AccessPolicy interface{ CanAccessService(...) bool }`)?
   Function suffices for one method; interface only earns its keep if
   we add more policies later. Leaning plain function.

3. **Test mode in `handleWS`.** Existing test infrastructure injects a
   `claimsExtractor` rather than going through JWT validation. Confirm
   that path still produces the same claims that get propagated to
   `ServeWS`.

## Risks

- **TTL on the ticket value increases payload size.** Negligible —
  the JSON for a typical caller is <512 bytes.
- **Client drop on policy change.** If a service's `requiredGroups`
  changes mid-session and the client no longer qualifies, the hub will
  silently stop sending that service's events to them. The client's
  REST snapshot will reflect the change on the next page load. This
  matches the behavior REST users see today and is acceptable; document
  it in the hub's package comment.
- **Behavior on the `claimsExtractor` test path** could diverge from
  prod if we hand-roll the propagation logic differently in the two
  paths. The fix is to thread one variable (`claims, authenticated`)
  through `handleWS` regardless of how it was obtained.

## Session-lifetime bound (added during implementation)

The original plan covered the per-client filter only. During implementation
it became clear that the ticket now carries a claims snapshot that can
outlive the JWT it was minted from — REST re-validates the JWT on every
request, but the WS session uses the snapshot for the connection's
lifetime, which on a long-held tab can be hours. To bound that staleness
without per-event cost, the hub installs a `time.AfterFunc` keyed on the
JWT `exp` carried through `TicketClaims.ExpiresAt`. When the timer fires,
the connection is closed; the SPA reconnects via a fresh ticket, which
re-validates a fresh JWT. Keycloak's default access-token lifetime (~5
min) becomes the effective revocation window for WS sessions.

Two guards prevent the timer from silently failing open:

1. `handleWS` rejects the upgrade (401) when `tc.ExpiresAt.IsZero() ||
   !tc.ExpiresAt.After(time.Now())`. This catches the case where a JWT
   was near-exp at issue time and is already past-exp at redeem, and the
   forward-compat case where a future token shape omits `exp`.
2. `Hub.ServeWS` only installs the timer when `ttl > 0`; combined with
   (1), an unbounded session at the hub level requires the upgrade guard
   to be bypassed, which it isn't from any code path.

These were added in response to the post-implementation review (the
`fail-open under expired exp` finding); they belong to the same security
control surface as the filter itself.

## Nil-policy regression guard

`NewHub` returns with `policy == nil` and the comment documents this as a
test-only convenience — production wiring in `cmd/main.go` always calls
`SetAccessPolicy`. To make the silent-regression case noisy: on the first
service-event broadcast with `policy == nil`, the hub logs once via
`sync.Once`. The warning is harmless in tests (which set or don't set the
policy at construction time) and immediately surfaces a future call site
that builds a `Hub` and forgets the setter.

## PR shape

- Branch: `fix/ws-per-client-access-filter`
- Target: current `main`
- Title: `fix(ws): apply per-client access filter to service events`
- Body references #95; closes it.
- Estimated diff: ~300-400 lines, 7 files (actual implementation grew to
  ~1100 lines across 10 files after the session-end bound and additional
  tests landed; see the PR for the final shape).
