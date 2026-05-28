import { useState } from 'react'
import { ChevronDown } from 'lucide-react'

interface Props {
  onPublish: (env: 'dev' | 'staging' | 'prod') => void
  disabled?: boolean
}

const envs: Array<'dev' | 'staging' | 'prod'> = ['dev', 'staging', 'prod']

export default function PublishDropdown({ onPublish, disabled }: Props) {
  const [open, setOpen] = useState(false)

  return (
    <div className="relative">
      <button
        className="inline-flex items-center gap-1 rounded-md bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
        disabled={disabled}
        onClick={() => setOpen((v) => !v)}
      >
        Publish
        <ChevronDown className="h-4 w-4" />
      </button>
      {open && (
        <div className="absolute right-0 z-10 mt-1 w-36 rounded-md border border-gray-200 bg-white shadow-lg">
          {envs.map((env) => (
            <button
              key={env}
              className="block w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
              onClick={() => {
                onPublish(env)
                setOpen(false)
              }}
            >
              {env}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
