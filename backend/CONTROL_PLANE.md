# RedisShake Control Plane

The control plane is the backend for the RedisShake Web console. It stores metadata in SQLite and will manage one RedisShake worker process per task. The worker data path and API are being implemented incrementally according to the project design document.

## Current foundation

The current implementation provides:

- versioned, idempotent SQLite migrations;
- persistent Connection, Task, and Run models;
- AES-256-GCM encryption helpers for connection credentials;
- log and error redaction helpers;
- Task and Run state-transition validation;
- `/healthz`, `/readyz`, and `/api/v1/system/info` endpoints;
- encrypted Redis connection CRUD for standalone, Sentinel, and cluster topologies;
- saved and unsaved connection tests with ACL, TLS, topology, and server metadata checks;
- target write-permission tests using a random temporary Key with a 60-second TTL and best-effort deletion;
- synchronization task drafts with optimistic config revisions and archive semantics;
- task-level source/target checks, filter validation, danger warnings, and READY transitions;
- RedisShake TOML generation that is parsed by the same backend configuration package before a task can become READY;
- startup validation that prevents opening an existing credential store with a missing or incorrect master key.

Worker process management and the Web UI are the next implementation phases.

## Data layout

By default, running from `backend/` creates:

```text
backend/data/
├── control-plane.db
└── runtime/
```

Production containers should set `REDISSHAKE_DATA_DIR=/var/lib/redis-shake-ui` and mount that directory as a persistent volume. Runtime files will be placed below `runtime/tasks/<task-id>/runs/<run-id>/`.

## Configuration

| Environment variable | Default | Purpose |
| --- | --- | --- |
| `REDISSHAKE_LISTEN_ADDR` | `127.0.0.1:8080` | HTTP listen address; loopback is the safe default |
| `REDISSHAKE_DATA_DIR` | `./data` | Base directory for SQLite and runtime files |
| `REDISSHAKE_DB_PATH` | `<data-dir>/control-plane.db` | Optional SQLite path override |
| `REDISSHAKE_RUNTIME_DIR` | `<data-dir>/runtime` | Optional worker runtime path override |
| `REDISSHAKE_MASTER_KEY` | empty | Base64-encoded 32-byte credential-encryption key |

Generate the master key with a cryptographically secure tool and keep it outside the repository. For example, `openssl rand -base64 32` prints a suitable value. Back up the key separately from the SQLite database; encrypted connection passwords cannot be recovered without it.

## Run locally

```shell
cd backend
go run ./cmd/redis-shake-server
```

Then query:

```shell
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
curl http://127.0.0.1:8080/api/v1/system/info
```

The server can start without a master key while no encrypted credentials exist.

Connection APIs already reject password or TLS PEM persistence when the master key is absent. Responses expose only `*_configured` flags and never return passwords, certificate PEM contents, private keys, or ciphertext.

## Connection API

The current endpoints are:

```text
GET    /api/v1/connections
POST   /api/v1/connections
GET    /api/v1/connections/{id}
PATCH  /api/v1/connections/{id}
DELETE /api/v1/connections/{id}
POST   /api/v1/connections/test
POST   /api/v1/connections/{id}/test
```

Use `purpose=source` for a read-only connectivity/topology check. Use `purpose=target` to additionally prove write permission. The target check writes a Key named `__redisshake_ui_precheck:<random-id>` with a 60-second TTL and immediately deletes it. If cleanup fails, the API returns a warning and the TTL remains the fallback cleanup.

See `api/openapi.yaml` for request and response schemas.

## Task API

```text
GET    /api/v1/tasks
POST   /api/v1/tasks
GET    /api/v1/tasks/{id}
PATCH  /api/v1/tasks/{id}
DELETE /api/v1/tasks/{id}
POST   /api/v1/tasks/{id}/precheck
```

Tasks begin in `DRAFT`. The creation wizard can persist the name and mode before source and target connections are selected. Every update requires `expected_revision`, increments `config_revision`, returns the task to `DRAFT`, and invalidates its previous precheck.

Precheck performs source and target connection tests, topology and filter checks, target write verification, and RedisShake TOML generation. It stores only a SHA-256 digest computed from credential-redacted configuration plus structured check results; generated runtime TOML may contain credentials and is never persisted as a precheck result. A task enters `READY` only when there are no failures and all danger warnings, such as `empty_db_before_sync`, are explicitly acknowledged.
