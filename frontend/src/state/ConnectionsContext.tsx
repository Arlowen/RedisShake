import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'

import { api } from '@/api/client'
import type { Connection, ConnectionInput, ConnectionTestResult, TestPurpose } from '@/api/types'

interface ConnectionsState {
  items: Connection[]
  loading: boolean
  loaded: boolean
  error?: string
  selectable: Array<{ label: string; value: string; connection: Connection }>
  load: (force?: boolean) => Promise<void>
  create: (input: ConnectionInput) => Promise<Connection>
  update: (id: string, patch: Partial<ConnectionInput>) => Promise<Connection>
  remove: (id: string) => Promise<void>
  copy: (connection: Connection) => Promise<Connection>
  testSaved: (id: string, purpose: TestPurpose) => Promise<ConnectionTestResult>
  upsert: (connection: Connection) => void
}

const ConnectionsContext = createContext<ConnectionsState | undefined>(undefined)

export function ConnectionsProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Connection[]>([])
  const [loading, setLoading] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const loadedRef = useRef(false)
  const [error, setError] = useState<string>()

  const load = useCallback(async (force = false) => {
    if (loadedRef.current && !force) return
    setLoading(true); setError(undefined)
    try {
      setItems(await api.listConnections())
      loadedRef.current = true; setLoaded(true)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '连接列表加载失败')
      throw cause
    } finally { setLoading(false) }
  }, [])

  const create = useCallback(async (input: ConnectionInput) => {
    const connection = await api.createConnection(input)
    setItems((current) => [...current, connection].sort((a, b) => a.name.localeCompare(b.name)))
    return connection
  }, [])

  const update = useCallback(async (id: string, patch: Partial<ConnectionInput>) => {
    const connection = await api.updateConnection(id, patch)
    setItems((current) => current.map((item) => item.id === id ? connection : item))
    return connection
  }, [])

  const remove = useCallback(async (id: string) => {
    await api.deleteConnection(id)
    setItems((current) => current.filter((item) => item.id !== id))
  }, [])

  const copy = useCallback(async (connection: Connection) => {
    const copied = await api.copyConnection(connection.id, `${connection.name} 副本`)
    setItems((current) => [...current, copied].sort((a, b) => a.name.localeCompare(b.name)))
    return copied
  }, [])

  const testSaved = useCallback(async (id: string, purpose: TestPurpose) => {
    const result = await api.testSavedConnection(id, purpose)
    const refreshed = await api.listConnections()
    setItems(refreshed); loadedRef.current = true; setLoaded(true)
    return result
  }, [])

  const upsert = useCallback((connection: Connection) => {
    setItems((current) => current.some((item) => item.id === connection.id)
      ? current.map((item) => item.id === connection.id ? connection : item)
      : [...current, connection].sort((a, b) => a.name.localeCompare(b.name)))
  }, [])

  const selectable = useMemo(() => items.map((item) => ({ label: item.name, value: item.id, connection: item })), [items])
  const value = useMemo<ConnectionsState>(() => ({ items, loading, loaded, error, selectable, load, create, update, remove, copy, testSaved, upsert }), [items, loading, loaded, error, selectable, load, create, update, remove, copy, testSaved, upsert])
  return <ConnectionsContext.Provider value={value}>{children}</ConnectionsContext.Provider>
}

export function useConnections() {
  const context = useContext(ConnectionsContext)
  if (!context) throw new Error('useConnections must be used inside ConnectionsProvider')
  return context
}
