# Web Console Deployment

The Web console manages Redis connections, task configuration, prechecks, RedisShake worker processes, status, and logs. RedisShake remains the data-movement engine; the control plane does not store copies of Redis keys and values.

The console uses React, TypeScript, and Webpack. Production assets are compiled into `redis-shake-server` with Go `embed.FS`, so the UI and API share one backend port without a Node.js runtime or external static directory.

## Local single-port build

```shell
cd backend
sh build_web.sh
REDISSHAKE_MASTER_KEY="$(openssl rand -base64 32)" ./bin/redis-shake-server
```

Open `http://127.0.0.1:8080`. Generated frontend and embedded-resource staging files are ignored by Git. `REDISSHAKE_WEB_DIR` remains an optional filesystem override.

## Demo environment

Start the single-image console with two disposable Redis instances:

```shell
docker compose -f deploy/compose.dev.yaml up -d --build --wait
```

Open `http://127.0.0.1:8080` and use `source-redis:6379` and `target-redis:6379` as connection addresses. The deterministic demo master key must never protect real credentials.

## Production

Generate a unique key, put it in `.env`, and keep a separate backup:

```shell
cp .env.example .env
openssl rand -base64 32
docker compose up -d --build --wait
```

The service binds to `127.0.0.1:8080` by default. Use an authenticated TLS reverse proxy for remote access. Persistent metadata and Run artifacts live under `/var/lib/redis-shake-ui` in the named volume.

The default global limit is four active/UNKNOWN Runs (`REDISSHAKE_MAX_CONCURRENT_RUNS`). Terminal Run files are retained for seven days; set `REDISSHAKE_LOG_RETENTION_DAYS=0` to disable artifact cleanup. SQLite history is retained.

## Backup, upgrade, and rollback

Stop the console so workers exit gracefully, then copy a consistent data snapshot:

```shell
docker compose stop redis-shake-ui
mkdir -p backup/redis-shake-ui
docker compose cp redis-shake-ui:/var/lib/redis-shake-ui/. backup/redis-shake-ui/
docker compose start redis-shake-ui
```

Back up `REDISSHAKE_MASTER_KEY` separately. SQLite migrations are forward-only. A safe rollback restores both the previous image and its matching data backup; running an older image against a newer schema is not supported.

After an upgrade, verify `/readyz`, the system version, saved connections, task history, and one real Redis-to-Redis test. Do not treat HTTP 200 or a RUNNING badge as proof that target data arrived.

## Troubleshooting

- `worker_unavailable`: verify `REDISSHAKE_WORKER_PATH` and `redis-shake --version` as the container user.
- `UNKNOWN`: ownership of a pre-restart PID could not be proven. Duplicate startup is blocked and the API will not signal that PID.
- Credentials cannot be saved: configure a Base64-encoded 32-byte `REDISSHAKE_MASTER_KEY`.
- The page loads but a task fails: check READY state, active/UNKNOWN Runs, target write precheck, Run logs, persisted status, and target Redis readback.
