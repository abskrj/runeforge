import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'

export default function EmbedEntryPage() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const [error, setError] = useState('')

  useEffect(() => {
    const token = params.get('token')
    if (!token?.startsWith('et_')) {
      navigate('/login', { replace: true })
      return
    }

    async function bootstrap() {
      try {
        const res = await fetch('/api/v1/embed/bootstrap', {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!res.ok) {
          throw new Error('Invalid or expired embed token')
        }
        const data = await res.json()
        localStorage.setItem('apiKey', token!)
        localStorage.setItem('tenantSlug', data.tenant.slug)
        navigate('/dashboard/snippets?embed=true', { replace: true })
      } catch (err) {
        setError(String(err))
      }
    }
    bootstrap()
  }, [])

  if (error) {
    return (
      <div className="flex h-screen items-center justify-center bg-gray-50">
        <div className="rounded-lg border border-red-200 bg-white p-8 text-center shadow-sm">
          <p className="text-sm font-medium text-red-600">{error}</p>
          <p className="mt-2 text-xs text-gray-400">The embed link may have expired. Request a new one from the workspace admin.</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-screen items-center justify-center bg-gray-50">
      <p className="text-sm text-gray-400">Loading workspace...</p>
    </div>
  )
}
