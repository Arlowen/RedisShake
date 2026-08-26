import { Alert, App, Button, Input, Modal, Select, Tabs } from 'antd'
import { ArrowLeft, ArrowsClockwise, DownloadSimple, FileText, Pause, Play, Stop, Warning } from '@phosphor-icons/react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { api } from '@/api/client'
import type { Run, Task } from '@/api/types'
import InlineError from '@/components/InlineError'
import PageHeader from '@/components/PageHeader'
import StatusPill from '@/components/StatusPill'
import { formatDate, formatNumber, modeLabel, runStateMeta, taskStateMeta } from '@/utils/presentation'

type LogLevel = 'all' | 'info' | 'warn' | 'error'
const isActive = (run?: Run) => Boolean(run && ['STARTING', 'RUNNING', 'STOPPING'].includes(run.state))

export default function TaskDetailView() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [task, setTask] = useState<Task>()
  const [runs, setRuns] = useState<Run[]>([])
  const [selectedRunId, setSelectedRunId] = useState<string>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string>()
  const [activeTab, setActiveTab] = useState('overview')
  const [actionLoading, setActionLoading] = useState(false)
  const [logs, setLogs] = useState('')
  const [logPaused, setLogPaused] = useState(false)
  const [logQuery, setLogQuery] = useState('')
  const [logLevel, setLogLevel] = useState<LogLevel>('all')
  const logOffset = useRef(0)
  const logsLoading = useRef(false)
  const logPausedRef = useRef(false)

  const selectedRun = runs.find((run) => run.id === selectedRunId) ?? runs[0]
  const metrics = selectedRun?.status?.total_entries_count
  const filteredLogs = useMemo(() => logs.split('\n').filter((line) => {
    const normalized = line.toLowerCase()
    const matchesQuery = !logQuery || normalized.includes(logQuery.toLowerCase())
    const matchesLevel = logLevel === 'all' || (logLevel === 'info' && /\b(inf|info)\b/i.test(line)) || (logLevel === 'warn' && /\b(wrn|warn|warning)\b/i.test(line)) || (logLevel === 'error' && /\b(err|error|panic|fatal)\b/i.test(line))
    return matchesQuery && matchesLevel
  }).join('\n'), [logs, logQuery, logLevel])

  useEffect(() => { logPausedRef.current = logPaused }, [logPaused])

  const load = useCallback(async () => {
    setLoading(true); setError(undefined)
    try {
      const [loadedTask, loadedRuns] = await Promise.all([api.getTask(id), api.listRuns(id)])
      setTask(loadedTask); setRuns(loadedRuns)
      setSelectedRunId((current) => current ?? loadedRuns[0]?.id)
    } catch (cause) { setError(cause instanceof Error ? cause.message : '任务详情加载失败') }
    finally { setLoading(false) }
  }, [id])

  useEffect(() => { void load() }, [load])

  const loadMoreLogs = useCallback(async (runId: string) => {
    if (logsLoading.current) return
    logsLoading.current = true
    try {
      const result = await api.readLogs(runId, logOffset.current, 131072)
      if (result.content) setLogs((current) => current + stripAnsi(result.content))
      logOffset.current = result.next_offset
    } catch { /* status remains available when logs lag */ }
    finally { logsLoading.current = false }
  }, [])

  useEffect(() => {
    if (!selectedRunId) return
    setLogs(''); logOffset.current = 0
    const run = runs.find((item) => item.id === selectedRunId)
    if (!run) return
    void loadMoreLogs(run.id)
    if (!isActive(run)) return
    const source = new EventSource(`/api/v1/runs/${run.id}/events`)
    const timer = setInterval(() => { if (!logPausedRef.current) void loadMoreLogs(run.id) }, 1600)
    source.addEventListener('status', (event) => {
      const updated = JSON.parse((event as MessageEvent).data) as Run
      setRuns((current) => current.map((item) => item.id === updated.id ? updated : item))
      if (!isActive(updated)) { source.close(); clearInterval(timer) }
    })
    return () => { source.close(); if (timer) clearInterval(timer) }
  }, [selectedRunId, loadMoreLogs])

  function selectRun(runId: string) { if (runId !== selectedRunId) setSelectedRunId(runId) }
  function downloadLogs() {
    if (!logs) return
    const url = URL.createObjectURL(new Blob([logs], { type: 'text/plain;charset=utf-8' }))
    const link = document.createElement('a'); link.href = url; link.download = `${task?.spec.name ?? 'redisshake'}-${selectedRun?.id.slice(0, 10) ?? 'run'}.log`; link.click(); URL.revokeObjectURL(url)
  }

  async function start() {
    if (!task) return
    setActionLoading(true)
    try { const run = await api.startRun(task.id, task.config_revision); setRuns((current) => [run, ...current]); setSelectedRunId(run.id); message.success(run.state === 'SUCCEEDED' ? '任务已完成' : '任务已启动') }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '启动失败') }
    finally { setActionLoading(false) }
  }

  async function requestStop(force = false) {
    if (!selectedRun) return
    setActionLoading(true)
    try { const updated = force ? await api.forceStopRun(selectedRun.id) : await api.stopRun(selectedRun.id); setRuns((current) => current.map((item) => item.id === updated.id ? updated : item)); message.success(force ? '已发送强制停止信号' : '正在优雅停止') }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '停止失败') }
    finally { setActionLoading(false) }
  }

  function confirmStop(force = false) {
    Modal.confirm({ title: force ? '强制终止 RedisShake？' : '停止当前同步？', content: force ? '进程会被立即终止，尚未刷写的队列可能丢失。' : '控制面会先发送 SIGTERM，等待 RedisShake 关闭 reader、刷写 writer 并释放文件锁。', okText: force ? '强制终止' : '优雅停止', okType: force ? 'danger' : 'primary', cancelText: '取消', onOk: () => requestStop(force) })
  }

  const tabItems = task ? [
    { key: 'overview', label: '运行概览', children: <div className="overview-grid"><section className="overview-main"><div className="section-heading"><h2>RedisShake 状态</h2><span>来自 worker 回环状态端口</span></div><div className="status-json-grid"><div><small>阶段</small><strong>{String(selectedRun?.status?.reader?.status ?? selectedRun?.state ?? '—')}</strong></div><div><small>内部一致</small><strong>{selectedRun?.status?.consistent === undefined ? '—' : selectedRun.status.consistent ? '是' : '否'}</strong></div><div><small>Writer 未响应</small><strong className="mono">{formatNumber(Number(selectedRun?.status?.writer?.unanswered_entries ?? 0))}</strong></div><div><small>最后心跳</small><strong>{formatDate(selectedRun?.last_heartbeat_at)}</strong></div></div><div className="section-heading"><h2>Reader / Writer 原始状态</h2><span>便于问题排查</span></div><pre className="json-view">{JSON.stringify({ reader: selectedRun?.status?.reader, writer: selectedRun?.status?.writer }, null, 2)}</pre></section><aside className="run-selector"><div className="section-heading"><h2>运行记录</h2><span>{runs.length}</span></div>{runs.map((run) => <button key={run.id} type="button" className={run.id === selectedRun?.id ? 'selected' : ''} onClick={() => selectRun(run.id)}><span><StatusPill label={runStateMeta[run.state].label} tone={runStateMeta[run.state].tone} /></span><strong>{formatDate(run.started_at)}</strong><small className="mono">{run.id.slice(0, 10)}</small></button>)}</aside></div> },
    { key: 'logs', label: '运行日志', children: <><div className="log-toolbar"><span><FileText size={17} />stdout / stderr（已脱敏）</span><div className="log-actions"><Input value={logQuery} allowClear size="small" placeholder="搜索日志" onChange={(event) => setLogQuery(event.target.value)} /><Select value={logLevel} size="small" options={[{ label: '全部级别', value: 'all' }, { label: 'Info', value: 'info' }, { label: 'Warn', value: 'warn' }, { label: 'Error', value: 'error' }]} onChange={(value) => setLogLevel(value)} /><Button size="small" icon={logPaused ? <Play size={15} /> : <Pause size={15} />} onClick={() => setLogPaused((value) => !value)}>{logPaused ? '继续' : '暂停'}</Button><Button size="small" icon={<DownloadSimple size={15} />} onClick={downloadLogs}>下载</Button></div></div><pre className="log-view">{filteredLogs || (logs ? '没有匹配的日志' : '等待日志输出…')}</pre></> },
    { key: 'history', label: '运行历史', children: <div className="data-surface history-table">{runs.map((run) => <div key={run.id} className="data-row history-row"><StatusPill label={runStateMeta[run.state].label} tone={runStateMeta[run.state].tone} /><span className="mono">{run.id}</span><span>{formatDate(run.started_at)}</span><span>{run.exit_reason || '—'}</span></div>)}</div> },
    { key: 'config', label: '配置快照', children: <><div className="config-note"><Warning size={18} /><span>快照不包含 Redis 密码或 TLS PEM；运行时 TOML 保存在受保护目录。</span></div><pre className="json-view">{JSON.stringify(selectedRun?.config_snapshot ?? task.spec, null, 2)}</pre></> },
  ] : []

  return <div className="page-wrap detail-page">
    <button className="back-link" type="button" onClick={() => navigate('/tasks')}><ArrowLeft size={16} />返回任务列表</button>
    {loading ? <div className="skeleton-list">{[0, 1, 2, 3, 4].map((item) => <div key={item} className="skeleton-row" />)}</div> : error ? <InlineError message={error} onRetry={() => void load()} /> : task ? <>
      <PageHeader title={task.spec.name} description={`${modeLabel[task.spec.mode]} · revision ${task.config_revision} · ${formatDate(task.updated_at)}`}>
        <Button type="text" icon={<ArrowsClockwise size={16} />} onClick={() => void load()}>刷新</Button>
        {task.state === 'READY' && !isActive(selectedRun) ? <Button type="primary" loading={actionLoading} icon={<Play size={17} weight="fill" />} onClick={() => void start()}>启动</Button> : null}
        {isActive(selectedRun) ? <Button danger loading={actionLoading} icon={<Stop size={17} weight="fill" />} onClick={() => confirmStop(false)}>停止</Button> : null}
        {selectedRun?.state === 'STOPPING' ? <Button danger type="primary" onClick={() => confirmStop(true)}>强制终止</Button> : null}
      </PageHeader>
      <div className="detail-summary">
        <div className="detail-statuses"><StatusPill label={taskStateMeta[task.state].label} tone={taskStateMeta[task.state].tone} />{selectedRun ? <><StatusPill label={runStateMeta[selectedRun.state].label} tone={runStateMeta[selectedRun.state].tone} pulse={selectedRun.state === 'RUNNING'} /><span className="mono">{selectedRun.id.slice(0, 12)}</span></> : <span className="muted">尚未运行</span>}</div>
        <div><small>心跳</small><strong>{selectedRun?.status_healthy ? '正常' : selectedRun ? '不可用' : '—'}</strong></div>
        <div><small>读取</small><strong className="mono">{formatNumber(metrics?.read_count)}</strong></div>
        <div><small>写入</small><strong className="mono">{formatNumber(metrics?.write_count)}</strong></div>
        <div><small>OPS</small><strong className="mono">{formatNumber(metrics?.write_ops)}</strong></div>
      </div>
      {selectedRun?.state === 'UNKNOWN' ? <Alert type="warning" showIcon message="运行归属无法确认" description="控制面重启后不会向这个 PID 发送信号，也不会允许同任务重复启动。请在主机上核对进程后处理。" /> : null}
      <Tabs activeKey={activeTab} className="detail-tabs" items={tabItems} onChange={setActiveTab} />
    </> : null}
  </div>
}

function stripAnsi(value: string) { return value.replace(/\u001b\[[0-9;]*m/g, '') }
