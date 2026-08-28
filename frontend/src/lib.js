export class ApiError extends Error {
  constructor(status, code, message, field) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.field = field
  }
}

async function request(path, init = {}) {
  const response = await fetch(path, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...(init.headers || {}),
    },
  })
  if (response.status === 204) return undefined
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    const error = payload?.error || {}
    throw new ApiError(response.status, error.code || 'request_failed', error.message || `请求失败（${response.status}）`, error.field)
  }
  return payload
}

export const api = {
  listConnections: async () => (await request('/api/v1/connections')).items,
  createConnection: (input) => request('/api/v1/connections', { method: 'POST', body: JSON.stringify(input) }),
  updateConnection: (id, patch) => request(`/api/v1/connections/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  deleteConnection: (id) => request(`/api/v1/connections/${id}`, { method: 'DELETE' }),
  copyConnection: (id, name) => request(`/api/v1/connections/${id}/copy`, { method: 'POST', body: JSON.stringify({ name }) }),
  testConnection: (connection, purpose) => request('/api/v1/connections/test', { method: 'POST', body: JSON.stringify({ connection, purpose }) }),
  testSavedConnection: (id, purpose) => request(`/api/v1/connections/${id}/test`, { method: 'POST', body: JSON.stringify({ purpose }) }),
  listTasks: async (includeArchived = false) => (await request(`/api/v1/tasks?include_archived=${includeArchived}`)).items,
  getTask: (id) => request(`/api/v1/tasks/${id}`),
  createTask: (spec) => request('/api/v1/tasks', { method: 'POST', body: JSON.stringify(spec) }),
  updateTask: (id, expectedRevision, patch) => request(`/api/v1/tasks/${id}`, { method: 'PATCH', body: JSON.stringify({ expected_revision: expectedRevision, ...patch }) }),
  archiveTask: (id) => request(`/api/v1/tasks/${id}`, { method: 'DELETE' }),
  copyTask: (id, name) => request(`/api/v1/tasks/${id}/copy`, { method: 'POST', body: JSON.stringify({ name }) }),
  precheckTask: (id, expectedRevision, acknowledgeWarnings) => request(`/api/v1/tasks/${id}/precheck`, { method: 'POST', body: JSON.stringify({ expected_revision: expectedRevision, acknowledge_warnings: acknowledgeWarnings }) }),
  listRuns: async (taskId) => (await request(`/api/v1/tasks/${taskId}/runs`)).items,
  startRun: (taskId, expectedRevision) => request(`/api/v1/tasks/${taskId}/runs`, { method: 'POST', body: JSON.stringify({ expected_revision: expectedRevision }) }),
  stopRun: (id) => request(`/api/v1/runs/${id}/stop`, { method: 'POST' }),
  forceStopRun: (id) => request(`/api/v1/runs/${id}/force-stop`, { method: 'POST' }),
  readLogs: (id, offset = 0, limit = 65536) => request(`/api/v1/runs/${id}/logs?offset=${offset}&limit=${limit}`),
}

export const taskStateMeta = {
  DRAFT: ['草稿', 'neutral'], READY: ['可启动', 'success'], ARCHIVED: ['已归档', 'neutral'],
}
export const runStateMeta = {
  STARTING: ['启动中', 'active'], RUNNING: ['运行中', 'active'], STOPPING: ['停止中', 'warning'],
  STOPPED: ['已停止', 'neutral'], SUCCEEDED: ['已完成', 'success'], FAILED: ['失败', 'danger'], UNKNOWN: ['状态未知', 'warning'],
}
export const checkStateMeta = { PASS: ['通过', 'success'], WARNING: ['警告', 'warning'], FAIL: ['阻断', 'danger'] }
export const topologyLabel = { standalone: '单机 / 主从', sentinel: 'Sentinel', cluster: 'Cluster' }
export const modeLabel = { sync: '增量同步', scan: '扫描迁移' }

export function defaultConnectionInput() {
  return {
    name: '', topology: 'standalone', address: '127.0.0.1:6379', username: '', password: '',
    tls: { enabled: false, server_name: '', insecure_skip_verify: false, ca_cert_pem: '', client_cert_pem: '', client_key_pem: '' },
    sentinel: { address: '127.0.0.1:26379', master_name: '', username: '', password: '', tls: { enabled: false, server_name: '', insecure_skip_verify: false } },
  }
}

export function defaultTaskSpec(mode = 'sync') {
  return {
    name: '', description: '', mode, source_connection_id: '', target_connection_id: '',
    sync_reader: mode === 'sync' ? { sync_rdb: true, sync_aof: true, prefer_replica: false, try_diskless: false } : undefined,
    scan_reader: mode === 'scan' ? { dbs: [], scan: true, ksn: false, count: 1, prefer_replica: false, skip_unknown_type: [] } : undefined,
    filter: {
      allow_keys: [], allow_key_prefix: [], allow_key_suffix: [], allow_key_regex: [], block_keys: [], block_key_prefix: [], block_key_suffix: [], block_key_regex: [],
      allow_db: [], block_db: [], allow_command: [], block_command: [], allow_command_group: [], block_command_group: [], function: '',
    },
    advanced: {
      rdb_restore_command_behavior: 'panic', pipeline_count_limit: 1024, target_redis_max_qps: 300000,
      target_redis_client_max_querybuf_len: 1073741824, target_redis_proto_max_bulk_len: 512000000, empty_db_before_sync: false,
    },
  }
}

export function escapeHtml(value = '') {
  return String(value).replace(/[&<>'"]/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' })[character])
}

export function formatDate(value) {
  if (!value) return '—'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value))
}

export function formatNumber(value) {
  return value === undefined || value === null ? '—' : new Intl.NumberFormat('zh-CN').format(value)
}

export function lines(value) { return String(value || '').split(/[\n,]/).map((item) => item.trim()).filter(Boolean) }
export function numbers(value) { return lines(value).map(Number).filter(Number.isInteger) }
export function clone(value) { return structuredClone(value) }
export function isActive(run) { return Boolean(run && ['STARTING', 'RUNNING', 'STOPPING'].includes(run.state)) }
export function stripAnsi(value) { return String(value || '').replace(/\u001b\[[0-9;]*m/g, '') }
