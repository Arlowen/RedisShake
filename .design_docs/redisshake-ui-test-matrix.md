# RedisShake UI 测试矩阵

## 1. 验收结论

截至 2026-08-21，Web 控制台已完成从连接创建、任务配置、预检查、RedisShake worker 生命周期、实时状态、日志到目标 Redis 数据读回的本地端到端验收。

绿色构建、HTTP 200、容器 Up 或页面 RUNNING 均不单独计为通过。真实链路必须同时满足：目标数据可读、页面状态来自 worker、日志与进程状态一致、停止后 worker/端口/锁释放。

## 2. 自动化矩阵

| 范围 | 自动化证据 | 结果 |
| --- | --- | --- |
| Go 控制面 | `go test ./...`、`go vet ./...` | 通过 |
| 并发与进程管理 | `go test -race ./internal/controlplane/...` | 通过 |
| SQLite 迁移 | v1 数据库带 Task/Run 外键升级至当前 schema；迁移重复执行 | 通过 |
| Run 治理 | 单任务重复启动、全局并发上限、UNKNOWN 阻断、终态文件保留清理 | 通过 |
| 复制流程 | 连接复制后凭据重新加密；任务复制回到 DRAFT/revision 1 | 通过 |
| 前端类型 | `npm run typecheck` | 通过 |
| 前端单测 | Vitest 3 个文件、6 个测试 | 通过 |
| 前端构建 | Vite production build，按路由拆包 | 通过 |
| 依赖安全 | npm 官方 registry，high audit | 0 漏洞 |
| 单镜像 E2E | Playwright 从页面创建连接和 scan 任务，目标 Redis 读回，过滤 Key 不存在 | 通过 |
| Dockerfile | BuildKit `--check`、非 root 运行、健康检查 | 通过 |
| 多架构镜像 | `linux/amd64,linux/arm64` 实际 cache-only build | 通过 |
| 文档 | VitePress build | 通过 |

## 3. Redis 兼容组合

`deploy/matrix/verify.sh` 使用真实 Redis 7.2 容器和单镜像控制面执行以下检查：

| 拓扑/安全 | 地址 | 检查内容 | 结果 |
| --- | --- | --- | --- |
| ACL standalone | `acl-redis:6379` | 命名用户、密码认证、目标临时 Key 写入与删除 | 通过 |
| Sentinel | `sentinel:26379` | hostname 解析、master name 发现、主节点 PING/INFO | 通过 |
| 三节点 Cluster | `cluster-1:6379` | cluster_enabled、MOVED 路由节点连接、目标写入与删除 | 通过 |
| TLS | `tls-redis:6379` | 临时 CA、SAN `tls-redis`、证书校验开启、PING/INFO | 通过 |

矩阵命令：

```shell
docker build -f backend/Dockerfile.web -t redisshake-ui:dev .
sh deploy/matrix/verify.sh
```

脚本使用临时证书目录、独立网络和 volume，并在退出时清理。

## 4. 真实数据链路

### 4.1 Scan

- 源端预置 String、Hash 和应过滤 Key。
- 页面创建 scan 任务并启动真实 RedisShake。
- Run 以退出码 0 进入 `SUCCEEDED`。
- 目标端 String/Hash 值与源端一致，过滤 Key `EXISTS=0`。

### 4.2 Sync

- 页面启动 sync 任务，状态进入 `syncing aof`。
- 全量 String/Hash 到达目标端。
- 源端新增 SET 后，目标端立即读到新值。
- UI 累计读写计数同步增加，内部 consistent 变为 true，writer unanswered 为 0。
- 页面优雅停止后 Run 进入 `STOPPED`，PID、状态端口和 pid.lockfile 均不再被占用。

## 5. 部署、持久化与恢复

- `backend/Dockerfile.web` 最终镜像约 21.9 MB（arm64 本地构建），运行用户为 `redisshake` UID 100。
- `/tasks/<id>` 深链由控制面回退至 SPA；缺失 `/assets/*` 返回 404，不返回 HTML。
- 静态资源使用 immutable cache，HTML/API 使用 no-store；CSP 只允许同源脚本、字体、图片和 SSE。
- Compose 容器重启前后 Task 数量和名称一致。
- 停止控制面后通过 `docker compose cp` 得到权限 `0600` 的 SQLite 一致备份；重启后 `/readyz` 正常。
- 旧应用不得直接读取新 schema；回滚需要旧镜像、对应数据备份和同一主密钥一起恢复。

## 6. 当前验证边界

- 本地没有向 GHCR 发布镜像；发布工作流将在 tag 或手工触发时构建 CLI 与 `-web` 多架构镜像。
- GitHub Actions 配置已做 YAML 解析，并在本地执行了对应命令；远端 CI 状态应在本提交推送后继续核对。
- 本矩阵验证 Redis 7.2 的 ACL/Sentinel/Cluster/TLS。原 RedisShake 的 Redis 2.8–8.0、Valkey 8/9 黑盒矩阵继续由现有 CI 负责。
