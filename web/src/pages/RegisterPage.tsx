import { FormEvent, useState } from 'react'
import { api } from '../api/client'
import {
  AuthCardHeader,
  centeredLayout, authCardStyle, inputStyle,
  primaryBtnStyle, linkBtnStyle,
  FormField, ErrorBanner, SuccessBanner,
} from '../ui'

interface Props {
  onSuccess: () => void  // called after successful registration → navigate to login
  onGoLogin: () => void
}

export default function RegisterPage({ onSuccess, onGoLogin }: Props) {
  const [email,    setEmail]    = useState('')
  const [password, setPassword] = useState('')
  const [confirm,  setConfirm]  = useState('')
  const [error,    setError]    = useState<string | null>(null)
  const [success,  setSuccess]  = useState(false)
  const [loading,  setLoading]  = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (password !== confirm) {
      setError('Passwords do not match.')
      return
    }
    setError(null)
    setLoading(true)
    try {
      await api.auth.register(email, password)
      setSuccess(true)
      // Give the user a moment to read the success message, then go to login.
      setTimeout(onSuccess, 1500)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={centeredLayout}>
      <div style={authCardStyle}>
        <AuthCardHeader title="Create account" subtitle="Sign up with your email address" />

        {error   && <ErrorBanner   message={error} />}
        {success && <SuccessBanner message="Account created! Redirecting to sign in…" />}

        {!success && (
          <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
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
                placeholder="Min. 8 chars, upper, digit, special"
                required
                autoComplete="new-password"
                style={inputStyle}
              />
            </FormField>

            <FormField label="Confirm password">
              <input
                type="password"
                value={confirm}
                onChange={e => setConfirm(e.target.value)}
                placeholder="••••••••"
                required
                autoComplete="new-password"
                style={inputStyle}
              />
            </FormField>

            <button type="submit" disabled={loading} style={primaryBtnStyle(loading)}>
              {loading ? 'Creating account…' : 'Create account'}
            </button>
          </form>
        )}

        <p style={{ textAlign: 'center', marginTop: '1.5rem', fontSize: 14, color: '#64748b', margin: '1.5rem 0 0' }}>
          Already have an account?{' '}
          <button onClick={onGoLogin} style={linkBtnStyle}>Sign in</button>
        </p>
      </div>
    </div>
  )
}
