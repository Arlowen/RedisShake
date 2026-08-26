import { App, Button, Dropdown, Input, Modal, Segmented, Select } from 'antd'
import { Archive, ArrowRight, ArrowsClockwise, Copy, DotsThree, MagnifyingGlass, Play } from '@phosphor-icons/react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { useNavigate } from 'react-router-dom'

import { api } from '@/api/client'
import type { Run, Task, TaskState } from '@/api/types'
import EmptyState from '@/components/EmptyState'
import InlineError from '@/components/InlineError'
import PageHeader from '@/components/PageHeader'
import StatusPill from '@/components/StatusPill'
import { useConnections } from '@/state/ConnectionsContext'
import { useTasks } from '@/state/TasksContext'
import { formatDate, formatNumber, modeLabel, runStateMeta, taskStateMeta } from '@/utils/presentation'

type StateFilter = 'all' | TaskState
type SortOrder = 'updated' | 'name' | 'state'

export default function TasksView() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const tasks = useTasks()
  const connections = useConnections()
  const [query, setQuery] = useState('')
  const [stateFilter, setStateFilter] = useState<StateFilter>('all')
  const [sortOrder, setSortOrder] = useState<SortOrder>('updated')
  const [latestRuns, setLatestRuns] = useState<Record<string, Run | undefined>>({})
  const [startingId, setStartingId] = useState<string>()

  const load = useCallback(async () => {
    try {
      await Promise.all([tasks.load(), connections.load()])
      const loadedTasks = await api.listTasks()
      const pairs = await Promise.all(loadedTasks.map(async (task) => [task.id, (await api.listRuns(task.id))[0]] as const))
      setLatestRuns(Object.fromEntries(pairs))
    } catch { /* context errors render inline */ }
  }, [tasks.load, connections.load])

  useEffect(() => { void load() }, [load])

  const filtered = useMemo(() => tasks.items.filter((task) => {
    const matchesQuery = !query || task.spec.name.toLowerCase().includes(query.toLowerCase())
    return matchesQuery && (stateFilter === 'all' || task.state === stateFilter)
  }).sort((a, b) => sortOrder === 'name' ? a.spec.name.localeCompare(b.spec.name) : sortOrder === 'state' ? a.state.localeCompare(b.state) : b.updated_at.localeCompare(a.updated_at)), [tasks.items, query, stateFilter, sortOrder])
  const runningCount = Object.values(latestRuns).filter((run) => run && ['STARTING', 'RUNNING', 'STOPPING'].includes(run.state)).length
  const readyCount = tasks.items.filter((task) => task.state === 'READY').length
  const totalWritten = Object.values(latestRuns).reduce((total, run) => total + (run?.status?.total_entries_count?.write_count ?? 0), 0)

  function connectionName(id?: string) { return id ? connections.items.find((item) => item.id === id)?.name ?? '连接已删除' : '未选择' }
  function create() { navigate('/tasks/new') }
  function edit(task: Task) { navigate(`/tasks/${task.id}/edit`) }

  async function start(task: Task) {
    setStartingId(task.id)
    try { const run = await tasks.start(task); setLatestRuns((current) => ({ ...current, [task.id]: run })); message.success(run.state === 'SUCCEEDED' ? '任务已完成' : '任务已启动'); navigate(`/tasks/${task.id}`) }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '启动失败') }
    finally { setStartingId(undefined) }
  }

  function archive(task: Task) {
    Modal.confirm({ title: `归档“${task.spec.name}”？`, content: '历史运行仍会保留；存在活动运行时无法归档。', okText: '归档任务', cancelText: '取消', async onOk() { await tasks.archive(task); message.success('任务已归档') } })
  }

  async function copy(task: Task) {
    try { await tasks.copy(task); message.success('任务副本已创建，需重新预检查') }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '复制失败') }
  }

  return <div className="page-wrap">
    <PageHeader title="同步任务" description="创建、预检并运行 Redis 数据同步任务。">
      <Button type="primary" onClick={create}>创建任务</Button>
    </PageHeader>
    {tasks.error ? <InlineError className="task-error" message={tasks.error} onRetry={() => void load()} /> : null}
    {tasks.loading && !tasks.items.length ? <div className="skeleton-list">{[0, 1, 2, 3, 4].map((item) => <div key={item} className="skeleton-row" />)}</div>
      : !tasks.items.length ? null
        : <>
          <div className="compact-summary"><span><strong>{formatNumber(tasks.items.length)}</strong>任务</span><span><strong>{formatNumber(runningCount)}</strong>运行中</span><span><strong>{formatNumber(readyCount)}</strong>可启动</span><span><strong>{formatNumber(totalWritten)}</strong>已写入</span></div>
          <div className="toolbar">
            <div className="toolbar-left search-box"><MagnifyingGlass size={16} /><Input value={query} variant="borderless" placeholder="搜索任务" onChange={(event) => setQuery(event.target.value)} /></div>
            <div className="toolbar-right"><Segmented size="small" value={stateFilter} options={[{ label: '全部', value: 'all' }, { label: '草稿', value: 'DRAFT' }, { label: '可启动', value: 'READY' }]} onChange={(value) => setStateFilter(value as StateFilter)} /><Select size="small" value={sortOrder} style={{ width: 112 }} options={[{ label: '最近更新', value: 'updated' }, { label: '任务名称', value: 'name' }, { label: '任务状态', value: 'state' }]} onChange={(value) => setSortOrder(value)} /><Button type="text" aria-label="刷新任务" loading={tasks.loading} icon={<ArrowsClockwise size={16} />} onClick={() => void load()} /></div>
          </div>
          {filtered.length ? <div className="data-surface task-table">{filtered.map((task, index) => {
          const latest = latestRuns[task.id]
          return <div key={task.id} className="data-row task-row" style={{ '--row-index': index } as CSSProperties} onDoubleClick={() => navigate(`/tasks/${task.id}`)}>
            <div className="task-identity"><strong>{task.spec.name}</strong><small>{modeLabel[task.spec.mode]} · revision {task.config_revision}</small></div>
            <div className="route-cell"><span>{connectionName(task.spec.source_connection_id)}</span><ArrowRight size={14} /><span>{connectionName(task.spec.target_connection_id)}</span></div>
            <div className="task-statuses"><StatusPill label={taskStateMeta[task.state].label} tone={taskStateMeta[task.state].tone} />{latest ? <StatusPill label={runStateMeta[latest.state].label} tone={runStateMeta[latest.state].tone} pulse={latest.state === 'RUNNING'} /> : <span className="muted">未运行</span>}</div>
            <div className="row-meta">{formatDate(task.updated_at)}</div>
            <div className="row-actions">
              {task.state === 'DRAFT' ? <Button size="small" onClick={() => edit(task)}>继续配置</Button> : <Button size="small" loading={startingId === task.id} disabled={latest?.state === 'RUNNING'} icon={<Play size={14} weight="fill" />} onClick={() => void start(task)}>启动</Button>}
              <Dropdown menu={{ items: [
                { key: 'view', label: '查看详情', onClick: () => navigate(`/tasks/${task.id}`) },
                { key: 'edit', label: '编辑配置', onClick: () => edit(task) },
                { key: 'copy', label: '复制任务', icon: <Copy size={15} />, onClick: () => void copy(task) },
                { key: 'archive', label: '归档', icon: <Archive size={15} />, danger: true, onClick: () => archive(task) },
              ] }}><Button type="text" size="small" aria-label={`${task.spec.name} 更多操作`} icon={<DotsThree size={18} />} /></Dropdown>
            </div>
          </div>
        })}</div> : <EmptyState title="没有匹配的任务" description="调整搜索或状态筛选。" />}
        </>}
  </div>
}
