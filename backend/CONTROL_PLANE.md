# RedisShake Control Plane

The control plane stores metadata in SQLite, manages one RedisShake worker process per Run, and serves the embedded browser-native Web console and API from the same HTTP port.

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
- one isolated RedisShake process group per Run, with worker version probing and duplicate-run protection;
- loopback-only RedisShake status endpoints, persisted status snapshots, heartbeats, logs, and SSE status events;
- scan completion, unexpected-exit, graceful-stop, force-stop, and restart-UNKNOWN state handling;
- startup validation that prevents opening an existing credential store with a missing or incorrect master key.
- HTML + CSS + ES Modules JavaScript console assets embedded in `redis-shake-server` through Go `embed.FS`.

## Data layout

By default, running from `backend/` creates:

```text
backend/data/
├── control-plane.db
└── runtime/tasks/<task-id>/runs/<run-id>/
    ├── shake.toml
    ├── process.json
    ├── stdout.log
    ├── certs/
    └── data/
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
| `REDISSHAKE_WORKER_PATH` | `./bin/redis-shake` | RedisShake worker binary; `--version` must return the RedisShake version banner |
| `REDISSHAKE_RUN_START_TIMEOUT` | `15s` | Maximum wait for the worker status endpoint to become readable |
| `REDISSHAKE_RUN_STOP_TIMEOUT` | `30s` | Graceful shutdown wait before the control plane force-terminates workers during its own shutdown |
| `REDISSHAKE_WEB_DIR` | empty | Optional filesystem SPA override; the compiled binary uses embedded native Web assets by default |
| `REDISSHAKE_MAX_CONCURRENT_RUNS` | `4` | Global cap for STARTING, RUNNING, STOPPING, and UNKNOWN Runs |
| `REDISSHAKE_LOG_RETENTION_DAYS` | `7` | Delete terminal Run artifact directories after this many days; `0` disables artifact cleanup |

Generate the master key with a cryptographically secure tool and keep it outside the repository. For example, `openssl rand -base64 32` prints a suitable value. Back up the key separately from the SQLite database; encrypted connection passwords cannot be recovered without it.

## Run locally

```shell
cd backend
sh build_web.sh
REDISSHAKE_MASTER_KEY="$(openssl rand -base64 32)" ./bin/redis-shake-server
```

`build_web.sh` validates and stages the native static modules in the Go resource package, then compiles both RedisShake and the control-plane server. The resulting server binary is self-contained and serves the UI and API from port `8080`.

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

## Run API

```text
POST /api/v1/tasks/{id}/runs
GET  /api/v1/tasks/{id}/runs
GET  /api/v1/runs/{id}
POST /api/v1/runs/{id}/stop
POST /api/v1/runs/{id}/force-stop
GET  /api/v1/runs/{id}/logs
GET  /api/v1/runs/{id}/events
```

Starting a READY task freezes a credential-free Task snapshot, allocates a loopback status port, writes the generated runtime files with `0600` permissions inside a `0700` Run directory, and starts one RedisShake process group. `RUNNING` is returned only after the status endpoint is readable; a short scan may finish as `SUCCEEDED` before that handshake completes.

The status poller persists the last valid RedisShake JSON separately from process state. Three consecutive status failures mark status unhealthy without claiming that the process exited. Graceful stop sends `SIGTERM` to the process group; force stop is a separate endpoint. Logs are redacted before disk writes and again before API responses.

At control-plane startup, persisted `STARTING`, `RUNNING`, or `STOPPING` rows for which in-memory ownership cannot be proven become `UNKNOWN`. Such a Run blocks duplicate task startup but cannot be signaled through the API, preventing PID reuse from killing an unrelated process.

## Single-image deployment

`backend/Dockerfile.web` validates the native Web application, copies it into the Go `embed.FS` resource package before compiling, and places only the RedisShake worker and self-contained control-plane binary in the non-root Alpine runtime. No `/app/web` directory or frontend runtime is required. State is stored under `/var/lib/redis-shake-ui`, and `/readyz` is used for health checks.

Use `compose.yaml` for a persistent deployment or `deploy/compose.dev.yaml` for the browser-to-Redis demo matrix. The original `backend/Dockerfile` and CLI release archives remain unchanged.
