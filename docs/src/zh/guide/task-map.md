---
outline: deep
---

# 按任务查找

这页只做入口。确定入口后，再看对应页面里的配置和限制说明。

## 模式与数据来源

| 你遇到的问题 | 先看这里 | 备注 |
| --- | --- | --- |
| 源端支持复制协议，需要全量 + 增量同步。 | [Sync Reader](../reader/sync_reader.md) | 大多数迁移场景优先选这个。 |
| 源端不支持复制协议，但可以接受 SCAN，并希望在全量之后继续接收变更。 | [Scan Reader](../reader/scan_reader.md) | 重点看 `ksn`。 |
| 源端支持复制协议，只想接后续增量，不把快照内容写到目标端。 | [Sync Reader](../reader/sync_reader.md) | 重点看 `sync_rdb = false`。 |
| 数据来自 `dump.rdb`。 | [RDB Reader](../reader/rdb_reader.md) | 备份恢复场景。 |
| 数据来自 AOF 文件。 | [AOF Reader](../reader/aof_reader.md) | 回放或闪回场景。 |

## 过滤、改写与目标端

| 你遇到的问题 | 先看这里 | 备注 |
| --- | --- | --- |
| 只同步部分 key、DB、命令或命令组。 | [内置过滤规则](../filter/filter.md) | 不改写命令时先看这里。 |
| 需要改 key、改 DB、拆命令，或者跳过不兼容命令。 | [什么是 function](../filter/function.md) | 命令改写都在这里。 |
| 目标端是 Redis 或 Redis Cluster。 | [Redis Writer](../writer/redis_writer.md) | 这里看目标端连接和写入行为。 |
| 想把 Redis 数据写成文件。 | [File Writer](../writer/file_writer.md) | 适合离线回放、审计或数据修复。 |
| 源端有多个 DB，目标端是 Redis Cluster。 | [Redis Writer](../writer/redis_writer.md)、[内置过滤规则](../filter/filter.md)、[什么是 function](../filter/function.md) | Cluster 只支持 `db 0`；只保留部分 DB 用 filter，改写 DB 用 function。 |

## 连接、兼容性与校验

| 你遇到的问题 | 先看这里 | 备注 |
| --- | --- | --- |
| 用的是 ElastiCache、Tair、Azure 这类托管服务，不确定能不能用 PSync。 | [迁移模式选择](mode.md)、[版本兼容性](../others/compatibility.md) | 先确认实例形态、权限和限制。 |
| TLS、ACL、用户名、密码这些连接参数该去哪配？ | [Sync Reader](../reader/sync_reader.md)、[Scan Reader](../reader/scan_reader.md)、[Redis Writer](../writer/redis_writer.md) | 连接参数都在 Reader / Writer 页面。 |
| 想确认同步是否完成，或者两端是否一致。 | [如何判断数据一致](../others/consistent.md) | 先看状态，再按文档校验。 |
| 想了解长期同步、断点续传、拓扑变化这些限制。 | [迁移模式选择](mode.md)、[断点续传](../others/checkpoint.md) | 这类限制最好在选型阶段确认。 |
