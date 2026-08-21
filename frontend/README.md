# RedisShake Web Console

Vue 3 control plane UI for creating, prechecking, running, and monitoring RedisShake tasks.

```shell
npm install
npm run dev
```

The Vite development server proxies `/api` to `http://127.0.0.1:8080`. Build and test with:

```shell
npm run typecheck
npm run test
npm run build
```

The Playwright E2E expects `deploy/compose.dev.yaml` to be running and verifies a real UI-created RedisShake scan:

```shell
docker compose -f ../deploy/compose.dev.yaml up -d --build --wait
npx playwright install chromium
npm run test:e2e
```
