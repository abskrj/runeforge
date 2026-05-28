import { useEffect, useMemo, useState } from 'react'
import MonacoEditor from '@monaco-editor/react'

type Snippet = {
  id: string
  name: string
  slug: string
  language: string
}

type Bootstrap = {
  tenant: {
    name: string
    slug: string
    branding?: {
      logo_url?: string
      accent_color?: string
      font_family?: string
    }
  }
}

const tokenFromUrl = new URLSearchParams(window.location.search).get('token') ?? ''
const apiBase = '/api'

async function api<T>(path: string): Promise<T> {
  const res = await fetch(`${apiBase}${path}`, {
    headers: {
      Authorization: `Bearer ${tokenFromUrl}`,
    },
  })
  if (!res.ok) {
    throw new Error(await res.text())
  }
  return res.json() as Promise<T>
}

export default function App() {
  const [bootstrap, setBootstrap] = useState<Bootstrap | null>(null)
  const [snippets, setSnippets] = useState<Snippet[]>([])
  const [selected, setSelected] = useState<Snippet | null>(null)
  const [detail, setDetail] = useState<any>(null)
  const [metrics, setMetrics] = useState<any>(null)
  const [logs, setLogs] = useState<any[]>([])
  const [windowValue, setWindowValue] = useState('24h')
  const [statusFilter, setStatusFilter] = useState('')
  const [error, setError] = useState('')

  const style = useMemo(() => {
    const accent = bootstrap?.tenant?.branding?.accent_color || '#6366f1'
    const font = bootstrap?.tenant?.branding?.font_family || 'Inter, system-ui, sans-serif'
    return { ['--accent' as string]: accent, fontFamily: font }
  }, [bootstrap])

  useEffect(() => {
    if (!tokenFromUrl) {
      setError('Missing token query parameter')
      return
    }
    Promise.all([api<Bootstrap>('/v1/embed/bootstrap'), api<Snippet[]>('/v1/embed/snippets')])
      .then(([b, s]) => {
        setBootstrap(b)
        setSnippets(s)
        if (s.length > 0) setSelected(s[0])
      })
      .catch((e) => setError(String(e)))
  }, [])

  useEffect(() => {
    if (!selected) return
    Promise.all([
      api<any>(`/v1/embed/snippets/${selected.id}`),
      api<any>(`/v1/embed/snippets/${selected.id}/metrics?window=${windowValue}`),
      api<any>(`/v1/embed/snippets/${selected.id}/logs?limit=20&status=${encodeURIComponent(statusFilter)}`),
    ])
      .then(([d, m, l]) => {
        setDetail(d)
        setMetrics(m)
        setLogs(l.items ?? [])
      })
      .catch((e) => setError(String(e)))
  }, [selected, windowValue, statusFilter])

  return (
    <div style={style}>
      <div className="layout">
        <aside className="panel">
          <h2 style={{ marginTop: 0 }}>{bootstrap?.tenant?.name ?? 'Embed Dashboard'}</h2>
          <input
            placeholder="Search snippets"
            onChange={(e) => {
              const q = e.target.value.toLowerCase()
              if (!q) {
                api<Snippet[]>('/v1/embed/snippets').then(setSnippets).catch((err) => setError(String(err)))
                return
              }
              setSnippets((prev) =>
                prev.filter((s) => s.name.toLowerCase().includes(q) || s.slug.toLowerCase().includes(q)),
              )
            }}
          />
          {snippets.map((snippet) => (
            <button key={snippet.id} className="list-item" onClick={() => setSelected(snippet)}>
              <strong>{snippet.name}</strong>
              <div style={{ fontSize: 12, color: '#6b7280' }}>
                {snippet.slug} · {snippet.language}
              </div>
            </button>
          ))}
        </aside>
        <main className="content">
          {error ? <div className="card" style={{ color: '#b91c1c' }}>{error}</div> : null}
          {!selected || !detail ? <div className="card">Select a snippet</div> : (
            <>
              <h2 style={{ marginTop: 0 }}>{detail.snippet?.name}</h2>
              <div className="grid-2">
                <section className="card">
                  <h3 style={{ marginTop: 0 }}>Versions</h3>
                  <div>{(detail.versions ?? []).map((v: any) => `v${v.version_number}`).join(', ') || 'None'}</div>
                </section>
                <section className="card">
                  <h3 style={{ marginTop: 0 }}>Environment Status</h3>
                  <pre>{JSON.stringify(detail.environments ?? {}, null, 2)}</pre>
                </section>
              </div>
              <section className="card">
                <h3 style={{ marginTop: 0 }}>Latest Code</h3>
                <MonacoEditor
                  height="300px"
                  language={selected?.language === 'bun' ? 'typescript' : 'python'}
                  value={(detail.versions ?? [])[detail.versions.length - 1]?.code ?? ''}
                  options={{
                    readOnly: true,
                    minimap: { enabled: false },
                    scrollBeyondLastLine: false,
                    fontSize: 13,
                    lineNumbers: 'on',
                    theme: 'vs-dark',
                  }}
                />
              </section>
              <div className="grid-2">
                <section className="card">
                  <h3 style={{ marginTop: 0 }}>Metrics</h3>
                  <select value={windowValue} onChange={(e) => setWindowValue(e.target.value)}>
                    <option value="1h">1h</option>
                    <option value="24h">24h</option>
                    <option value="7d">7d</option>
                  </select>
                  <pre>{JSON.stringify(metrics ?? {}, null, 2)}</pre>
                </section>
                <section className="card">
                  <h3 style={{ marginTop: 0 }}>Logs</h3>
                  <input
                    placeholder="status filter (optional)"
                    value={statusFilter}
                    onChange={(e) => setStatusFilter(e.target.value)}
                  />
                  <pre>{JSON.stringify(logs, null, 2)}</pre>
                </section>
              </div>
            </>
          )}
        </main>
      </div>
    </div>
  )
}
