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
        colorPrimary: '#34705f', colorInfo: '#34705f', colorText: '#17211f',
        colorTextSecondary: '#66736f', colorBorder: '#dce3e0', borderRadius: 10,
        fontFamily: 'Geist Variable, Geist, system-ui, sans-serif', controlHeight: 38,
      },
    }}>
      <AntApp>
        <ConnectionsProvider>
          <TasksProvider>
            <div className="app-shell">
              <aside className="app-sidebar">
                <NavLink className="brand" to="/tasks" aria-label="RedisShake Console">
                  <span className="brand-mark"><Pulse size={22} weight="bold" /></span>
                  <span><strong>RedisShake</strong><small>Control plane</small></span>
                </NavLink>
                <nav className="primary-nav" aria-label="主导航">
                  {navigation.map((item) => {
                    const Icon = item.icon
                    const active = activeName === item.name
                    return (
                      <NavLink key={item.name} to={item.to} aria-label={item.label} className={`nav-item${active ? ' active' : ''}`}>
                        <Icon size={19} weight={active ? 'fill' : 'regular'} />
                        <span>{item.label}</span>
                      </NavLink>
                    )
                  })}
                </nav>
                <div className="sidebar-foot">
                  <span className="status-beacon" />
                  <div><strong>本地控制面</strong><small>仅监听受控网络</small></div>
                </div>
              </aside>
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
