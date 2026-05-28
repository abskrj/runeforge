export interface User {
  id: string
  email: string
  created_at: string
  updated_at: string
}

export interface Session {
  session_token: string
  expires_at: string
}

export interface Branding {
  logo_url?: string
  accent_color?: string
  font_family?: string
  custom_domain?: string
  hide_branding?: boolean
}

export interface TenantMember {
  tenant_id: string
  user_id: string
  email: string
  role: string
  invited_at: string
}

export interface InviteToken {
  id: string
  tenant_id: string
  email: string
  role: string
  expires_at: string
  accepted_at?: string
  created_at: string
}

export interface UsageTopSnippet {
  snippet_id: string
  name: string
  invocations: number
  p95_ms: number
}

export interface UsageSummary {
  tenant_id: string
  window: string
  total_invocations: number
  error_rate: number
  avg_duration_ms: number
  top_snippets: UsageTopSnippet[]
}

export interface APIKey {
  id: string
  name: string
  scopes: string[]
  key_prefix: string
  key?: string // only present on creation
  last_used_at?: string
  created_at: string
}

export interface EgressPolicy {
  blocked_cidrs: string[]
  blocked_domains: string[]
}

export interface Snippet {
  id: string
  name: string
  slug: string
  language: string
  description: string
  created_at: string
}

export interface SnippetVersion {
  id: string
  snippet_id: string
  version_number: number
  code: string
  status: 'draft' | 'published' | 'archived'
  created_at: string
}

export interface SnippetEnvironment {
  env: string
  active_version_number: number | null
}

export interface InvocationResult {
  output: string
  error: string
  duration_ms: number
  exit_code: number
}
