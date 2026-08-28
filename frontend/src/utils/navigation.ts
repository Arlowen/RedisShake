export interface PageContext {
  parent?: string
  title: string
  action?: {
    label: string
    to: string
  }
}

export function resolvePageContext(pathname: string): PageContext {
  if (pathname === '/tasks/new') return { parent: '同步任务', title: '创建任务' }
  if (/^\/tasks\/[^/]+\/edit$/.test(pathname)) return { parent: '同步任务', title: '编辑任务' }
  if (/^\/tasks\/[^/]+$/.test(pathname)) return { parent: '同步任务', title: '任务详情' }
  if (pathname === '/connections/new') return { parent: '连接管理', title: '新建连接' }
  if (pathname.startsWith('/connections')) return { title: '连接管理' }
  if (pathname.startsWith('/system')) return { parent: '系统', title: '系统信息' }
  return { title: '同步任务' }
}
