const BASE = import.meta.env.VITE_API_URL ?? ''

// ── Response types ────────────────────────────────────────────────────────────
export interface UserResponse {
  id: string
  email: string
  role: string
  is_active: boolean
  created_at: string
}

export interface TokenResponse {
  access_token: string
  expires_at: string
}

export interface UsersListResponse {
  data: UserResponse[]
  total: number
  page: number
  per_page: number
}

export interface TenantResponse {
  id: string
  name: string
  slug: string
}

// ── In-memory token store ─────────────────────────────────────────────────────
// Access token lives in memory only — never localStorage / sessionStorage.
// localStorage is accessible to any XSS payload; memory is safer for SPAs.
let _token: string | null = null
let _tenantId: string | null = null
export const setToken  = (t: string) => { _token = t }
export const clearToken = ()          => { _token = null }
export const getToken  = ()           => _token
export const setTenantId = (id: string) => { _tenantId = id }
export const getTenantId = () => _tenantId

// ── Core fetch wrapper ────────────────────────────────────────────────────────
async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (_token) headers['Authorization'] = `Bearer ${_token}`

  let url = BASE + path
  if (_tenantId) {
    const separator = path.includes('?') ? '&' : '?'
    url = `${url}${separator}tenant_id=${_tenantId}`
  }

  const res = await fetch(url, {
    method,
    headers,
    credentials: 'include', // sends refresh_token httpOnly cookie automatically
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    const payload = await res.json().catch(() => ({}))
    const msg = (payload as { error?: { message?: string } })?.error?.message
    throw new Error(msg ?? `HTTP ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

// ── API surface ───────────────────────────────────────────────────────────────
export const api = {
  auth: {
    register: (email: string, password: string) =>
      request<UserResponse>('POST', '/api/v1/auth/register', { email, password }),

    login: (email: string, password: string) =>
      request<TokenResponse>('POST', '/api/v1/auth/login', { email, password }),

    logout: () =>
      request<void>('POST', '/api/v1/auth/logout'),

    refresh: () =>
      request<TokenResponse>('POST', '/api/v1/auth/refresh'),
  },

  users: {
    me: () =>
      request<UserResponse>('GET', '/api/v1/users/me'),

    list: (page = 1, perPage = 20) =>
      request<UsersListResponse>('GET', `/api/v1/users?page=${page}&per_page=${perPage}`),

    delete: (id: string) =>
      request<void>('DELETE', `/api/v1/users/${id}`),
  },

  tenants: {
    list: () =>
      request<TenantResponse[]>('GET', '/api/v1/tenants'),
  },
}
