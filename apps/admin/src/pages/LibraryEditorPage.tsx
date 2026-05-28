import Editor from '@monaco-editor/react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { CheckCircle, Clock } from 'lucide-react'
import { api } from '../lib/api'
import type { Library, LibraryVersion } from '../types'
import LanguageBadge from '../components/LanguageBadge'
import { Toast, useToast } from '../components/Toast'

const STARTER: Record<string, string> = {
  bun: `// Export any functions, classes, or constants you want to share across snippets.
// Import in a snippet: import { greet } from '@tenant/my-lib'

export function greet(name: string): string {
  return \`Hello, \${name}!\`
}
`,
  python: `# Export functions and classes to share across snippets.
# Import in a snippet: from tenant import my_lib

def greet(name: str) -> str:
    return f"Hello, {name}!"
`,
}

export default function LibraryEditorPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { toast, showToast, dismissToast } = useToast()

  const [library, setLibrary] = useState<Library | null>(null)
  const [versions, setVersions] = useState<LibraryVersion[]>([])
  const [code, setCode] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [publishing, setPublishing] = useState(false)

  const autosaveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (!id) return
    async function load() {
      try {
        const [libs, vs] = await Promise.all([api.listLibraries(), api.listLibraryVersions(id!)])
        const lib = [...(libs.platform ?? []), ...(libs.tenant ?? [])].find(l => l.id === id) as Library | undefined
        if (!lib) { navigate('/dashboard/libraries'); return }
        setLibrary(lib as Library)
        setVersions(vs)
        setCode(vs.length > 0 ? vs[vs.length - 1].code : STARTER[lib.language] ?? STARTER.bun)
      } catch (err) {
        showToast(String(err), 'error')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [id])

  const latestVersion = versions[0] ?? null
  const publishedVersion = versions.find(v => v.status === 'published') ?? null
  const monacoLang = library?.language === 'python' ? 'python' : 'typescript'

  const handleCodeChange = useCallback((value: string | undefined) => {
    const newCode = value ?? ''
    setCode(newCode)
    if (autosaveTimer.current) clearTimeout(autosaveTimer.current)
    autosaveTimer.current = setTimeout(() => handleSave(newCode), 2000)
  }, [])

  async function handleSave(codeToSave: string = code) {
    if (!id || saving) return
    setSaving(true)
    try {
      const v = await api.createLibraryVersion(id, codeToSave)
      setVersions(prev => [v, ...prev])
    } catch (err) {
      showToast(String(err), 'error')
    } finally {
      setSaving(false)
    }
  }

  async function handlePublish() {
    if (!latestVersion) { showToast('Save a version first', 'error'); return }
    setPublishing(true)
    try {
      const v = await api.publishLibraryVersion(id!, latestVersion.version_number)
      setVersions(prev => prev.map(ver => ver.id === v.id ? v : { ...ver, status: ver.status === 'published' ? 'archived' : ver.status }))
      showToast('Published!', 'success')
    } catch (err) {
      showToast(String(err), 'error')
    } finally {
      setPublishing(false)
    }
  }

  if (loading) return <div className="flex h-full items-center justify-center text-sm text-gray-400">Loading...</div>
  if (!library) return null

  const importPath = library.language === 'python'
    ? `from tenant import ${library.slug.replace(/-/g, '_')}`
    : `import { ... } from '@tenant/${library.slug}'`

  return (
    <div className="flex h-full flex-col">
      {toast && <Toast message={toast.message} type={toast.type} onDismiss={dismissToast} />}

      {/* Top bar */}
      <div className="flex items-center justify-between border-b border-gray-200 bg-white px-6 py-3">
        <div className="flex items-center gap-3">
          <button onClick={() => navigate('/dashboard/libraries')} className="text-sm text-gray-500 hover:text-gray-700">
            ← Libraries
          </button>
          <span className="text-gray-300">/</span>
          <span className="text-sm font-medium text-gray-900">{library.name}</span>
          <LanguageBadge language={library.language} />
        </div>

        <div className="flex items-center gap-3">
          <code className="hidden rounded bg-indigo-50 px-2 py-1 text-xs text-indigo-700 sm:block">
            {importPath}
          </code>
          {saving && <span className="text-xs text-gray-400">Saving...</span>}
          <button
            onClick={() => handleSave()}
            disabled={saving}
            className="rounded-md border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
          >
            Save
          </button>
          <button
            onClick={handlePublish}
            disabled={publishing || !latestVersion}
            className="flex items-center gap-1.5 rounded-md bg-gray-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50"
          >
            Publish
          </button>
        </div>
      </div>

      {/* Editor + sidebar */}
      <div className="flex flex-1 overflow-hidden">
        <Editor
          className="flex-1"
          language={monacoLang}
          value={code}
          onChange={handleCodeChange}
          theme="vs-dark"
          options={{
            fontSize: 13,
            minimap: { enabled: false },
            wordWrap: 'on',
            scrollBeyondLastLine: false,
          }}
        />

        {/* Version sidebar */}
        <aside className="w-56 shrink-0 overflow-y-auto border-l border-gray-200 bg-white">
          <div className="border-b border-gray-200 px-4 py-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Versions</p>
            {publishedVersion && (
              <p className="mt-1 flex items-center gap-1 text-xs text-green-600">
                <CheckCircle size={11} />
                v{publishedVersion.version_number} published
              </p>
            )}
          </div>
          {versions.length === 0 ? (
            <p className="p-4 text-xs text-gray-400">No versions yet. Save to create one.</p>
          ) : (
            <ul className="divide-y divide-gray-100">
              {versions.map(v => (
                <li
                  key={v.id}
                  className="cursor-pointer px-4 py-3 hover:bg-gray-50"
                  onClick={() => setCode(v.code)}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-medium text-gray-700">v{v.version_number}</span>
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                      v.status === 'published' ? 'bg-green-100 text-green-700' :
                      v.status === 'archived' ? 'bg-gray-100 text-gray-500' :
                      'bg-yellow-100 text-yellow-700'
                    }`}>
                      {v.status}
                    </span>
                  </div>
                  <p className="mt-0.5 flex items-center gap-1 text-xs text-gray-400">
                    <Clock size={10} />
                    {new Date(v.created_at).toLocaleDateString()}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </aside>
      </div>
    </div>
  )
}
