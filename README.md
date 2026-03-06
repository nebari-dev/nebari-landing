# nebari-landing

Landing page service for Nebari. Consists of two pods deployed via the `charts/nebari-landing` Helm chart:

- **webapi** — Go REST API + WebSocket hub backed by Redis
- **frontend** — Vite/React SPA served by nginx, fronted by an OAuth2 Proxy sidecar

---

## API Reference

See [docs/api.md](docs/api.md) for the full HTTP API reference.

To regenerate after editing route definitions:

```sh
go generate ./internal/api/...
```
