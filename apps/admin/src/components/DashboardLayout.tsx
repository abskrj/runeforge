import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  Key,
  Users,
  Paintbrush,
  BarChart2,
  Shield,
  Code,
  MonitorSmartphone,
  LogOut,
  Lock,
  BookOpen,
  Layers,
} from 'lucide-react'
import clsx from 'clsx'
import { api } from '../lib/api'
import { useEmbedMode } from '../hooks/useEmbedMode'

const allNavItems = [
  { to: '/dashboard/overview', label: 'Overview', icon: LayoutDashboard, embedHidden: false },
  { to: '/dashboard/snippets', label: 'Snippets', icon: Code, embedHidden: false },
  { to: '/dashboard/libraries', label: 'Libraries', icon: BookOpen, embedHidden: false },
  { to: '/dashboard/variables', label: 'Variables', icon: Lock, embedHidden: false },
  { to: '/dashboard/api-keys', label: 'API Keys', icon: Key, embedHidden: true },
  { to: '/dashboard/team', label: 'Team', icon: Users, embedHidden: true },
  { to: '/dashboard/branding', label: 'Branding', icon: Paintbrush, embedHidden: true },
  { to: '/dashboard/usage', label: 'Usage', icon: BarChart2, embedHidden: false },
  { to: '/dashboard/egress', label: 'Egress Policy', icon: Shield, embedHidden: true },
  { to: '/dashboard/embed', label: 'Embed', icon: MonitorSmartphone, embedHidden: true },
]

export default function DashboardLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const isEmbedMode = useEmbedMode()
  const isEditorRoute = /^\/dashboard\/(snippets|libraries)\/.+/.test(location.pathname)

  const navItems = allNavItems.filter(item => !isEmbedMode || !item.embedHidden)

  const handleLogout = async () => {
    try {
      await api.logout()
    } catch {
      // ignore — proceed with local logout
    }
    localStorage.removeItem('sessionToken')
    localStorage.removeItem('tenantSlug')
    localStorage.removeItem('apiKey')
    navigate('/login')
  }

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="flex w-64 flex-col border-r border-gray-200 bg-white">
        <div className="flex h-14 items-center gap-2.5 border-b border-gray-200 px-5">
          <div className="flex h-7 w-7 items-center justify-center rounded-md bg-gray-900">
            <Layers size={14} className="text-white" />
          </div>
          <span className="text-sm font-semibold text-gray-900">Velane</span>
        </div>

        <nav className="flex-1 overflow-y-auto px-3 py-4">
          {navItems.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                clsx(
                  'mb-0.5 flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
                  isActive
                    ? 'bg-gray-100 font-medium text-gray-900'
                    : 'font-normal text-gray-500 hover:bg-gray-100 hover:text-gray-800',
                )
              }
            >
              <Icon size={16} />
              {label}
            </NavLink>
          ))}
        </nav>

        {!isEmbedMode && (
          <div className="border-t border-gray-200 p-3">
            <button
              onClick={handleLogout}
              className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-normal text-gray-500 hover:bg-gray-100 hover:text-gray-700"
            >
              <LogOut size={16} />
              Logout
            </button>
          </div>
        )}
      </aside>

      {/* Main content */}
      <main className={clsx('flex-1 overflow-auto', isEditorRoute ? 'flex flex-col p-0' : 'p-8')}>
        <Outlet />
      </main>
    </div>
  )
}
