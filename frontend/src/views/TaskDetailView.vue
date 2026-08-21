<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Modal, message } from 'ant-design-vue'
import { PhArrowLeft, PhArrowsClockwise, PhFileText, PhPlay, PhStop, PhWarning } from '@phosphor-icons/vue'

import { api } from '@/api/client'
import type { Run, Task } from '@/api/types'
import InlineError from '@/components/InlineError.vue'
import PageHeader from '@/components/PageHeader.vue'
import StatusPill from '@/components/StatusPill.vue'
import { formatDate, formatNumber, modeLabel, runStateMeta, taskStateMeta } from '@/utils/presentation'

const route = useRoute()
const router = useRouter()
const task = ref<Task>()
const runs = ref<Run[]>([])
const selectedRunId = ref<string>()
const loading = ref(true)
const error = ref<string>()
const activeTab = ref('overview')
const actionLoading = ref(false)
const logs = ref('')
const logOffset = ref(0)
const logsLoading = ref(false)
let eventSource: EventSource | undefined
let logTimer: ReturnType<typeof setInterval> | undefined

const selectedRun = computed(() => runs.value.find((run) => run.id === selectedRunId.value) ?? runs.value[0])
const active = computed(() => selectedRun.value && ['STARTING', 'RUNNING', 'STOPPING'].includes(selectedRun.value.state))
const metrics = computed(() => selectedRun.value?.status?.total_entries_count)

onMounted(load)
onBeforeUnmount(cleanupStreams)
watch(selectedRunId, () => { logs.value = ''; logOffset.value = 0; connectStreams() })

async function load() {
  loading.value = true; error.value = undefined
  try {
    const id = route.params.id as string
    const [loadedTask, loadedRuns] = await Promise.all([api.getTask(id), api.listRuns(id)])
    task.value = loadedTask; runs.value = loadedRuns
    const nextRunId = selectedRunId.value ?? loadedRuns[0]?.id
    if (selectedRunId.value === nextRunId) connectStreams()
    else selectedRunId.value = nextRunId
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '任务详情加载失败' }
  finally { loading.value = false }
}

function cleanupStreams() {
  eventSource?.close(); eventSource = undefined
  if (logTimer) clearInterval(logTimer); logTimer = undefined
}

function connectStreams() {
  cleanupStreams()
  const run = selectedRun.value
  if (!run) return
  if (['STARTING', 'RUNNING', 'STOPPING'].includes(run.state)) {
    eventSource = new EventSource(`/api/v1/runs/${run.id}/events`)
    eventSource.addEventListener('status', (event) => {
      const updated = JSON.parse((event as MessageEvent).data) as Run
      const index = runs.value.findIndex((item) => item.id === updated.id)
      if (index >= 0) runs.value.splice(index, 1, updated)
      if (!['STARTING', 'RUNNING', 'STOPPING'].includes(updated.state)) cleanupStreams()
    })
    logTimer = setInterval(() => void loadMoreLogs(), 1600)
  }
  void loadMoreLogs()
}

async function loadMoreLogs() {
  const run = selectedRun.value
  if (!run || logsLoading.value) return
  logsLoading.value = true
  try {
    const result = await api.readLogs(run.id, logOffset.value, 131072)
    if (result.content) logs.value += stripAnsi(result.content)
    logOffset.value = result.next_offset
  } catch { /* detail status remains usable when logs lag */ }
  finally { logsLoading.value = false }
}

function stripAnsi(value: string) { return value.replace(/\u001b\[[0-9;]*m/g, '') }

async function start() {
  if (!task.value) return
  actionLoading.value = true
  try {
    const run = await api.startRun(task.value.id, task.value.config_revision)
    runs.value.unshift(run); selectedRunId.value = run.id
    message.success(run.state === 'SUCCEEDED' ? '任务已完成' : '任务已启动')
  } catch (cause) { message.error(cause instanceof Error ? cause.message : '启动失败') }
  finally { actionLoading.value = false }
}

async function requestStop(force = false) {
  const run = selectedRun.value
  if (!run) return
  actionLoading.value = true
  try {
    const updated = force ? await api.forceStopRun(run.id) : await api.stopRun(run.id)
    const index = runs.value.findIndex((item) => item.id === updated.id)
    if (index >= 0) runs.value.splice(index, 1, updated)
    message.success(force ? '已发送强制停止信号' : '正在优雅停止')
  } catch (cause) { message.error(cause instanceof Error ? cause.message : '停止失败') }
  finally { actionLoading.value = false }
}

function confirmStop(force = false) {
  Modal.confirm({
    title: force ? '强制终止 RedisShake？' : '停止当前同步？',
    content: force ? '进程会被立即终止，尚未刷写的队列可能丢失。' : '控制面会先发送 SIGTERM，等待 RedisShake 关闭 reader、刷写 writer 并释放文件锁。',
    okText: force ? '强制终止' : '优雅停止', okType: force ? 'danger' : 'primary', cancelText: '取消',
    onOk: () => requestStop(force),
  })
}
</script>

<template>
  <div class="page-wrap detail-page">
    <button class="back-link" type="button" @click="router.push('/tasks')"><PhArrowLeft :size="16" />返回任务列表</button>
    <div v-if="loading" class="skeleton-list"><div v-for="i in 5" :key="i" class="skeleton-row" /></div>
    <InlineError v-else-if="error" :message="error" @retry="load" />
    <template v-else-if="task">
      <PageHeader eyebrow="Task detail" :title="task.spec.name" :description="`${modeLabel[task.spec.mode]} · revision ${task.config_revision} · 最近更新 ${formatDate(task.updated_at)}`">
        <a-button @click="load"><template #icon><PhArrowsClockwise :size="17" /></template>刷新</a-button>
        <a-button v-if="task.state === 'READY' && !active" type="primary" :loading="actionLoading" @click="start"><template #icon><PhPlay :size="17" weight="fill" /></template>启动</a-button>
        <a-button v-if="active" danger :loading="actionLoading" @click="confirmStop(false)"><template #icon><PhStop :size="17" weight="fill" /></template>停止</a-button>
        <a-button v-if="selectedRun?.state === 'STOPPING'" danger type="primary" @click="confirmStop(true)">强制终止</a-button>
      </PageHeader>

      <div class="detail-status-line">
        <StatusPill :label="taskStateMeta[task.state].label" :tone="taskStateMeta[task.state].tone" />
        <template v-if="selectedRun"><StatusPill :label="runStateMeta[selectedRun.state].label" :tone="runStateMeta[selectedRun.state].tone" :pulse="selectedRun.state === 'RUNNING'" /><span class="mono">Run {{ selectedRun.id.slice(0, 12) }}</span></template>
      </div>

      <a-alert v-if="selectedRun?.state === 'UNKNOWN'" type="warning" show-icon message="运行归属无法确认" description="控制面重启后不会向这个 PID 发送信号，也不会允许同任务重复启动。请在主机上核对进程后处理。" />
      <div class="metric-strip detail-metrics">
        <div class="metric"><label>RedisShake 状态</label><strong>{{ selectedRun?.status_healthy ? '心跳正常' : selectedRun ? '无实时心跳' : '尚未运行' }}</strong></div>
        <div class="metric"><label>累计读取</label><strong class="mono">{{ formatNumber(metrics?.read_count) }}</strong></div>
        <div class="metric"><label>累计写入</label><strong class="mono">{{ formatNumber(metrics?.write_count) }}</strong></div>
        <div class="metric"><label>当前 OPS</label><strong class="mono">{{ formatNumber(metrics?.write_ops) }}</strong></div>
      </div>

      <a-tabs v-model:active-key="activeTab" class="detail-tabs">
        <a-tab-pane key="overview" tab="运行概览">
          <div class="overview-grid">
            <section class="overview-main">
              <div class="section-heading"><h2>RedisShake 状态</h2><span>来自 worker 回环状态端口</span></div>
              <div class="status-json-grid">
                <div><small>阶段</small><strong>{{ selectedRun?.status?.reader?.status ?? (selectedRun?.state || '—') }}</strong></div>
                <div><small>内部一致</small><strong>{{ selectedRun?.status?.consistent === undefined ? '—' : selectedRun.status.consistent ? '是' : '否' }}</strong></div>
                <div><small>Writer 未响应</small><strong class="mono">{{ formatNumber(Number(selectedRun?.status?.writer?.unanswered_entries ?? 0)) }}</strong></div>
                <div><small>最后心跳</small><strong>{{ formatDate(selectedRun?.last_heartbeat_at) }}</strong></div>
              </div>
              <div class="section-heading"><h2>Reader / Writer 原始状态</h2><span>便于问题排查</span></div>
              <pre class="json-view">{{ JSON.stringify({ reader: selectedRun?.status?.reader, writer: selectedRun?.status?.writer }, null, 2) }}</pre>
            </section>
            <aside class="run-selector">
              <div class="section-heading"><h2>运行记录</h2><span>{{ runs.length }}</span></div>
              <button v-for="run in runs" :key="run.id" type="button" :class="{ selected: run.id === selectedRun?.id }" @click="selectedRunId=run.id">
                <span><StatusPill :label="runStateMeta[run.state].label" :tone="runStateMeta[run.state].tone" /></span><strong>{{ formatDate(run.started_at) }}</strong><small class="mono">{{ run.id.slice(0, 10) }}</small>
              </button>
            </aside>
          </div>
        </a-tab-pane>
        <a-tab-pane key="logs" tab="运行日志">
          <div class="log-toolbar"><span><PhFileText :size="17" />stdout / stderr（已脱敏）</span><a-button size="small" @click="loadMoreLogs">读取新增日志</a-button></div>
          <pre class="log-view">{{ logs || '等待日志输出…' }}</pre>
        </a-tab-pane>
        <a-tab-pane key="history" tab="运行历史">
          <div class="data-surface history-table"><div v-for="run in runs" :key="run.id" class="data-row history-row"><StatusPill :label="runStateMeta[run.state].label" :tone="runStateMeta[run.state].tone" /><span class="mono">{{ run.id }}</span><span>{{ formatDate(run.started_at) }}</span><span>{{ run.exit_reason || '—' }}</span></div></div>
        </a-tab-pane>
        <a-tab-pane key="config" tab="配置快照">
          <div class="config-note"><PhWarning :size="18" /><span>快照不包含 Redis 密码或 TLS PEM；运行时 TOML 保存在受保护目录。</span></div>
          <pre class="json-view">{{ JSON.stringify(selectedRun?.config_snapshot ?? task.spec, null, 2) }}</pre>
        </a-tab-pane>
      </a-tabs>
    </template>
  </div>
</template>

<style scoped>
.back-link{display:flex;align-items:center;gap:6px;margin:0 0 22px;padding:0;color:var(--muted);font-size:12px;background:none;border:0;cursor:pointer}.detail-status-line{display:flex;align-items:center;gap:9px;margin:-12px 0 22px;color:var(--muted);font-size:11px}.detail-page :deep(.ant-alert){margin-bottom:18px}.detail-tabs{margin-top:24px}.overview-grid{display:grid;grid-template-columns:minmax(0,1.6fr) minmax(250px,.55fr);gap:30px}.status-json-grid{display:grid;grid-template-columns:repeat(4,1fr);border-top:1px solid var(--line);border-bottom:1px solid var(--line)}.status-json-grid>div{padding:16px;border-right:1px solid var(--line)}.status-json-grid>div:last-child{border-right:0}.status-json-grid small,.status-json-grid strong{display:block}.status-json-grid small{color:var(--muted)}.status-json-grid strong{margin-top:7px;font-size:13px}.json-view,.log-view{margin:0;padding:18px;overflow:auto;color:#d6e2dd;background:#1d2926;border-radius:12px;font:11px/1.65 'Geist Mono',ui-monospace,monospace;white-space:pre-wrap}.json-view{max-height:430px}.run-selector{padding-left:24px;border-left:1px solid var(--line)}.run-selector>button{width:100%;display:grid;grid-template-columns:auto 1fr;gap:7px 10px;padding:13px 0;text-align:left;background:none;border:0;border-bottom:1px solid var(--line);cursor:pointer}.run-selector>button.selected{color:var(--accent)}.run-selector strong{font-size:11px}.run-selector small{grid-column:2;color:var(--muted);font-size:9px}.log-toolbar{display:flex;align-items:center;justify-content:space-between;padding:10px 0}.log-toolbar>span{display:flex;align-items:center;gap:7px;color:var(--muted);font-size:11px}.log-view{min-height:420px;max-height:65vh}.history-row{grid-template-columns:110px 1fr .7fr 1fr;font-size:11px}.config-note{display:flex;gap:8px;align-items:center;margin-bottom:12px;color:var(--warning);font-size:11px}@media(max-width:900px){.overview-grid{grid-template-columns:1fr}.run-selector{padding-left:0;border-left:0}.status-json-grid{grid-template-columns:1fr 1fr}}@media(max-width:620px){.detail-metrics{grid-template-columns:1fr 1fr}.history-row{grid-template-columns:90px 1fr}.history-row>span:nth-child(n+3){display:none}}
</style>
