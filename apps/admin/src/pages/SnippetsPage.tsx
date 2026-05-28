import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Trash2, FileCode2, AlertCircle } from 'lucide-react'
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
      <div className="mb-8 flex items-start justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Snippets</h1>
          <p className="mt-1.5 text-sm text-gray-500">Manage your code snippets and deployments.</p>
        </div>
        <button
          className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-gray-800"
          onClick={() => setShowModal(true)}
        >
          <Plus size={15} />
          New Snippet
        </button>
      </div>

      {error && (
        <div className="mb-6 flex items-center gap-2.5 rounded-lg border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-600">
          <AlertCircle size={16} className="shrink-0" />
          {error}
        </div>
      )}

      {loading && (
        <p className="text-sm text-gray-400">Loading snippets...</p>
      )}

      {!loading && snippets.length === 0 && (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-gray-300 bg-white py-24">
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
            <FileCode2 size={22} className="text-gray-400" />
          </div>
          <p className="text-base font-semibold text-gray-800">No snippets yet</p>
          <p className="mt-1.5 max-w-xs text-center text-sm text-gray-500">
            You haven't created any snippets. Create one to get started with your deployments.
          </p>
          <button
            className="mt-6 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
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
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-400"
                placeholder="My Snippet"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                autoFocus
              />
            </div>

            <div className="mb-4">
              <label className="mb-1 block text-sm font-medium text-gray-700">Language</label>
              <select
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-400"
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
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-400"
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
                className="rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50"
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
