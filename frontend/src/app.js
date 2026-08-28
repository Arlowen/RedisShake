import { escapeHtml } from './lib.js'
import { icon, initSearches, initSelects, select } from './components.js'
import { mountConnectionEditor, mountConnections, mountSystem, mountTaskDetail, mountTaskEditor, mountTasks } from './pages.js'

const app = document.querySelector('#app')
let cleanup
let themePreference = localStorage.getItem('redisshake-theme') || 'system'

const routes = [
  [/^\/tasks\/new$/, ['同步任务', '创建任务'], (root) => mountTaskEditor(root, navigate)],
  [/^\/tasks\/([^/]+)\/edit$/, ['同步任务', '编辑任务'], (root, match) => mountTaskEditor(root, navigate, match[1])],
  [/^\/tasks\/([^/]+)$/, ['同步任务', '任务详情'], (root, match) => mountTaskDetail(root, navigate, match[1])],
  [/^\/connections\/new$/, ['连接管理', '新建连接'], (root) => mountConnectionEditor(root, navigate)],
  [/^\/connections$/, ['', '连接管理'], (root) => mountConnections(root, navigate)],
  [/^\/system$/, ['', '系统信息'], (root) => mountSystem(root)],
  [/^\/tasks$/, ['', '同步任务'], (root) => mountTasks(root, navigate)],
]

function resolveRoute(pathname) {
  for (const [pattern, context, mount] of routes) {
    const match = pathname.match(pattern)
    if (match) return { context, mount, match }
  }
  return { context: ['', '同步任务'], mount: (root) => mountTasks(root, navigate), match: [] }
}

function applyTheme() {
  const resolved = themePreference === 'system' ? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light') : themePreference
  document.documentElement.dataset.theme = resolved
  document.documentElement.style.colorScheme = resolved
}

function shell(context) {
  const active = location.pathname.startsWith('/connections') ? 'connections' : location.pathname.startsWith('/system') ? 'system' : 'tasks'
  return `<div class="app-shell">
    <aside class="sidebar" id="sidebar"><a href="/tasks" data-link class="brand"><strong>RedisShake</strong></a>
      <nav aria-label="主导航"><a href="/tasks" data-link class="${active === 'tasks' ? 'active' : ''}"><span>同步任务</span><small>Tasks</small></a><a href="/connections" data-link class="${active === 'connections' ? 'active' : ''}"><span>连接管理</span><small>Connections</small></a><a href="/system" data-link class="${active === 'system' ? 'active' : ''}"><span>系统信息</span><small>System</small></a></nav>
      <div class="sidebar-status"><i></i><span><strong>Control plane</strong><small>Ready</small></span></div>
    </aside>
    <section class="workspace"><header class="topbar"><div class="mobile-brand">RedisShake</div><div class="page-context">${context[0] ? `<span>${escapeHtml(context[0])}</span><i>/</i>` : ''}<h1>${escapeHtml(context[1])}</h1></div><div class="topbar-tools">${select('theme-select', '外观设置', themePreference, [['system', '跟随系统'], ['light', '浅色模式'], ['dark', '深色模式']], { align: 'end', size: 'compact' })}<span class="ready-dot"><i></i>Ready</span><button id="mobile-menu" aria-label="打开侧边栏">${icon('menu', 20)}</button></div></header><main id="page-root"></main></section>
    <button id="sidebar-backdrop" aria-label="关闭侧边栏"></button><div id="toast-root" aria-live="polite"></div>
  </div>`
}

async function render() {
  if (typeof cleanup === 'function') cleanup()
  const route = resolveRoute(location.pathname)
  app.innerHTML = shell(route.context)
  applyTheme()
  document.title = `${route.context[1]} · RedisShake`
  document.querySelectorAll('[data-link]').forEach((link) => link.addEventListener('click', (event) => { event.preventDefault(); navigate(link.getAttribute('href')) }))
  document.querySelector('#theme-select').addEventListener('change', (event) => { themePreference = event.target.value; localStorage.setItem('redisshake-theme', themePreference); applyTheme() })
  const sidebar = document.querySelector('#sidebar')
  const backdrop = document.querySelector('#sidebar-backdrop')
  document.querySelector('#mobile-menu').addEventListener('click', () => { sidebar.classList.toggle('open'); backdrop.classList.toggle('visible') })
  backdrop.addEventListener('click', () => { sidebar.classList.remove('open'); backdrop.classList.remove('visible') })
  cleanup = await route.mount(document.querySelector('#page-root'), route.match)
}

export function navigate(path) {
  history.pushState({}, '', path)
  void render()
}

window.addEventListener('popstate', render)
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { if (themePreference === 'system') applyTheme() })
initSelects()
initSearches()
void render()
