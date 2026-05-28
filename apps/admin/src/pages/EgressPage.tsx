import { useEffect, useState, type FormEvent, type KeyboardEvent } from 'react'
import { X, Plus } from 'lucide-react'
import { api } from '../lib/api'
import type { EgressPolicy } from '../types'

export default function EgressPage() {
  const [policy, setPolicy] = useState<EgressPolicy>({ blocked_cidrs: [], blocked_domains: [] })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')

  const [newCIDR, setNewCIDR] = useState('')
  const [newDomain, setNewDomain] = useState('')

  useEffect(() => {
    api
      .getEgressPolicy()
      .then((p) =>
        setPolicy({
          blocked_cidrs: p.blocked_cidrs ?? [],
          blocked_domains: p.blocked_domains ?? [],
        }),
      )
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const addCIDR = () => {
    const v = newCIDR.trim()
    if (!v || policy.blocked_cidrs.includes(v)) return
    setPolicy((p) => ({ ...p, blocked_cidrs: [...p.blocked_cidrs, v] }))
    setNewCIDR('')
  }

  const removeCIDR = (cidr: string) => {
    setPolicy((p) => ({ ...p, blocked_cidrs: p.blocked_cidrs.filter((c) => c !== cidr) }))
  }

  const addDomain = () => {
    const v = newDomain.trim()
    if (!v || policy.blocked_domains.includes(v)) return
    setPolicy((p) => ({ ...p, blocked_domains: [...p.blocked_domains, v] }))
    setNewDomain('')
  }

  const removeDomain = (d: string) => {
    setPolicy((p) => ({ ...p, blocked_domains: p.blocked_domains.filter((x) => x !== d) }))
  }

  const handleKeyDown = (fn: () => void) => (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') { e.preventDefault(); fn() }
  }

  const handleSave = async (e: FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError('')
    try {
      await api.updateEgressPolicy(policy)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save policy')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <p className="text-sm text-gray-500">Loading...</p>

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold text-gray-900">Egress Policy</h1>

      {error && (
        <div className="mb-4 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</div>
      )}
      {saved && (
        <div className="mb-4 rounded-md bg-green-50 p-3 text-sm text-green-700">Egress policy saved!</div>
      )}

      <form onSubmit={handleSave} className="space-y-6">
        {/* Blocked CIDRs */}
        <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
          <h2 className="mb-3 text-base font-semibold text-gray-900">Blocked CIDRs</h2>
          <div className="mb-3 flex flex-wrap gap-2">
            {policy.blocked_cidrs.map((cidr) => (
              <span
                key={cidr}
                className="flex items-center gap-1 rounded-full bg-gray-100 px-3 py-1 text-sm text-gray-700"
              >
                {cidr}
                <button type="button" onClick={() => removeCIDR(cidr)} className="ml-1 text-gray-400 hover:text-gray-700">
                  <X size={12} />
                </button>
              </span>
            ))}
          </div>
          <div className="flex gap-2">
            <input
              type="text"
              value={newCIDR}
              onChange={(e) => setNewCIDR(e.target.value)}
              onKeyDown={handleKeyDown(addCIDR)}
              className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
              placeholder="10.0.0.0/8"
            />
            <button
              type="button"
              onClick={addCIDR}
              className="flex items-center gap-1 rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
            >
              <Plus size={14} /> Add
            </button>
          </div>
        </div>

        {/* Blocked Domains */}
        <div className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
          <h2 className="mb-3 text-base font-semibold text-gray-900">Blocked Domains</h2>
          <div className="mb-3 flex flex-wrap gap-2">
            {policy.blocked_domains.map((d) => (
              <span
                key={d}
                className="flex items-center gap-1 rounded-full bg-gray-100 px-3 py-1 text-sm text-gray-700"
              >
                {d}
                <button type="button" onClick={() => removeDomain(d)} className="ml-1 text-gray-400 hover:text-gray-700">
                  <X size={12} />
                </button>
              </span>
            ))}
          </div>
          <div className="flex gap-2">
            <input
              type="text"
              value={newDomain}
              onChange={(e) => setNewDomain(e.target.value)}
              onKeyDown={handleKeyDown(addDomain)}
              className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
              placeholder="example.com"
            />
            <button
              type="button"
              onClick={addDomain}
              className="flex items-center gap-1 rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
            >
              <Plus size={14} /> Add
            </button>
          </div>
        </div>

        <button
          type="submit"
          disabled={saving}
          className="rounded-md bg-gray-900 px-6 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50"
        >
          {saving ? 'Saving...' : 'Save Policy'}
        </button>
      </form>
    </div>
  )
}
