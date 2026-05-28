import { useEffect, useState } from 'react'
import { Trash2, Plus, ExternalLink } from 'lucide-react'
import { api } from '../lib/api'
import type { EmbedToken, Snippet } from '../types'

const EMBED_BASE = `http://localhost:8092`

export default function EmbedPage() {
  const [tokens, setTokens] = useState<EmbedToken[]>([])
  const [snippets, setSnippets] = useState<Snippet[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)
  const [newToken, setNewToken] = useState<{ token: string; expires_at: string } | null>(null)
  const [selectedSnippets, setSelectedSnippets] = useState<string[]>([])
  const [ttl, setTtl] = useState(3600)
  const [copied, setCopied] = useState('')

  useEffect(() => {
    load()
  }, [])

  async function load() {
    setLoading(true)
    try {
      const [toks, snips] = await Promise.all([api.listEmbedTokens(), api.listSnippets()])
      setTokens(toks)
      setSnippets(snips)
    } catch (err) {
      setError(String(err))
    } finally {
      setLoading(false)
    }
  }

  async function handleCreate() {
    setCreating(true)
    setError('')
    setNewToken(null)
    try {
      const res = await api.createEmbedToken(selectedSnippets, ttl)
      setNewToken(res)
      setSelectedSnippets([])
      await load()
    } catch (err) {
      setError(String(err))
    } finally {
      setCreating(false)
    }
  }

  async function handleRevoke(id: string) {
    if (!confirm('Revoke this embed token? The embed dashboard using it will stop working.')) return
    try {
      await api.revokeEmbedToken(id)
      setTokens(t => t.filter(tok => tok.id !== id))
    } catch (err) {
      setError(String(err))
    }
  }

  function copy(text: string, key: string) {
    navigator.clipboard.writeText(text)
    setCopied(key)
    setTimeout(() => setCopied(''), 2000)
  }

  function toggleSnippet(id: string) {
    setSelectedSnippets(prev =>
      prev.includes(id) ? prev.filter(s => s !== id) : [...prev, id]
    )
  }

  return (
    <div>
      <h1 className="mb-1 text-2xl font-bold text-gray-900">Embed Dashboard</h1>
      <p className="mb-8 text-sm text-gray-500">
        Generate tokens to embed a read-only snippet dashboard in any web page.
      </p>

      {error && (
        <div className="mb-6 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>
      )}

      {newToken && (
        <div className="mb-6 rounded-md border border-green-200 bg-green-50 p-4">
          <p className="mb-2 text-sm font-medium text-green-800">Token created — copy it now, it won't be shown again.</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 truncate rounded bg-green-100 px-2 py-1 text-xs text-green-900">
              {newToken.token}
            </code>
            <button
              onClick={() => copy(newToken.token, 'newtoken')}
              className="shrink-0 rounded-md bg-green-600 px-3 py-1 text-xs text-white hover:bg-green-700"
            >
              {copied === 'newtoken' ? 'Copied!' : 'Copy token'}
            </button>
            <a
              href={`${EMBED_BASE}?embed=true&token=${newToken.token}`}
              target="_blank"
              rel="noopener noreferrer"
              className="flex shrink-0 items-center gap-1 rounded-md border border-green-300 px-3 py-1 text-xs text-green-800 hover:bg-green-100"
            >
              Open embed <ExternalLink size={12} />
            </a>
          </div>
          <p className="mt-2 text-xs text-green-700">Expires: {new Date(newToken.expires_at).toLocaleString()}</p>
        </div>
      )}

      {/* Create new token */}
      <div className="mb-8 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        <h2 className="mb-4 text-base font-semibold text-gray-900">Create new token</h2>

        <div className="mb-4">
          <label className="mb-2 block text-sm font-medium text-gray-700">
            Snippets to expose <span className="font-normal text-gray-400">(leave empty to expose all)</span>
          </label>
          {loading ? (
            <p className="text-sm text-gray-400">Loading snippets...</p>
          ) : snippets.length === 0 ? (
            <p className="text-sm text-gray-400">No snippets yet.</p>
          ) : (
            <div className="flex flex-wrap gap-2">
              {snippets.map(s => (
                <button
                  key={s.id}
                  onClick={() => toggleSnippet(s.id)}
                  className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors ${
                    selectedSnippets.includes(s.id)
                      ? 'border-gray-400 bg-gray-100 text-gray-900'
                      : 'border-gray-300 text-gray-600 hover:border-gray-400 hover:text-gray-900'
                  }`}
                >
                  {s.name}
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="mb-4 flex items-center gap-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">Expiry</label>
            <select
              value={ttl}
              onChange={e => setTtl(Number(e.target.value))}
              className="rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
            >
              <option value={3600}>1 hour</option>
              <option value={86400}>24 hours</option>
              <option value={604800}>7 days</option>
              <option value={2592000}>30 days</option>
            </select>
          </div>
        </div>

        <button
          onClick={handleCreate}
          disabled={creating}
          className="flex items-center gap-2 rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50"
        >
          <Plus size={14} />
          {creating ? 'Creating...' : 'Create token'}
        </button>
      </div>

      {/* Existing tokens */}
      <div className="rounded-lg border border-gray-200 bg-white shadow-sm">
        <div className="border-b border-gray-200 px-6 py-4">
          <h2 className="text-base font-semibold text-gray-900">Active tokens</h2>
        </div>
        {loading ? (
          <p className="p-6 text-sm text-gray-400">Loading...</p>
        ) : tokens.length === 0 ? (
          <p className="p-6 text-sm text-gray-500">No active embed tokens.</p>
        ) : (
          <table className="w-full text-sm">
            <thead className="border-b border-gray-200 bg-gray-50 text-xs font-medium text-gray-500">
              <tr>
                <th className="px-6 py-3 text-left">ID</th>
                <th className="px-6 py-3 text-left">Snippets</th>
                <th className="px-6 py-3 text-left">Expires</th>
                <th className="px-6 py-3 text-left">Last used</th>
                <th className="px-6 py-3 text-left"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {tokens.map(tok => (
                <tr key={tok.id} className="hover:bg-gray-50">
                  <td className="px-6 py-3">
                    <code className="text-xs text-gray-500">{tok.id.slice(0, 8)}…</code>
                  </td>
                  <td className="px-6 py-3 text-gray-600">
                    {tok.allowed_snippet_ids.length === 0
                      ? 'All snippets'
                      : `${tok.allowed_snippet_ids.length} snippet${tok.allowed_snippet_ids.length > 1 ? 's' : ''}`}
                  </td>
                  <td className="px-6 py-3 text-gray-600">
                    {new Date(tok.expires_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-3 text-gray-500">
                    {tok.last_used_at ? new Date(tok.last_used_at).toLocaleString() : 'Never'}
                  </td>
                  <td className="px-6 py-3">
                    <button
                      onClick={() => handleRevoke(tok.id)}
                      className="text-red-500 hover:text-red-700"
                      title="Revoke token"
                    >
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
