import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Trash2, BookOpen } from 'lucide-react'
import { api } from '../lib/api'
import type { Library, PlatformLibrary } from '../types'
import LanguageBadge from '../components/LanguageBadge'
import DocPanel from '../components/DocPanel'

export default function LibrariesPage() {
  const navigate = useNavigate()
  const [platform, setPlatform] = useState<PlatformLibrary[]>([])
  const [tenant, setTenant] = useState<Library[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [newName, setNewName] = useState('')
  const [newSlug, setNewSlug] = useState('')
  const [newLanguage, setNewLanguage] = useState<'bun' | 'python'>('bun')
  const [newDescription, setNewDescription] = useState('')
  const [creating, setCreating] = useState(false)
  const [docLib, setDocLib] = useState<PlatformLibrary | null>(null)

  useEffect(() => { load() }, [])

  async function load() {
    setLoading(true)
    try {
      const data = await api.listLibraries()
      setPlatform(data.platform ?? [])
      setTenant(data.tenant ?? [])
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }

  async function handleCreate() {
    if (!newName.trim() || !newSlug.trim()) return
    setCreating(true)
    setError('')
    try {
      const lib = await api.createLibrary({
        name: newName.trim(),
        slug: newSlug.trim(),
        language: newLanguage,
        description: newDescription.trim() || undefined,
      })
      setTenant(prev => [...prev, lib])
      setShowModal(false)
      setNewName(''); setNewSlug(''); setNewDescription('')
      navigate(`/dashboard/libraries/${lib.id}`)
    } catch (err) {
      setError(String(err))
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(e: React.MouseEvent, id: string) {
    e.stopPropagation()
    if (!confirm('Delete this library and all its versions? This cannot be undone.')) return
    try {
      await api.deleteLibrary(id)
      setTenant(prev => prev.filter(l => l.id !== id))
    } catch (err) {
      setError(String(err))
    }
  }

  function deriveSlug(name: string) {
    return name.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
  }

  return (
    <div>
      {docLib && <DocPanel lib={docLib} onClose={() => setDocLib(null)} />}
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Libraries</h1>
          <p className="mt-1 text-sm text-gray-500">
            Reusable code modules available to all snippets via{' '}
            <code className="rounded bg-gray-100 px-1 text-xs">import {'{'} fn {'}'} from '@tenant/lib-name'</code>
          </p>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="flex items-center gap-2 rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
        >
          <Plus size={16} />
          New Library
        </button>
      </div>

      {error && <div className="mb-6 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>}

      {/* Platform libraries */}
      {!loading && platform.length > 0 && (
        <div className="mb-8">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-gray-400">
            Built-in (@velane/*)
          </h2>
          <div className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 text-xs font-medium uppercase tracking-wider text-gray-500">
                <tr>
                  <th className="px-6 py-3 text-left">Name</th>
                  <th className="px-6 py-3 text-left">Import path</th>
                  <th className="px-6 py-3 text-left">Language</th>
                  <th className="px-6 py-3"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {platform.map(lib => (
                  <tr
                    key={lib.id}
                    className="cursor-pointer bg-gray-50/50 hover:bg-gray-100/60"
                    onClick={() => setDocLib(lib)}
                  >
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-gray-700">{lib.name}</span>
                      </div>
                      {lib.description && <p className="mt-0.5 text-xs text-gray-400">{lib.description}</p>}
                    </td>
                    <td className="px-6 py-4">
                      <code className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                        @velane/{lib.slug}
                      </code>
                    </td>
                    <td className="px-6 py-4">
                      <LanguageBadge language={lib.language} />
                    </td>
                    <td className="px-6 py-4 text-right">
                      <span className="flex items-center justify-end gap-1 text-xs text-gray-700">
                        <BookOpen size={12} />
                        Docs
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Tenant libraries */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-gray-400">
          Your Libraries
        </h2>
        {loading ? (
          <p className="text-sm text-gray-500">Loading...</p>
        ) : tenant.length === 0 ? (
          <div className="rounded-lg border border-dashed border-gray-300 bg-white py-16 text-center">
            <p className="text-gray-500">No libraries yet.</p>
            <button
              className="mt-4 text-sm font-medium text-gray-900 hover:underline"
              onClick={() => setShowModal(true)}
            >
              Create your first library
            </button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 text-xs font-medium uppercase tracking-wider text-gray-500">
                <tr>
                  <th className="px-6 py-3 text-left">Name</th>
                  <th className="px-6 py-3 text-left">Import path</th>
                  <th className="px-6 py-3 text-left">Language</th>
                  <th className="px-6 py-3 text-left">Created</th>
                  <th className="px-6 py-3"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {tenant.map(lib => (
                  <tr
                    key={lib.id}
                    className="cursor-pointer hover:bg-gray-50"
                    onClick={() => navigate(`/dashboard/libraries/${lib.id}`)}
                  >
                    <td className="px-6 py-4">
                      <span className="font-medium text-gray-900">{lib.name}</span>
                      {lib.description && <p className="mt-0.5 text-xs text-gray-400">{lib.description}</p>}
                    </td>
                    <td className="px-6 py-4">
                      <code className="rounded bg-indigo-50 px-2 py-0.5 text-xs text-indigo-700">
                        @tenant/{lib.slug}
                      </code>
                    </td>
                    <td className="px-6 py-4"><LanguageBadge language={lib.language} /></td>
                    <td className="px-6 py-4 text-gray-400">{new Date(lib.created_at).toLocaleDateString()}</td>
                    <td className="px-6 py-4 text-right">
                      <button
                        className="rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600"
                        onClick={e => handleDelete(e, lib.id)}
                        title="Delete library"
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
      </div>

      {/* Create modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
          <div className="w-full max-w-md rounded-xl border border-gray-200 bg-white p-6 shadow-xl">
            <h2 className="mb-4 text-lg font-semibold text-gray-900">New Library</h2>

            <div className="mb-4">
              <label className="mb-1 block text-sm font-medium text-gray-700">Name</label>
              <input
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-400"
                placeholder="HTTP Client"
                value={newName}
                onChange={e => { setNewName(e.target.value); setNewSlug(deriveSlug(e.target.value)) }}
                autoFocus
              />
            </div>

            <div className="mb-4">
              <label className="mb-1 block text-sm font-medium text-gray-700">Slug</label>
              <div className="flex items-center gap-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus-within:ring-2 focus-within:ring-gray-400">
                <span className="text-gray-400">@tenant/</span>
                <input
                  className="flex-1 focus:outline-none"
                  placeholder="http-client"
                  value={newSlug}
                  onChange={e => setNewSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                />
              </div>
            </div>

            <div className="mb-4">
              <label className="mb-1 block text-sm font-medium text-gray-700">Language</label>
              <select
                className="w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-400"
                value={newLanguage}
                onChange={e => setNewLanguage(e.target.value as 'bun' | 'python')}
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
                placeholder="What does this library provide?"
                value={newDescription}
                onChange={e => setNewDescription(e.target.value)}
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
                disabled={creating || !newName.trim() || !newSlug.trim()}
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
