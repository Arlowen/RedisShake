# RedisShake Web Console

React 19 + TypeScript control-plane UI for creating, prechecking, running, and monitoring RedisShake tasks. Webpack provides the development server and production build; Vite is not used.

## Development

```shell
npm ci
npm run dev
```

The Webpack development server listens on `127.0.0.1:5173` and proxies `/api`, `/healthz`, and `/readyz` to `http://127.0.0.1:8080`.

## Embedded single-port build

From the repository root:

```shell
cd backend
sh build_web.sh
REDISSHAKE_MASTER_KEY="$(openssl rand -base64 32)" ./bin/redis-shake-server
```

`build_web.sh` compiles React, copies ignored build output into the Go resource package, and embeds it in `redis-shake-server`. The Web console and API are then both served from `http://127.0.0.1:8080`; no frontend files are required beside the binary.

## Verification

```shell
npm run typecheck
npm run test
npm run build
```

The Playwright E2E expects `deploy/compose.dev.yaml` to be running and verifies a real UI-created RedisShake scan through the backend port:

```shell
docker compose -f ../deploy/compose.dev.yaml up -d --build --wait
npx playwright install chromium
npm run test:e2e
```
