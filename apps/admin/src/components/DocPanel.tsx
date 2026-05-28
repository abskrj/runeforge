import { X } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { PlatformLibrary } from '../types'
import LanguageBadge from './LanguageBadge'

interface Props {
  lib: PlatformLibrary
  onClose: () => void
}

export default function DocPanel({ lib, onClose }: Props) {
  const importPath = lib.language === 'python'
    ? `from velane import ${lib.slug.replace(/-/g, '_')}`
    : `import { ... } from '@velane/${lib.slug}'`

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-40 bg-black/20"
        onClick={onClose}
      />

      {/* Panel */}
      <aside className="fixed inset-y-0 right-0 z-50 flex w-full max-w-2xl flex-col bg-white shadow-xl">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-gray-200 px-6 py-4">
          <div className="flex items-center gap-3">
            <span className="text-base font-semibold text-gray-900">{lib.name}</span>
            <LanguageBadge language={lib.language} />
          </div>
          <button
            onClick={onClose}
            className="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
          >
            <X size={18} />
          </button>
        </div>

        {/* Import path */}
        <div className="border-b border-gray-100 bg-indigo-50 px-6 py-3">
          <p className="mb-1 text-xs font-medium text-indigo-500">Import</p>
          <code className="text-sm text-indigo-800">{importPath}</code>
        </div>

        {/* Docs */}
        <div className="flex-1 overflow-y-auto px-6 py-6">
          {lib.docs ? (
            <div className="prose-sm prose max-w-none">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  h1: ({ children }) => (
                    <h1 className="mb-4 text-xl font-bold text-gray-900">{children}</h1>
                  ),
                  h2: ({ children }) => (
                    <h2 className="mb-3 mt-6 text-base font-semibold text-gray-800">{children}</h2>
                  ),
                  h3: ({ children }) => (
                    <h3 className="mb-2 mt-5 text-sm font-semibold text-gray-800">{children}</h3>
                  ),
                  p: ({ children }) => (
                    <p className="mb-3 text-sm leading-relaxed text-gray-600">{children}</p>
                  ),
                  code: ({ inline, children, ...props }: { inline?: boolean; children?: React.ReactNode }) =>
                    inline ? (
                      <code
                        className="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs text-gray-800"
                        {...props}
                      >
                        {children}
                      </code>
                    ) : (
                      <code {...props}>{children}</code>
                    ),
                  pre: ({ children }) => (
                    <pre className="mb-4 overflow-x-auto rounded-lg bg-gray-900 px-4 py-3 text-xs leading-relaxed text-gray-100">
                      {children}
                    </pre>
                  ),
                  table: ({ children }) => (
                    <div className="mb-4 overflow-x-auto">
                      <table className="w-full border-collapse text-sm">{children}</table>
                    </div>
                  ),
                  thead: ({ children }) => (
                    <thead className="bg-gray-50">{children}</thead>
                  ),
                  th: ({ children }) => (
                    <th className="border border-gray-200 px-4 py-2 text-left text-xs font-semibold text-gray-600">
                      {children}
                    </th>
                  ),
                  td: ({ children }) => (
                    <td className="border border-gray-200 px-4 py-2 text-xs text-gray-700">{children}</td>
                  ),
                  ul: ({ children }) => (
                    <ul className="mb-3 list-disc pl-5 text-sm text-gray-600">{children}</ul>
                  ),
                  li: ({ children }) => <li className="mb-1">{children}</li>,
                  strong: ({ children }) => (
                    <strong className="font-semibold text-gray-800">{children}</strong>
                  ),
                  em: ({ children }) => <em className="italic text-gray-600">{children}</em>,
                  hr: () => <hr className="my-5 border-gray-200" />,
                }}
              >
                {lib.docs}
              </ReactMarkdown>
            </div>
          ) : (
            <p className="text-sm text-gray-400">No documentation available.</p>
          )}
        </div>
      </aside>
    </>
  )
}
