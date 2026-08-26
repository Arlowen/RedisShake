import { App, Button, Dropdown, Modal } from 'antd'
import { ArrowsClockwise, Copy, DotsThree, Plus, ShieldCheck, Trash } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'

import type { Connection } from '@/api/types'
import ConnectionDrawer from '@/components/ConnectionDrawer'
import EmptyState from '@/components/EmptyState'
import InlineError from '@/components/InlineError'
import PageHeader from '@/components/PageHeader'
import StatusPill from '@/components/StatusPill'
import { useConnections } from '@/state/ConnectionsContext'
import { formatDate, topologyLabel } from '@/utils/presentation'

export default function ConnectionsView() {
  const store = useConnections()
  const { message } = App.useApp()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<Connection>()
  const [testingId, setTestingId] = useState<string>()

  useEffect(() => { void store.load().catch(() => undefined) }, [store.load])

  async function test(connection: Connection) {
    setTestingId(connection.id)
    try { const result = await store.testSaved(connection.id, 'source'); result.success ? message.success('连接检查通过') : message.warning('连接存在阻断项') }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '测试失败') }
    finally { setTestingId(undefined) }
  }

  function remove(connection: Connection) {
    Modal.confirm({ title: `删除“${connection.name}”？`, content: '被任务引用的连接不会被删除。凭据删除后无法恢复。', okText: '删除连接', okType: 'danger', cancelText: '取消', async onOk() { await store.remove(connection.id); message.success('连接已删除') } })
  }

  async function copy(connection: Connection) {
    try { await store.copy(connection); message.success('连接副本已创建，凭据已重新加密') }
    catch (cause) { message.error(cause instanceof Error ? cause.message : '复制失败') }
  }

  return <div className="page-wrap">
    <PageHeader eyebrow="Inventory" title="连接管理" description="集中保存 Redis、Sentinel 和 Cluster 连接；密码与 TLS 材料由控制面加密，页面只展示配置状态。">
      <Button type="primary" icon={<Plus size={17} />} onClick={() => { setEditing(undefined); setDrawerOpen(true) }}>新建连接</Button>
    </PageHeader>
    {store.error ? <InlineError message={store.error} onRetry={() => void store.load(true)} /> : null}
    <div className="toolbar"><div className="toolbar-left"><span className="muted">{store.items.length} 个连接</span></div><div className="toolbar-right"><Button type="text" loading={store.loading} icon={<ArrowsClockwise size={17} />} onClick={() => void store.load(true)}>刷新</Button></div></div>
    {store.loading && !store.loaded ? <div className="skeleton-list">{[0, 1, 2, 3].map((item) => <div key={item} className="skeleton-row" />)}</div>
      : !store.items.length ? <EmptyState title="还没有 Redis 连接" description="先添加源端和目标端。创建任务时可以直接复用并执行连通性检查。"><Button type="primary" onClick={() => setDrawerOpen(true)}>创建第一个连接</Button></EmptyState>
        : <div className="data-surface connection-table">{store.items.map((connection) => <div key={connection.id} className="data-row connection-row">
          <div className="connection-name"><span className="connection-icon"><ShieldCheck size={20} /></span><div><strong>{connection.name}</strong><small className="mono">{connection.address || connection.sentinel.address}</small></div></div>
          <div><small>拓扑</small><strong>{topologyLabel[connection.topology]}</strong></div>
          <div><small>凭据</small><StatusPill label={connection.password_configured ? '已加密' : '无密码'} tone={connection.password_configured ? 'success' : 'neutral'} /></div>
          <div><small>最近检查</small><strong>{formatDate(connection.last_tested_at)}</strong></div>
          <div className="row-actions"><Button loading={testingId === connection.id} onClick={() => void test(connection)}>测试</Button><Dropdown menu={{ items: [
            { key: 'edit', label: '编辑连接', onClick: () => { setEditing(connection); setDrawerOpen(true) } },
            { key: 'copy', icon: <Copy size={15} />, label: '复制连接', onClick: () => void copy(connection) },
            { key: 'delete', icon: <Trash size={15} />, danger: true, label: '删除', onClick: () => remove(connection) },
          ] }}><Button type="text" aria-label={`${connection.name} 更多操作`} icon={<DotsThree size={20} />} /></Dropdown></div>
        </div>)}</div>}
    <ConnectionDrawer open={drawerOpen} connection={editing} onClose={() => setDrawerOpen(false)} onSaved={(connection) => { store.upsert(connection); setDrawerOpen(false) }} />
  </div>
}
