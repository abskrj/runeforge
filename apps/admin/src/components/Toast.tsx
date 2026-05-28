import { useEffect, useState } from 'react'
import clsx from 'clsx'

interface ToastProps {
  message: string
  type: 'success' | 'error'
  onDismiss: () => void
}

export function Toast({ message, type, onDismiss }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, 3000)
    return () => clearTimeout(timer)
  }, [onDismiss])

  return (
    <div
      className={clsx(
        'fixed bottom-4 right-4 z-50 rounded-lg px-4 py-3 text-sm font-medium shadow-lg',
        type === 'success' ? 'bg-green-600 text-white' : 'bg-red-600 text-white',
      )}
    >
      {message}
    </div>
  )
}

interface ToastState {
  message: string
  type: 'success' | 'error'
}

export function useToast() {
  const [toast, setToast] = useState<ToastState | null>(null)

  function showToast(message: string, type: 'success' | 'error' = 'success') {
    setToast({ message, type })
  }

  function dismissToast() {
    setToast(null)
  }

  return { toast, showToast, dismissToast }
}
