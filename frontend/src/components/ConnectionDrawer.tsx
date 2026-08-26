import { App, Button, Collapse, Drawer, Input, Segmented, Select, Switch } from 'antd'
import { Flask, FloppyDisk } from '@phosphor-icons/react'
import { useEffect, useMemo, useState } from 'react'

import { api } from '@/api/client'
import type { Connection, ConnectionInput, ConnectionTestResult, TestPurpose } from '@/api/types'
import CheckResultPanel from '@/components/CheckResultPanel'
import { defaultConnectionInput } from '@/utils/defaults'

interface ConnectionDrawerProps {
  open: boolean
  connection?: Connection
  onClose: () => void
  onSaved: (connection: Connection) => void
}

export default function ConnectionDrawer({ open, connection, onClose, onSaved }: ConnectionDrawerProps) {
  const { message } = App.useApp()
  const [form, setForm] = useState<ConnectionInput>(defaultConnectionInput)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testPurpose, setTestPurpose] = useState<TestPurpose>('source')
  const [testResult, setTestResult] = useState<ConnectionTestResult>()
  const editing = Boolean(connection)

  useEffect(() => {
    if (!open) return
    const defaults = defaultConnectionInput()
    setForm(connection ? {
      ...defaults,
      name: connection.name,
      topology: connection.topology,
      address: connection.address ?? '',
      username: connection.username ?? '',
      tls: { ...defaults.tls, enabled: connection.tls.enabled, server_name: connection.tls.server_name ?? '', insecure_skip_verify: connection.tls.insecure_skip_verify },
      sentinel: {
        ...defaults.sentinel,
        address: connection.sentinel.address ?? '', master_name: connection.sentinel.master_name ?? '', username: connection.sentinel.username ?? '',
        tls: { ...defaults.sentinel.tls, enabled: connection.sentinel.tls.enabled, server_name: connection.sentinel.tls.server_name ?? '', insecure_skip_verify: connection.sentinel.tls.insecure_skip_verify },
      },
    } : defaults)
    setTestResult(undefined)
    setTestPurpose('source')
  }, [open, connection])

  const tlsItems = useMemo(() => [{
    key: 'certs', label: '证书材料（可选）', children: (
      <div className="cert-fields">
        <label><span>CA certificate PEM</span><Input.TextArea value={form.tls.ca_cert_pem} rows={3} onChange={(event) => setTLS({ ca_cert_pem: event.target.value })} /></label>
        <div className="form-grid two">
          <label><span>Client certificate PEM</span><Input.TextArea value={form.tls.client_cert_pem} rows={3} onChange={(event) => setTLS({ client_cert_pem: event.target.value })} /></label>
          <label><span>Client private key PEM</span><Input.TextArea value={form.tls.client_key_pem} rows={3} onChange={(event) => setTLS({ client_key_pem: event.target.value })} /></label>
        </div>
      </div>
    ),
  }], [form.tls.ca_cert_pem, form.tls.client_cert_pem, form.tls.client_key_pem])

  function setTLS(patch: Partial<ConnectionInput['tls']>) { setForm((current) => ({ ...current, tls: { ...current.tls, ...patch } })) }
  function setSentinel(patch: Partial<ConnectionInput['sentinel']>) { setForm((current) => ({ ...current, sentinel: { ...current.sentinel, ...patch } })) }

  function validate() {
    if (!form.name.trim()) throw new Error('请输入连接名称')
    if (form.topology === 'sentinel') {
      if (!form.sentinel.address.trim()) throw new Error('请输入 Sentinel 地址')
      if (!form.sentinel.master_name.trim()) throw new Error('请输入 Master name')
    } else if (!form.address.trim()) throw new Error('请输入 Redis 地址')
  }

  async function runTest() {
    try {
      validate(); setTesting(true)
      const result = editing && connection ? await api.testSavedConnection(connection.id, testPurpose) : await api.testConnection(structuredClone(form), testPurpose)
      setTestResult(result)
      result.success ? message.success('连接测试通过') : message.warning('连接测试存在阻断项')
    } catch (cause) { message.error(cause instanceof Error ? cause.message : '连接测试失败') }
    finally { setTesting(false) }
  }

  function compactTLS(input: ConnectionInput['tls']) {
    return {
      enabled: input.enabled, server_name: input.server_name, insecure_skip_verify: input.insecure_skip_verify,
      ...(input.ca_cert_pem ? { ca_cert_pem: input.ca_cert_pem } : {}),
      ...(input.client_cert_pem ? { client_cert_pem: input.client_cert_pem } : {}),
      ...(input.client_key_pem ? { client_key_pem: input.client_key_pem } : {}),
    }
  }

  async function save() {
    try {
      validate(); setSaving(true)
      let saved: Connection
      if (connection) {
        const patch: Partial<ConnectionInput> = {
          name: form.name, topology: form.topology, address: form.address, username: form.username,
          tls: compactTLS(form.tls),
          sentinel: {
            ...form.sentinel,
            tls: compactTLS(form.sentinel.tls),
            password: form.sentinel.password || undefined,
          },
          password: form.password || undefined,
        }
        saved = await api.updateConnection(connection.id, patch)
      } else saved = await api.createConnection(structuredClone(form))
      onSaved(saved)
      message.success(editing ? '连接已更新' : '连接已创建')
    } catch (cause) { message.error(cause instanceof Error ? cause.message : '保存失败') }
    finally { setSaving(false) }
  }

  return (
    <Drawer open={open} width={560} closable={false} rootClassName="console-drawer" onClose={onClose}
      title={<div className="drawer-title"><span>{editing ? '编辑连接' : '新建连接'}</span><small>凭据加密存储且不会回显。</small></div>}
      footer={<div className="drawer-footer"><Button onClick={onClose}>取消</Button><Button type="primary" loading={saving} icon={<FloppyDisk size={17} />} onClick={() => void save()}>保存连接</Button></div>}>
      <div className="connection-form">
        <section className="form-section">
          <div className="form-section-title"><span>基础连接</span><small>RedisShake 用这些信息连接实际数据节点</small></div>
          <div className="form-grid two">
            <label><span>连接名称</span><Input value={form.name} placeholder="例如：生产源端" onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} /></label>
            <label><span>拓扑类型</span><Select value={form.topology} options={[{ value: 'standalone', label: '单机 / 主从' }, { value: 'sentinel', label: 'Sentinel' }, { value: 'cluster', label: 'Cluster' }]} onChange={(topology) => setForm((current) => ({ ...current, topology }))} /></label>
          </div>
          {form.topology !== 'sentinel' ? <label><span>Redis 地址</span><Input value={form.address} placeholder="host:port" onChange={(event) => setForm((current) => ({ ...current, address: event.target.value }))} /></label> : null}
          <div className="form-grid two">
            <label><span>Redis 用户名</span><Input value={form.username} autoComplete="off" placeholder="未启用 ACL 可留空" onChange={(event) => setForm((current) => ({ ...current, username: event.target.value }))} /></label>
            <label><span>Redis 密码</span><Input.Password value={form.password} autoComplete="new-password" placeholder={editing ? '留空表示保持不变' : '未设置密码可留空'} onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))} /></label>
          </div>
        </section>
        {form.topology === 'sentinel' ? <section className="form-section">
          <div className="form-section-title"><span>Sentinel 发现</span><small>Sentinel 凭据和 Redis 主节点凭据彼此独立</small></div>
          <div className="form-grid two">
            <label><span>Sentinel 地址</span><Input value={form.sentinel.address} placeholder="host:26379" onChange={(event) => setSentinel({ address: event.target.value })} /></label>
            <label><span>Master name</span><Input value={form.sentinel.master_name} placeholder="mymaster" onChange={(event) => setSentinel({ master_name: event.target.value })} /></label>
            <label><span>Sentinel 用户名</span><Input value={form.sentinel.username} onChange={(event) => setSentinel({ username: event.target.value })} /></label>
            <label><span>Sentinel 密码</span><Input.Password value={form.sentinel.password} placeholder={editing ? '留空表示保持不变' : ''} onChange={(event) => setSentinel({ password: event.target.value })} /></label>
          </div>
        </section> : null}
        <section className="form-section">
          <div className="form-section-title"><span>TLS</span><small>默认校验证书；关闭校验仅适合受控测试环境</small></div>
          <div className="tls-switch"><span>Redis TLS</span><Switch checked={form.tls.enabled} onChange={(enabled) => setTLS({ enabled })} /></div>
          {form.tls.enabled ? <>
            <div className="form-grid two">
              <label><span>Server name</span><Input value={form.tls.server_name} placeholder="redis.internal" onChange={(event) => setTLS({ server_name: event.target.value })} /></label>
              <label className="switch-field"><span>跳过证书校验</span><Switch checked={form.tls.insecure_skip_verify} onChange={(insecure_skip_verify) => setTLS({ insecure_skip_verify })} /></label>
            </div>
            <Collapse ghost items={tlsItems} />
          </> : null}
        </section>
        <section className="test-section">
          <div className="test-bar"><Segmented value={testPurpose} options={[{ label: '源端检查', value: 'source' }, { label: '目标写检查', value: 'target' }]} onChange={(value) => setTestPurpose(value as TestPurpose)} /><Button loading={testing} icon={<Flask size={17} />} onClick={() => void runTest()}>测试连接</Button></div>
          {testPurpose === 'target' ? <p className="side-effect-note">目标检查会写入一个带 60 秒 TTL 的随机 Key，并立即尝试删除。</p> : null}
          {testResult ? <CheckResultPanel checks={testResult.checks} title="检查结果" /> : null}
        </section>
      </div>
    </Drawer>
  )
}
