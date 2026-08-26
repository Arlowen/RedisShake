import { App as AntApp, ConfigProvider } from 'antd'
import { Database, GearSix, ListChecks, Pulse } from '@phosphor-icons/react'
import { Navigate, NavLink, Route, Routes, useLocation } from 'react-router-dom'

import { ConnectionsProvider } from '@/state/ConnectionsContext'
import { TasksProvider } from '@/state/TasksContext'
import ConnectionsView from '@/views/ConnectionsView'
import SystemView from '@/views/SystemView'
import TaskDetailView from '@/views/TaskDetailView'
import TasksView from '@/views/TasksView'

const navigation = [
  { name: 'tasks', label: '同步任务', to: '/tasks', icon: ListChecks },
  { name: 'connections', label: '连接管理', to: '/connections', icon: Database },
  { name: 'system', label: '系统信息', to: '/system', icon: GearSix },
]

export default function App() {
  const location = useLocation()
  const activeName = location.pathname.startsWith('/tasks') ? 'tasks'
    : location.pathname.startsWith('/connections') ? 'connections'
      : location.pathname.startsWith('/system') ? 'system' : ''

  return (
    <ConfigProvider theme={{
      token: {
        colorPrimary: '#326b5b', colorInfo: '#326b5b', colorText: '#18201e',
        colorTextSecondary: '#6f7875', colorBorder: '#e4e8e6', borderRadius: 8,
        colorBgContainer: '#ffffff', boxShadow: 'none', boxShadowSecondary: 'none',
        fontFamily: 'Geist Variable, Geist, system-ui, sans-serif', fontSize: 13, controlHeight: 34,
      },
      components: { Button: { primaryShadow: 'none', dangerShadow: 'none' }, Drawer: { padding: 20, paddingLG: 20 } },
    }}>
      <AntApp>
        <ConnectionsProvider>
          <TasksProvider>
            <div className="app-shell">
              <header className="app-topbar">
                <div className="topbar-inner">
                  <NavLink className="brand" to="/tasks" aria-label="RedisShake Console">
                    <span className="brand-mark"><Pulse size={18} weight="bold" /></span>
                    <strong>RedisShake</strong>
                  </NavLink>
                  <nav className="primary-nav" aria-label="主导航">
                    {navigation.map((item) => {
                      const Icon = item.icon
                      const active = activeName === item.name
                      return (
                        <NavLink key={item.name} to={item.to} aria-label={item.label} className={`nav-item${active ? ' active' : ''}`}>
                          <Icon size={16} weight={active ? 'fill' : 'regular'} />
                          <span>{item.label}</span>
                        </NavLink>
                      )
                    })}
                  </nav>
                  <div className="service-state" title="控制面运行正常"><span className="status-beacon" /><span>Ready</span></div>
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
