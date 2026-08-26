import { App as AntApp, ConfigProvider, theme as antdTheme } from 'antd'
import { List, Moon, Sun, X } from '@phosphor-icons/react'
import { useEffect, useState } from 'react'
import { Navigate, NavLink, Route, Routes, useLocation } from 'react-router-dom'

import { ConnectionsProvider } from '@/state/ConnectionsContext'
import { TasksProvider } from '@/state/TasksContext'
import { initialTheme } from '@/utils/theme'
import type { ThemeMode } from '@/utils/theme'
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
  const [themeMode, setThemeMode] = useState<ThemeMode>(initialTheme)
  const [menuOpen, setMenuOpen] = useState(false)
  const dark = themeMode === 'dark'
  const activeName = location.pathname.startsWith('/tasks') ? 'tasks'
    : location.pathname.startsWith('/connections') ? 'connections'
      : location.pathname.startsWith('/system') ? 'system' : ''

  useEffect(() => {
    document.documentElement.dataset.theme = themeMode
    document.documentElement.style.colorScheme = themeMode
    localStorage.setItem('redisshake-theme', themeMode)
  }, [themeMode])

  useEffect(() => { setMenuOpen(false) }, [location.pathname])

  useEffect(() => {
    if (!menuOpen) return
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') setMenuOpen(false) }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [menuOpen])

  return (
    <ConfigProvider theme={{
      algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
      token: {
        colorPrimary: dark ? '#3e63dd' : '#5672cd', colorInfo: dark ? '#3e63dd' : '#5672cd',
        colorText: dark ? 'rgba(255,255,245,.86)' : '#3c3c43', colorTextSecondary: dark ? 'rgba(235,235,245,.6)' : 'rgba(60,60,67,.78)',
        colorBorder: dark ? '#2e2e32' : '#e2e2e3', borderRadius: 12,
        colorBgBase: dark ? '#1b1b1f' : '#ffffff', colorBgContainer: dark ? '#202127' : '#f6f6f7', colorBgElevated: dark ? '#202127' : '#ffffff', colorFillAlter: dark ? '#32363f' : '#ebebef',
        boxShadow: 'none', boxShadowSecondary: 'none',
        fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif', fontSize: 14, controlHeight: 40,
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
              <header className="app-topbar">
                <div className="topbar-inner">
                  <NavLink className="brand" to="/tasks" aria-label="RedisShake Console">
                    <strong>RedisShake</strong>
                  </NavLink>
                  <span />
                  <div className="topbar-actions">
                    <button type="button" className="appearance-toggle" aria-label={dark ? '切换到浅色模式' : '切换到深色模式'} onClick={() => setThemeMode(dark ? 'light' : 'dark')}>{dark ? <Sun size={16} /> : <Moon size={16} />}</button>
                    <div className="service-state" title="控制面运行正常"><span className="status-beacon" /><span>Ready</span></div>
                    <button type="button" className="mobile-menu-button" aria-label={menuOpen ? '关闭侧边栏' : '打开侧边栏'} aria-controls="app-sidebar" aria-expanded={menuOpen} onClick={() => setMenuOpen((open) => !open)}>{menuOpen ? <X size={21} /> : <List size={21} />}</button>
                  </div>
                </div>
              </header>
              <div className="app-body">
                {menuOpen ? <button type="button" className="sidebar-backdrop" aria-label="关闭侧边栏" onClick={() => setMenuOpen(false)} /> : null}
                <aside id="app-sidebar" className={`app-sidebar${menuOpen ? ' open' : ''}`}>
                  <div className="sidebar-scroll">
                    <span className="sidebar-label">数据同步</span>
                    <nav className="sidebar-nav" aria-label="主导航">
                      {navigation.slice(0, 2).map((item) => <NavLink key={item.name} to={item.to} aria-label={item.label} className={`sidebar-item${activeName === item.name ? ' active' : ''}`}>{item.label}</NavLink>)}
                    </nav>
                    <span className="sidebar-label">系统</span>
                    <nav className="sidebar-nav" aria-label="系统导航">
                      {navigation.slice(2).map((item) => <NavLink key={item.name} to={item.to} aria-label={item.label} className={`sidebar-item${activeName === item.name ? ' active' : ''}`}>{item.label}</NavLink>)}
                    </nav>
                  </div>
                  <div className="sidebar-footer"><span className="status-beacon" /><span>Control plane ready</span></div>
                </aside>
                <main className="app-main">
                  <Routes>
                    <Route path="/tasks" element={<TasksView />} />
                    <Route path="/tasks/new" element={<TaskEditorPage />} />
                    <Route path="/tasks/:id/edit" element={<TaskEditorPage />} />
                    <Route path="/tasks/:id" element={<TaskDetailView />} />
                    <Route path="/connections" element={<ConnectionsView />} />
                    <Route path="/system" element={<SystemView />} />
                    <Route path="*" element={<Navigate to="/tasks" replace />} />
                  </Routes>
                </main>
              </div>
            </div>
          </TasksProvider>
        </ConnectionsProvider>
      </AntApp>
    </ConfigProvider>
  )
}
