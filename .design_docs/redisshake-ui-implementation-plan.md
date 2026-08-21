# RedisShake UI 实施计划

## 1. 背景与问题

当前 RedisShake 以命令行工具运行，用户需要手工编辑 TOML、启动进程、查询状态端口并查看日志。目标是在不替换 RedisShake 数据同步内核的前提下，增加一个自部署 Web 控制台，让用户通过页面完成 Redis 连接管理、同步任务创建、预检查、启停、运行监控和日志排查。

本计划按以下首版假设推进，开发开始前可以调整：

- 参考 CloudCanal 的分步任务向导和任务列表组织方式。
- 参考 NineData 的连接测试、预检查、运行监控和错误反馈思路；不声称复刻其当前线上实现。
- 面向单团队、自部署场景，首版不实现注册、租户、RBAC 和计费。
- 首版仅支持 Redis 到 Redis，覆盖 `sync_reader`、`scan_reader`、单机、哨兵、集群、ACL 和 TLS。
- 首版不支持 RDB/AOF 文件导入导出，不改变 RedisShake 本身“不支持断点续传”和“运行期间不感知集群拓扑变更”的限制。

## 2. 范围与影响面

### 2.1 首版范围

- Redis 连接：新增、编辑、复制、删除、连通性测试和凭据保护。
- 同步任务：草稿、预检查、启动、优雅停止、强制停止、复制和归档。
- 任务配置：源端、目标端、同步模式、DB/Key/命令过滤、限速及必要高级参数。
- 运行实例：每次启动生成不可变配置快照，保留状态、时间、退出原因和日志位置。
- 运行监控：进程状态、RedisShake 状态、读写计数、OPS、同步阶段、延迟/堆积指标和日志流。
- 部署：单节点控制面，SQLite 持久化，RedisShake 子进程与控制面运行在同一主机或容器内。

### 2.2 暂不包含

- 多租户、细粒度权限、审计审批、计费和 SaaS 控制面。
- 多节点调度、远程 Agent、高可用控制面和跨节点故障转移。
- 定时任务、断点续传、自动切换业务流量和目标端校验对账。
- RDB/AOF 文件任务、Redis 之外的数据源或目标端。
- 浏览器直接编辑任意 TOML；高级能力由受控字段生成配置。

### 2.3 代码与部署影响

| 区域 | 计划变更 |
| --- | --- |
| `backend/cmd/redis-shake` | 保持为同步工作进程；只增加控制面必须的兼容性小改动，不嵌入 Web 业务 |
| `backend/cmd/redis-shake-server` | 新增控制面服务入口，提供 API、静态前端和任务调度 |
| `backend/internal/` | 新增连接、任务、运行记录、配置生成、进程管理、存储和 API 模块 |
| `frontend/` | 新增 Vue 3、TypeScript、Vite 管理控制台，默认采用 Ant Design Vue |
| `.github/workflows/` | 增加前端检查、控制面测试和端到端测试，不破坏现有 RedisShake CLI 发布 |
| 部署产物 | 保留 CLI 镜像；新增包含控制面、前端静态资源和 RedisShake worker 的单机版镜像 |

## 3. 总体方案

### 3.1 架构边界

```text
Browser
  -> Web UI
  -> redis-shake-server REST/SSE API
       -> SQLite: connection/task/run metadata
       -> Secret provider: encrypted credentials
       -> Engine manager
            -> task A redis-shake process -> source/target Redis
            -> task B redis-shake process -> source/target Redis
       -> task runtime directory: config/status/log/data
```

控制面不直接实现 Redis 数据复制。它负责把页面字段转换为 RedisShake TOML、启动和监管 RedisShake、读取状态端口、汇总日志，并把运行状态提供给前端。

### 3.2 为什么采用独立子进程

现有 RedisShake 使用全局 `config.Opt`、全局 `status` 状态、进程级信号处理，并会切换到 `advanced.dir` 后获取文件锁。把多个任务直接放进同一个 Go 进程会产生配置覆盖、状态串扰、工作目录冲突和退出信号互相影响。因此首版必须做到“一次运行一个 RedisShake 子进程”，而不是把 `main` 流程直接改造成多任务库。

每个运行实例使用独立目录：

```text
runtime/tasks/<task-id>/runs/<run-id>/
  shake.toml
  data/
  shake.log
  stdout.log
  process.json
```

- 目录权限默认 `0700`，含凭据的配置文件默认 `0600`。
- `shake.toml` 是本次运行的不可变快照，任务后续编辑不影响正在运行的实例。
- 状态端口只绑定回环地址，由控制面访问，不直接暴露给浏览器。
- 控制面通过进程组发送信号，先 `SIGTERM` 优雅停止，超时后才允许用户强制终止。

### 3.3 首版技术选择

- 后端：现有 Go 模块，控制面优先使用标准 `net/http`；业务层与 HTTP 层分离。
- 存储：SQLite，所有迁移脚本版本化；数据库和运行目录放入可挂载数据卷。
- 前端：Vue 3 + TypeScript + Vite + Vue Router + Pinia + Ant Design Vue。
- 实时状态：状态摘要使用轮询或 SSE；日志流使用 SSE，首版不引入 WebSocket。
- 部署：单镜像运行，控制面负责回收子进程；现有纯 CLI 构建和镜像继续保留。

## 4. UI 设计

### 4.1 导航与页面

- `同步任务`：默认入口，展示任务及最新运行状态。
- `连接管理`：管理源端和目标端 Redis 连接。
- `任务详情`：概览、监控、配置快照、运行记录、日志五个区域。
- `系统设置`：首版只放运行目录、并发上限、日志保留期和 RedisShake 版本信息；敏感运行参数仍通过环境变量配置。

### 4.2 任务列表

列表字段包括任务名称、模式、源端、目标端、最新状态、同步阶段、当前 OPS、最近启动时间和操作。支持按名称搜索、按状态/模式筛选和按更新时间排序。

重要操作规则：

- 草稿显示“继续配置”；预检查通过且没有活动运行时显示“启动”。
- 运行中只允许查看、停止和复制，不允许直接修改当前运行配置。
- 删除采用归档语义；存在活动运行时禁用归档并说明原因。
- 空状态提供“创建第一个同步任务”按钮；加载使用骨架屏；接口错误保留重试入口。

### 4.3 创建任务向导

1. **基本信息**：任务名称、描述、`sync_reader` 或 `scan_reader`。
2. **源端 Redis**：选择已有连接或新建连接；选择单机、哨兵、集群；执行连接测试。
3. **目标端 Redis**：同源端，并执行目标写权限测试；测试会写入带 TTL 的临时 Key 并尽力删除，页面必须提前说明副作用。
4. **同步范围**：DB、Key、前后缀、正则、命令和命令组过滤；互斥的 allow/block 配置给出即时校验。
5. **高级配置**：RDB/AOF 阶段选择、限速、Pipeline、冲突处理、是否清空目标库等；危险项折叠展示并要求二次确认。
6. **预检查与确认**：显示阻断项、警告项和通过项；存在阻断项时禁用“保存并启动”，允许保存草稿。

向导支持前后切换和自动保存草稿。密码回显始终为空或掩码，后端不返回明文。

### 4.4 任务详情与监控

- 顶部：任务名称、运行状态、运行时长、模式、源端到目标端以及启动/停止操作。
- 概览：当前同步阶段、累计读取/写入、实时 OPS、是否一致、最近心跳和退出原因。
- 监控：根据 reader 类型展示 RDB 接收进度、AOF offset、Scan DB/游标/百分比和 writer 未响应条目/字节数。
- 运行记录：每次运行单独一行，可打开当次配置快照、状态和日志。
- 日志：实时追加、暂停滚动、级别筛选、关键词搜索、下载；服务端在写入和返回前屏蔽密码、用户名之外的敏感 Token 和连接串凭据。

停止任务时弹窗说明“先优雅停止，超时后可强制停止”。`empty_db_before_sync` 等危险配置在详情页持续显示醒目标记。

## 5. 核心模型与接口契约

### 5.1 核心模型

**Connection**

- `id`、`name`、`topology`（standalone/sentinel/cluster）
- `address`、`username`、加密后的 `password`
- `tls_enabled`、TLS 配置引用、Sentinel master name
- `created_at`、`updated_at`、`last_tested_at`、`last_test_result`

**Task**

- `id`、`name`、`description`、`mode`（sync/scan）
- `source_connection_id`、`target_connection_id`
- `reader_options`、`filter_options`、`advanced_options`
- `state`（DRAFT/READY/ARCHIVED）、`config_revision`
- `created_at`、`updated_at`

**Run**

- `id`、`task_id`、`config_revision`、脱敏后的配置快照
- `state`（STARTING/RUNNING/STOPPING/STOPPED/SUCCEEDED/FAILED/UNKNOWN）
- `pid`、进程启动标识、状态端口、运行目录
- `started_at`、`finished_at`、`exit_code`、`exit_reason`、`last_heartbeat_at`
- `stop_requested_by_user`，用于区分正常停止和意外退出

状态映射规则：

- `scan_reader` 正常退出码为 0：`SUCCEEDED`。
- `sync_reader` 在用户停止后退出：`STOPPED`；未请求停止却退出：`FAILED`。
- 控制面重启后无法确认原进程身份：先标记 `UNKNOWN`，不得直接用旧 PID 发送信号。
- RedisShake 状态接口连续超时只标记运行不健康，不立即认定进程退出；进程状态和状态心跳分别展示。

### 5.2 API 草案

| 方法与路径 | 用途 |
| --- | --- |
| `GET/POST /api/v1/connections` | 查询或创建连接 |
| `GET/PATCH/DELETE /api/v1/connections/{id}` | 查看、编辑或删除连接 |
| `POST /api/v1/connections/test` | 测试未保存连接，返回分项结果 |
| `POST /api/v1/connections/{id}/test` | 重新测试已保存连接 |
| `GET/POST /api/v1/tasks` | 查询或创建任务草稿 |
| `GET/PATCH/DELETE /api/v1/tasks/{id}` | 查看、编辑或归档任务 |
| `POST /api/v1/tasks/{id}/precheck` | 生成配置并执行预检查 |
| `POST /api/v1/tasks/{id}/runs` | 按当前 revision 启动一次运行 |
| `GET /api/v1/tasks/{id}/runs` | 查询运行历史 |
| `GET /api/v1/runs/{id}` | 查询单次运行状态和指标 |
| `POST /api/v1/runs/{id}/stop` | 请求优雅停止 |
| `POST /api/v1/runs/{id}/force-stop` | 二次确认后强制停止 |
| `GET /api/v1/runs/{id}/events` | SSE 推送状态摘要和日志事件 |
| `GET /api/v1/runs/{id}/logs` | 分页查询历史日志 |

所有写接口校验当前状态和 `config_revision`，冲突时返回 `409`，防止重复启动或覆盖并发编辑。错误响应包含稳定错误码、用户可读消息和脱敏后的诊断信息。

### 5.3 凭据和安全

- 连接密码使用由 `REDISSHAKE_MASTER_KEY` 派生的密钥加密后存储；API 永不返回密文或明文。
- 数据库中已有凭据但启动时缺少/错误主密钥，控制面拒绝进入可写状态并给出恢复说明。
- RedisShake 配置文件只在运行目录生成，停止后按保留策略清理；日志写入和 API 返回均执行脱敏。
- API 默认只监听回环地址；对外部署通过反向代理提供 TLS。首版虽不做用户权限，也不得默认暴露到公网。

## 6. 关键流程与实施阶段

### 6.1 创建并启动

1. 用户完成向导并保存草稿，后端持久化 Task 和配置 revision。
2. 用户执行预检查：验证字段、连接、拓扑、Redis 版本、源目标是否相同、PSync 能力、目标写权限和已知 RedisShake 限制。
3. 阻断项必须修复；警告项需要用户确认。通过后 Task 进入 READY。
4. 启动时创建 Run，冻结配置快照，分配独立运行目录和状态端口，原子写入 TOML。
5. Engine manager 启动 RedisShake 子进程，捕获 stdout/stderr，并轮询状态接口。
6. 进程成功启动且状态接口可读后进入 RUNNING；超时或提前退出则进入 FAILED 并保留日志。

### 6.2 停止与控制面重启

1. 停止请求把 Run 原子切换为 STOPPING，并向对应进程组发送 `SIGTERM`。
2. RedisShake 完成 reader 取消、writer 刷写和文件锁释放后退出，Run 进入 STOPPED。
3. 超过超时时间后页面显示“强制停止”，强制停止必须单独调用接口并记录原因。
4. 控制面启动时扫描 RUNNING/STOPPING 记录，通过 PID、进程启动标识、可执行文件和状态端口联合确认身份；确认失败时标记 UNKNOWN，不误杀其他进程。

### 6.3 分阶段实施与验收门

#### Phase 0：契约冻结与测试基线

- 固化 Task/Run 状态机、API OpenAPI 草案、配置字段映射和首版非目标。
- 为现有 RedisShake 保存 Go 单测、构建、CLI 启动和状态 JSON 样例基线。
- 确认前端技术栈、数据目录、端口和主密钥配置方式。

**验收门：** 设计评审通过；现有 `go test ./...`、`build.sh`、Dockerfile 和文档构建保持通过。

#### Phase 1：控制面骨架和持久化

- 新增 `redis-shake-server`、健康检查、配置加载、SQLite migration 和 repository 层。
- 实现 Connection/Task/Run 模型、状态机单测和 OpenAPI 文档。
- 引入主密钥、凭据加密、日志脱敏和运行目录权限控制。

**验收门：** 数据库迁移可重复执行；服务重启后模型数据完整；敏感字段不会出现在 API、日志和测试快照中。

#### Phase 2：连接管理和预检查

- 实现连接 CRUD、单机/哨兵/集群发现、ACL/TLS 连接测试。
- 实现 sync/scan 模式检查、目标临时 Key 写入/删除检查和危险配置警告。
- 生成规范化 RedisShake TOML，并做快照测试。

**验收门：** 两个真实 Redis 实例上能区分成功、认证失败、TLS 失败、目标无写权限和源目标相同；生成配置可由 RedisShake 成功加载。

#### Phase 3：Engine manager 和真实任务生命周期

- 实现独立运行目录、端口分配、子进程启动、状态采集、优雅/强制停止和退出码映射。
- 为 RedisShake 增加可配置的状态监听地址，使控制面能固定使用回环地址，同时保持 CLI 兼容。
- 实现控制面重启后的运行记录核对和 UNKNOWN 保护逻辑。

**验收门：** 可通过 API 完成 source Redis 到 target Redis 的 scan 和 sync；同步中新增 Key 可到达目标端；停止后无残留 worker 和文件锁；控制面重启不会重复启动或误杀任务。

#### Phase 4：前端纵向闭环

- 实现连接管理、任务列表、六步向导、预检查结果和任务详情。
- 接入实时状态、日志、停止确认、错误重试和所有空/加载/失败/禁用状态。
- 桌面端优先，保证 1280px 以上布局完整；窄屏允许表格转卡片，不承诺完整移动端运维体验。

**验收门：** 用户无需编辑 TOML 或使用终端，即可在页面完成“建连接—建任务—预检查—启动—观察增量—停止—查看日志”的完整流程。

#### Phase 5：打包、矩阵回归和发布准备

- 提供开发 Compose：控制面、source Redis、target Redis；补充 Sentinel、Cluster、ACL 和 TLS 测试组合。
- 构建单机版镜像并验证数据卷、信号转发、子进程回收、升级和回滚。
- 增加后端单测/集成测试、前端单测、API 契约测试和 Playwright 端到端测试。
- 更新 README、部署文档、故障排查和版本发布流程。

**验收门：** CI 全绿；镜像冷启动后可完成真实同步；重启容器后历史任务和日志可读；回滚不会损坏 SQLite；现有 RedisShake CLI 产物和使用方式不受影响。

## 7. 兼容、迁移与降级

- 保留 `redis-shake <config.toml>` 的 CLI 行为、配置格式和现有发布产物，Web 控制面是新增入口。
- RedisShake worker 与控制面通过版本探测校验兼容性；版本不匹配时禁止启动并给出要求版本。
- SQLite schema 只允许向前迁移；升级前备份数据库，回滚时恢复数据库和对应应用版本。
- 控制面不可用时，已运行 worker 的处理策略必须显式配置。首版默认在控制面正常退出时优雅停止其管理的 worker；异常退出后的存活进程由重启核对流程接管。
- 状态接口不可用时，页面降级为进程状态和日志，不伪造同步健康状态。
- 不把 RedisShake 的 `consistent=true` 等同于业务数据完成对账，页面文案仅显示“RedisShake 内部状态一致”。

## 8. 验收标准与验证方式

### 8.1 功能验收

- [ ] 用户可以创建并测试 standalone、sentinel、cluster 连接，ACL/TLS 失败有明确反馈。
- [ ] 用户可以创建 sync 和 scan 任务，配置过滤、限速和冲突处理。
- [ ] 预检查能区分阻断、警告和通过项，阻断时无法启动。
- [ ] 同一任务不会重复启动；不同任务使用独立目录、端口和 RedisShake 进程。
- [ ] 页面能显示 RedisShake 的真实阶段、读写计数、OPS、reader/writer 指标和脱敏日志。
- [ ] 优雅停止会等待 RedisShake 清理；强制停止必须二次确认。
- [ ] 任务配置修改不会影响正在运行的配置快照。
- [ ] 控制面和容器重启后不会丢失历史任务，不会误杀无关 PID。

### 8.2 真实链路验收

至少使用两个隔离 Redis 实例执行以下链路：

1. 通过 UI 创建源端和目标端连接并测试成功。
2. 创建 scan 任务，源端预置多 DB、多类型 Key，启动后核对目标数据和过滤结果。
3. 创建 sync 任务，完成全量后继续写入新增、更新、删除命令，确认目标端持续变化。
4. 在运行中观察状态和日志，执行优雅停止，确认 worker 退出且目标端无未刷写队列。
5. 覆盖认证失败、目标写权限不足、端口冲突、worker 提前退出、状态接口超时和控制面重启。

### 8.3 自动化验证

- 后端：状态机、配置生成、加密、脱敏、repository、进程身份确认的单元测试。
- 集成：SQLite migration、真实 Redis 连接测试、RedisShake 子进程启动/停止和状态读取。
- 前端：表单规则、向导草稿、状态操作权限和错误展示测试。
- E2E：Playwright 驱动完整 UI 同步流程，并读取目标 Redis 证明数据真实到达。
- 回归：保留现有 RedisShake Go 单测、黑盒测试、跨版本 Redis/Valkey 矩阵、Docker 和 release 构建。

只有真实 Redis 数据写入目标端、页面状态与 worker/日志一致、停止后进程得到回收，才能判定首版闭环完成；HTTP 200、进程存在或页面显示 RUNNING 均不能单独作为验收依据。
