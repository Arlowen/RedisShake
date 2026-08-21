<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Modal, message } from 'ant-design-vue'
import { PhArrowsClockwise, PhDotsThree, PhPlus, PhShieldCheck, PhTrash } from '@phosphor-icons/vue'

import type { Connection } from '@/api/types'
import ConnectionDrawer from '@/components/ConnectionDrawer.vue'
import EmptyState from '@/components/EmptyState.vue'
import InlineError from '@/components/InlineError.vue'
import PageHeader from '@/components/PageHeader.vue'
import StatusPill from '@/components/StatusPill.vue'
import { useConnectionsStore } from '@/stores/connections'
import { formatDate, topologyLabel } from '@/utils/presentation'

const store = useConnectionsStore()
const drawerOpen = ref(false)
const editing = ref<Connection>()
const testingId = ref<string>()

onMounted(() => store.load().catch(() => undefined))

function openCreate() { editing.value = undefined; drawerOpen.value = true }
function openEdit(connection: Connection) { editing.value = connection; drawerOpen.value = true }
function onSaved(connection: Connection) { store.items = store.items.map((item) => item.id === connection.id ? connection : item); if (!store.items.some((item) => item.id === connection.id)) store.items.push(connection); drawerOpen.value = false }

async function test(connection: Connection) {
  testingId.value = connection.id
  try {
    const result = await store.testSaved(connection.id, 'source')
    result.success ? message.success('连接检查通过') : message.warning('连接存在阻断项')
  } catch (cause) { message.error(cause instanceof Error ? cause.message : '测试失败') }
  finally { testingId.value = undefined }
}

function remove(connection: Connection) {
  Modal.confirm({
    title: `删除“${connection.name}”？`,
    content: '被任务引用的连接不会被删除。凭据删除后无法恢复。',
    okText: '删除连接', okType: 'danger', cancelText: '取消',
    async onOk() { await store.remove(connection.id); message.success('连接已删除') },
  })
}
</script>

<template>
  <div class="page-wrap">
    <PageHeader eyebrow="Inventory" title="连接管理" description="集中保存 Redis、Sentinel 和 Cluster 连接；密码与 TLS 材料由控制面加密，页面只展示配置状态。">
      <a-button type="primary" @click="openCreate"><template #icon><PhPlus :size="17" /></template>新建连接</a-button>
    </PageHeader>
    <InlineError v-if="store.error" :message="store.error" @retry="store.load(true)" />
    <div class="toolbar">
      <div class="toolbar-left"><span class="muted">{{ store.items.length }} 个连接</span></div>
      <div class="toolbar-right"><a-button type="text" :loading="store.loading" @click="store.load(true)"><template #icon><PhArrowsClockwise :size="17" /></template>刷新</a-button></div>
    </div>
    <div v-if="store.loading && !store.loaded" class="skeleton-list"><div v-for="i in 4" :key="i" class="skeleton-row" /></div>
    <EmptyState v-else-if="!store.items.length" title="还没有 Redis 连接" description="先添加源端和目标端。创建任务时可以直接复用并执行连通性检查。"><a-button type="primary" @click="openCreate">创建第一个连接</a-button></EmptyState>
    <div v-else class="data-surface connection-table">
      <div v-for="connection in store.items" :key="connection.id" class="data-row connection-row">
        <div class="connection-name"><span class="connection-icon"><PhShieldCheck :size="20" /></span><div><strong>{{ connection.name }}</strong><small class="mono">{{ connection.address || connection.sentinel.address }}</small></div></div>
        <div><small>拓扑</small><strong>{{ topologyLabel[connection.topology] }}</strong></div>
        <div><small>凭据</small><StatusPill :label="connection.password_configured ? '已加密' : '无密码'" :tone="connection.password_configured ? 'success' : 'neutral'" /></div>
        <div><small>最近检查</small><strong>{{ formatDate(connection.last_tested_at) }}</strong></div>
        <div class="row-actions"><a-button :loading="testingId === connection.id" @click="test(connection)">测试</a-button><a-dropdown><a-button type="text"><PhDotsThree :size="20" /></a-button><template #overlay><a-menu><a-menu-item @click="openEdit(connection)">编辑连接</a-menu-item><a-menu-item danger @click="remove(connection)"><PhTrash :size="15" /> 删除</a-menu-item></a-menu></template></a-dropdown></div>
      </div>
    </div>
    <ConnectionDrawer :open="drawerOpen" :connection="editing" @close="drawerOpen=false" @saved="onSaved" />
  </div>
</template>

<style scoped>
.connection-row{grid-template-columns:minmax(220px,1.5fr) .7fr .7fr .8fr auto}.connection-name{display:flex;align-items:center;gap:12px}.connection-icon{width:38px;height:38px;display:grid;place-items:center;color:var(--accent);border-radius:10px;background:#eaf2ef}.connection-name strong,.connection-name small,.connection-row>div>small,.connection-row>div>strong{display:block}.connection-name small{margin-top:3px;color:var(--muted);font-size:11px}.connection-row>div>small{margin-bottom:5px;color:var(--muted);font-size:10px}.connection-row>div>strong{font-size:12px}.row-actions{display:flex;gap:5px;justify-content:flex-end}
@media(max-width:1000px){.connection-row{grid-template-columns:1.4fr .7fr .7fr auto}.connection-row>div:nth-child(4){display:none}}@media(max-width:700px){.connection-row{grid-template-columns:1fr auto}.connection-row>div:nth-child(2),.connection-row>div:nth-child(3){display:none}}
</style>
