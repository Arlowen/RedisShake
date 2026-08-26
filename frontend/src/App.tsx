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

  const navigationLinks = navigation.map((item) => {
    const active = activeName === item.name
    return <NavLink key={item.name} to={item.to} aria-label={item.label} className={`nav-item${active ? ' active' : ''}`}>{item.label}</NavLink>
  })

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
                  <nav className="primary-nav" aria-label="主导航">{navigationLinks}</nav>
                  <div className="topbar-actions">
                    <button type="button" className="appearance-toggle" aria-label={dark ? '切换到浅色模式' : '切换到深色模式'} onClick={() => setThemeMode(dark ? 'light' : 'dark')}>{dark ? <Sun size={16} /> : <Moon size={16} />}</button>
                    <div className="service-state" title="控制面运行正常"><span className="status-beacon" /><span>Ready</span></div>
                    <button type="button" className="mobile-menu-button" aria-label={menuOpen ? '关闭导航菜单' : '打开导航菜单'} aria-controls="mobile-navigation" aria-expanded={menuOpen} onClick={() => setMenuOpen((open) => !open)}>{menuOpen ? <X size={21} /> : <List size={21} />}</button>
                  </div>
                  {menuOpen ? <div id="mobile-navigation" className="mobile-nav-panel"><nav aria-label="移动导航">{navigationLinks}</nav><button type="button" className="mobile-theme-action" onClick={() => setThemeMode(dark ? 'light' : 'dark')}><span>外观</span><span>{dark ? '浅色模式' : '深色模式'}</span></button><div className="mobile-ready"><span className="status-beacon" />Ready</div></div> : null}
                </div>
              </header>
              <main className="app-main">
                <Routes>
                  <Route path="/tasks" element={<TasksView />} />
                  <Route path="/tasks/:id" element={<TaskDetailView />} />
                  <Route path="/connections" element={<ConnectionsView />} />
                  <Route path="/system" element={<SystemView />} />
                  <Route path="*" element={<Navigate to="/tasks" replace />} />
                </Routes>
              </main>
            </div>
          </TasksProvider>
        </ConnectionsProvider>
      </AntApp>
    </ConfigProvider>
  )
}
