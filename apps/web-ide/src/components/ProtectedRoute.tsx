import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'

interface Props {
  children: ReactNode
}

export default function ProtectedRoute({ children }: Props) {
  const auth = sessionStorage.getItem('velane_auth')
  if (!auth) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}
