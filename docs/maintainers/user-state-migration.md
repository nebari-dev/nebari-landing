# User State Identity Migration

Nebari Landing versions before the immutable-subject migration stored durable
user state under `preferred_username`. After upgrading, pins, notification read
state, access-request ownership, and access-request dedup keys are keyed by the
stable `(issuer, subject)` identity.

Use `cmd/migrate-user-state` once per environment to rewrite existing Redis
state. The command is dry-run only unless `--apply` is passed, and it refuses to
apply when collisions are detected.

Run the apply step during a maintenance window, or while the webapi is scaled
down, so new username-keyed state is not written while Redis is being rewritten.

## Mapping File

Create a JSON mapping from each legacy username key to the Keycloak issuer and
subject that owns that state:

```json
[
  {
    "username": "alice",
    "issuer": "https://keycloak.example.com/realms/nebari",
    "subject": "0f8fad5b-d9cb-469f-a165-70867728950e",
    "displayUsername": "alice.renamed"
  }
]
```

`displayUsername` is optional and defaults to `username`. It is stored only as
display metadata on migrated access requests.

## Run

Dry-run first:

```bash
go run ./cmd/migrate-user-state \
  --mapping-file user-state-mapping.json \
  --redis-addr localhost:6379
```

Review the JSON output. Resolve every `collisions` entry before applying:

```bash
go run ./cmd/migrate-user-state \
  --mapping-file user-state-mapping.json \
  --redis-addr localhost:6379 \
  --apply
```

The command rewrites:

- `nebari:pins:{username}` to the stable user key.
- `nebari:reads:{username}` to the stable user key.
- `nebari:ar:{id}` user and resolver identity fields.
- `nebari:ar:user:{username}` per-user request indexes.
- pending `nebari:ar:dedup:{username}:{serviceUID}` keys.

It reports and refuses to apply collisions such as multiple usernames mapping to
the same stable identity, pre-existing target pins/read keys, mismatched stored
subjects, or conflicting pending dedup keys.
