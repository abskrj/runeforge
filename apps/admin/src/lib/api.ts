import type {
  User,
  Branding,
  TenantMember,
  InviteToken,
  UsageSummary,
  APIKey,
  EgressPolicy,
  Snippet,
  SnippetVersion,
  InvocationResult,
  EmbedToken,
  Secret,
  Library,
  LibraryVersion,
  PlatformLibrary,
} from '../types'

const BASE = '/api'

function getToken(): string {
  return localStorage.getItem('sessionToken') ?? ''
}

function getSlug(): string {
  return localStorage.getItem('tenantSlug') ?? ''
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  authType: 'session' | 'apikey' | 'none' = 'session',
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (authType === 'session') {
    const token = getToken()
    if (token) headers['Authorization'] = `Bearer ${token}`
  } else if (authType === 'apikey') {
    // Fall back to session token if no dedicated API key is stored — the backend
    // accepts JWT session tokens on all scoped endpoints when X-Tenant is provided.
    const key = localStorage.getItem('apiKey') ?? getToken()
    if (key) headers['Authorization'] = `Bearer ${key}`
  }
  // Always send X-Tenant so the backend can resolve session user's membership role.
  const slug = getSlug()
  if (slug) headers['X-Tenant'] = slug

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  // Auth
  async login(email: string, password: string): Promise<{ session_token: string; expires_at: string; tenant_slug: string }> {
    return request('POST', '/v1/admin/auth/login', { email, password }, 'none')
  },

  async register(
    email: string,
    password: string,
    inviteToken?: string,
  ): Promise<{ user: User; session_token: string; tenant_slug: string }> {
    return request('POST', '/v1/admin/auth/register', { email, password, invite_token: inviteToken }, 'none')
  },

  async logout(): Promise<void> {
    return request('POST', '/v1/admin/auth/logout', undefined, 'session')
  },

  async me(): Promise<User> {
    return request('GET', '/v1/admin/auth/me', undefined, 'session')
  },

  // Branding
  async getBranding(): Promise<Branding> {
    return request('GET', `/v1/tenants/${getSlug()}/branding`, undefined, 'apikey')
  },

  async updateBranding(b: Branding): Promise<Branding> {
    return request('PUT', `/v1/tenants/${getSlug()}/branding`, b, 'apikey')
  },

  // Members
  async listMembers(): Promise<TenantMember[]> {
    return request('GET', `/v1/tenants/${getSlug()}/members`, undefined, 'apikey')
  },

  async inviteMember(email: string, role: string): Promise<{ invite_token: string; expires_at: string }> {
    return request('POST', `/v1/tenants/${getSlug()}/members/invite`, { email, role }, 'apikey')
  },

  async removeMember(userID: string): Promise<void> {
    return request('DELETE', `/v1/tenants/${getSlug()}/members/${userID}`, undefined, 'apikey')
  },

  async listInvites(): Promise<InviteToken[]> {
    return request('GET', `/v1/tenants/${getSlug()}/members/invites`, undefined, 'apikey')
  },

  // Usage
  async getUsage(window: string): Promise<UsageSummary> {
    return request('GET', `/v1/tenants/${getSlug()}/usage?window=${window}`, undefined, 'apikey')
  },

  // API Keys
  async listAPIKeys(): Promise<APIKey[]> {
    return request('GET', `/v1/tenants/${getSlug()}/api-keys`, undefined, 'apikey')
  },

  async createAPIKey(name: string, scopes: string[]): Promise<APIKey> {
    return request('POST', `/v1/tenants/${getSlug()}/api-keys`, { name, scopes }, 'apikey')
  },

  async deleteAPIKey(id: string): Promise<void> {
    return request('DELETE', `/v1/tenants/${getSlug()}/api-keys/${id}`, undefined, 'apikey')
  },

  // Egress
  async getEgressPolicy(): Promise<EgressPolicy> {
    return request('GET', `/v1/tenants/${getSlug()}/egress`, undefined, 'apikey')
  },

  async updateEgressPolicy(p: EgressPolicy): Promise<EgressPolicy> {
    return request('PUT', `/v1/tenants/${getSlug()}/egress`, p, 'apikey')
  },

  // Snippets
  async listSnippets(): Promise<Snippet[]> {
    return request('GET', `/v1/snippets`, undefined, 'apikey')
  },

  async getSnippet(id: string): Promise<Snippet> {
    return request('GET', `/v1/snippets/${id}`, undefined, 'apikey')
  },

  async createSnippet(data: { name: string; language: string; description?: string }): Promise<Snippet> {
    const slug = data.name.toLowerCase().replace(/\s+/g, '-')
    return request('POST', `/v1/snippets`, { ...data, slug }, 'apikey')
  },

  async updateSnippet(id: string, data: Partial<{ name: string; description: string }>): Promise<Snippet> {
    return request('PATCH', `/v1/snippets/${id}`, data, 'apikey')
  },

  async deleteSnippet(id: string): Promise<void> {
    return request('DELETE', `/v1/snippets/${id}`, undefined, 'apikey')
  },

  // Versions
  async listVersions(snippetId: string): Promise<SnippetVersion[]> {
    return request('GET', `/v1/snippets/${snippetId}/versions`, undefined, 'apikey')
  },

  async createVersion(snippetId: string, code: string): Promise<SnippetVersion> {
    return request('POST', `/v1/snippets/${snippetId}/versions`, { code }, 'apikey')
  },

  async publishVersion(snippetId: string, versionNum: number, env: string): Promise<void> {
    return request('POST', `/v1/snippets/${snippetId}/versions/${versionNum}/publish?env=${env}`, undefined, 'apikey')
  },

  // Libraries
  async listLibraries(language?: string): Promise<{ platform: PlatformLibrary[]; tenant: Library[] }> {
    const qs = language ? `?language=${language}` : ''
    return request('GET', `/v1/libraries${qs}`, undefined, 'apikey')
  },

  async createLibrary(data: { name: string; slug: string; language: string; description?: string }): Promise<Library> {
    return request('POST', '/v1/libraries', data, 'apikey')
  },

  async deleteLibrary(id: string): Promise<void> {
    return request('DELETE', `/v1/libraries/${id}`, undefined, 'apikey')
  },

  async listLibraryVersions(libraryId: string): Promise<LibraryVersion[]> {
    return request('GET', `/v1/libraries/${libraryId}/versions`, undefined, 'apikey')
  },

  async createLibraryVersion(libraryId: string, code: string): Promise<LibraryVersion> {
    return request('POST', `/v1/libraries/${libraryId}/versions`, { code }, 'apikey')
  },

  async publishLibraryVersion(libraryId: string, versionNumber: number): Promise<LibraryVersion> {
    return request('POST', `/v1/libraries/${libraryId}/versions/${versionNumber}/publish`, undefined, 'apikey')
  },

  // Variables & Credentials (secrets)
  async listSecrets(): Promise<Secret[]> {
    return request('GET', '/v1/secrets', undefined, 'apikey')
  },

  async createSecret(data: { name: string; value: string; is_secret: boolean; environments?: string[] }): Promise<Secret> {
    return request('POST', '/v1/secrets', data, 'apikey')
  },

  async updateSecret(id: string, data: { name?: string; value?: string }): Promise<Secret> {
    return request('PATCH', `/v1/secrets/${id}`, data, 'apikey')
  },

  async deleteSecret(id: string): Promise<void> {
    return request('DELETE', `/v1/secrets/${id}`, undefined, 'apikey')
  },

  // Embed tokens
  async listEmbedTokens(): Promise<EmbedToken[]> {
    return request('GET', '/v1/embed/tokens', undefined, 'apikey')
  },

  async createEmbedToken(snippetIds: string[], ttlSeconds = 3600): Promise<{ id: string; token: string; expires_at: string }> {
    return request('POST', '/v1/embed/tokens', { snippet_ids: snippetIds, ttl_seconds: ttlSeconds }, 'apikey')
  },

  async revokeEmbedToken(id: string): Promise<void> {
    return request('DELETE', `/v1/embed/tokens/${id}`, undefined, 'apikey')
  },

  // Invocation
  async invokeSnippet(snippetSlug: string, input: string, env = 'dev'): Promise<InvocationResult> {
    const tenantSlug = getSlug()
    return request('POST', `/v1/invoke/${tenantSlug}/${snippetSlug}?env=${env}`, JSON.parse(input || '{}'), 'apikey')
  },
}
