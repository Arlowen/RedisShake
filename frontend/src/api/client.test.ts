import { afterEach, describe, expect, it, jest } from '@jest/globals'

import { api } from '@/api/client'

const originalFetch = globalThis.fetch

afterEach(() => {
  jest.restoreAllMocks()
  if (originalFetch) Object.defineProperty(globalThis, 'fetch', { value: originalFetch, writable: true, configurable: true })
  else delete (globalThis as { fetch?: typeof fetch }).fetch
})

function response(status: number, payload?: unknown): Response {
  return { status, ok: status >= 200 && status < 300, json: async () => payload } as Response
}

function mockFetch(result: Response) {
  const fetchMock = jest.fn<typeof fetch>().mockResolvedValue(result)
  Object.defineProperty(globalThis, 'fetch', { value: fetchMock, writable: true, configurable: true })
  return fetchMock
}

describe('api client', () => {
  it('maps structured API errors without leaking response internals', async () => {
    mockFetch(response(409, {
      error: { code: 'revision_conflict', message: '配置已变化', field: 'expected_revision' },
    }))

    await expect(api.getTask('task-1')).rejects.toMatchObject({
      status: 409,
      code: 'revision_conflict',
      message: '配置已变化',
      field: 'expected_revision',
    })
  })

  it('sends the task revision when starting a worker', async () => {
    const fetchMock = mockFetch(response(201, {
      id: 'run-1', task_id: 'task-1', config_revision: 7, config_snapshot: {}, state: 'RUNNING',
      started_at: '2026-08-21T12:00:00Z', stop_requested_by_user: false, status_healthy: true, updated_at: '2026-08-21T12:00:01Z',
    }))

    await api.startRun('task-1', 7)
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/tasks/task-1/runs', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ expected_revision: 7 }),
    }))
  })

  it('treats 204 responses as successful void operations', async () => {
    mockFetch(response(204))
    await expect(api.archiveTask('task-1')).resolves.toBeUndefined()
  })
})
