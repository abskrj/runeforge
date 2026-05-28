import type { CreateSnippetInput, InvocationResult, Snippet, SnippetVersion } from '../types'

function getAuth(): { tenant: string; apiKey: string } | null {
  const raw = sessionStorage.getItem('velane_auth')
  if (!raw) return null
  try {
    return JSON.parse(raw)
  } catch {
    return null
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const auth = getAuth()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }
  if (auth) {
    headers['Authorization'] = `Bearer ${auth.apiKey}`
  }

  const resp = await fetch(path, { ...options, headers })
  if (!resp.ok) {
    const text = await resp.text()
    throw new Error(`HTTP ${resp.status}: ${text}`)
  }
  if (resp.status === 204) return undefined as unknown as T
  return resp.json()
}

export const api = {
  listSnippets(): Promise<Snippet[]> {
    return request<Snippet[]>('/v1/snippets')
  },

  getSnippet(id: string): Promise<Snippet> {
    return request<Snippet>(`/v1/snippets/${id}`)
  },

  createSnippet(data: CreateSnippetInput): Promise<Snippet> {
    return request<Snippet>('/v1/snippets', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  updateSnippet(id: string, data: Partial<CreateSnippetInput>): Promise<Snippet> {
    return request<Snippet>(`/v1/snippets/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
  },

  deleteSnippet(id: string): Promise<void> {
    return request<void>(`/v1/snippets/${id}`, { method: 'DELETE' })
  },

  listVersions(snippetId: string): Promise<SnippetVersion[]> {
    return request<SnippetVersion[]>(`/v1/snippets/${snippetId}/versions`)
  },

  createVersion(snippetId: string, code: string): Promise<SnippetVersion> {
    return request<SnippetVersion>(`/v1/snippets/${snippetId}/versions`, {
      method: 'POST',
      body: JSON.stringify({ code }),
    })
  },

  publishVersion(snippetId: string, versionNum: number, env: string): Promise<SnippetVersion> {
    return request<SnippetVersion>(
      `/v1/snippets/${snippetId}/versions/${versionNum}/publish?env=${env}`,
      { method: 'POST' },
    )
  },

  invokeSnippet(id: string, input: string, env = 'prod'): Promise<InvocationResult> {
    const auth = getAuth()
    const tenant = auth?.tenant ?? ''
    return request<InvocationResult>(`/v1/invoke/${tenant}/${id}?env=${env}`, {
      method: 'POST',
      body: input,
    })
  },

  invokeSnippetStream(id: string, _input: string, env = 'prod'): EventSource {
    const auth = getAuth()
    const tenant = auth?.tenant ?? ''
    return new EventSource(`/v1/invoke/${tenant}/${id}?env=${env}&stream=1`)
  },
}
