/**
 * Salesforce Cases — @velane/salesforce-cases
 *
 * Credentials (set as Credentials in the Variables tab):
 *   SALESFORCE_INSTANCE_URL     e.g. https://yourorg.my.salesforce.com
 *   SALESFORCE_ACCESS_TOKEN     long-lived / session token  ← simplest
 *   — OR username-password OAuth flow —
 *   SALESFORCE_CLIENT_ID
 *   SALESFORCE_CLIENT_SECRET
 *   SALESFORCE_USERNAME
 *   SALESFORCE_PASSWORD
 *   SALESFORCE_SECURITY_TOKEN   (append to password if required by your org)
 *
 * Usage:
 *   import { createCase, getCase, updateCase, deleteCase } from '@velane/salesforce-cases'
 */

const SF_API_VERSION = 'v60.0'

async function auth(): Promise<{ instanceUrl: string; token: string }> {
  const instanceUrl = (process.env.SALESFORCE_INSTANCE_URL ?? '').replace(/\/$/, '')
  if (!instanceUrl) throw new Error('SALESFORCE_INSTANCE_URL is required')

  const accessToken = process.env.SALESFORCE_ACCESS_TOKEN
  if (accessToken) return { instanceUrl, token: accessToken }

  const clientId     = process.env.SALESFORCE_CLIENT_ID
  const clientSecret = process.env.SALESFORCE_CLIENT_SECRET
  const username     = process.env.SALESFORCE_USERNAME
  const password     = process.env.SALESFORCE_PASSWORD ?? ''
  const secToken     = process.env.SALESFORCE_SECURITY_TOKEN ?? ''

  if (!clientId || !clientSecret || !username) {
    throw new Error(
      'Set SALESFORCE_ACCESS_TOKEN, or all of: SALESFORCE_CLIENT_ID, ' +
      'SALESFORCE_CLIENT_SECRET, SALESFORCE_USERNAME, SALESFORCE_PASSWORD'
    )
  }

  const body = new URLSearchParams({
    grant_type:    'password',
    client_id:     clientId,
    client_secret: clientSecret,
    username,
    password:      password + secToken,
  })

  const res = await fetch(`${instanceUrl}/services/oauth2/token`, {
    method:  'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body:    body.toString(),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(`Salesforce auth failed: ${err.error_description ?? res.statusText}`)
  }
  const data = await res.json()
  return { instanceUrl: data.instance_url ?? instanceUrl, token: data.access_token }
}

async function sfFetch(method: string, path: string, body?: unknown): Promise<unknown> {
  const { instanceUrl, token } = await auth()
  const res = await fetch(`${instanceUrl}/services/data/${SF_API_VERSION}${path}`, {
    method,
    headers: {
      Authorization:  `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => [{ message: res.statusText }])
    const msg = Array.isArray(err) ? (err[0]?.message ?? res.statusText) : res.statusText
    throw new Error(`Salesforce ${method} ${path} failed (${res.status}): ${msg}`)
  }
  if (res.status === 204) return null
  return res.json()
}

export interface CaseFields {
  Subject?:     string
  Description?: string
  Status?:      string   // 'New' | 'Working' | 'Escalated' | 'Closed'
  Priority?:    string   // 'High' | 'Medium' | 'Low'
  Origin?:      string   // 'Email' | 'Phone' | 'Web'
  Type?:        string
  Reason?:      string
  AccountId?:   string
  ContactId?:   string
  [key: string]: unknown
}

export interface CaseRecord extends CaseFields {
  Id:               string
  CaseNumber:       string
  CreatedDate:      string
  LastModifiedDate: string
}

export interface CreateCaseResult {
  id:      string
  success: boolean
  errors:  unknown[]
}

/** Create a new Salesforce Case. */
export async function createCase(fields: CaseFields): Promise<CreateCaseResult> {
  return sfFetch('POST', '/sobjects/Case', fields) as Promise<CreateCaseResult>
}

/** Fetch a Salesforce Case by its 15- or 18-character ID. */
export async function getCase(id: string): Promise<CaseRecord> {
  return sfFetch('GET', `/sobjects/Case/${id}`) as Promise<CaseRecord>
}

/** Update fields on an existing Case. */
export async function updateCase(id: string, fields: Partial<CaseFields>): Promise<void> {
  await sfFetch('PATCH', `/sobjects/Case/${id}`, fields)
}

/** Permanently delete a Case. */
export async function deleteCase(id: string): Promise<void> {
  await sfFetch('DELETE', `/sobjects/Case/${id}`)
}
