import { App as AntApp, ConfigProvider, Dropdown, theme as antdTheme } from 'antd'
import { Desktop, List, Moon, Plus, Sun, X } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'
import { Navigate, NavLink, Route, Routes, useLocation } from 'react-router-dom'

import { ConnectionsProvider } from '@/state/ConnectionsContext'
import { TasksProvider } from '@/state/TasksContext'
import { initialThemePreference, resolveTheme, systemTheme } from '@/utils/theme'
import type { ThemeMode, ThemePreference } from '@/utils/theme'
import { resolvePageContext } from '@/utils/navigation'
import ConnectionCreatePage from '@/views/ConnectionCreatePage'
import ConnectionsView from '@/views/ConnectionsView'
import SystemView from '@/views/SystemView'
import TaskDetailView from '@/views/TaskDetailView'
import TaskEditorPage from '@/views/TaskEditorPage'
import TasksView from '@/views/TasksView'

const navigation = [
  { name: 'tasks', label: '同步任务', to: '/tasks' },
  { name: 'connections', label: '连接管理', to: '/connections' },
  { name: 'system', label: '系统信息', to: '/system' },
]

export default function App() {
  const location = useLocation()
  const [themePreference, setThemePreference] = useState<ThemePreference>(initialThemePreference)
  const [systemMode, setSystemMode] = useState<ThemeMode>(systemTheme)
  const [menuOpen, setMenuOpen] = useState(false)
  const themeMode = resolveTheme(themePreference, systemMode)
  const dark = themeMode === 'dark'
  const pageContext = resolvePageContext(location.pathname)
  const activeName = location.pathname.startsWith('/tasks') ? 'tasks'
    : location.pathname.startsWith('/connections') ? 'connections'
      : location.pathname.startsWith('/system') ? 'system' : ''

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const update = () => setSystemMode(media.matches ? 'dark' : 'light')
    update()
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [])

  useEffect(() => {
    document.documentElement.dataset.theme = themeMode
    document.documentElement.style.colorScheme = themeMode
    localStorage.setItem('redisshake-theme', themePreference)
  }, [themeMode, themePreference])

  useEffect(() => { setMenuOpen(false) }, [location.pathname])

  useEffect(() => {
    if (!menuOpen) return
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') setMenuOpen(false) }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [menuOpen])

  const themeLabel = themePreference === 'system' ? '跟随系统' : themePreference === 'dark' ? '深色模式' : '浅色模式'
  const themeIcon = themePreference === 'system' ? <Desktop size={16} /> : themePreference === 'dark' ? <Moon size={16} /> : <Sun size={16} />

  return (
    <ConfigProvider theme={{
      algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
      token: {
        colorPrimary: dark ? '#3e63dd' : '#5672cd', colorInfo: dark ? '#3e63dd' : '#5672cd',
        colorText: dark ? 'rgba(255,255,245,.86)' : '#3c3c43', colorTextSecondary: dark ? 'rgba(235,235,245,.6)' : 'rgba(60,60,67,.78)',
        colorBorder: dark ? '#2e2e32' : '#e2e2e3', borderRadius: 12,
        colorBgBase: dark ? '#1b1b1f' : '#ffffff', colorBgContainer: dark ? '#202127' : '#f6f6f7', colorBgElevated: dark ? '#202127' : '#ffffff', colorFillAlter: dark ? '#32363f' : '#ebebef',
        boxShadow: 'none', boxShadowSecondary: 'none',
        fontFamily: 'Geist, "SF Pro Text", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif', fontSize: 14, controlHeight: 40,
      },
      components: {
        Button: { borderRadius: 20, primaryShadow: 'none', dangerShadow: 'none', defaultBg: dark ? '#32363f' : '#ebebef', defaultBorderColor: 'transparent' },
        Drawer: { padding: 24, paddingLG: 24 },
      },
    }}>
      <AntApp>
        <ConnectionsProvider>
          <TasksProvider>
            <div className="app-shell">
              {menuOpen ? <button type="button" className="sidebar-backdrop" aria-label="关闭侧边栏" onClick={() => setMenuOpen(false)} /> : null}
              <aside id="app-sidebar" className={`app-sidebar${menuOpen ? ' open' : ''}`}>
                <div className="sidebar-brand">
                  <NavLink className="brand" to="/tasks" aria-label="RedisShake Console">
                    <strong>RedisShake</strong>
                  </NavLink>
                </div>
                <div className="sidebar-scroll">
                  <nav className="sidebar-nav" aria-label="主导航">
                    {navigation.map((item) => <NavLink key={item.name} to={item.to} aria-label={item.label} className={`sidebar-item${activeName === item.name ? ' active' : ''}`}>{item.label}</NavLink>)}
                  </nav>
                </div>
                <div className="sidebar-footer"><span className="status-beacon" /><span>Control plane ready</span></div>
              </aside>
              <section className="app-workspace">
                <header className="app-topbar">
                  <div className="topbar-inner">
                    <NavLink className="brand mobile-brand" to="/tasks" aria-label="RedisShake Console"><strong>RedisShake</strong></NavLink>
                    <div className="topbar-context" aria-label={`${pageContext.parent} / ${pageContext.title}`}>
                      <span>{pageContext.parent}</span><span className="topbar-separator">/</span><h1>{pageContext.title}</h1>
                    </div>
                  <div className="topbar-actions">
                    {pageContext.action ? <NavLink className="topbar-primary-action" to={pageContext.action.to} aria-label={pageContext.action.label}><Plus size={16} /><span>{pageContext.action.label}</span></NavLink> : null}
                    <Dropdown trigger={['click']} placement="bottomRight" menu={{ selectable: true, selectedKeys: [themePreference], items: [{ key: 'system', icon: <Desktop size={16} />, label: '跟随系统' }, { key: 'light', icon: <Sun size={16} />, label: '浅色模式' }, { key: 'dark', icon: <Moon size={16} />, label: '深色模式' }], onClick: ({ key }) => setThemePreference(key as ThemePreference) }}><button type="button" className="appearance-toggle" aria-label={`外观设置：${themeLabel}`} aria-haspopup="menu" title={themeLabel}>{themeIcon}</button></Dropdown>
                    <div className="service-state" title="控制面运行正常"><span className="status-beacon" /><span>Ready</span></div>
                    <button type="button" className="mobile-menu-button" aria-label={menuOpen ? '关闭侧边栏' : '打开侧边栏'} aria-controls="app-sidebar" aria-expanded={menuOpen} onClick={() => setMenuOpen((open) => !open)}>{menuOpen ? <X size={21} /> : <List size={21} />}</button>
                  </div>
                  </div>
                </header>
                <main className="app-main">
                  <Routes>
                    <Route path="/tasks" element={<TasksView />} />
                    <Route path="/tasks/new" element={<TaskEditorPage />} />
                    <Route path="/tasks/:id/edit" element={<TaskEditorPage />} />
                    <Route path="/tasks/:id" element={<TaskDetailView />} />
                    <Route path="/connections" element={<ConnectionsView />} />
                    <Route path="/connections/new" element={<ConnectionCreatePage />} />
                    <Route path="/system" element={<SystemView />} />
                    <Route path="*" element={<Navigate to="/tasks" replace />} />
                  </Routes>
                </main>
              </section>
            </div>
          </TasksProvider>
        </ConnectionsProvider>
      </AntApp>
    </ConfigProvider>
  )
}
