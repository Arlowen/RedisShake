import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { api } from '@/api/client'
import type { Connection, ConnectionInput, ConnectionTestResult, TestPurpose } from '@/api/types'

export const useConnectionsStore = defineStore('connections', () => {
  const items = ref<Connection[]>([])
  const loading = ref(false)
  const loaded = ref(false)
  const error = ref<string>()

  const selectable = computed(() => items.value.map((item) => ({ label: item.name, value: item.id, connection: item })))

  async function load(force = false) {
    if (loaded.value && !force) return
    loading.value = true
    error.value = undefined
    try {
      items.value = await api.listConnections()
      loaded.value = true
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : '连接列表加载失败'
      throw cause
    } finally {
      loading.value = false
    }
  }

  async function create(input: ConnectionInput) {
    const connection = await api.createConnection(input)
    items.value = [...items.value, connection].sort((a, b) => a.name.localeCompare(b.name))
    return connection
  }

  async function update(id: string, patch: Partial<ConnectionInput>) {
    const connection = await api.updateConnection(id, patch)
    items.value = items.value.map((item) => (item.id === id ? connection : item))
    return connection
  }

  async function remove(id: string) {
    await api.deleteConnection(id)
    items.value = items.value.filter((item) => item.id !== id)
  }

  async function copy(connection: Connection) {
    const copied = await api.copyConnection(connection.id, `${connection.name} 副本`)
    items.value = [...items.value, copied].sort((a, b) => a.name.localeCompare(b.name))
    return copied
  }

  async function testSaved(id: string, purpose: TestPurpose): Promise<ConnectionTestResult> {
    const result = await api.testSavedConnection(id, purpose)
    await load(true)
    return result
  }

  return { items, loading, loaded, error, selectable, load, create, update, remove, copy, testSaved }
})
