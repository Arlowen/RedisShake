import type {
  Connection,
  ConnectionInput,
  ConnectionTestResult,
  LogResult,
  PrecheckResult,
  Run,
  SystemInfo,
  Task,
  TaskSpec,
  TestPurpose,
} from '@/api/types'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly field?: string

  constructor(status: number, code: string, message: string, field?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.field = field
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
  })
  if (response.status === 204) return undefined as T
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    const error = payload?.error ?? {}
    throw new ApiError(response.status, error.code ?? 'request_failed', error.message ?? `请求失败（${response.status}）`, error.field)
  }
  return payload as T
}

export const api = {
  systemInfo: () => request<SystemInfo>('/api/v1/system/info'),

  listConnections: async () => (await request<{ items: Connection[] }>('/api/v1/connections')).items,
  createConnection: (input: ConnectionInput) => request<Connection>('/api/v1/connections', { method: 'POST', body: JSON.stringify(input) }),
  updateConnection: (id: string, patch: Partial<ConnectionInput>) =>
    request<Connection>(`/api/v1/connections/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  deleteConnection: (id: string) => request<void>(`/api/v1/connections/${id}`, { method: 'DELETE' }),
  copyConnection: (id: string, name: string) => request<Connection>(`/api/v1/connections/${id}/copy`, { method: 'POST', body: JSON.stringify({ name }) }),
  testConnection: (connection: ConnectionInput, purpose: TestPurpose) =>
    request<ConnectionTestResult>('/api/v1/connections/test', { method: 'POST', body: JSON.stringify({ connection, purpose }) }),
  testSavedConnection: (id: string, purpose: TestPurpose) =>
    request<ConnectionTestResult>(`/api/v1/connections/${id}/test`, { method: 'POST', body: JSON.stringify({ purpose }) }),

  listTasks: async (includeArchived = false) =>
    (await request<{ items: Task[] }>(`/api/v1/tasks?include_archived=${includeArchived}`)).items,
  getTask: (id: string) => request<Task>(`/api/v1/tasks/${id}`),
  createTask: (spec: Pick<TaskSpec, 'name' | 'mode'> & Partial<TaskSpec>) =>
    request<Task>('/api/v1/tasks', { method: 'POST', body: JSON.stringify(spec) }),
  updateTask: (id: string, expectedRevision: number, patch: Partial<TaskSpec>) =>
    request<Task>(`/api/v1/tasks/${id}`, { method: 'PATCH', body: JSON.stringify({ expected_revision: expectedRevision, ...patch }) }),
  archiveTask: (id: string) => request<void>(`/api/v1/tasks/${id}`, { method: 'DELETE' }),
  copyTask: (id: string, name: string) => request<Task>(`/api/v1/tasks/${id}/copy`, { method: 'POST', body: JSON.stringify({ name }) }),
  precheckTask: (id: string, expectedRevision: number, acknowledgeWarnings: boolean) =>
    request<PrecheckResult>(`/api/v1/tasks/${id}/precheck`, {
      method: 'POST',
      body: JSON.stringify({ expected_revision: expectedRevision, acknowledge_warnings: acknowledgeWarnings }),
    }),

  listRuns: async (taskId: string) => (await request<{ items: Run[] }>(`/api/v1/tasks/${taskId}/runs`)).items,
  getRun: (id: string) => request<Run>(`/api/v1/runs/${id}`),
  startRun: (taskId: string, expectedRevision: number) =>
    request<Run>(`/api/v1/tasks/${taskId}/runs`, { method: 'POST', body: JSON.stringify({ expected_revision: expectedRevision }) }),
  stopRun: (id: string) => request<Run>(`/api/v1/runs/${id}/stop`, { method: 'POST' }),
  forceStopRun: (id: string) => request<Run>(`/api/v1/runs/${id}/force-stop`, { method: 'POST' }),
  readLogs: (id: string, offset = 0, limit = 65536) => request<LogResult>(`/api/v1/runs/${id}/logs?offset=${offset}&limit=${limit}`),
}
