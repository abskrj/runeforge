import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { UsageSummary } from '../types'

type Window = '24h' | '7d' | '30d'

export default function UsagePage() {
  const [window, setWindow] = useState<Window>('24h')
  const [data, setData] = useState<UsageSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    api
      .getUsage(window)
      .then(setData)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load usage'))
      .finally(() => setLoading(false))
  }, [window])

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Usage</h1>
        <div className="flex gap-1 rounded-md border border-gray-200 bg-white p-1">
          {(['24h', '7d', '30d'] as Window[]).map((w) => (
            <button
              key={w}
              onClick={() => setWindow(w)}
              className={`rounded px-3 py-1 text-sm font-medium transition-colors ${
                window === w ? 'bg-gray-900 text-white' : 'text-gray-600 hover:bg-gray-100'
              }`}
            >
              {w}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>
      )}

      {loading ? (
        <p className="text-sm text-gray-500">Loading...</p>
      ) : data ? (
        <>
          <div className="mb-8 grid grid-cols-1 gap-6 sm:grid-cols-3">
            <StatCard label="Total Invocations" value={String(data.total_invocations)} />
            <StatCard label="Error Rate" value={`${(data.error_rate * 100).toFixed(1)}%`} />
            <StatCard label="Avg Duration" value={`${data.avg_duration_ms.toFixed(1)} ms`} />
          </div>

          {data.top_snippets.length > 0 && (
            <div className="rounded-lg border border-gray-200 bg-white shadow-sm">
              <h2 className="border-b border-gray-200 px-4 py-3 text-base font-semibold text-gray-900">
                Top Snippets
              </h2>
              <table className="w-full text-sm">
                <thead className="border-b border-gray-200 bg-gray-50 text-left">
                  <tr>
                    <th className="px-4 py-3 font-medium text-gray-600">Snippet</th>
                    <th className="px-4 py-3 font-medium text-gray-600">Invocations</th>
                    <th className="px-4 py-3 font-medium text-gray-600">p95 Latency</th>
                  </tr>
                </thead>
                <tbody>
                  {data.top_snippets.map((s) => (
                    <tr key={s.snippet_id} className="border-b border-gray-100 last:border-0">
                      <td className="px-4 py-3 font-medium text-gray-900">{s.name}</td>
                      <td className="px-4 py-3 text-gray-600">{s.invocations.toLocaleString()}</td>
                      <td className="px-4 py-3 text-gray-600">{s.p95_ms.toFixed(1)} ms</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      ) : null}
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
