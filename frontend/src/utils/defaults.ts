import type { AdvancedOptions, ConnectionInput, FilterOptions, TaskMode, TaskSpec } from '@/api/types'

export function defaultConnectionInput(): ConnectionInput {
  return {
    name: '',
    topology: 'standalone',
    address: '127.0.0.1:6379',
    username: '',
    password: '',
    tls: {
      enabled: false,
      server_name: '',
      insecure_skip_verify: false,
      ca_cert_pem: '',
      client_cert_pem: '',
      client_key_pem: '',
    },
    sentinel: {
      address: '127.0.0.1:26379',
      master_name: '',
      username: '',
      password: '',
      tls: { enabled: false, server_name: '', insecure_skip_verify: false },
    },
  }
}

export function defaultFilterOptions(): FilterOptions {
  return {
    allow_keys: [], allow_key_prefix: [], allow_key_suffix: [], allow_key_regex: [],
    block_keys: [], block_key_prefix: [], block_key_suffix: [], block_key_regex: [],
    allow_db: [], block_db: [], allow_command: [], block_command: [],
    allow_command_group: [], block_command_group: [], function: '',
  }
}

export function defaultAdvancedOptions(): AdvancedOptions {
  return {
    rdb_restore_command_behavior: 'panic',
    pipeline_count_limit: 1024,
    target_redis_max_qps: 300000,
    target_redis_client_max_querybuf_len: 1073741824,
    target_redis_proto_max_bulk_len: 512000000,
    empty_db_before_sync: false,
  }
}

export function defaultTaskSpec(mode: TaskMode = 'sync'): TaskSpec {
  return {
    name: '',
    description: '',
    mode,
    source_connection_id: undefined,
    target_connection_id: undefined,
    sync_reader: mode === 'sync' ? { sync_rdb: true, sync_aof: true, prefer_replica: false, try_diskless: false } : undefined,
    scan_reader: mode === 'scan' ? { dbs: [], scan: true, ksn: false, count: 1, prefer_replica: false, skip_unknown_type: [] } : undefined,
    filter: defaultFilterOptions(),
    advanced: defaultAdvancedOptions(),
  }
}
