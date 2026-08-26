import { Alert, App, Button, Checkbox, Drawer, Input, InputNumber, Radio, Segmented, Select, Steps, Switch } from 'antd'
import { ArrowLeft, ArrowRight, CheckCircle, FloppyDisk, Play, Warning } from '@phosphor-icons/react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { api } from '@/api/client'
import type { PrecheckResult, Run, Task, TaskMode, TaskSpec } from '@/api/types'
import CheckResultPanel from '@/components/CheckResultPanel'
import { useConnections } from '@/state/ConnectionsContext'
import { useTasks } from '@/state/TasksContext'
import { defaultTaskSpec } from '@/utils/defaults'

interface TaskWizardProps {
  open: boolean
  initialTask?: Task
  onClose: () => void
  onCompleted: (task: Task, run?: Run) => void
}

type SaveState = 'idle' | 'saving' | 'saved' | 'error'
type FilterMode = 'none' | 'allow' | 'block'

const steps = [{ title: '基本信息' }, { title: '源端 Redis' }, { title: '目标端 Redis' }, { title: '同步范围' }, { title: '高级配置' }, { title: '预检查' }]

export default function TaskWizard({ open, initialTask, onClose, onCompleted }: TaskWizardProps) {
  const { message } = App.useApp()
  const tasks = useTasks()
  const connections = useConnections()
  const [current, setCurrent] = useState(0)
  const [task, setTask] = useState<Task>()
  const taskRef = useRef<Task | undefined>(undefined)
  const [spec, setSpec] = useState<TaskSpec>(defaultTaskSpec)
  const [saveState, setSaveState] = useState<SaveState>('idle')
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
  const saveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const saveChain = useRef<Promise<void>>(Promise.resolve())
  const lastSavedSnapshot = useRef('')
  const applyingServer = useRef(false)

  const sourceConnection = connections.items.find((item) => item.id === spec.source_connection_id)
  const targetConnection = connections.items.find((item) => item.id === spec.target_connection_id)
  const hasWarnings = precheckResult?.checks.some((item) => item.state === 'WARNING') ?? false
  const canStart = precheckResult?.ready === true

  useEffect(() => {
    if (!open) return
    applyingServer.current = true
    const initial = initialTask ? structuredClone(initialTask) : undefined
    const initialSpec = initial?.spec ?? defaultTaskSpec()
    setSpec(initialSpec); setTask(initial); taskRef.current = initial
    lastSavedSnapshot.current = initial ? JSON.stringify(initial.spec) : ''
    setCurrent(0); setSaveState(initial ? 'saved' : 'idle'); setPrecheckResult(initial?.last_precheck_result); setAcknowledgeWarnings(false)
    const hasAllow = Boolean(initialSpec.filter.allow_key_prefix.length || initialSpec.filter.allow_key_regex.length || initialSpec.filter.allow_command.length || initialSpec.filter.allow_command_group.length)
    const hasBlock = Boolean(initialSpec.filter.block_key_prefix.length || initialSpec.filter.block_key_regex.length || initialSpec.filter.block_command.length || initialSpec.filter.block_command_group.length)
    const nextFilterMode: FilterMode = hasAllow ? 'allow' : hasBlock ? 'block' : 'none'
    setFilterMode(nextFilterMode)
    setKeyPrefixes((hasAllow ? initialSpec.filter.allow_key_prefix : initialSpec.filter.block_key_prefix).join('\n'))
    setKeyRegex((hasAllow ? initialSpec.filter.allow_key_regex : initialSpec.filter.block_key_regex).join('\n'))
    setCommands((hasAllow ? initialSpec.filter.allow_command : initialSpec.filter.block_command).join('\n'))
    setCommandGroups((hasAllow ? initialSpec.filter.allow_command_group : initialSpec.filter.block_command_group).join('\n'))
    setDatabaseIds((initialSpec.mode === 'scan' ? initialSpec.scan_reader?.dbs : hasAllow ? initialSpec.filter.allow_db : initialSpec.filter.block_db)?.join(', ') ?? '')
    void connections.load().catch(() => undefined).finally(() => { applyingServer.current = false })
    return () => { if (saveTimer.current) clearTimeout(saveTimer.current) }
  }, [open, initialTask, connections.load])

  const materializedSpec = useCallback(() => {
    const next = structuredClone(spec)
    const splitLines = (value: string) => value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean)
    const numbers = (value: string) => splitLines(value).map(Number).filter((item) => Number.isInteger(item))
    next.filter.allow_key_prefix = filterMode === 'allow' ? splitLines(keyPrefixes) : []
    next.filter.block_key_prefix = filterMode === 'block' ? splitLines(keyPrefixes) : []
    next.filter.allow_key_regex = filterMode === 'allow' ? splitLines(keyRegex) : []
    next.filter.block_key_regex = filterMode === 'block' ? splitLines(keyRegex) : []
    next.filter.allow_command = filterMode === 'allow' ? splitLines(commands) : []
    next.filter.block_command = filterMode === 'block' ? splitLines(commands) : []
    next.filter.allow_command_group = filterMode === 'allow' ? splitLines(commandGroups) : []
    next.filter.block_command_group = filterMode === 'block' ? splitLines(commandGroups) : []
    if (next.mode === 'scan' && next.scan_reader) next.scan_reader.dbs = numbers(databaseIds)
    else { next.filter.allow_db = filterMode === 'allow' ? numbers(databaseIds) : []; next.filter.block_db = filterMode === 'block' ? numbers(databaseIds) : [] }
    return next
  }, [spec, filterMode, keyPrefixes, keyRegex, commands, commandGroups, databaseIds])

  const persistDraft = useCallback(async () => {
    if (!taskRef.current) return
    const nextSpec = materializedSpec()
    const snapshot = JSON.stringify(nextSpec)
    if (snapshot === lastSavedSnapshot.current) return
    setSaveState('saving')
    saveChain.current = saveChain.current.catch(() => undefined).then(async () => {
      const currentTask = taskRef.current
      if (!currentTask) return
      try {
        const updated = await tasks.updateDraft(currentTask, nextSpec)
        taskRef.current = updated; setTask(updated); setSpec(updated.spec)
        lastSavedSnapshot.current = JSON.stringify(updated.spec); setSaveState('saved')
      } catch (cause) { setSaveState('error'); throw cause }
    })
    return saveChain.current
  }, [materializedSpec, tasks.updateDraft])

  useEffect(() => {
    if (!task || applyingServer.current || current === 5 || !open) return
    if (saveTimer.current) clearTimeout(saveTimer.current)
    saveTimer.current = setTimeout(() => { void persistDraft().catch(() => undefined) }, 700)
    return () => { if (saveTimer.current) clearTimeout(saveTimer.current) }
  }, [spec, filterMode, keyPrefixes, keyRegex, databaseIds, commands, commandGroups, current, open, task, persistDraft])

  function validateStep(step: number) {
    if (step === 0 && !spec.name.trim()) throw new Error('请输入任务名称')
    if (step === 1 && !spec.source_connection_id) throw new Error('请选择源端 Redis')
    if (step === 2 && !spec.target_connection_id) throw new Error('请选择目标 Redis')
    if (step === 2 && spec.source_connection_id === spec.target_connection_id) throw new Error('源端和目标端不能使用同一个连接')
  }

  async function ensureDraft() {
    validateStep(0)
    if (taskRef.current) return
    setSaveState('saving')
    const created = await tasks.createDraft({ name: spec.name, description: spec.description, mode: spec.mode })
    taskRef.current = created; setTask(created); lastSavedSnapshot.current = JSON.stringify(created.spec); setSaveState('saved')
  }

  async function next() {
    try {
      validateStep(current); await ensureDraft(); await persistDraft()
      const nextStep = current + 1; setCurrent(nextStep)
      if (nextStep === 5) await runPrecheck()
    } catch (cause) { message.error(cause instanceof Error ? cause.message : '保存草稿失败') }
  }

  async function runPrecheck() {
    if (!taskRef.current) return
    if (saveTimer.current) clearTimeout(saveTimer.current)
    try {
      await persistDraft(); setPrechecking(true)
      const currentTask = taskRef.current
      if (!currentTask) return
      const result = await tasks.precheck(currentTask, acknowledgeWarnings)
      setPrecheckResult(result)
      const refreshed = await api.getTask(currentTask.id)
      taskRef.current = refreshed; setTask(refreshed)
      if (result.ready) message.success('预检查通过，任务可以启动')
    } catch (cause) { message.error(cause instanceof Error ? cause.message : '预检查失败') }
    finally { setPrechecking(false) }
  }

  async function start() {
    const currentTask = taskRef.current
    if (!currentTask || !canStart) return
    setStarting(true)
    try { const run = await tasks.start(currentTask); onCompleted(currentTask, run); message.success(run.state === 'SUCCEEDED' ? '扫描任务已完成' : '同步任务已启动') }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '任务启动失败') }
    finally { setStarting(false) }
  }

  function changeMode(mode: TaskMode) {
    setSpec((currentSpec) => mode === 'sync' ? { ...currentSpec, mode, sync_reader: currentSpec.sync_reader ?? { sync_rdb: true, sync_aof: true, prefer_replica: false, try_diskless: false }, scan_reader: undefined }
      : { ...currentSpec, mode, scan_reader: currentSpec.scan_reader ?? { dbs: [], scan: true, ksn: false, count: 1, prefer_replica: false, skip_unknown_type: [] }, sync_reader: undefined })
  }

  const saveLabel = saveState === 'saving' ? '正在保存' : saveState === 'error' ? '保存失败' : task ? '草稿已保存' : '尚未保存'
  const connectionOptions = useMemo(() => connections.selectable.map(({ label, value }) => ({ label, value })), [connections.selectable])

  return <Drawer open={open} width={920} closable={false} rootClassName="task-wizard-drawer" onClose={onClose}
    title={<div className="wizard-heading"><div><span>创建同步任务</span><small>任务内核由 RedisShake 执行；页面负责生成配置、预检查与生命周期管理。</small></div><span className={`save-indicator ${saveState}`}><FloppyDisk size={15} />{saveLabel}</span></div>}
    footer={<div className="wizard-footer"><div>{current > 0 ? <Button icon={<ArrowLeft size={17} />} onClick={() => setCurrent((value) => value - 1)}>上一步</Button> : null}</div><div className="footer-actions"><Button onClick={onClose}>稍后继续</Button>{current < 5 ? <Button type="primary" onClick={() => void next()}>下一步<ArrowRight size={17} /></Button> : !canStart ? <Button type="primary" loading={prechecking} disabled={hasWarnings && !acknowledgeWarnings} onClick={() => void runPrecheck()}>执行预检查</Button> : <Button type="primary" loading={starting} icon={<Play size={17} weight="fill" />} onClick={() => void start()}>启动任务</Button>}</div></div>}>
    <div className="wizard-layout">
      <aside><Steps direction="vertical" size="small" current={current} items={steps} /></aside>
      <section className="wizard-body">
        {current === 0 ? <div className="step-panel">
          <StepIntro number="01" title="先定义迁移意图" description="任务名称用于列表检索；同步模式决定 RedisShake reader。" />
          <label><span>任务名称</span><Input value={spec.name} size="large" placeholder="例如：订单缓存迁移" onChange={(event) => setSpec((value) => ({ ...value, name: event.target.value }))} /></label>
          <label><span>任务描述</span><Input.TextArea value={spec.description} rows={3} placeholder="记录迁移窗口、负责人或业务范围" onChange={(event) => setSpec((value) => ({ ...value, description: event.target.value }))} /></label>
          <label><span>同步模式</span><Radio.Group value={spec.mode} className="mode-picker" onChange={(event) => changeMode(event.target.value as TaskMode)}><Radio.Button value="sync"><strong>增量同步</strong><small>RDB 全量 + AOF 持续同步</small></Radio.Button><Radio.Button value="scan"><strong>扫描迁移</strong><small>SCAN 一次性搬迁，结束后退出</small></Radio.Button></Radio.Group></label>
        </div> : current === 1 ? <div className="step-panel">
          <StepIntro number="02" title="选择源端 Redis" description="预检查会读取版本、角色和拓扑，不会写入源端。" />
          <label><span>源端连接</span><Select value={spec.source_connection_id} showSearch size="large" placeholder="选择已保存连接" options={connectionOptions} optionFilterProp="label" onChange={(source_connection_id) => setSpec((value) => ({ ...value, source_connection_id }))} /></label>
          {sourceConnection ? <ConnectionPreview connection={sourceConnection} /> : null}
          {!connections.items.length ? <Alert type="warning" showIcon message="还没有连接" description="请先关闭向导并在连接管理中创建源端和目标端。" /> : null}
        </div> : current === 2 ? <div className="step-panel">
          <StepIntro number="03" title="选择目标 Redis" description="预检查会写入一个 60 秒 TTL 的随机 Key 来证明写权限，并立即清理。" />
          <label><span>目标连接</span><Select value={spec.target_connection_id} showSearch size="large" placeholder="选择已保存连接" options={connectionOptions} optionFilterProp="label" onChange={(target_connection_id) => setSpec((value) => ({ ...value, target_connection_id }))} /></label>
          {targetConnection ? <ConnectionPreview connection={targetConnection} /> : null}
          {spec.source_connection_id && spec.source_connection_id === spec.target_connection_id ? <Alert type="error" showIcon message="源端和目标端相同" description="请选择不同的连接，避免数据覆盖或同步回环。" /> : null}
        </div> : current === 3 ? <div className="step-panel">
          <StepIntro number="04" title="限定同步范围" description="同一维度只能选择 allow 或 block。每行一个值，也支持逗号分隔。" />
          <label><span>过滤策略</span><Segmented value={filterMode} options={[{ label: '不过滤', value: 'none' }, { label: '仅允许', value: 'allow' }, { label: '排除', value: 'block' }]} onChange={(value) => setFilterMode(value as FilterMode)} /></label>
          {filterMode !== 'none' ? <><div className="form-grid two"><label><span>Key 前缀</span><Input.TextArea value={keyPrefixes} rows={4} placeholder={'cache:\nsession:'} onChange={(event) => setKeyPrefixes(event.target.value)} /></label><label><span>Key 正则</span><Input.TextArea value={keyRegex} rows={4} placeholder="^order:\\d+$" onChange={(event) => setKeyRegex(event.target.value)} /></label></div><div className="form-grid two"><label><span>命令</span><Input.TextArea value={commands} rows={3} placeholder={'SET\nHSET'} onChange={(event) => setCommands(event.target.value)} /></label><label><span>命令组</span><Input.TextArea value={commandGroups} rows={3} placeholder={'SCRIPTING\nPUBSUB'} onChange={(event) => setCommandGroups(event.target.value)} /></label></div></> : null}
          <label><span>{spec.mode === 'scan' ? '扫描 DB' : 'DB 过滤'}</span><Input value={databaseIds} placeholder="例如：0, 1, 5；留空表示全部" onChange={(event) => setDatabaseIds(event.target.value)} /></label>
          {spec.mode === 'scan' && spec.scan_reader ? <div className="form-grid two"><label><span>SCAN Count</span><InputNumber value={spec.scan_reader.count} min={1} max={10000} onChange={(count) => setSpec((value) => ({ ...value, scan_reader: value.scan_reader ? { ...value.scan_reader, count: count ?? 1 } : value.scan_reader }))} /></label><label className="switch-field"><span>订阅 Keyspace Notification</span><Switch checked={spec.scan_reader.ksn} onChange={(ksn) => setSpec((value) => ({ ...value, scan_reader: value.scan_reader ? { ...value.scan_reader, ksn } : value.scan_reader }))} /></label></div> : null}
        </div> : current === 4 ? <div className="step-panel">
          <StepIntro number="05" title="性能与冲突处理" description="默认参数适用于一般迁移；危险操作会在预检查中要求确认。" />
          <div className="form-grid two"><label><span>目标最大 QPS</span><InputNumber value={spec.advanced.target_redis_max_qps} min={1} max={300000} onChange={(target_redis_max_qps) => setSpec((value) => ({ ...value, advanced: { ...value.advanced, target_redis_max_qps: target_redis_max_qps ?? 1 } }))} /></label><label><span>Pipeline Count</span><InputNumber value={spec.advanced.pipeline_count_limit} min={1} onChange={(pipeline_count_limit) => setSpec((value) => ({ ...value, advanced: { ...value.advanced, pipeline_count_limit: pipeline_count_limit ?? 1 } }))} /></label></div>
          <label><span>目标 Key 已存在时</span><Select value={spec.advanced.rdb_restore_command_behavior} options={[{ value: 'panic', label: '停止任务（推荐）' }, { value: 'rewrite', label: '覆盖目标 Key' }, { value: 'skip', label: '跳过冲突 Key' }]} onChange={(rdb_restore_command_behavior) => setSpec((value) => ({ ...value, advanced: { ...value.advanced, rdb_restore_command_behavior } }))} /></label>
          <div className="danger-setting"><div><Warning size={22} /><span><strong>启动前清空目标 Redis</strong><small>会执行 FLUSHALL，目标端现有数据不可恢复。</small></span></div><Switch checked={spec.advanced.empty_db_before_sync} onChange={(empty_db_before_sync) => setSpec((value) => ({ ...value, advanced: { ...value.advanced, empty_db_before_sync } }))} /></div>
          {spec.mode === 'sync' && spec.sync_reader ? <div className="form-grid two"><label className="switch-field"><span>同步 RDB 阶段</span><Switch checked={spec.sync_reader.sync_rdb} onChange={(sync_rdb) => setSpec((value) => ({ ...value, sync_reader: value.sync_reader ? { ...value.sync_reader, sync_rdb } : value.sync_reader }))} /></label><label className="switch-field"><span>持续同步 AOF</span><Switch checked={spec.sync_reader.sync_aof} onChange={(sync_aof) => setSpec((value) => ({ ...value, sync_reader: value.sync_reader ? { ...value.sync_reader, sync_aof } : value.sync_reader }))} /></label></div> : null}
        </div> : <div className="step-panel review-panel">
          <StepIntro number="06" title="预检查与启动" description="只有阻断项为零，并确认所有危险警告后，任务才会进入 READY。" />
          <div className="review-route"><ReviewConnection title="源端" connection={sourceConnection} /><ArrowRight size={22} /><ReviewConnection title="目标端" connection={targetConnection} /></div>
          {precheckResult ? <CheckResultPanel checks={precheckResult.checks} title="任务预检查" /> : null}
          {hasWarnings ? <div className="warning-ack"><Checkbox checked={acknowledgeWarnings} onChange={(event) => setAcknowledgeWarnings(event.target.checked)}>我已阅读并确认上述危险警告</Checkbox><Button loading={prechecking} onClick={() => void runPrecheck()}>重新检查</Button></div> : null}
          {precheckResult?.config_digest ? <div className="digest-line"><CheckCircle size={18} weight="fill" /><span>配置摘要</span><code>{precheckResult.config_digest}</code></div> : null}
        </div>}
      </section>
    </div>
  </Drawer>
}

function StepIntro({ number, title, description }: { number: string; title: string; description: string }) {
  return <div className="step-intro"><span>{number}</span><div><h2>{title}</h2><p>{description}</p></div></div>
}

function ConnectionPreview({ connection }: { connection: import('@/api/types').Connection }) {
  return <div className="connection-preview"><strong>{connection.name}</strong><span className="mono">{connection.address || connection.sentinel.address}</span><small>{connection.topology}</small></div>
}

function ReviewConnection({ title, connection }: { title: string; connection?: import('@/api/types').Connection }) {
  return <div><small>{title}</small><strong>{connection?.name ?? '未选择'}</strong><span className="mono">{connection?.address || connection?.sentinel.address || '—'}</span></div>
}
