import { App, Button, Dropdown, Input, Modal, Select } from 'antd'
import { ArrowsClockwise, Copy, DotsThree, MagnifyingGlass, Trash } from '@phosphor-icons/react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import type { Connection, Topology } from '@/api/types'
import ConnectionDrawer from '@/components/ConnectionDrawer'
import InlineError from '@/components/InlineError'
import PageHeader from '@/components/PageHeader'
import StatusPill from '@/components/StatusPill'
import { useConnections } from '@/state/ConnectionsContext'
import { formatDate, topologyLabel } from '@/utils/presentation'

export default function ConnectionsView() {
  const navigate = useNavigate()
  const store = useConnections()
  const { message } = App.useApp()
  const [editing, setEditing] = useState<Connection>()
  const [testingId, setTestingId] = useState<string>()
  const [query, setQuery] = useState('')
  const [topologyFilter, setTopologyFilter] = useState<'all' | Topology>('all')

  useEffect(() => { void store.load().catch(() => undefined) }, [store.load])

  const filtered = useMemo(() => store.items.filter((connection) => {
    const keyword = query.trim().toLowerCase()
    const matchesQuery = !keyword || connection.name.toLowerCase().includes(keyword) || (connection.address || connection.sentinel.address || '').toLowerCase().includes(keyword)
    return matchesQuery && (topologyFilter === 'all' || connection.topology === topologyFilter)
  }), [store.items, query, topologyFilter])

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
    <PageHeader title="连接管理" description="保存并测试 Redis 连接，凭据由控制面加密。">
      <Button type="primary" onClick={() => navigate('/connections/new')}>新建连接</Button>
    </PageHeader>
    {store.error ? <InlineError message={store.error} onRetry={() => void store.load(true)} /> : null}
    <div className="list-toolbar">
      <Input className="list-search" allowClear prefix={<MagnifyingGlass size={16} />} value={query} placeholder="搜索连接名称或地址" aria-label="搜索连接" onChange={(event) => setQuery(event.target.value)} />
      <div className="toolbar-right"><Select size="small" value={topologyFilter} aria-label="拓扑筛选" style={{ width: 124 }} options={[{ label: '全部拓扑', value: 'all' }, { label: '单机 / 主从', value: 'standalone' }, { label: 'Sentinel', value: 'sentinel' }, { label: 'Cluster', value: 'cluster' }]} onChange={(value) => setTopologyFilter(value)} /><Button type="text" aria-label="刷新连接" loading={store.loading} icon={<ArrowsClockwise size={16} />} onClick={() => void store.load(true)} /></div>
    </div>
    {store.loading && !store.loaded ? <div className="skeleton-list">{[0, 1, 2, 3].map((item) => <div key={item} className="skeleton-row" />)}</div>
      : !store.items.length ? <div className="list-empty-row">暂无 Redis 连接</div>
        : <><div className="compact-summary"><span><strong>{store.items.length}</strong>连接</span><span>密码与 TLS 材料已加密保存</span></div>{filtered.length ? <div className="data-surface connection-table"><div className="data-table-header connection-table-header"><span>连接名称 / 地址</span><span>拓扑</span><span>凭据</span><span>最近检查</span><span>操作</span></div>{filtered.map((connection) => <div key={connection.id} className="data-row connection-row">
          <div className="connection-name"><strong>{connection.name}</strong><small className="mono">{connection.address || connection.sentinel.address}</small></div>
          <div><small>拓扑</small><strong>{topologyLabel[connection.topology]}</strong></div>
          <div><small>凭据</small><StatusPill label={connection.password_configured ? '已加密' : '无密码'} tone={connection.password_configured ? 'success' : 'neutral'} /></div>
          <div><small>最近检查</small><strong>{formatDate(connection.last_tested_at)}</strong></div>
          <div className="row-actions"><Button size="small" loading={testingId === connection.id} onClick={() => void test(connection)}>测试</Button><Dropdown menu={{ items: [
            { key: 'edit', label: '编辑连接', onClick: () => setEditing(connection) },
            { key: 'copy', icon: <Copy size={15} />, label: '复制连接', onClick: () => void copy(connection) },
            { key: 'delete', icon: <Trash size={15} />, danger: true, label: '删除', onClick: () => remove(connection) },
          ] }}><Button type="text" size="small" aria-label={`${connection.name} 更多操作`} icon={<DotsThree size={18} />} /></Dropdown></div>
        </div>)}</div> : <div className="list-empty-row">没有匹配的连接，请调整搜索或筛选条件</div>}</>}
    <ConnectionDrawer open={Boolean(editing)} connection={editing} onClose={() => setEditing(undefined)} onSaved={(connection) => { store.upsert(connection); setEditing(undefined) }} />
  </div>
}
