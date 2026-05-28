import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Trash2 } from 'lucide-react'
import { api } from '../lib/api'
import type { Snippet } from '../types'
import LanguageBadge from '../components/LanguageBadge'

export default function SnippetsPage() {
  const navigate = useNavigate()
  const [snippets, setSnippets] = useState<Snippet[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [newName, setNewName] = useState('')
  const [newLanguage, setNewLanguage] = useState<'bun' | 'python'>('bun')
  const [newDescription, setNewDescription] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    load()
  }, [])

  async function load() {
    setLoading(true)
    try {
      const data = await api.listSnippets()
      setSnippets(data)
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }

  async function handleCreate() {
    if (!newName.trim()) return
    setCreating(true)
    try {
      const sn = await api.createSnippet({
        name: newName.trim(),
        language: newLanguage,
        description: newDescription.trim() || undefined,
      })
      setShowModal(false)
      setNewName('')
      setNewLanguage('bun')
      setNewDescription('')
      navigate(`/dashboard/snippets/${sn.id}`)
    } catch (err) {
      setError(String(err))
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(e: React.MouseEvent, id: string) {
    e.stopPropagation()
    if (!confirm('Delete this snippet? This cannot be undone.')) return
    try {
      await api.deleteSnippet(id)
      setSnippets((prev) => prev.filter((s) => s.id !== id))
    } catch (err) {
      setError(String(err))
    }
  }

  return (
    <div>
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Snippets</h1>
          <p className="mt-1 text-sm text-gray-500">Manage your code snippets and deployments</p>
        </div>
        <button
          className="inline-flex items-center gap-2 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
          onClick={() => setShowModal(true)}
        >
          <Plus size={16} />
          New Snippet
        </button>
      </div>

      {error && (
        <div className="mb-6 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>
      )}

      {loading && (
        <p className="text-sm text-gray-500">Loading snippets...</p>
      )}

      {!loading && snippets.length === 0 && (
        <div className="rounded-lg border border-dashed border-gray-300 bg-white py-16 text-center">
          <p className="text-gray-500">No snippets yet.</p>
          <button
            className="mt-4 text-sm font-medium text-indigo-600 hover:underline"
            onClick={() => setShowModal(true)}
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
                <th className="px-6 py-3">Slug</th>
                <th className="px-6 py-3">Created</th>
                <th className="px-6 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {snippets.map((sn) => (
                <tr
                  key={sn.id}
                  className="cursor-pointer hover:bg-gray-50"
                  onClick={() => navigate(`/dashboard/snippets/${sn.id}`)}
                >
                  <td className="px-6 py-4 font-medium text-gray-900">{sn.name}</td>
                  <td className="px-6 py-4">
                    <LanguageBadge language={sn.language} />
                  </td>
                  <td className="px-6 py-4 font-mono text-xs text-gray-500">{sn.slug}</td>
                  <td className="px-6 py-4 text-gray-500">
                    {new Date(sn.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button
                      className="rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600"
                      onClick={(e) => handleDelete(e, sn.id)}
                      title="Delete snippet"
                    >
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
          <div className="w-full max-w-md rounded-xl border border-gray-200 bg-white p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-semibold text-gray-900">New Snippet</h2>

            <div className="mb-4">
              <label className="mb-1 block text-sm font-medium text-gray-700">Name</label>
              <input
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                placeholder="My Snippet"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                autoFocus
              />
            </div>

            <div className="mb-4">
              <label className="mb-1 block text-sm font-medium text-gray-700">Language</label>
              <select
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                value={newLanguage}
                onChange={(e) => setNewLanguage(e.target.value as 'bun' | 'python')}
              >
                <option value="bun">Bun (TypeScript)</option>
                <option value="python">Python</option>
              </select>
            </div>

            <div className="mb-6">
              <label className="mb-1 block text-sm font-medium text-gray-700">
                Description <span className="font-normal text-gray-400">(optional)</span>
              </label>
              <input
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                placeholder="What does this snippet do?"
                value={newDescription}
                onChange={(e) => setNewDescription(e.target.value)}
              />
            </div>

            <div className="flex justify-end gap-3">
              <button
                className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
                onClick={() => setShowModal(false)}
              >
                Cancel
              </button>
              <button
                className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
                onClick={handleCreate}
                disabled={creating || !newName.trim()}
              >
                {creating ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
