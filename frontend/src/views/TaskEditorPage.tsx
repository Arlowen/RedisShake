import { Alert, App, Button, Checkbox, Collapse, Input, InputNumber, Radio, Segmented, Select, Spin, Switch } from 'antd'
import { ArrowLeft, CheckCircle, FloppyDisk, Play, Warning } from '@phosphor-icons/react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { api } from '@/api/client'
import type { Connection, PrecheckResult, Task, TaskMode, TaskSpec } from '@/api/types'
import CheckResultPanel from '@/components/CheckResultPanel'
import PageHeader from '@/components/PageHeader'
import { useConnections } from '@/state/ConnectionsContext'
import { useTasks } from '@/state/TasksContext'
import { defaultTaskSpec } from '@/utils/defaults'

type FilterMode = 'none' | 'allow' | 'block'

export default function TaskEditorPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const { message } = App.useApp()
  const tasks = useTasks()
  const connections = useConnections()
  const [initialTask, setInitialTask] = useState<Task>()
  const [loadingInitial, setLoadingInitial] = useState(Boolean(id))
  const [loadError, setLoadError] = useState<string>()
  const [task, setTask] = useState<Task>()
  const taskRef = useRef<Task | undefined>(undefined)
  const [spec, setSpec] = useState<TaskSpec>(defaultTaskSpec)
  const [saving, setSaving] = useState(false)
  const [prechecking, setPrechecking] = useState(false)
  const [starting, setStarting] = useState(false)
  const [precheckResult, setPrecheckResult] = useState<PrecheckResult>()
  const [acknowledgeWarnings, setAcknowledgeWarnings] = useState(false)
  const [filterMode, setFilterMode] = useState<FilterMode>('none')
  const [keyPrefixes, setKeyPrefixes] = useState('')
  const [keyRegex, setKeyRegex] = useState('')
  const [databaseIds, setDatabaseIds] = useState('')
  const [commands, setCommands] = useState('')
  const [commandGroups, setCommandGroups] = useState('')

  useEffect(() => {
    let active = true
    setLoadError(undefined)
    if (!id) {
      setInitialTask(undefined)
      setLoadingInitial(false)
      return () => { active = false }
    }
    setLoadingInitial(true)
    void api.getTask(id).then((loaded) => {
      if (active) setInitialTask(loaded)
    }).catch((cause) => {
      if (active) setLoadError(cause instanceof Error ? cause.message : '任务加载失败')
    }).finally(() => {
      if (active) setLoadingInitial(false)
    })
    return () => { active = false }
  }, [id])

  useEffect(() => {
    const initial = initialTask ? structuredClone(initialTask) : undefined
    const initialSpec = initial?.spec ?? defaultTaskSpec()
    setTask(initial); taskRef.current = initial; setSpec(initialSpec)
    setPrecheckResult(initial?.last_precheck_result); setAcknowledgeWarnings(false)
    const hasAllow = Boolean(initialSpec.filter.allow_key_prefix.length || initialSpec.filter.allow_key_regex.length || initialSpec.filter.allow_command.length || initialSpec.filter.allow_command_group.length)
    const hasBlock = Boolean(initialSpec.filter.block_key_prefix.length || initialSpec.filter.block_key_regex.length || initialSpec.filter.block_command.length || initialSpec.filter.block_command_group.length)
    setFilterMode(hasAllow ? 'allow' : hasBlock ? 'block' : 'none')
    setKeyPrefixes((hasAllow ? initialSpec.filter.allow_key_prefix : initialSpec.filter.block_key_prefix).join('\n'))
    setKeyRegex((hasAllow ? initialSpec.filter.allow_key_regex : initialSpec.filter.block_key_regex).join('\n'))
    setCommands((hasAllow ? initialSpec.filter.allow_command : initialSpec.filter.block_command).join('\n'))
    setCommandGroups((hasAllow ? initialSpec.filter.allow_command_group : initialSpec.filter.block_command_group).join('\n'))
    setDatabaseIds((initialSpec.mode === 'scan' ? initialSpec.scan_reader?.dbs : hasAllow ? initialSpec.filter.allow_db : initialSpec.filter.block_db)?.join(', ') ?? '')
    void connections.load().catch(() => undefined)
  }, [initialTask, connections.load])

  const materializedSpec = useCallback(() => {
    const next = structuredClone(spec)
    const lines = (value: string) => value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean)
    const numbers = (value: string) => lines(value).map(Number).filter((item) => Number.isInteger(item))
    next.filter.allow_key_prefix = filterMode === 'allow' ? lines(keyPrefixes) : []
    next.filter.block_key_prefix = filterMode === 'block' ? lines(keyPrefixes) : []
    next.filter.allow_key_regex = filterMode === 'allow' ? lines(keyRegex) : []
    next.filter.block_key_regex = filterMode === 'block' ? lines(keyRegex) : []
    next.filter.allow_command = filterMode === 'allow' ? lines(commands) : []
    next.filter.block_command = filterMode === 'block' ? lines(commands) : []
    next.filter.allow_command_group = filterMode === 'allow' ? lines(commandGroups) : []
    next.filter.block_command_group = filterMode === 'block' ? lines(commandGroups) : []
    if (next.mode === 'scan' && next.scan_reader) next.scan_reader.dbs = numbers(databaseIds)
    else { next.filter.allow_db = filterMode === 'allow' ? numbers(databaseIds) : []; next.filter.block_db = filterMode === 'block' ? numbers(databaseIds) : [] }
    return next
  }, [spec, filterMode, keyPrefixes, keyRegex, commands, commandGroups, databaseIds])

  const sourceConnection = connections.items.find((item) => item.id === spec.source_connection_id)
  const targetConnection = connections.items.find((item) => item.id === spec.target_connection_id)
  const hasWarnings = precheckResult?.checks.some((item) => item.state === 'WARNING') ?? false
  const savedSnapshot = task ? JSON.stringify(task.spec) : ''
  const currentSnapshot = JSON.stringify(materializedSpec())
  const canStart = precheckResult?.ready === true && Boolean(task) && currentSnapshot === savedSnapshot
  const connectionOptions = useMemo(() => connections.selectable.map(({ label, value }) => ({ label, value })), [connections.selectable])

  function validateName() { if (!spec.name.trim()) throw new Error('请输入任务名称') }
  function validateFull() {
    validateName()
    if (!spec.source_connection_id) throw new Error('请选择源端 Redis')
    if (!spec.target_connection_id) throw new Error('请选择目标 Redis')
    if (spec.source_connection_id === spec.target_connection_id) throw new Error('源端和目标端不能使用同一个连接')
  }

  async function ensureDraft() {
    validateName()
    if (taskRef.current) return taskRef.current
    const created = await tasks.createDraft({ name: spec.name, description: spec.description, mode: spec.mode })
    taskRef.current = created; setTask(created)
    return created
  }

  async function persistDraft() {
    const current = await ensureDraft()
    const nextSpec = materializedSpec()
    if (JSON.stringify(current.spec) === JSON.stringify(nextSpec)) return current
    const updated = await tasks.updateDraft(current, nextSpec)
    taskRef.current = updated; setTask(updated); setSpec(updated.spec); setPrecheckResult(undefined); setAcknowledgeWarnings(false)
    return updated
  }

  async function saveDraft() {
    setSaving(true)
    try { await persistDraft(); message.success('草稿已保存'); navigate('/tasks') }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '保存草稿失败') }
    finally { setSaving(false) }
  }

  async function runPrecheck() {
    setPrechecking(true)
    try {
      validateFull()
      const saved = await persistDraft()
      const result = await tasks.precheck(saved, acknowledgeWarnings)
      const refreshed = await api.getTask(saved.id)
      taskRef.current = refreshed; setTask(refreshed); setPrecheckResult(result); tasks.replace(refreshed)
      if (!id) navigate(`/tasks/${refreshed.id}/edit`, { replace: true })
      result.ready ? message.success('预检查通过，可以启动') : message.warning('请处理预检查结果')
    } catch (cause) { message.error(cause instanceof Error ? cause.message : '预检查失败') }
    finally { setPrechecking(false) }
  }

  async function start() {
    const current = taskRef.current
    if (!current || !canStart) return
    setStarting(true)
    try { const run = await tasks.start(current); message.success(run.state === 'SUCCEEDED' ? '扫描任务已完成' : '同步任务已启动'); navigate(`/tasks/${current.id}`) }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '任务启动失败') }
    finally { setStarting(false) }
  }

  function changeMode(mode: TaskMode) {
    setSpec((current) => mode === 'sync' ? { ...current, mode, sync_reader: current.sync_reader ?? { sync_rdb: true, sync_aof: true, prefer_replica: false, try_diskless: false }, scan_reader: undefined }
      : { ...current, mode, scan_reader: current.scan_reader ?? { dbs: [], scan: true, ksn: false, count: 1, prefer_replica: false, skip_unknown_type: [] }, sync_reader: undefined })
  }

  const advancedItems = [{ key: 'advanced', label: '高级设置', children: <div className="advanced-fields">
    <div className="form-grid two"><label><span>目标最大 QPS</span><InputNumber value={spec.advanced.target_redis_max_qps} min={1} max={300000} onChange={(value) => setSpec((current) => ({ ...current, advanced: { ...current.advanced, target_redis_max_qps: value ?? 1 } }))} /></label><label><span>Pipeline Count</span><InputNumber value={spec.advanced.pipeline_count_limit} min={1} onChange={(value) => setSpec((current) => ({ ...current, advanced: { ...current.advanced, pipeline_count_limit: value ?? 1 } }))} /></label></div>
    <label><span>目标 Key 已存在时</span><Select value={spec.advanced.rdb_restore_command_behavior} options={[{ value: 'panic', label: '停止任务（推荐）' }, { value: 'rewrite', label: '覆盖目标 Key' }, { value: 'skip', label: '跳过冲突 Key' }]} onChange={(value) => setSpec((current) => ({ ...current, advanced: { ...current.advanced, rdb_restore_command_behavior: value } }))} /></label>
    <div className="danger-setting"><div><Warning size={20} /><span><strong>启动前清空目标 Redis</strong><small>会执行 FLUSHALL，目标端数据不可恢复。</small></span></div><Switch checked={spec.advanced.empty_db_before_sync} onChange={(value) => setSpec((current) => ({ ...current, advanced: { ...current.advanced, empty_db_before_sync: value } }))} /></div>
    {spec.mode === 'sync' && spec.sync_reader ? <div className="form-grid two"><label className="switch-field"><span>同步 RDB</span><Switch checked={spec.sync_reader.sync_rdb} onChange={(value) => setSpec((current) => ({ ...current, sync_reader: current.sync_reader ? { ...current.sync_reader, sync_rdb: value } : current.sync_reader }))} /></label><label className="switch-field"><span>持续同步 AOF</span><Switch checked={spec.sync_reader.sync_aof} onChange={(value) => setSpec((current) => ({ ...current, sync_reader: current.sync_reader ? { ...current.sync_reader, sync_aof: value } : current.sync_reader }))} /></label></div> : null}
  </div> }]

  if (loadingInitial) return <div className="page-wrap task-editor-page"><div className="editor-loading"><Spin tip="正在加载任务" /></div></div>
  if (loadError) return <div className="page-wrap task-editor-page"><button type="button" className="back-link" onClick={() => navigate('/tasks')}><ArrowLeft size={14} />返回任务列表</button><PageHeader title="任务加载失败" description="无法打开这条同步任务。" /><Alert type="error" showIcon message={loadError} action={<Button onClick={() => navigate('/tasks')}>返回任务列表</Button>} /></div>

  return <div className="page-wrap task-editor-page">
    <button type="button" className="back-link" onClick={() => navigate('/tasks')}><ArrowLeft size={14} />返回任务列表</button>
    <PageHeader title={id ? '编辑同步任务' : '创建同步任务'} description="在一个页面中配置同步链路、数据范围和高级参数。">
      <Button onClick={() => navigate('/tasks')}>取消</Button>
      <Button loading={saving} icon={<FloppyDisk size={15} />} onClick={() => void saveDraft()}>保存草稿</Button>
      {canStart ? <Button type="primary" loading={starting} icon={<Play size={15} />} onClick={() => void start()}>启动任务</Button> : <Button type="primary" loading={prechecking} disabled={hasWarnings && !acknowledgeWarnings} onClick={() => void runPrecheck()}>执行预检查</Button>}
    </PageHeader>
    <div className="task-editor-surface">
      <div className="task-editor-form">
      <section className="form-section">
        <div className="form-section-title"><span>基本信息</span><small>定义名称和同步模式</small></div>
        <label><span>任务名称</span><Input value={spec.name} placeholder="例如：订单缓存迁移" onChange={(event) => setSpec((current) => ({ ...current, name: event.target.value }))} /></label>
        <label><span>任务描述</span><Input.TextArea value={spec.description} rows={2} placeholder="可选" onChange={(event) => setSpec((current) => ({ ...current, description: event.target.value }))} /></label>
        <label><span>同步模式</span><Radio.Group value={spec.mode} className="mode-picker" onChange={(event) => changeMode(event.target.value as TaskMode)}><Radio.Button value="sync"><strong>增量同步</strong><small>RDB + AOF 持续同步</small></Radio.Button><Radio.Button value="scan"><strong>扫描迁移</strong><small>SCAN 一次性迁移</small></Radio.Button></Radio.Group></label>
      </section>

      <section className="form-section">
        <div className="form-section-title"><span>同步链路</span><small>选择不同的源端和目标端</small></div>
        <div className="form-grid two"><label><span>源端连接</span><Select value={spec.source_connection_id} showSearch placeholder="选择源端" options={connectionOptions} optionFilterProp="label" onChange={(value) => setSpec((current) => ({ ...current, source_connection_id: value }))} /></label><label><span>目标连接</span><Select value={spec.target_connection_id} showSearch placeholder="选择目标端" options={connectionOptions} optionFilterProp="label" onChange={(value) => setSpec((current) => ({ ...current, target_connection_id: value }))} /></label></div>
        <div className="route-preview"><ConnectionValue title="源端" connection={sourceConnection} /><span>→</span><ConnectionValue title="目标端" connection={targetConnection} /></div>
        {spec.source_connection_id && spec.source_connection_id === spec.target_connection_id ? <Alert type="error" showIcon message="源端和目标端不能相同" /> : null}
        {!connections.items.length ? <Alert type="warning" showIcon message="请先在左侧“连接管理”中创建连接" /> : null}
      </section>

      <section className="form-section">
        <div className="form-section-title"><span>同步范围</span><small>默认同步全部数据</small></div>
        <label><span>过滤策略</span><Segmented value={filterMode} options={[{ label: '不过滤', value: 'none' }, { label: '仅允许', value: 'allow' }, { label: '排除', value: 'block' }]} onChange={(value) => setFilterMode(value as FilterMode)} /></label>
        {filterMode !== 'none' ? <><div className="form-grid two"><label><span>Key 前缀</span><Input.TextArea value={keyPrefixes} rows={3} placeholder={'cache:\nsession:'} onChange={(event) => setKeyPrefixes(event.target.value)} /></label><label><span>Key 正则</span><Input.TextArea value={keyRegex} rows={3} placeholder="^order:\\d+$" onChange={(event) => setKeyRegex(event.target.value)} /></label></div><div className="form-grid two"><label><span>命令</span><Input.TextArea value={commands} rows={2} placeholder={'SET\nHSET'} onChange={(event) => setCommands(event.target.value)} /></label><label><span>命令组</span><Input.TextArea value={commandGroups} rows={2} placeholder={'SCRIPTING\nPUBSUB'} onChange={(event) => setCommandGroups(event.target.value)} /></label></div></> : null}
        <label><span>{spec.mode === 'scan' ? '扫描 DB' : 'DB 过滤'}</span><Input value={databaseIds} placeholder="例如：0, 1, 5；留空表示全部" onChange={(event) => setDatabaseIds(event.target.value)} /></label>
        {spec.mode === 'scan' && spec.scan_reader ? <div className="form-grid two"><label><span>SCAN Count</span><InputNumber value={spec.scan_reader.count} min={1} max={10000} onChange={(value) => setSpec((current) => ({ ...current, scan_reader: current.scan_reader ? { ...current.scan_reader, count: value ?? 1 } : current.scan_reader }))} /></label><label className="switch-field"><span>Keyspace Notification</span><Switch checked={spec.scan_reader.ksn} onChange={(value) => setSpec((current) => ({ ...current, scan_reader: current.scan_reader ? { ...current.scan_reader, ksn: value } : current.scan_reader }))} /></label></div> : null}
      </section>

      <Collapse ghost items={advancedItems} />
      {precheckResult ? <CheckResultPanel checks={precheckResult.checks} title="预检查结果" /> : null}
      {hasWarnings ? <div className="warning-ack"><Checkbox checked={acknowledgeWarnings} onChange={(event) => setAcknowledgeWarnings(event.target.checked)}>我已确认上述危险警告</Checkbox><Button loading={prechecking} onClick={() => void runPrecheck()}>重新检查</Button></div> : null}
      {precheckResult?.config_digest ? <div className="digest-line"><CheckCircle size={18} weight="fill" /><span>配置摘要</span><code>{precheckResult.config_digest}</code></div> : null}
      </div>
    </div>
  </div>
}

function ConnectionValue({ title, connection }: { title: string; connection?: Connection }) {
  return <div><small>{title}</small><strong>{connection?.name ?? '未选择'}</strong><span className="mono">{connection?.address || connection?.sentinel.address || '—'}</span></div>
}
