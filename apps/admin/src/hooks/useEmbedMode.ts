import { useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'

export function useEmbedMode(): boolean {
  const [searchParams] = useSearchParams()
  return useMemo(() => {
    if (searchParams.get('embed') === 'true') return true
    try {
      return window.self !== window.top
    } catch {
      // cross-origin iframe — can't access window.top, so we're embedded
      return true
    }
  }, [searchParams])
}
