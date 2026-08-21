import { ref } from 'vue'
import { defineStore } from 'pinia'

import { api } from '@/api/client'
import type { PrecheckResult, Run, Task, TaskSpec } from '@/api/types'

export const useTasksStore = defineStore('tasks', () => {
  const items = ref<Task[]>([])
  const loading = ref(false)
  const error = ref<string>()

  async function load(includeArchived = false) {
    loading.value = true
    error.value = undefined
    try {
      items.value = await api.listTasks(includeArchived)
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : '任务列表加载失败'
      throw cause
    } finally {
      loading.value = false
    }
  }

  async function createDraft(spec: Pick<TaskSpec, 'name' | 'mode'> & Partial<TaskSpec>) {
    const task = await api.createTask(spec)
    items.value = [task, ...items.value]
    return task
  }

  async function updateDraft(task: Task, patch: Partial<TaskSpec>) {
    const updated = await api.updateTask(task.id, task.config_revision, patch)
    replace(updated)
    return updated
  }

  async function precheck(task: Task, acknowledgeWarnings: boolean): Promise<PrecheckResult> {
    const result = await api.precheckTask(task.id, task.config_revision, acknowledgeWarnings)
    const refreshed = await api.getTask(task.id)
    replace(refreshed)
    return result
  }

  async function start(task: Task): Promise<Run> {
    return api.startRun(task.id, task.config_revision)
  }

  async function archive(task: Task) {
    await api.archiveTask(task.id)
    items.value = items.value.filter((item) => item.id !== task.id)
  }

  function replace(task: Task) {
    const index = items.value.findIndex((item) => item.id === task.id)
    if (index === -1) items.value = [task, ...items.value]
    else items.value.splice(index, 1, task)
  }

  return { items, loading, error, load, createDraft, updateDraft, precheck, start, archive, replace }
})
