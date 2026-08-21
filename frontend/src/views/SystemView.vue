<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { PhCheckCircle, PhHardDrive, PhKey, PhPath, PhShieldCheck } from '@phosphor-icons/vue'

import { api } from '@/api/client'
import type { SystemInfo } from '@/api/types'
import InlineError from '@/components/InlineError.vue'
import PageHeader from '@/components/PageHeader.vue'
import StatusPill from '@/components/StatusPill.vue'

const info = ref<SystemInfo>()
const error = ref<string>()
const loading = ref(true)

async function load() {
  loading.value = true; error.value = undefined
  try { info.value = await api.systemInfo() }
  catch (cause) { error.value = cause instanceof Error ? cause.message : '系统信息加载失败' }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="page-wrap">
    <PageHeader eyebrow="Runtime" title="系统信息" description="确认控制面存储、凭据主密钥和 RedisShake worker 的实际运行路径。" />
    <InlineError v-if="error" :message="error" @retry="load" />
    <div v-if="loading" class="skeleton-list"><div v-for="i in 4" :key="i" class="skeleton-row" /></div>
    <template v-else-if="info">
      <div class="system-hero">
        <div><span class="system-icon"><PhCheckCircle :size="30" weight="fill" /></span><div><small>控制面状态</small><h2>服务与 SQLite 正常</h2><p>版本 {{ info.version }} · commit {{ info.git_commit }}</p></div></div>
        <StatusPill label="Ready" tone="success" pulse />
      </div>
      <div class="section-heading"><h2>运行配置</h2><span>来自当前进程，不是静态文档</span></div>
      <div class="system-list">
        <div><PhHardDrive :size="21" /><span><small>元数据存储</small><strong>{{ info.storage }}</strong></span><code>{{ info.data_dir }}</code></div>
        <div><PhPath :size="21" /><span><small>运行目录</small><strong>Task / Run artifacts</strong></span><code>{{ info.runtime_dir }}</code></div>
        <div><PhShieldCheck :size="21" /><span><small>RedisShake Worker</small><strong>独立进程模式</strong></span><code>{{ info.worker_path }}</code></div>
        <div><PhKey :size="21" /><span><small>凭据加密</small><strong>{{ info.secrets_configured ? '主密钥已配置' : '主密钥未配置' }}</strong></span><StatusPill :label="info.secrets_configured ? '可保存凭据' : '只允许无密码连接'" :tone="info.secrets_configured ? 'success' : 'warning'" /></div>
        <div><PhShieldCheck :size="21" /><span><small>Web 控制台</small><strong>{{ info.web_ui_configured ? '由控制面直接提供' : '开发服务器模式' }}</strong></span><StatusPill :label="info.web_ui_configured ? '单进程部署' : 'Vite proxy'" tone="neutral" /></div>
        <div><PhHardDrive :size="21" /><span><small>运行约束</small><strong>最多 {{ info.max_concurrent_runs }} 个活动 Run</strong></span><code>日志保留 {{ info.log_retention_days === 0 ? '不限期' : `${info.log_retention_days} 天` }}</code></div>
      </div>
      <div class="security-note"><PhShieldCheck :size="23" /><div><strong>部署边界</strong><p>控制面默认监听回环地址。对外提供页面时，请通过带 TLS 和访问控制的反向代理暴露，不要直接发布内部状态端口。</p></div></div>
    </template>
  </div>
</template>

<style scoped>
.system-hero{display:flex;align-items:center;justify-content:space-between;gap:24px;padding:24px 0;border-top:1px solid var(--line);border-bottom:1px solid var(--line)}.system-hero>div{display:flex;align-items:center;gap:15px}.system-icon{width:54px;height:54px;display:grid;place-items:center;color:var(--accent);border-radius:15px;background:#e9f2ee}.system-hero small{color:var(--muted)}.system-hero h2{margin:4px 0 0;font-size:20px}.system-hero p{margin:4px 0 0;color:var(--muted);font-size:11px}.system-list{background:#fff;border:1px solid var(--line);border-radius:14px;overflow:hidden}.system-list>div{display:grid;grid-template-columns:24px minmax(180px,.7fr) 1.3fr;align-items:center;gap:14px;padding:17px 19px;border-bottom:1px solid #e8eeeb}.system-list>div:last-child{border-bottom:0}.system-list>div>svg{color:var(--accent)}.system-list small,.system-list strong{display:block}.system-list small{color:var(--muted);font-size:10px}.system-list strong{margin-top:3px;font-size:12px}.system-list code{overflow:hidden;color:#51605b;font-size:11px;text-overflow:ellipsis}.security-note{display:flex;gap:14px;margin-top:22px;padding:18px;color:#365c50;border:1px solid #cfe0d9;border-radius:12px;background:#edf5f1}.security-note strong{font-size:13px}.security-note p{max-width:700px;margin:4px 0 0;font-size:11px;line-height:1.55}@media(max-width:700px){.system-list>div{grid-template-columns:24px 1fr}.system-list code,.system-list .status-pill{grid-column:2}.system-hero{align-items:flex-start}}
</style>
