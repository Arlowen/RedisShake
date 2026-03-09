---
outline: deep
---

# Find by Task

This page is only an index. Once you find the right entry, read that page for configuration and limits.

## Mode and Data Source

| Problem | Read this first | Notes |
| --- | --- | --- |
| The source supports replication access and I need full + incremental sync. | [Sync Reader](../reader/sync_reader.md) | This is usually the first choice. |
| The source does not support replication access, but I can accept SCAN and still need incremental updates after full sync. | [Scan Reader](../reader/scan_reader.md) | See `ksn`. |
| The source supports replication access and I only want later incremental changes, not the snapshot written to the target. | [Sync Reader](../reader/sync_reader.md) | See `sync_rdb = false`. |
| The data comes from `dump.rdb`. | [RDB Reader](../reader/rdb_reader.md) | Backup restore workflows. |
| The data comes from an AOF file. | [AOF Reader](../reader/aof_reader.md) | Replay or point-in-time recovery workflows. |

## Filtering, Rewriting, and Target Side

| Problem | Read this first | Notes |
| --- | --- | --- |
| I only want some keys, DBs, commands, or command groups. | [Built-in Filter Rules](../filter/filter.md) | Start here when commands do not need rewriting. |
| I need to rename keys, move data to another DB, split commands, or drop unsupported commands. | [What is function](../filter/function.md) | This page is for command rewriting. |
| The target is Redis or Redis Cluster. | [Redis Writer](../writer/redis_writer.md) | Read this for target connection and write behavior. |
| I want to write Redis data into a file. | [File Writer](../writer/file_writer.md) | Useful for offline replay, auditing, and cleanup. |
| The source has multiple DBs, but the target is a Redis cluster. | [Redis Writer](../writer/redis_writer.md), [Built-in Filter Rules](../filter/filter.md), [What is function](../filter/function.md) | Redis cluster only supports `db 0`. Use filter rules to drop DBs, or `function` to rewrite them. |

## Connectivity, Compatibility, and Verification

| Problem | Read this first | Notes |
| --- | --- | --- |
| I am using ElastiCache, Tair, Azure, or another managed service, and I am not sure whether PSync is available. | [Migration Mode Selection](mode.md), [Version Compatibility](../others/compatibility.md) | Check product form, permissions, and limits first. |
| How do I configure TLS, ACL, username, or password? | [Sync Reader](../reader/sync_reader.md), [Scan Reader](../reader/scan_reader.md), [Redis Writer](../writer/redis_writer.md) | Connection settings live in reader and writer pages. |
| How do I know whether sync is finished or data is consistent? | [How to Verify Data Consistency](../others/consistent.md) | Check status first, then verify with the documented rules. |
| What are the limits around long-running sync, checkpoint, and topology change? | [Migration Mode Selection](mode.md), [Checkpoint](../others/checkpoint.md) | Check these limits during planning, not after rollout. |
