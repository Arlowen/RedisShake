import { ShieldCheck } from '@phosphor-icons/react'
import { useCallback, useEffect, useMemo, useState } from 'react'

import { api } from '@/api/client'
import type { SystemInfo } from '@/api/types'
import DataTable from '@/components/DataTable'
import PageScaffold from '@/components/PageScaffold'
import PageToolbar from '@/components/PageToolbar'
import SummaryBar from '@/components/SummaryBar'

export default function SystemView() {
  const [info, setInfo] = useState<SystemInfo>()
  const [error, setError] = useState<string>()
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')
  const load = useCallback(async () => { setLoading(true); setError(undefined); try { setInfo(await api.systemInfo()) } catch (cause) { setError(cause instanceof Error ? cause.message : '系统信息加载失败') } finally { setLoading(false) } }, [])
  useEffect(() => { void load() }, [load])

  const rows = useMemo(() => info ? [
    { label: '元数据存储', description: '控制面元数据', state: info.storage, value: info.data_dir, kind: 'code' },
    { label: '运行目录', description: 'Task / Run artifacts', state: '已配置', value: info.runtime_dir, kind: 'code' },
    { label: 'RedisShake Worker', description: '每个 Run 使用独立进程', state: '可执行', value: info.worker_path, kind: 'code' },
    { label: '凭据保护', description: info.secrets_configured ? '主密钥已配置' : '主密钥未配置', state: info.secrets_configured ? '可用' : '受限', value: info.secrets_configured ? '可加密保存连接凭据' : '仅支持无密码连接', kind: info.secrets_configured ? 'ok' : 'warning' },
    { label: 'Web 控制台', description: info.web_ui_configured ? '前端资源已内嵌' : '前端资源未编译', state: info.web_ui_configured ? '可用' : 'API only', value: info.web_ui_configured ? 'Go embed.FS' : '仅提供控制面 API', kind: info.web_ui_configured ? 'ok' : 'warning' },
    { label: '运行约束', description: `最多 ${info.max_concurrent_runs} 个活动 Run`, state: '已生效', value: `日志保留 ${info.log_retention_days === 0 ? '不限期' : `${info.log_retention_days} 天`}`, kind: 'text' },
  ] : [], [info])
  const filtered = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    return !keyword ? rows : rows.filter((row) => [row.label, row.description, row.state, row.value].some((value) => value.toLowerCase().includes(keyword)))
  }, [query, rows])

  return <PageScaffold title="系统信息" description="查看当前控制面、存储和 RedisShake 运行配置。" error={error} onRetry={() => void load()}>
    <PageToolbar ariaLabel="系统信息工具栏" search={{ value: query, placeholder: '搜索配置项或路径', ariaLabel: '搜索系统信息', onChange: setQuery }} refreshing={loading} refreshLabel="刷新系统信息" onRefresh={() => void load()} />
    {loading ? <div className="skeleton-list">{[0, 1, 2, 3].map((item) => <div key={item} className="skeleton-row" />)}</div> : info ? <>
      <SummaryBar><span className="ready-summary"><span className="status-beacon" /><strong>Ready</strong></span><span>{info.storage} 正常</span><span className="mono">{info.version} · {info.git_commit}</span></SummaryBar>
      {filtered.length ? <DataTable className="system-table" ariaLabel="系统运行配置" headers={['配置项', '状态', '配置值']}>{filtered.map((row) => <div key={row.label} className="data-row system-row">
        <div className="system-key"><strong>{row.label}</strong><small>{row.description}</small></div>
        <span className={row.kind === 'ok' ? 'value-ok' : row.kind === 'warning' ? 'value-warning' : ''}>{row.state}</span>
        {row.kind === 'code' ? <code>{row.value}</code> : <span>{row.value}</span>}
      </div>)}</DataTable> : <div className="list-empty-row">没有匹配的系统信息，请调整搜索条件</div>}
      <div className="security-note"><ShieldCheck size={23} /><div><strong>部署边界</strong><p>控制面默认监听回环地址。对外提供页面时，请通过带 TLS 和访问控制的反向代理暴露，不要直接发布内部状态端口。</p></div></div>
    </> : null}
  </PageScaffold>
}
