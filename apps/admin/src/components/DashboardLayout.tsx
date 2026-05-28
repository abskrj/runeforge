import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  Key,
  Users,
  Paintbrush,
  BarChart2,
  Shield,
  Code,
  LogOut,
} from 'lucide-react'
import clsx from 'clsx'
import { api } from '../lib/api'

const navItems = [
  { to: '/dashboard/overview', label: 'Overview', icon: LayoutDashboard },
  { to: '/dashboard/snippets', label: 'Snippets', icon: Code },
  { to: '/dashboard/api-keys', label: 'API Keys', icon: Key },
  { to: '/dashboard/team', label: 'Team', icon: Users },
  { to: '/dashboard/branding', label: 'Branding', icon: Paintbrush },
  { to: '/dashboard/usage', label: 'Usage', icon: BarChart2 },
  { to: '/dashboard/egress', label: 'Egress Policy', icon: Shield },
]

export default function DashboardLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const isEditorRoute = /^\/dashboard\/snippets\/.+/.test(location.pathname)

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
      <aside className="flex w-56 flex-col border-r border-gray-200 bg-white">
        <div className="flex h-14 items-center border-b border-gray-200 px-4">
          <span className="text-lg font-bold text-indigo-600">Runeforge</span>
        </div>

        <nav className="flex-1 overflow-y-auto px-2 py-4">
          {navItems.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                clsx(
                  'mb-1 flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-indigo-50 text-indigo-700'
                    : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900',
                )
              }
            >
              <Icon size={16} />
              {label}
            </NavLink>
          ))}
        </nav>

        <div className="border-t border-gray-200 p-2">
          <button
            onClick={handleLogout}
            className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-red-600 hover:bg-red-50"
          >
            <LogOut size={16} />
            Logout
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className={clsx('flex-1 overflow-auto', isEditorRoute ? 'flex flex-col p-0' : 'p-8')}>
        <Outlet />
      </main>
    </div>
  )
}
