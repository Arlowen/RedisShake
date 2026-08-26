import { App, Button, Dropdown, Input, Modal, Segmented, Select } from 'antd'
import { Archive, ArrowRight, ArrowsClockwise, Copy, DotsThree, MagnifyingGlass, Play, Plus } from '@phosphor-icons/react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { useNavigate } from 'react-router-dom'

import { api } from '@/api/client'
import type { Run, Task, TaskState } from '@/api/types'
import EmptyState from '@/components/EmptyState'
import InlineError from '@/components/InlineError'
import PageHeader from '@/components/PageHeader'
import StatusPill from '@/components/StatusPill'
import TaskWizard from '@/components/TaskWizard'
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
  const [wizardOpen, setWizardOpen] = useState(false)
  const [editingTask, setEditingTask] = useState<Task>()
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
  function create() { setEditingTask(undefined); setWizardOpen(true) }
  function edit(task: Task) { setEditingTask(task); setWizardOpen(true) }
  function completed(task: Task, run?: Run) { setWizardOpen(false); tasks.replace(task); if (run) setLatestRuns((current) => ({ ...current, [task.id]: run })); navigate(`/tasks/${task.id}`) }

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
    <PageHeader eyebrow="Migration control" title="同步任务" description="用向导生成 RedisShake 配置，先验证连接和危险参数，再启动独立 worker 并持续读取真实状态。">
      <Button type="primary" icon={<Plus size={17} />} onClick={create}>创建同步任务</Button>
    </PageHeader>
    <div className="metric-strip task-metrics">
      <div className="metric"><label>任务总数</label><strong className="mono">{formatNumber(tasks.items.length)}</strong></div>
      <div className="metric"><label>活动运行</label><strong className="mono">{formatNumber(runningCount)}</strong></div>
      <div className="metric"><label>可启动</label><strong className="mono">{formatNumber(readyCount)}</strong></div>
      <div className="metric"><label>累计写入</label><strong className="mono">{formatNumber(totalWritten)}</strong></div>
    </div>
    {tasks.error ? <InlineError className="task-error" message={tasks.error} onRetry={() => void load()} /> : null}
    <div className="toolbar">
      <div className="toolbar-left search-box"><MagnifyingGlass size={18} /><Input value={query} variant="borderless" placeholder="搜索任务名称" onChange={(event) => setQuery(event.target.value)} /></div>
      <div className="toolbar-right"><Segmented value={stateFilter} options={[{ label: '全部', value: 'all' }, { label: '草稿', value: 'DRAFT' }, { label: '可启动', value: 'READY' }]} onChange={(value) => setStateFilter(value as StateFilter)} /><Select value={sortOrder} style={{ width: 124 }} options={[{ label: '最近更新', value: 'updated' }, { label: '任务名称', value: 'name' }, { label: '任务状态', value: 'state' }]} onChange={(value) => setSortOrder(value)} /><Button type="text" aria-label="刷新任务" loading={tasks.loading} icon={<ArrowsClockwise size={17} />} onClick={() => void load()} /></div>
    </div>
    {tasks.loading && !tasks.items.length ? <div className="skeleton-list">{[0, 1, 2, 3, 4].map((item) => <div key={item} className="skeleton-row" />)}</div>
      : !tasks.items.length ? <EmptyState title="创建第一条同步链路" description="准备好源端与目标端 Redis 连接后，通过六步向导生成配置、执行预检查并启动 RedisShake。"><Button type="primary" onClick={create}>创建同步任务</Button></EmptyState>
        : filtered.length ? <div className="data-surface task-table">{filtered.map((task, index) => {
          const latest = latestRuns[task.id]
          return <div key={task.id} className="data-row task-row" style={{ '--row-index': index } as CSSProperties} onDoubleClick={() => navigate(`/tasks/${task.id}`)}>
            <div className="task-identity"><span className="mode-code">{task.spec.mode === 'sync' ? 'SY' : 'SC'}</span><div><strong>{task.spec.name}</strong><small>{modeLabel[task.spec.mode]} · revision {task.config_revision}</small></div></div>
            <div className="route-cell"><span>{connectionName(task.spec.source_connection_id)}</span><ArrowRight size={14} /><span>{connectionName(task.spec.target_connection_id)}</span></div>
            <div><small>任务状态</small><StatusPill label={taskStateMeta[task.state].label} tone={taskStateMeta[task.state].tone} /></div>
            <div><small>最新运行</small>{latest ? <StatusPill label={runStateMeta[latest.state].label} tone={runStateMeta[latest.state].tone} pulse={latest.state === 'RUNNING'} /> : <span className="muted">尚未运行</span>}</div>
            <div><small>更新时间</small><strong>{formatDate(task.updated_at)}</strong></div>
            <div className="row-actions">
              {task.state === 'DRAFT' ? <Button onClick={() => edit(task)}>继续配置</Button> : <Button type="primary" ghost loading={startingId === task.id} disabled={latest?.state === 'RUNNING'} icon={<Play size={15} weight="fill" />} onClick={() => void start(task)}>启动</Button>}
              <Dropdown menu={{ items: [
                { key: 'view', label: '查看详情', onClick: () => navigate(`/tasks/${task.id}`) },
                { key: 'edit', label: '编辑配置', onClick: () => edit(task) },
                { key: 'copy', label: '复制任务', icon: <Copy size={15} />, onClick: () => void copy(task) },
                { key: 'archive', label: '归档', icon: <Archive size={15} />, danger: true, onClick: () => archive(task) },
              ] }}><Button type="text" aria-label={`${task.spec.name} 更多操作`} icon={<DotsThree size={20} />} /></Dropdown>
            </div>
          </div>
        })}</div> : <EmptyState title="没有匹配的任务" description="调整搜索词或状态筛选后再试。" />}
    <TaskWizard open={wizardOpen} initialTask={editingTask} onClose={() => setWizardOpen(false)} onCompleted={completed} />
  </div>
}
