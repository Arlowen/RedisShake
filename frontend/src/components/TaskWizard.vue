<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { PhArrowLeft, PhArrowRight, PhCheckCircle, PhFloppyDisk, PhPlay, PhWarning } from '@phosphor-icons/vue'

import type { PrecheckResult, Run, Task, TaskMode, TaskSpec } from '@/api/types'
import CheckResultPanel from '@/components/CheckResultPanel.vue'
import { useConnectionsStore } from '@/stores/connections'
import { useTasksStore } from '@/stores/tasks'
import { defaultTaskSpec } from '@/utils/defaults'
import { modeLabel } from '@/utils/presentation'

const props = defineProps<{ open: boolean; initialTask?: Task }>()
const emit = defineEmits<{ close: []; completed: [task: Task, run?: Run] }>()

const tasksStore = useTasksStore()
const connectionsStore = useConnectionsStore()
const current = ref(0)
const task = ref<Task>()
const spec = reactive<TaskSpec>(defaultTaskSpec())
const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
const prechecking = ref(false)
const starting = ref(false)
const precheckResult = ref<PrecheckResult>()
const acknowledgeWarnings = ref(false)
const applyingServer = ref(false)
let saveTimer: ReturnType<typeof setTimeout> | undefined
let saveChain: Promise<void> = Promise.resolve()
let lastSavedSnapshot = ''
const plainSpec = () => JSON.parse(JSON.stringify(spec)) as TaskSpec

const steps = [
  { title: '基本信息' }, { title: '源端 Redis' }, { title: '目标端 Redis' },
  { title: '同步范围' }, { title: '高级配置' }, { title: '预检查' },
]
const sourceConnection = computed(() => connectionsStore.items.find((item) => item.id === spec.source_connection_id))
const targetConnection = computed(() => connectionsStore.items.find((item) => item.id === spec.target_connection_id))
const hasWarnings = computed(() => precheckResult.value?.checks.some((item) => item.state === 'WARNING') ?? false)
const canStart = computed(() => precheckResult.value?.ready === true)

const filterMode = ref<'none' | 'allow' | 'block'>('none')
const keyPrefixes = ref('')
const keyRegex = ref('')
const databaseIds = ref('')
const commands = ref('')
const commandGroups = ref('')

watch(() => [props.open, props.initialTask] as const, async ([open]) => {
  if (!open) return
  applyingServer.value = true
  const initial = props.initialTask ? JSON.parse(JSON.stringify(props.initialTask)) as Task : undefined
  Object.assign(spec, initial?.spec ?? defaultTaskSpec())
  task.value = initial
  lastSavedSnapshot = initial ? JSON.stringify(initial.spec) : ''
  current.value = 0
  saveState.value = initial ? 'saved' : 'idle'
  precheckResult.value = initial?.last_precheck_result
  acknowledgeWarnings.value = false
  const hasAllow = spec.filter.allow_key_prefix.length || spec.filter.allow_key_regex.length || spec.filter.allow_command.length || spec.filter.allow_command_group.length
  const hasBlock = spec.filter.block_key_prefix.length || spec.filter.block_key_regex.length || spec.filter.block_command.length || spec.filter.block_command_group.length
  filterMode.value = hasAllow ? 'allow' : hasBlock ? 'block' : 'none'
  keyPrefixes.value = (hasAllow ? spec.filter.allow_key_prefix : spec.filter.block_key_prefix).join('\n')
  keyRegex.value = (hasAllow ? spec.filter.allow_key_regex : spec.filter.block_key_regex).join('\n')
  commands.value = (hasAllow ? spec.filter.allow_command : spec.filter.block_command).join('\n')
  commandGroups.value = (hasAllow ? spec.filter.allow_command_group : spec.filter.block_command_group).join('\n')
  databaseIds.value = (spec.mode === 'scan' ? spec.scan_reader?.dbs : hasAllow ? spec.filter.allow_db : spec.filter.block_db)?.join(', ') ?? ''
  await connectionsStore.load().catch(() => undefined)
  applyingServer.value = false
})

watch(() => spec.mode, (mode: TaskMode) => {
  if (mode === 'sync') {
    spec.sync_reader ??= { sync_rdb: true, sync_aof: true, prefer_replica: false, try_diskless: false }
    spec.scan_reader = undefined
  } else {
    spec.scan_reader ??= { dbs: [], scan: true, ksn: false, count: 1, prefer_replica: false, skip_unknown_type: [] }
    spec.sync_reader = undefined
  }
})

watch(spec, () => {
  if (!task.value || applyingServer.value || current.value === 5) return
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => void persistDraft(), 700)
}, { deep: true })

function validateStep(step: number) {
  if (step === 0 && !spec.name.trim()) throw new Error('请输入任务名称')
  if (step === 1 && !spec.source_connection_id) throw new Error('请选择源端 Redis')
  if (step === 2 && !spec.target_connection_id) throw new Error('请选择目标端 Redis')
  if (step === 2 && spec.source_connection_id === spec.target_connection_id) throw new Error('源端和目标端不能使用同一个连接')
}

function lines(value: string) { return value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean) }
function numbers(value: string) { return lines(value).map(Number).filter((item) => Number.isInteger(item)) }
function applyFilterFields() {
  spec.filter.allow_key_prefix = filterMode.value === 'allow' ? lines(keyPrefixes.value) : []
  spec.filter.block_key_prefix = filterMode.value === 'block' ? lines(keyPrefixes.value) : []
  spec.filter.allow_key_regex = filterMode.value === 'allow' ? lines(keyRegex.value) : []
  spec.filter.block_key_regex = filterMode.value === 'block' ? lines(keyRegex.value) : []
  spec.filter.allow_command = filterMode.value === 'allow' ? lines(commands.value) : []
  spec.filter.block_command = filterMode.value === 'block' ? lines(commands.value) : []
  spec.filter.allow_command_group = filterMode.value === 'allow' ? lines(commandGroups.value) : []
  spec.filter.block_command_group = filterMode.value === 'block' ? lines(commandGroups.value) : []
  if (spec.mode === 'scan' && spec.scan_reader) spec.scan_reader.dbs = numbers(databaseIds.value)
  else {
    spec.filter.allow_db = filterMode.value === 'allow' ? numbers(databaseIds.value) : []
    spec.filter.block_db = filterMode.value === 'block' ? numbers(databaseIds.value) : []
  }
}

async function ensureDraft() {
  validateStep(0)
  if (!task.value) {
    saveState.value = 'saving'
    task.value = await tasksStore.createDraft({ name: spec.name, description: spec.description, mode: spec.mode })
    lastSavedSnapshot = JSON.stringify(task.value.spec)
    saveState.value = 'saved'
  }
}

async function persistDraft() {
  if (!task.value) return
  saveChain = saveChain.catch(() => undefined).then(async () => {
    if (!task.value) return
    applyFilterFields()
    const snapshot = JSON.stringify(spec)
    if (snapshot === lastSavedSnapshot) return
    saveState.value = 'saving'
    try {
      const updated = await tasksStore.updateDraft(task.value, plainSpec())
      task.value = updated
      lastSavedSnapshot = JSON.stringify(updated.spec)
      saveState.value = 'saved'
    } catch (cause) {
      saveState.value = 'error'
      throw cause
    }
  })
  return saveChain
}

async function next() {
  try {
    validateStep(current.value)
    await ensureDraft()
    await persistDraft()
    current.value++
    if (current.value === 5) await runPrecheck()
  } catch (cause) { message.error(cause instanceof Error ? cause.message : '保存草稿失败') }
}

async function runPrecheck() {
  if (!task.value) return
  clearTimeout(saveTimer)
  try {
    await persistDraft()
    prechecking.value = true
    precheckResult.value = await tasksStore.precheck(task.value, acknowledgeWarnings.value)
    task.value = await import('@/api/client').then(({ api }) => api.getTask(task.value!.id))
    if (precheckResult.value.ready) message.success('预检查通过，任务可以启动')
  } catch (cause) { message.error(cause instanceof Error ? cause.message : '预检查失败') }
  finally { prechecking.value = false }
}

async function start() {
  if (!task.value || !canStart.value) return
  starting.value = true
  try {
    const run = await tasksStore.start(task.value)
    emit('completed', task.value, run)
    message.success(run.state === 'SUCCEEDED' ? '扫描任务已完成' : '同步任务已启动')
  } catch (cause) { message.error(cause instanceof Error ? cause.message : '任务启动失败') }
  finally { starting.value = false }
}
</script>

<template>
  <a-drawer :open="open" :width="920" :closable="false" root-class-name="task-wizard-drawer" @close="emit('close')">
    <template #title>
      <div class="wizard-heading"><div><span>创建同步任务</span><small>任务内核由 RedisShake 执行；页面负责生成配置、预检查与生命周期管理。</small></div><span class="save-indicator" :class="saveState"><PhFloppyDisk :size="15" />{{ saveState === 'saving' ? '正在保存' : saveState === 'error' ? '保存失败' : task ? '草稿已保存' : '尚未保存' }}</span></div>
    </template>
    <div class="wizard-layout">
      <aside><a-steps direction="vertical" size="small" :current="current" :items="steps" /></aside>
      <section class="wizard-body">
        <div v-if="current === 0" class="step-panel">
          <div class="step-intro"><span>01</span><div><h2>先定义迁移意图</h2><p>任务名称用于列表检索；同步模式决定 RedisShake reader。</p></div></div>
          <label><span>任务名称</span><a-input v-model:value="spec.name" size="large" placeholder="例如：订单缓存迁移" /></label>
          <label><span>任务描述</span><a-textarea v-model:value="spec.description" :rows="3" placeholder="记录迁移窗口、负责人或业务范围" /></label>
          <label><span>同步模式</span><a-radio-group v-model:value="spec.mode" class="mode-picker">
            <a-radio-button value="sync"><strong>增量同步</strong><small>RDB 全量 + AOF 持续同步</small></a-radio-button>
            <a-radio-button value="scan"><strong>扫描迁移</strong><small>SCAN 一次性搬迁，结束后退出</small></a-radio-button>
          </a-radio-group></label>
        </div>

        <div v-else-if="current === 1" class="step-panel">
          <div class="step-intro"><span>02</span><div><h2>选择源端 Redis</h2><p>预检查会读取版本、角色和拓扑，不会写入源端。</p></div></div>
          <label><span>源端连接</span><a-select v-model:value="spec.source_connection_id" show-search size="large" placeholder="选择已保存连接" :options="connectionsStore.selectable" option-filter-prop="label" /></label>
          <div v-if="sourceConnection" class="connection-preview"><strong>{{ sourceConnection.name }}</strong><span class="mono">{{ sourceConnection.address || sourceConnection.sentinel.address }}</span><small>{{ sourceConnection.topology }}</small></div>
          <a-alert v-if="!connectionsStore.items.length" type="warning" show-icon message="还没有连接" description="请先关闭向导并在连接管理中创建源端和目标端。" />
        </div>

        <div v-else-if="current === 2" class="step-panel">
          <div class="step-intro"><span>03</span><div><h2>选择目标 Redis</h2><p>预检查会写入一个 60 秒 TTL 的随机 Key 来证明写权限，并立即清理。</p></div></div>
          <label><span>目标连接</span><a-select v-model:value="spec.target_connection_id" show-search size="large" placeholder="选择已保存连接" :options="connectionsStore.selectable" option-filter-prop="label" /></label>
          <div v-if="targetConnection" class="connection-preview"><strong>{{ targetConnection.name }}</strong><span class="mono">{{ targetConnection.address || targetConnection.sentinel.address }}</span><small>{{ targetConnection.topology }}</small></div>
          <a-alert v-if="spec.source_connection_id && spec.source_connection_id === spec.target_connection_id" type="error" show-icon message="源端和目标端相同" description="请选择不同的连接，避免数据覆盖或同步回环。" />
        </div>

        <div v-else-if="current === 3" class="step-panel">
          <div class="step-intro"><span>04</span><div><h2>限定同步范围</h2><p>同一维度只能选择 allow 或 block。每行一个值，也支持逗号分隔。</p></div></div>
          <label><span>过滤策略</span><a-segmented v-model:value="filterMode" :options="[{label:'不过滤',value:'none'},{label:'仅允许',value:'allow'},{label:'排除',value:'block'}]" /></label>
          <template v-if="filterMode !== 'none'">
            <div class="form-grid two"><label><span>Key 前缀</span><a-textarea v-model:value="keyPrefixes" :rows="4" placeholder="cache:&#10;session:" /></label><label><span>Key 正则</span><a-textarea v-model:value="keyRegex" :rows="4" placeholder="^order:\\d+$" /></label></div>
            <div class="form-grid two"><label><span>命令</span><a-textarea v-model:value="commands" :rows="3" placeholder="SET&#10;HSET" /></label><label><span>命令组</span><a-textarea v-model:value="commandGroups" :rows="3" placeholder="SCRIPTING&#10;PUBSUB" /></label></div>
          </template>
          <label><span>{{ spec.mode === 'scan' ? '扫描 DB' : 'DB 过滤' }}</span><a-input v-model:value="databaseIds" placeholder="例如：0, 1, 5；留空表示全部" /></label>
          <div v-if="spec.mode === 'scan' && spec.scan_reader" class="form-grid two"><label><span>SCAN Count</span><a-input-number v-model:value="spec.scan_reader.count" :min="1" :max="10000" /></label><label class="switch-field"><span>订阅 Keyspace Notification</span><a-switch v-model:checked="spec.scan_reader.ksn" /></label></div>
        </div>

        <div v-else-if="current === 4" class="step-panel">
          <div class="step-intro"><span>05</span><div><h2>性能与冲突处理</h2><p>默认参数适用于一般迁移；危险操作会在预检查中要求确认。</p></div></div>
          <div class="form-grid two"><label><span>目标最大 QPS</span><a-input-number v-model:value="spec.advanced.target_redis_max_qps" :min="1" :max="300000" /></label><label><span>Pipeline Count</span><a-input-number v-model:value="spec.advanced.pipeline_count_limit" :min="1" /></label></div>
          <label><span>目标 Key 已存在时</span><a-select v-model:value="spec.advanced.rdb_restore_command_behavior" :options="[{value:'panic',label:'停止任务（推荐）'},{value:'rewrite',label:'覆盖目标 Key'},{value:'skip',label:'跳过冲突 Key'}]" /></label>
          <div class="danger-setting"><div><PhWarning :size="22" /><span><strong>启动前清空目标 Redis</strong><small>会执行 FLUSHALL，目标端现有数据不可恢复。</small></span></div><a-switch v-model:checked="spec.advanced.empty_db_before_sync" /></div>
          <template v-if="spec.mode === 'sync' && spec.sync_reader"><div class="form-grid two"><label class="switch-field"><span>同步 RDB 阶段</span><a-switch v-model:checked="spec.sync_reader.sync_rdb" /></label><label class="switch-field"><span>持续同步 AOF</span><a-switch v-model:checked="spec.sync_reader.sync_aof" /></label></div></template>
        </div>

        <div v-else class="step-panel review-panel">
          <div class="step-intro"><span>06</span><div><h2>预检查与启动</h2><p>只有阻断项为零，并确认所有危险警告后，任务才会进入 READY。</p></div></div>
          <div class="review-route"><div><small>源端</small><strong>{{ sourceConnection?.name }}</strong><span class="mono">{{ sourceConnection?.address || sourceConnection?.sentinel.address }}</span></div><PhArrowRight :size="22" /><div><small>目标端</small><strong>{{ targetConnection?.name }}</strong><span class="mono">{{ targetConnection?.address || targetConnection?.sentinel.address }}</span></div></div>
          <CheckResultPanel v-if="precheckResult" :checks="precheckResult.checks" title="任务预检查" />
          <div v-if="hasWarnings" class="warning-ack"><a-checkbox v-model:checked="acknowledgeWarnings">我已阅读并确认上述危险警告</a-checkbox><a-button :loading="prechecking" @click="runPrecheck">重新检查</a-button></div>
          <div v-if="precheckResult?.config_digest" class="digest-line"><PhCheckCircle :size="18" weight="fill" /><span>配置摘要</span><code>{{ precheckResult.config_digest }}</code></div>
        </div>
      </section>
    </div>
    <template #footer>
      <div class="wizard-footer"><div><a-button v-if="current > 0" @click="current--"><template #icon><PhArrowLeft :size="17" /></template>上一步</a-button></div><div class="footer-actions"><a-button @click="emit('close')">稍后继续</a-button><a-button v-if="current < 5" type="primary" @click="next">下一步<template #icon><PhArrowRight :size="17" /></template></a-button><a-button v-else-if="!canStart" type="primary" :loading="prechecking" :disabled="hasWarnings && !acknowledgeWarnings" @click="runPrecheck">执行预检查</a-button><a-button v-else type="primary" :loading="starting" @click="start"><template #icon><PhPlay :size="17" weight="fill" /></template>启动任务</a-button></div></div>
    </template>
  </a-drawer>
</template>

<style scoped>
.wizard-heading,.wizard-footer{display:flex;align-items:center;justify-content:space-between;gap:20px}.wizard-heading>div>span,.wizard-heading small{display:block}.wizard-heading small{margin-top:3px;color:var(--muted);font-size:11px;font-weight:400}.save-indicator{display:inline-flex;align-items:center;gap:6px;color:var(--muted);font-size:11px}.save-indicator.saved{color:var(--accent)}.save-indicator.error{color:var(--danger)}
.wizard-layout{display:grid;grid-template-columns:180px minmax(0,1fr);min-height:610px}.wizard-layout>aside{padding:10px 24px 0 0;border-right:1px solid var(--line)}.wizard-body{padding:4px 0 20px 34px}.step-panel{display:grid;gap:18px;animation:step-in .3s cubic-bezier(.16,1,.3,1)}.step-intro{display:grid;grid-template-columns:44px 1fr;gap:14px;padding-bottom:17px;border-bottom:1px solid var(--line)}.step-intro>span{width:38px;height:38px;display:grid;place-items:center;color:var(--accent);font-size:12px;font-weight:700;border:1px solid #cadeD6;border-radius:11px;background:#eaf2ef}.step-intro h2{margin:0;font-size:23px;letter-spacing:-.04em}.step-intro p{margin:5px 0 0;color:var(--muted);font-size:12px}.step-panel label{display:grid;gap:7px;color:#53615d;font-size:12px}.mode-picker{display:grid;grid-template-columns:1fr 1fr;gap:12px}.mode-picker :deep(.ant-radio-button-wrapper){height:auto;padding:17px;border:1px solid var(--line)!important;border-radius:11px!important}.mode-picker :deep(.ant-radio-button-wrapper::before){display:none}.mode-picker strong,.mode-picker small{display:block}.mode-picker small{margin-top:5px;color:var(--muted);font-size:11px}.connection-preview{display:grid;grid-template-columns:1fr auto;gap:5px 14px;padding:16px;border:1px solid var(--line);border-radius:12px;background:#f9fbfa}.connection-preview small{grid-column:1/-1;color:var(--muted);text-transform:uppercase}.form-grid{display:grid;gap:14px}.form-grid.two{grid-template-columns:1fr 1fr}.switch-field{grid-template-columns:1fr auto;align-items:center}.danger-setting{display:flex;justify-content:space-between;align-items:center;gap:20px;padding:17px;color:#7b4f28;border:1px solid #ead7b9;border-radius:12px;background:#fbf4e9}.danger-setting>div{display:flex;gap:11px;align-items:center}.danger-setting strong,.danger-setting small{display:block}.danger-setting small{margin-top:3px}.review-route{display:grid;grid-template-columns:1fr auto 1fr;align-items:center;gap:17px}.review-route>div{display:grid;gap:4px;padding:15px;border:1px solid var(--line);border-radius:11px}.review-route small{color:var(--muted)}.warning-ack{display:flex;justify-content:space-between;align-items:center;padding:13px;border:1px solid #ead7b9;border-radius:10px;background:#fbf6ee}.digest-line{display:grid;grid-template-columns:auto auto 1fr;gap:9px;align-items:center;color:var(--accent);font-size:11px}.digest-line code{overflow:hidden;color:var(--muted);text-overflow:ellipsis}.footer-actions{display:flex;gap:9px}
@keyframes step-in{from{opacity:0;transform:translateX(8px)}to{opacity:1;transform:none}}@media(max-width:760px){.wizard-layout{grid-template-columns:1fr}.wizard-layout>aside{display:none}.wizard-body{padding-left:0}.form-grid.two,.mode-picker{grid-template-columns:1fr}.review-route{grid-template-columns:1fr}.review-route>svg{transform:rotate(90deg)}}
</style>
