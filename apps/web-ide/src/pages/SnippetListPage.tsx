import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { api } from '../lib/api'
import type { Snippet } from '../types'
import LanguageBadge from '../components/LanguageBadge'

export default function SnippetListPage() {
  const [snippets, setSnippets] = useState<Snippet[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  useEffect(() => {
    api
      .listSnippets()
      .then(setSnippets)
      .catch((err) => setError(String(err)))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="border-b border-gray-200 bg-white px-6 py-4">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-bold text-gray-900">Velane</h1>
          <button
            className="inline-flex items-center gap-2 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
            onClick={() => navigate('/snippets/new')}
          >
            <Plus className="h-4 w-4" />
            New Snippet
          </button>
        </div>
      </header>

      <main className="mx-auto max-w-4xl px-6 py-8">
        {loading && <p className="text-sm text-gray-500">Loading snippets...</p>}
        {error && <p className="text-sm text-red-600">{error}</p>}
        {!loading && snippets.length === 0 && (
          <div className="py-16 text-center">
            <p className="text-gray-500">No snippets yet.</p>
            <button
              className="mt-4 text-sm font-medium text-indigo-600 hover:underline"
              onClick={() => navigate('/snippets/new')}
            >
              Create your first snippet
            </button>
          </div>
        )}
        {snippets.length > 0 && (
          <div className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 text-left text-xs font-medium uppercase tracking-wider text-gray-500">
                <tr>
                  <th className="px-6 py-3">Name</th>
                  <th className="px-6 py-3">Language</th>
                  <th className="px-6 py-3">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {snippets.map((sn) => (
                  <tr
                    key={sn.id}
                    className="cursor-pointer hover:bg-gray-50"
                    onClick={() => navigate(`/snippets/${sn.id}`)}
                  >
                    <td className="px-6 py-4 font-medium text-gray-900">{sn.name}</td>
                    <td className="px-6 py-4">
                      <LanguageBadge language={sn.language} />
                    </td>
                    <td className="px-6 py-4 text-gray-500">
                      {new Date(sn.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>
    </div>
  )
}
