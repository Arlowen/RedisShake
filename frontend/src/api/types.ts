export type Topology = 'standalone' | 'sentinel' | 'cluster'
export type TestPurpose = 'source' | 'target'
export type TaskMode = 'sync' | 'scan'
export type TaskState = 'DRAFT' | 'READY' | 'ARCHIVED'
export type RunState = 'STARTING' | 'RUNNING' | 'STOPPING' | 'STOPPED' | 'SUCCEEDED' | 'FAILED' | 'UNKNOWN'
export type CheckState = 'PASS' | 'WARNING' | 'FAIL'

export interface TLSInput {
  enabled: boolean
  server_name?: string
  insecure_skip_verify: boolean
  ca_cert_pem?: string
  client_cert_pem?: string
  client_key_pem?: string
}

export interface SentinelInput {
  address: string
  master_name: string
  username?: string
  password?: string
  tls: TLSInput
}

export interface ConnectionInput {
  name: string
  topology: Topology
  address: string
  username?: string
  password?: string
  tls: TLSInput
  sentinel: SentinelInput
}

export interface TLSView {
  enabled: boolean
  server_name?: string
  insecure_skip_verify: boolean
  ca_cert_configured: boolean
  client_cert_configured: boolean
  client_key_configured: boolean
}

export interface Connection {
  id: string
  name: string
  topology: Topology
  address?: string
  username?: string
  password_configured: boolean
  tls: TLSView
  sentinel: {
    address?: string
    master_name?: string
    username?: string
    password_configured: boolean
    tls: TLSView
  }
  created_at: string
  updated_at: string
  last_tested_at?: string
  last_test_result?: ConnectionTestResult
}

export interface CheckItem {
  code: string
  state: CheckState
  message: string
}

export interface ConnectionTestResult {
  success: boolean
  purpose: TestPurpose
  effective_address?: string
  server_product?: string
  server_version?: string
  role?: string
  cluster_enabled: boolean
  latency_ms: number
  checks: CheckItem[]
  tested_at: string
}

export interface SyncReaderOptions {
  sync_rdb: boolean
  sync_aof: boolean
  prefer_replica: boolean
  try_diskless: boolean
}

export interface ScanReaderOptions {
  dbs: number[]
  scan: boolean
  ksn: boolean
  count: number
  prefer_replica: boolean
  skip_unknown_type: string[]
}

export interface FilterOptions {
  allow_keys: string[]
  allow_key_prefix: string[]
  allow_key_suffix: string[]
  allow_key_regex: string[]
  block_keys: string[]
  block_key_prefix: string[]
  block_key_suffix: string[]
  block_key_regex: string[]
  allow_db: number[]
  block_db: number[]
  allow_command: string[]
  block_command: string[]
  allow_command_group: string[]
  block_command_group: string[]
  function: string
}

export interface AdvancedOptions {
  rdb_restore_command_behavior: 'panic' | 'rewrite' | 'skip'
  pipeline_count_limit: number
  target_redis_max_qps: number
  target_redis_client_max_querybuf_len: number
  target_redis_proto_max_bulk_len: number
  empty_db_before_sync: boolean
}

export interface TaskSpec {
  name: string
  description?: string
  mode: TaskMode
  source_connection_id?: string
  target_connection_id?: string
  sync_reader?: SyncReaderOptions
  scan_reader?: ScanReaderOptions
  filter: FilterOptions
  advanced: AdvancedOptions
}

export interface PrecheckResult {
  task_id: string
  config_revision: number
  ready: boolean
  config_digest?: string
  checks: CheckItem[]
  checked_at: string
}

export interface Task {
  id: string
  spec: TaskSpec
  state: TaskState
  config_revision: number
  created_at: string
  updated_at: string
  last_prechecked_at?: string
  last_precheck_result?: PrecheckResult
}

export interface RedisShakeStatus {
  start_time?: string
  consistent?: boolean
  total_entries_count?: {
    read_count?: number
    read_ops?: number
    write_count?: number
    write_ops?: number
  }
  per_cmd_entries_count?: Record<string, unknown>
  reader?: Record<string, unknown> | null
  writer?: Record<string, unknown> | null
}

export interface Run {
  id: string
  task_id: string
  config_revision: number
  config_snapshot: TaskSpec
  state: RunState
  pid?: number
  status_port?: number
  started_at: string
  finished_at?: string
  exit_code?: number
  exit_reason?: string
  last_heartbeat_at?: string
  stop_requested_by_user: boolean
  status_healthy: boolean
  status?: RedisShakeStatus
  updated_at: string
}

export interface LogResult {
  content: string
  next_offset: number
  eof: boolean
}

export interface SystemInfo {
  version: string
  git_commit: string
  storage: string
  data_dir: string
  runtime_dir: string
  secrets_configured: boolean
  worker_path: string
}
