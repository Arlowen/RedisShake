import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/api/client'

afterEach(() => vi.unstubAllGlobals())

describe('api client', () => {
  it('maps structured API errors without leaking response internals', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      error: { code: 'revision_conflict', message: '配置已变化', field: 'expected_revision' },
    }), { status: 409, headers: { 'Content-Type': 'application/json' } })))

    await expect(api.getTask('task-1')).rejects.toMatchObject({
      status: 409,
      code: 'revision_conflict',
      message: '配置已变化',
      field: 'expected_revision',
    })
  })

  it('sends the task revision when starting a worker', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      id: 'run-1', task_id: 'task-1', config_revision: 7, config_snapshot: {}, state: 'RUNNING',
      started_at: '2026-08-21T12:00:00Z', stop_requested_by_user: false, status_healthy: true, updated_at: '2026-08-21T12:00:01Z',
    }), { status: 201, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    await api.startRun('task-1', 7)
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/tasks/task-1/runs', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ expected_revision: 7 }),
    }))
  })

  it('treats 204 responses as successful void operations', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })))
    await expect(api.archiveTask('task-1')).resolves.toBeUndefined()
  })
})
