import { FormEvent, useState, useEffect } from 'react'
import { api, setToken, setTenantId, TenantResponse, UserResponse } from '../api/client'
import {
  AuthCardHeader,
  centeredLayout, authCardStyle, inputStyle,
  primaryBtnStyle, linkBtnStyle,
  FormField, ErrorBanner,
} from '../ui'

interface Props {
  onLogin: (token: string, user: UserResponse) => void
  onGoRegister: () => void
}

export default function LoginPage({ onLogin, onGoRegister }: Props) {
  const [email,    setEmail]    = useState('')
  const [password, setPassword] = useState('')
  const [error,    setError]    = useState<string | null>(null)
  const [loading,  setLoading]  = useState(false)
  const [tenants, setTenants]   = useState<TenantResponse[]>([])
  const [selectedTenant, setSelectedTenant] = useState<string>('')

  useEffect(() => {
    api.tenants.list()
      .then(setTenants)
      .catch(() => {})
  }, [])

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      if (selectedTenant) {
        setTenantId(selectedTenant)
      }
      const tok  = await api.auth.login(email, password)
      setToken(tok.access_token)
      const user = await api.users.me()
      onLogin(tok.access_token, user)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={centeredLayout}>
      <div style={authCardStyle}>
        <AuthCardHeader title="Sign in" subtitle="Enter your credentials to continue" />

        {error && <ErrorBanner message={error} />}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {tenants.length > 0 && (
            <FormField label="Organization">
              <select
                value={selectedTenant}
                onChange={e => setSelectedTenant(e.target.value)}
                style={{ ...inputStyle, padding: '0.75rem' }}
                required={tenants.length > 1}
              >
                <option value="">{tenants.length > 1 ? 'Select your organization' : tenants[0].name}</option>
                {tenants.length > 1 && tenants.map(t => (
                  <option key={t.id} value={t.id}>{t.name}</option>
                ))}
              </select>
            </FormField>
          )}

          <FormField label="Email">
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              placeholder="you@example.com"
              required
              autoComplete="email"
              style={inputStyle}
            />
          </FormField>

          <FormField label="Password">
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="••••••••"
              required
              autoComplete="current-password"
              style={inputStyle}
            />
          </FormField>

          <button type="submit" disabled={loading} style={primaryBtnStyle(loading)}>
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
        </form>

        <p style={{ textAlign: 'center', marginTop: '1.5rem', fontSize: 14, color: '#64748b', margin: '1.5rem 0 0' }}>
          Don't have an account?{' '}
          <button onClick={onGoRegister} style={linkBtnStyle}>Create one</button>
        </p>
      </div>
    </div>
  )
}
