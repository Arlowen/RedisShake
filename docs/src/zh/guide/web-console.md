# Web 控制台部署

Web 控制台通过页面管理 Redis 连接、任务配置、预检查、RedisShake worker、状态和日志。数据同步仍由原始 RedisShake 内核执行；控制面不会保存 Redis Key/Value 副本。

前端采用浏览器原生 HTML、CSS 和 ES Modules JavaScript。生产构建通过 Go `embed.FS` 内嵌进 `redis-shake-server`，页面和 API 使用同一个后端端口，不需要单独部署 Node.js、前端服务器或静态资源目录。

## 本地单端口构建

```shell
cd backend
sh build_web.sh
REDISSHAKE_MASTER_KEY="$(openssl rand -base64 32)" ./bin/redis-shake-server
```

打开 `http://127.0.0.1:8080`。`frontend/dist/` 和后端内嵌资源的中间产物均不会提交到 Git；`REDISSHAKE_WEB_DIR` 仅用于需要从文件系统覆盖内嵌页面的特殊部署。

## 快速体验

开发 Compose 包含单镜像 Web 控制台和两个无密码 Redis：

```shell
docker compose -f deploy/compose.dev.yaml up -d --build --wait
```

打开 `http://127.0.0.1:8080`，创建连接时使用容器网络地址：

- 源端：`source-redis:6379`
- 目标端：`target-redis:6379`

宿主机调试端口默认为 `26379` 和 `26380`，可通过 `REDISSHAKE_DEMO_SOURCE_PORT`、`REDISSHAKE_DEMO_TARGET_PORT` 修改。开发 Compose 内置的主密钥只能用于演示，不能保存真实凭据。

停止并删除演示数据：

```shell
docker compose -f deploy/compose.dev.yaml down --volumes
```

## 生产部署

1. 复制环境变量示例并生成独立主密钥：

```shell
cp .env.example .env
openssl rand -base64 32
```

2. 把生成结果填入 `.env` 的 `REDISSHAKE_MASTER_KEY`。该密钥不进入数据库，必须单独备份；丢失后无法解密已保存的连接凭据。

3. 启动单镜像控制台：

```shell
docker compose up -d --build --wait
```

镜像运行层只包含 RedisShake worker 和已经内嵌原生 Web 页面的控制面二进制，不包含 Node.js 或独立 `/app/web` 目录。

默认只发布到 `127.0.0.1:8080`。如需远程访问，请在前面部署带 TLS、身份认证和访问控制的反向代理，不要直接发布 RedisShake 内部状态端口。

持久数据卷保存：

```text
/var/lib/redis-shake-ui/
├── control-plane.db
└── runtime/tasks/<task-id>/runs/<run-id>/
```

连接、任务、运行历史存入 SQLite；每次运行的 TOML、证书、日志和临时数据位于独立 Run 目录。目录权限为 `0700`，敏感文件权限为 `0600`。

默认最多同时运行 4 个活动/UNKNOWN Run，可通过 `REDISSHAKE_MAX_CONCURRENT_RUNS` 调整。终态 Run 的文件默认保留 7 天，`REDISSHAKE_LOG_RETENTION_DAYS=0` 可关闭文件清理；SQLite 中的运行历史不会随文件清理删除。

## 备份与恢复

数据库采用只向前迁移。升级前必须同时备份数据卷和 `REDISSHAKE_MASTER_KEY`。

为获得一致快照，先让控制面优雅停止所有 worker，再复制数据：

```shell
docker compose stop redis-shake-ui
mkdir -p backup/redis-shake-ui
docker compose cp redis-shake-ui:/var/lib/redis-shake-ui/. backup/redis-shake-ui/
docker compose start redis-shake-ui
```

恢复时停止容器，将备份复制回同一路径，再使用与备份匹配的应用版本和主密钥启动。不要用旧版本应用直接打开已被新版本迁移过的 SQLite。

## 升级与回滚

1. 记录当前镜像标签和主密钥来源。
2. 按上一节创建一致备份。
3. 构建或拉取新镜像，并仅替换控制台：

```shell
docker compose up -d --build --no-deps --wait redis-shake-ui
curl --fail http://127.0.0.1:8080/readyz
```

4. 从页面检查系统版本、连接列表和历史任务，再执行一条真实测试任务。

如果健康检查、数据迁移或真实同步失败，停止新容器，恢复备份，将镜像标签切回旧版本后重新启动。仅切回旧镜像但继续使用新 schema 不属于安全回滚。

## 常见问题

### `worker_unavailable`

控制面会运行 `redis-shake --version` 校验 worker。单镜像默认使用 `/app/redis-shake`；自定义部署需设置 `REDISSHAKE_WORKER_PATH` 并确保运行用户可执行。

### Run 变成 `UNKNOWN`

控制面异常重启后，如果无法证明旧 PID 仍属于原 worker，会把活动记录标记为 `UNKNOWN`。该记录会阻止重复启动，但 API 不会向不可信 PID 发送信号。请在主机或容器中核对进程后再处理。

### 无法保存密码或 TLS PEM

确认 `REDISSHAKE_MASTER_KEY` 是 Base64 编码的 32 字节值。数据库已有加密凭据时，缺少或错误主密钥会直接阻止控制面启动。

### 页面可访问但任务不运行

依次检查任务是否为 `READY`、是否存在活动/UNKNOWN Run、目标写检查、Run 日志以及 `/api/v1/runs/<id>` 的真实状态。HTTP 200 或页面显示 RUNNING 不能替代目标 Redis 数据读回。
