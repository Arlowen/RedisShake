import { ShieldCheck } from '@phosphor-icons/react'
import { useCallback, useEffect, useState } from 'react'

import { api } from '@/api/client'
import type { SystemInfo } from '@/api/types'
import InlineError from '@/components/InlineError'
import PageHeader from '@/components/PageHeader'

export default function SystemView() {
  const [info, setInfo] = useState<SystemInfo>()
  const [error, setError] = useState<string>()
  const [loading, setLoading] = useState(true)
  const load = useCallback(async () => { setLoading(true); setError(undefined); try { setInfo(await api.systemInfo()) } catch (cause) { setError(cause instanceof Error ? cause.message : '系统信息加载失败') } finally { setLoading(false) } }, [])
  useEffect(() => { void load() }, [load])

  return <div className="page-wrap">
    <PageHeader title="系统信息" description="查看当前控制面、存储和 RedisShake 运行配置。" />
    {error ? <InlineError message={error} onRetry={() => void load()} /> : null}
    {loading ? <div className="skeleton-list">{[0, 1, 2, 3].map((item) => <div key={item} className="skeleton-row" />)}</div> : info ? <>
      <div className="system-status-line"><span className="status-beacon" /><strong>Ready</strong><span>SQLite 正常 · {info.version} · {info.git_commit}</span></div>
      <div className="system-list">
        <div><span><small>元数据</small><strong>{info.storage}</strong></span><code>{info.data_dir}</code></div>
        <div><span><small>运行目录</small><strong>Task / Run artifacts</strong></span><code>{info.runtime_dir}</code></div>
        <div><span><small>RedisShake Worker</small><strong>独立进程</strong></span><code>{info.worker_path}</code></div>
        <div><span><small>凭据</small><strong>{info.secrets_configured ? '主密钥已配置' : '主密钥未配置'}</strong></span><span className={info.secrets_configured ? 'value-ok' : 'value-warning'}>{info.secrets_configured ? '可保存凭据' : '仅无密码连接'}</span></div>
        <div><span><small>Web 控制台</small><strong>{info.web_ui_configured ? '已内嵌' : '未编译'}</strong></span><span className={info.web_ui_configured ? 'value-ok' : 'value-warning'}>{info.web_ui_configured ? 'Go embed.FS' : 'API only'}</span></div>
        <div><span><small>运行约束</small><strong>最多 {info.max_concurrent_runs} 个活动 Run</strong></span><code>日志保留 {info.log_retention_days === 0 ? '不限期' : `${info.log_retention_days} 天`}</code></div>
      </div>
      <div className="security-note"><ShieldCheck size={23} /><div><strong>部署边界</strong><p>控制面默认监听回环地址。对外提供页面时，请通过带 TLS 和访问控制的反向代理暴露，不要直接发布内部状态端口。</p></div></div>
    </> : null}
  </div>
}
