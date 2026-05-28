import { useEffect, useState } from 'react'
import { Plus, Trash2, Pencil, Eye, EyeOff, Check, X } from 'lucide-react'
import { api } from '../lib/api'
import type { Secret } from '../types'

type Tab = 'variables' | 'credentials'

export default function VariablesPage() {
  const [items, setItems] = useState<Secret[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [tab, setTab] = useState<Tab>('variables')

  // Create form state
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newValue, setNewValue] = useState('')
  const [creating, setCreating] = useState(false)

  // Inline edit state (variables only)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')
  const [editValue, setEditValue] = useState('')
  const [saving, setSaving] = useState(false)

  // Credential value replace state
  const [replacingId, setReplacingId] = useState<string | null>(null)
  const [replaceValue, setReplaceValue] = useState('')
  const [showReplaceValue, setShowReplaceValue] = useState(false)

  useEffect(() => {
    load()
  }, [])

  async function load() {
    setLoading(true)
    try {
      const data = await api.listSecrets()
      setItems(data ?? [])
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }

  const variables = items.filter(i => !i.is_secret)
  const credentials = items.filter(i => i.is_secret)

  async function handleCreate() {
    if (!newName.trim() || !newValue.trim()) return
    setCreating(true)
    setError('')
    try {
      const created = await api.createSecret({
        name: newName.trim(),
        value: newValue.trim(),
        is_secret: tab === 'credentials',
      })
      setItems(prev => [created, ...prev])
      setNewName('')
      setNewValue('')
      setShowCreate(false)
    } catch (err) {
      setError(String(err))
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(id: string, isSecret: boolean) {
    const label = isSecret ? 'credential' : 'variable'
    if (!confirm(`Delete this ${label}? This cannot be undone.`)) return
    try {
      await api.deleteSecret(id)
      setItems(prev => prev.filter(i => i.id !== id))
    } catch (err) {
      setError(String(err))
    }
  }

  function startEdit(item: Secret) {
    setEditingId(item.id)
    setEditName(item.name)
    setEditValue(item.value ?? '')
  }

  function cancelEdit() {
    setEditingId(null)
    setEditName('')
    setEditValue('')
  }

  async function saveEdit(id: string) {
    setSaving(true)
    try {
      const updated = await api.updateSecret(id, { name: editName, value: editValue })
      setItems(prev => prev.map(i => i.id === id ? updated : i))
      cancelEdit()
    } catch (err) {
      setError(String(err))
    } finally {
      setSaving(false)
    }
  }

  async function handleReplaceCredential(id: string) {
    if (!replaceValue.trim()) return
    setSaving(true)
    try {
      const updated = await api.updateSecret(id, { value: replaceValue })
      setItems(prev => prev.map(i => i.id === id ? updated : i))
      setReplacingId(null)
      setReplaceValue('')
      setShowReplaceValue(false)
    } catch (err) {
      setError(String(err))
    } finally {
      setSaving(false)
    }
  }

  function handleTabChange(t: Tab) {
    setTab(t)
    setShowCreate(false)
    setNewName('')
    setNewValue('')
    cancelEdit()
    setReplacingId(null)
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Variables & Credentials</h1>
        <p className="mt-1 text-sm text-gray-500">
          Injected as environment variables into every snippet invocation.
          Credentials are write-only — values are never displayed after creation.
        </p>
      </div>

      {error && (
        <div className="mb-6 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>
      )}

      {/* Tabs */}
      <div className="mb-6 flex gap-1 border-b border-gray-200">
        {(['variables', 'credentials'] as Tab[]).map(t => (
          <button
            key={t}
            onClick={() => handleTabChange(t)}
            className={`px-4 py-2 text-sm font-medium capitalize transition-colors ${
              tab === t
                ? 'border-b-2 border-gray-900 text-gray-900'
                : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            {t}
            <span className="ml-2 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
              {t === 'variables' ? variables.length : credentials.length}
            </span>
          </button>
        ))}
      </div>

      {/* Add button */}
      <div className="mb-4 flex justify-end">
        <button
          onClick={() => setShowCreate(v => !v)}
          className="flex items-center gap-2 rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
        >
          <Plus size={14} />
          Add {tab === 'variables' ? 'variable' : 'credential'}
        </button>
      </div>

      {/* Create form */}
      {showCreate && (
        <div className="mb-6 rounded-lg border border-indigo-200 bg-indigo-50 p-4">
          <h3 className="mb-3 text-sm font-medium text-indigo-900">
            New {tab === 'variables' ? 'variable' : 'credential'}
          </h3>
          <div className="flex gap-3">
            <input
              className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
              placeholder="NAME"
              value={newName}
              onChange={e => setNewName(e.target.value.toUpperCase().replace(/\s/g, '_'))}
              autoFocus
            />
            <input
              className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
              placeholder="Value"
              type={tab === 'credentials' ? 'password' : 'text'}
              value={newValue}
              onChange={e => setNewValue(e.target.value)}
            />
            <button
              onClick={handleCreate}
              disabled={creating || !newName.trim() || !newValue.trim()}
              className="rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50"
            >
              {creating ? 'Adding...' : 'Add'}
            </button>
            <button
              onClick={() => { setShowCreate(false); setNewName(''); setNewValue('') }}
              className="rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-600 hover:bg-gray-50"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Variables tab */}
      {tab === 'variables' && (
        <div className="rounded-lg border border-gray-200 bg-white shadow-sm">
          {loading ? (
            <p className="p-6 text-sm text-gray-400">Loading...</p>
          ) : variables.length === 0 ? (
            <p className="p-6 text-sm text-gray-500">
              No variables yet. Variables are plaintext key-value pairs visible in this UI.
            </p>
          ) : (
            <table className="w-full text-sm">
              <thead className="border-b border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
                <tr>
                  <th className="px-6 py-3 text-left">Name</th>
                  <th className="px-6 py-3 text-left">Value</th>
                  <th className="px-6 py-3 text-left">Updated</th>
                  <th className="px-6 py-3 text-left"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {variables.map(item => (
                  <tr key={item.id} className="hover:bg-gray-50">
                    <td className="px-6 py-3">
                      {editingId === item.id ? (
                        <input
                          className="w-full rounded border border-gray-300 px-2 py-1 text-xs font-mono focus:outline-none"
                          value={editName}
                          onChange={e => setEditName(e.target.value.toUpperCase().replace(/\s/g, '_'))}
                        />
                      ) : (
                        <code className="text-xs font-mono text-gray-800">{item.name}</code>
                      )}
                    </td>
                    <td className="px-6 py-3">
                      {editingId === item.id ? (
                        <input
                          className="w-full rounded border border-gray-300 px-2 py-1 text-xs focus:outline-none"
                          value={editValue}
                          onChange={e => setEditValue(e.target.value)}
                        />
                      ) : (
                        <span className="text-gray-700">{item.value}</span>
                      )}
                    </td>
                    <td className="px-6 py-3 text-gray-400">
                      {new Date(item.updated_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-3">
                      <div className="flex items-center gap-2">
                        {editingId === item.id ? (
                          <>
                            <button
                              onClick={() => saveEdit(item.id)}
                              disabled={saving}
                              className="text-green-600 hover:text-green-800 disabled:opacity-50"
                              title="Save"
                            >
                              <Check size={14} />
                            </button>
                            <button onClick={cancelEdit} className="text-gray-400 hover:text-gray-600" title="Cancel">
                              <X size={14} />
                            </button>
                          </>
                        ) : (
                          <button
                            onClick={() => startEdit(item)}
                            className="text-gray-400 hover:text-gray-900"
                            title="Edit"
                          >
                            <Pencil size={14} />
                          </button>
                        )}
                        <button
                          onClick={() => handleDelete(item.id, false)}
                          className="text-gray-400 hover:text-red-600"
                          title="Delete"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* Credentials tab */}
      {tab === 'credentials' && (
        <div className="rounded-lg border border-gray-200 bg-white shadow-sm">
          {loading ? (
            <p className="p-6 text-sm text-gray-400">Loading...</p>
          ) : credentials.length === 0 ? (
            <p className="p-6 text-sm text-gray-500">
              No credentials yet. Credential values are write-only and never shown after creation.
            </p>
          ) : (
            <table className="w-full text-sm">
              <thead className="border-b border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
                <tr>
                  <th className="px-6 py-3 text-left">Name</th>
                  <th className="px-6 py-3 text-left">Value</th>
                  <th className="px-6 py-3 text-left">Updated</th>
                  <th className="px-6 py-3 text-left"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {credentials.map(item => (
                  <tr key={item.id} className="hover:bg-gray-50">
                    <td className="px-6 py-3">
                      <code className="text-xs font-mono text-gray-800">{item.name}</code>
                    </td>
                    <td className="px-6 py-3">
                      {replacingId === item.id ? (
                        <div className="flex items-center gap-2">
                          <div className="relative flex-1">
                            <input
                              className="w-full rounded border border-gray-300 px-2 py-1 pr-8 text-xs focus:outline-none"
                              type={showReplaceValue ? 'text' : 'password'}
                              placeholder="New value"
                              value={replaceValue}
                              onChange={e => setReplaceValue(e.target.value)}
                              autoFocus
                            />
                            <button
                              onClick={() => setShowReplaceValue(v => !v)}
                              className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                            >
                              {showReplaceValue ? <EyeOff size={12} /> : <Eye size={12} />}
                            </button>
                          </div>
                          <button
                            onClick={() => handleReplaceCredential(item.id)}
                            disabled={saving || !replaceValue.trim()}
                            className="text-green-600 hover:text-green-800 disabled:opacity-50"
                            title="Save"
                          >
                            <Check size={14} />
                          </button>
                          <button
                            onClick={() => { setReplacingId(null); setReplaceValue(''); setShowReplaceValue(false) }}
                            className="text-gray-400 hover:text-gray-600"
                            title="Cancel"
                          >
                            <X size={14} />
                          </button>
                        </div>
                      ) : (
                        <span className="font-mono text-xs tracking-widest text-gray-400">••••••••</span>
                      )}
                    </td>
                    <td className="px-6 py-3 text-gray-400">
                      {new Date(item.updated_at).toLocaleDateString()}
                    </td>
                    <td className="px-6 py-3">
                      <div className="flex items-center gap-2">
                        {replacingId !== item.id && (
                          <button
                            onClick={() => { setReplacingId(item.id); setReplaceValue('') }}
                            className="text-xs text-gray-500 hover:text-gray-900"
                            title="Replace value"
                          >
                            Replace
                          </button>
                        )}
                        <button
                          onClick={() => handleDelete(item.id, true)}
                          className="text-gray-400 hover:text-red-600"
                          title="Delete"
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}
