import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import type { ReactNode } from 'react'

import { api } from '@/api/client'
import type { PrecheckResult, Run, Task, TaskSpec } from '@/api/types'

interface TasksState {
  items: Task[]
  loading: boolean
  error?: string
  load: (includeArchived?: boolean) => Promise<void>
  createDraft: (spec: Pick<TaskSpec, 'name' | 'mode'> & Partial<TaskSpec>) => Promise<Task>
  updateDraft: (task: Task, patch: Partial<TaskSpec>) => Promise<Task>
  precheck: (task: Task, acknowledgeWarnings: boolean) => Promise<PrecheckResult>
  start: (task: Task) => Promise<Run>
  archive: (task: Task) => Promise<void>
  copy: (task: Task) => Promise<Task>
  replace: (task: Task) => void
}

const TasksContext = createContext<TasksState | undefined>(undefined)

export function TasksProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Task[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string>()

  const replace = useCallback((task: Task) => {
    setItems((current) => current.some((item) => item.id === task.id)
      ? current.map((item) => item.id === task.id ? task : item)
      : [task, ...current])
  }, [])

  const load = useCallback(async (includeArchived = false) => {
    setLoading(true); setError(undefined)
    try { setItems(await api.listTasks(includeArchived)) }
    catch (cause) { setError(cause instanceof Error ? cause.message : '任务列表加载失败'); throw cause }
    finally { setLoading(false) }
  }, [])

  const createDraft = useCallback(async (spec: Pick<TaskSpec, 'name' | 'mode'> & Partial<TaskSpec>) => {
    const task = await api.createTask(spec)
    setItems((current) => [task, ...current])
    return task
  }, [])

  const updateDraft = useCallback(async (task: Task, patch: Partial<TaskSpec>) => {
    const updated = await api.updateTask(task.id, task.config_revision, patch)
    replace(updated)
    return updated
  }, [replace])

  const precheck = useCallback(async (task: Task, acknowledgeWarnings: boolean) => {
    const result = await api.precheckTask(task.id, task.config_revision, acknowledgeWarnings)
    replace(await api.getTask(task.id))
    return result
  }, [replace])

  const start = useCallback((task: Task) => api.startRun(task.id, task.config_revision), [])
  const archive = useCallback(async (task: Task) => { await api.archiveTask(task.id); setItems((current) => current.filter((item) => item.id !== task.id)) }, [])
  const copy = useCallback(async (task: Task) => { const copied = await api.copyTask(task.id, `${task.spec.name} 副本`); setItems((current) => [copied, ...current]); return copied }, [])

  const value = useMemo<TasksState>(() => ({ items, loading, error, load, createDraft, updateDraft, precheck, start, archive, copy, replace }), [items, loading, error, load, createDraft, updateDraft, precheck, start, archive, copy, replace])
  return <TasksContext.Provider value={value}>{children}</TasksContext.Provider>
}

export function useTasks() {
  const context = useContext(TasksContext)
  if (!context) throw new Error('useTasks must be used inside TasksProvider')
  return context
}
