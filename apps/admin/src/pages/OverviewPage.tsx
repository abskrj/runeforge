import { useEffect, useState } from 'react'
import { api } from '../lib/api'

interface Stats {
  apiKeyCount: number
  memberCount: number
  invocations24h: number
  snippetCount: number
  loading: boolean
  error: string
}

export default function OverviewPage() {
  const slug = localStorage.getItem('tenantSlug') ?? '—'
  const [stats, setStats] = useState<Stats>({
    apiKeyCount: 0,
    memberCount: 0,
    invocations24h: 0,
    snippetCount: 0,
    loading: true,
    error: '',
  })

  useEffect(() => {
    const load = async () => {
      try {
        const [keys, members, usage, snippets] = await Promise.allSettled([
          api.listAPIKeys(),
          api.listMembers(),
          api.getUsage('24h'),
          api.listSnippets(),
        ])
        setStats({
          apiKeyCount: keys.status === 'fulfilled' ? keys.value.length : 0,
          memberCount: members.status === 'fulfilled' ? members.value.length : 0,
          invocations24h:
            usage.status === 'fulfilled' ? usage.value.total_invocations : 0,
          snippetCount: snippets.status === 'fulfilled' ? snippets.value.length : 0,
          loading: false,
          error: '',
        })
      } catch {
        setStats((s) => ({ ...s, loading: false, error: 'Failed to load stats' }))
      }
    }
    load()
  }, [])

  return (
    <div>
      <h1 className="mb-1 text-2xl font-bold text-gray-900">Overview</h1>
      <p className="mb-8 text-sm text-gray-500">Tenant: <span className="font-medium">{slug}</span></p>

      {stats.error && (
        <div className="mb-6 rounded-md bg-red-50 p-3 text-sm text-red-700">{stats.error}</div>
      )}

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-4">
        <StatCard label="Snippets" value={stats.loading ? '...' : String(stats.snippetCount)} />
        <StatCard label="API Keys" value={stats.loading ? '...' : String(stats.apiKeyCount)} />
        <StatCard label="Team Members" value={stats.loading ? '...' : String(stats.memberCount)} />
        <StatCard label="Invocations (24h)" value={stats.loading ? '...' : String(stats.invocations24h)} />
      </div>
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
      <p className="text-sm font-medium text-gray-500">{label}</p>
      <p className="mt-2 text-3xl font-bold text-gray-900">{value}</p>
    </div>
  )
}
