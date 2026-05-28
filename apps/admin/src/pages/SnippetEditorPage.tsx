import Editor from '@monaco-editor/react'
import { Trash2 } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import LanguageBadge from '../components/LanguageBadge'
import PublishDropdown from '../components/PublishDropdown'
import { Toast, useToast } from '../components/Toast'
import { api } from '../lib/api'
import type { InvocationResult, Snippet, SnippetVersion } from '../types'

type ActiveTab = 'test' | 'logs'

const STARTER_TEMPLATES: Record<string, string> = {
  bun: `export default async function handler(input: Record<string, unknown>) {
  // input is the parsed JSON body sent to /v1/invoke/:tenant/:snippet
  return { message: "Hello from Runeforge!", input }
}
`,
  python: `def handler(input: dict) -> dict:
    # input is the parsed JSON body sent to /v1/invoke/:tenant/:snippet
    return {"message": "Hello from Runeforge!", "input": input}
`,
}

export default function SnippetEditorPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [snippet, setSnippet] = useState<Snippet | null>(null)
  const [versions, setVersions] = useState<SnippetVersion[]>([])
  const [code, setCode] = useState('')
  const [loading, setLoading] = useState(!!id)
  const [saving, setSaving] = useState(false)
  const [activeTab, setActiveTab] = useState<ActiveTab>('test')
  const [testInput, setTestInput] = useState('{}')
  const [testEnv, setTestEnv] = useState<'dev' | 'staging' | 'prod'>('dev')
  const [invokeResult, setInvokeResult] = useState<InvocationResult | null>(null)
  const [invoking, setInvoking] = useState(false)
  const [selectedVersion, setSelectedVersion] = useState<SnippetVersion | null>(null)

  const autosaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const { toast, showToast, dismissToast } = useToast()

  useEffect(() => {
    if (!id) return
    async function load() {
      try {
        const [sn, vs] = await Promise.all([api.getSnippet(id!), api.listVersions(id!)])
        setSnippet(sn)
        setVersions(vs)
        if (vs.length > 0) {
          setCode(vs[vs.length - 1].code)
        } else {
          setCode(STARTER_TEMPLATES[sn.language] ?? STARTER_TEMPLATES.bun)
        }
      } catch (err) {
        showToast(String(err), 'error')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [id])

  async function reloadVersions() {
    if (!id) return
    try {
      const vs = await api.listVersions(id)
      setVersions(vs)
    } catch {
      // ignore
    }
  }

  const monacoLanguage = snippet?.language === 'python' ? 'python' : 'typescript'

  const handleCodeChange = useCallback(
    (value: string | undefined) => {
      const newCode = value ?? ''
      setCode(newCode)
      if (!id) return

      if (autosaveTimer.current) clearTimeout(autosaveTimer.current)
      autosaveTimer.current = setTimeout(async () => {
        try {
          await api.createVersion(id, newCode)
          reloadVersions()
        } catch {
          // silent auto-save failure
        }
      }, 1500)
    },
    [id],
  )

  async function handleSaveDraft() {
    if (!id) return
    setSaving(true)
    try {
      await api.createVersion(id, code)
      await reloadVersions()
      showToast('Draft saved')
    } catch (err) {
      showToast(String(err), 'error')
    } finally {
      setSaving(false)
    }
  }

  async function handlePublish(env: 'dev' | 'staging' | 'prod') {
    if (!id) return
    try {
      const allVersions = await api.listVersions(id)
      const latest = allVersions[allVersions.length - 1]
      if (!latest) return
      await api.publishVersion(id, latest.version_number, env)
      await reloadVersions()
      showToast(`Published to ${env}`)
    } catch (err) {
      showToast(String(err), 'error')
    }
  }

  async function handleDelete() {
    if (!id) return
    if (!confirm('Delete this snippet? This cannot be undone.')) return
    try {
      await api.deleteSnippet(id)
      navigate('/dashboard/snippets')
    } catch (err) {
      showToast(String(err), 'error')
    }
  }

  async function handleRun() {
    if (!snippet?.slug) return
    setInvoking(true)
    setInvokeResult(null)
    try {
      const result = await api.invokeSnippet(snippet.slug, testInput, testEnv)
      setInvokeResult(result)
    } catch (err) {
      setInvokeResult({
        output: '',
        error: String(err),
        duration_ms: 0,
        exit_code: 1,
      })
    } finally {
      setInvoking(false)
    }
  }

  if (loading) {
    return <div className="flex h-full items-center justify-center text-sm text-gray-500">Loading...</div>
  }

  const displayCode = selectedVersion ? selectedVersion.code : code
  const isReadOnly = !!selectedVersion

  return (
    <div className="flex h-full flex-col">
      {toast && <Toast message={toast.message} type={toast.type} onDismiss={dismissToast} />}

      <header className="flex shrink-0 items-center justify-between border-b border-gray-200 bg-white px-4 py-2">
        <div className="flex items-center gap-3">
          <button
            className="text-sm text-gray-500 hover:text-gray-900"
            onClick={() => navigate('/dashboard/snippets')}
          >
            &larr; Snippets
          </button>
          <span className="font-medium text-gray-900">{snippet?.name}</span>
          {snippet && <LanguageBadge language={snippet.language} />}
        </div>
        <div className="flex items-center gap-2">
          <button
            className="rounded-md border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
            onClick={handleSaveDraft}
            disabled={saving}
          >
            {saving ? 'Saving...' : 'Save Draft'}
          </button>
          <PublishDropdown onPublish={handlePublish} disabled={!id} />
          {id && (
            <button
              className="rounded-md border border-red-200 px-2 py-1.5 text-sm text-red-600 hover:bg-red-50"
              onClick={handleDelete}
              title="Delete snippet"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          )}
        </div>
      </header>

      <div className="flex min-h-0 flex-1">
        <div className="flex-1 border-r border-gray-200">
          <Editor
            height="100%"
            language={monacoLanguage}
            value={displayCode}
            onChange={isReadOnly ? undefined : handleCodeChange}
            options={{
              readOnly: isReadOnly,
              minimap: { enabled: false },
              fontSize: 14,
              lineNumbers: 'on',
              scrollBeyondLastLine: false,
            }}
            theme="vs"
          />
        </div>

        <div className="flex w-96 shrink-0 flex-col bg-white">
          <div className="flex border-b border-gray-200">
            {(['test', 'logs'] as ActiveTab[]).map((tab) => (
              <button
                key={tab}
                className={`px-4 py-2 text-sm font-medium capitalize ${
                  activeTab === tab
                    ? 'border-b-2 border-indigo-600 text-indigo-600'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
                onClick={() => setActiveTab(tab)}
              >
                {tab}
              </button>
            ))}
          </div>

          {activeTab === 'test' && (
            <div className="flex flex-1 flex-col gap-3 overflow-auto p-4">
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-700">Input JSON</label>
                <textarea
                  className="h-32 w-full rounded-md border border-gray-300 p-2 font-mono text-xs focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  value={testInput}
                  onChange={(e) => setTestInput(e.target.value)}
                  spellCheck={false}
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-gray-700">Environment</label>
                <select
                  className="w-full rounded-md border border-gray-300 px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-indigo-500"
                  value={testEnv}
                  onChange={(e) => setTestEnv(e.target.value as 'dev' | 'staging' | 'prod')}
                >
                  <option value="dev">dev</option>
                  <option value="staging">staging</option>
                  <option value="prod">prod</option>
                </select>
              </div>
              <button
                className="w-full rounded-md bg-indigo-600 py-2 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
                onClick={handleRun}
                disabled={invoking || !id}
              >
                {invoking ? 'Running...' : '▶ Run'}
              </button>
              {invokeResult && (
                <div
                  className={`rounded-md p-3 ${
                    invokeResult.error ? 'bg-red-950' : 'bg-gray-900'
                  }`}
                >
                  <pre
                    className={`overflow-auto text-xs ${
                      invokeResult.error ? 'text-red-300' : 'text-green-300'
                    }`}
                  >
                    {JSON.stringify(invokeResult, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          )}

          {activeTab === 'logs' && (
            <div className="flex flex-1 items-center justify-center text-sm text-gray-500">
              Live logs coming soon
            </div>
          )}
        </div>
      </div>

      {versions.length > 0 && (
        <footer className="flex shrink-0 items-center gap-2 border-t border-gray-200 bg-white px-4 py-2">
          <span className="text-xs text-gray-500">Versions:</span>
          {selectedVersion && (
            <button
              className="rounded px-2 py-0.5 text-xs text-gray-500 hover:bg-gray-100"
              onClick={() => setSelectedVersion(null)}
            >
              Latest (editable)
            </button>
          )}
          {versions.map((v) => (
            <button
              key={v.id}
              className={`rounded px-2 py-0.5 text-xs ${
                selectedVersion?.id === v.id
                  ? 'bg-indigo-100 font-medium text-indigo-700'
                  : 'text-gray-600 hover:bg-gray-100'
              }`}
              onClick={() => setSelectedVersion(selectedVersion?.id === v.id ? null : v)}
            >
              v{v.version_number}
              {v.status === 'published' && (
                <span className="ml-1 text-green-600">●</span>
              )}
            </button>
          ))}
        </footer>
      )}
    </div>
  )
}
