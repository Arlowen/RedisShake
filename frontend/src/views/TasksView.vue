<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Modal, message } from 'ant-design-vue'
import { PhArchive, PhArrowRight, PhArrowsClockwise, PhCopy, PhDotsThree, PhMagnifyingGlass, PhPlay, PhPlus } from '@phosphor-icons/vue'

import { api } from '@/api/client'
import type { Run, Task, TaskState } from '@/api/types'
import EmptyState from '@/components/EmptyState.vue'
import InlineError from '@/components/InlineError.vue'
import PageHeader from '@/components/PageHeader.vue'
import StatusPill from '@/components/StatusPill.vue'
import TaskWizard from '@/components/TaskWizard.vue'
import { useTasksStore } from '@/stores/tasks'
import { useConnectionsStore } from '@/stores/connections'
import { formatDate, formatNumber, modeLabel, runStateMeta, taskStateMeta } from '@/utils/presentation'

const router = useRouter()
const store = useTasksStore()
const connectionsStore = useConnectionsStore()
const wizardOpen = ref(false)
const editingTask = ref<Task>()
const query = ref('')
const stateFilter = ref<'all' | TaskState>('all')
const latestRuns = ref<Record<string, Run | undefined>>({})
const startingId = ref<string>()

const filtered = computed(() => store.items.filter((task) => {
  const matchesQuery = !query.value || task.spec.name.toLowerCase().includes(query.value.toLowerCase())
  return matchesQuery && (stateFilter.value === 'all' || task.state === stateFilter.value)
}))
const runningCount = computed(() => Object.values(latestRuns.value).filter((run) => run && ['STARTING', 'RUNNING', 'STOPPING'].includes(run.state)).length)
const readyCount = computed(() => store.items.filter((task) => task.state === 'READY').length)
const totalWritten = computed(() => Object.values(latestRuns.value).reduce((total, run) => total + (run?.status?.total_entries_count?.write_count ?? 0), 0))

onMounted(() => load())

async function load() {
  try {
    await Promise.all([store.load(), connectionsStore.load()])
    const pairs = await Promise.all(store.items.map(async (task) => [task.id, (await api.listRuns(task.id))[0]] as const))
    latestRuns.value = Object.fromEntries(pairs)
  } catch { /* store error renders inline */ }
}

function connectionName(id?: string) {
  if (!id) return '未选择'
  return connectionsStore.items.find((item) => item.id === id)?.name ?? '连接已删除'
}

function create() { editingTask.value = undefined; wizardOpen.value = true }
function edit(task: Task) { editingTask.value = task; wizardOpen.value = true }
function completed(task: Task, run?: Run) {
  wizardOpen.value = false
  store.replace(task)
  if (run) latestRuns.value[task.id] = run
  void router.push(`/tasks/${task.id}`)
}

async function start(task: Task) {
  startingId.value = task.id
  try {
    const run = await store.start(task)
    latestRuns.value[task.id] = run
    message.success(run.state === 'SUCCEEDED' ? '任务已完成' : '任务已启动')
    await router.push(`/tasks/${task.id}`)
  } catch (cause) { message.error(cause instanceof Error ? cause.message : '启动失败') }
  finally { startingId.value = undefined }
}

function archive(task: Task) {
  Modal.confirm({
    title: `归档“${task.spec.name}”？`,
    content: '历史运行仍会保留；存在活动运行时无法归档。',
    okText: '归档任务', cancelText: '取消',
    async onOk() { await store.archive(task); message.success('任务已归档') },
  })
}

async function copy(task: Task) {
  try { await store.copy(task); message.success('任务副本已创建，需重新预检查') }
  catch (cause) { message.error(cause instanceof Error ? cause.message : '复制失败') }
}
</script>

<template>
  <div class="page-wrap">
    <PageHeader eyebrow="Migration control" title="同步任务" description="用向导生成 RedisShake 配置，先验证连接和危险参数，再启动独立 worker 并持续读取真实状态。">
      <a-button type="primary" @click="create"><template #icon><PhPlus :size="17" /></template>创建同步任务</a-button>
    </PageHeader>

    <div class="metric-strip task-metrics">
      <div class="metric"><label>任务总数</label><strong class="mono">{{ formatNumber(store.items.length) }}</strong></div>
      <div class="metric"><label>活动运行</label><strong class="mono">{{ formatNumber(runningCount) }}</strong></div>
      <div class="metric"><label>可启动</label><strong class="mono">{{ formatNumber(readyCount) }}</strong></div>
      <div class="metric"><label>累计写入</label><strong class="mono">{{ formatNumber(totalWritten) }}</strong></div>
    </div>

    <InlineError v-if="store.error" class="task-error" :message="store.error" @retry="load" />
    <div class="toolbar">
      <div class="toolbar-left search-box"><PhMagnifyingGlass :size="18" /><a-input v-model:value="query" :bordered="false" placeholder="搜索任务名称" /></div>
      <div class="toolbar-right"><a-segmented v-model:value="stateFilter" :options="[{label:'全部',value:'all'},{label:'草稿',value:'DRAFT'},{label:'可启动',value:'READY'}]" /><a-button type="text" :loading="store.loading" @click="load"><template #icon><PhArrowsClockwise :size="17" /></template></a-button></div>
    </div>

    <div v-if="store.loading && !store.items.length" class="skeleton-list"><div v-for="i in 5" :key="i" class="skeleton-row" /></div>
    <EmptyState v-else-if="!store.items.length" title="创建第一条同步链路" description="准备好源端与目标端 Redis 连接后，通过六步向导生成配置、执行预检查并启动 RedisShake。"><a-button type="primary" @click="create">创建同步任务</a-button></EmptyState>
    <div v-else-if="filtered.length" class="data-surface task-table">
      <div v-for="(task, index) in filtered" :key="task.id" class="data-row task-row" :style="{ '--row-index': index }" @dblclick="router.push(`/tasks/${task.id}`)">
        <div class="task-identity"><span class="mode-code">{{ task.spec.mode === 'sync' ? 'SY' : 'SC' }}</span><div><strong>{{ task.spec.name }}</strong><small>{{ modeLabel[task.spec.mode] }} · revision {{ task.config_revision }}</small></div></div>
        <div class="route-cell"><span>{{ connectionName(task.spec.source_connection_id) }}</span><PhArrowRight :size="14" /><span>{{ connectionName(task.spec.target_connection_id) }}</span></div>
        <div><small>任务状态</small><StatusPill :label="taskStateMeta[task.state].label" :tone="taskStateMeta[task.state].tone" /></div>
        <div><small>最新运行</small><StatusPill v-if="latestRuns[task.id]" :label="runStateMeta[latestRuns[task.id]!.state].label" :tone="runStateMeta[latestRuns[task.id]!.state].tone" :pulse="latestRuns[task.id]!.state === 'RUNNING'" /><span v-else class="muted">尚未运行</span></div>
        <div><small>更新时间</small><strong>{{ formatDate(task.updated_at) }}</strong></div>
        <div class="row-actions"><a-button v-if="task.state === 'DRAFT'" @click="edit(task)">继续配置</a-button><a-button v-else type="primary" ghost :loading="startingId === task.id" :disabled="latestRuns[task.id]?.state === 'RUNNING'" @click="start(task)"><template #icon><PhPlay :size="15" weight="fill" /></template>启动</a-button><a-dropdown><a-button type="text"><PhDotsThree :size="20" /></a-button><template #overlay><a-menu><a-menu-item @click="router.push(`/tasks/${task.id}`)">查看详情</a-menu-item><a-menu-item @click="edit(task)">编辑配置</a-menu-item><a-menu-item @click="copy(task)"><PhCopy :size="15" /> 复制任务</a-menu-item><a-menu-item danger @click="archive(task)"><PhArchive :size="15" /> 归档</a-menu-item></a-menu></template></a-dropdown></div>
      </div>
    </div>
    <EmptyState v-else title="没有匹配的任务" description="调整搜索词或状态筛选后再试。" />

    <TaskWizard :open="wizardOpen" :initial-task="editingTask" @close="wizardOpen=false" @completed="completed" />
  </div>
</template>

<style scoped>
.task-error{margin:18px 0}.search-box{min-width:260px;padding:0 10px;color:var(--muted);border:1px solid var(--line);border-radius:9px;background:#fff}.task-row{grid-template-columns:minmax(210px,1.3fr) minmax(180px,1fr) .65fr .7fr .7fr auto;animation:row-in .35s cubic-bezier(.16,1,.3,1) both;animation-delay:calc(var(--row-index) * 45ms)}.task-identity{display:flex;align-items:center;gap:12px}.mode-code{width:39px;height:39px;display:grid;place-items:center;color:var(--accent);font-size:11px;font-weight:750;letter-spacing:.05em;border-radius:11px;background:#e9f2ee}.task-identity strong,.task-identity small,.task-row>div>small,.task-row>div>strong{display:block}.task-identity small{margin-top:4px;color:var(--muted);font-size:10px}.route-cell{display:flex;align-items:center;gap:7px;color:var(--muted);font-size:11px}.task-row>div>small{margin-bottom:6px;color:var(--muted);font-size:10px}.task-row>div>strong{font-size:11px}.row-actions{display:flex;justify-content:flex-end;gap:5px}@keyframes row-in{from{opacity:0;transform:translateY(7px)}to{opacity:1;transform:none}}@media(max-width:1180px){.task-row{grid-template-columns:1.3fr .8fr .8fr .7fr auto}.route-cell{display:none}}@media(max-width:760px){.task-metrics{grid-template-columns:1fr 1fr}.task-row{grid-template-columns:1fr auto}.task-row>div:nth-child(3),.task-row>div:nth-child(4),.task-row>div:nth-child(5){display:none}.toolbar-right{justify-content:space-between}.search-box{min-width:100%}}
</style>
