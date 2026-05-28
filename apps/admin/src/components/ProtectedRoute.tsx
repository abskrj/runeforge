import { Navigate } from 'react-router-dom'
import type { ReactNode } from 'react'

interface Props {
  children: ReactNode
}

export default function ProtectedRoute({ children }: Props) {
  const sessionToken = localStorage.getItem('sessionToken')
  const apiKey = localStorage.getItem('apiKey')
  if (!sessionToken && !apiKey) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}
