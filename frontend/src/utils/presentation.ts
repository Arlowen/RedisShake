import type { CheckState, RunState, TaskMode, TaskState, Topology } from '@/api/types'

export const taskStateMeta: Record<TaskState, { label: string; tone: string }> = {
  DRAFT: { label: '草稿', tone: 'neutral' },
  READY: { label: '可启动', tone: 'success' },
  ARCHIVED: { label: '已归档', tone: 'neutral' },
}

export const runStateMeta: Record<RunState, { label: string; tone: string }> = {
  STARTING: { label: '启动中', tone: 'active' },
  RUNNING: { label: '运行中', tone: 'active' },
  STOPPING: { label: '停止中', tone: 'warning' },
  STOPPED: { label: '已停止', tone: 'neutral' },
  SUCCEEDED: { label: '已完成', tone: 'success' },
  FAILED: { label: '失败', tone: 'danger' },
  UNKNOWN: { label: '状态未知', tone: 'warning' },
}

export const checkStateMeta: Record<CheckState, { label: string; tone: string }> = {
  PASS: { label: '通过', tone: 'success' },
  WARNING: { label: '警告', tone: 'warning' },
  FAIL: { label: '阻断', tone: 'danger' },
}

export const topologyLabel: Record<Topology, string> = {
  standalone: '单机 / 主从', sentinel: 'Sentinel', cluster: 'Cluster',
}

export const modeLabel: Record<TaskMode, string> = { sync: '增量同步', scan: '扫描迁移' }

export function formatDate(value?: string) {
  if (!value) return '—'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value))
}

export function formatNumber(value?: number) {
  if (value === undefined) return '—'
  return new Intl.NumberFormat('zh-CN').format(value)
}

export function shortId(value: string) {
  return `${value.slice(0, 7)}…${value.slice(-4)}`
}
