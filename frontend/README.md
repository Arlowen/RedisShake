# RedisShake Web Console

Browser-native HTML, CSS, and ES Modules JavaScript control-plane UI for creating, prechecking, running, and monitoring RedisShake tasks. It has no framework or runtime UI dependency.

The UI hierarchy, tokens, responsive rules, and acceptance criteria are defined in [`../design.md`](../design.md).

## Development

```shell
npm ci
npm run dev
```

The small Node development server listens on `127.0.0.1:5173`, serves the native modules directly, and proxies `/api`, `/healthz`, and `/readyz` to `http://127.0.0.1:8080`.

## Embedded single-port build

From the repository root:

```shell
cd backend
sh build_web.sh
REDISSHAKE_MASTER_KEY="$(openssl rand -base64 32)" ./bin/redis-shake-server
```

`build_web.sh` validates and copies the static HTML/CSS/JavaScript modules into the Go resource package, then embeds them in `redis-shake-server`. The Web console and API are both served from `http://127.0.0.1:8080`; no frontend files are required beside the binary.

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
