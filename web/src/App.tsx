import { useEffect, useState, type ReactNode } from 'react'
import { api, setToken, clearToken, UserResponse } from './api/client'
import { c } from './ui'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import ProfilePage from './pages/ProfilePage'
import AdminPage from './pages/AdminPage'

type Page = 'login' | 'register' | 'profile' | 'admin'

// ── App ───────────────────────────────────────────────────────────────────────
export default function App() {
  const [page, setPage]     = useState<Page>('login')
  const [user, setUser]     = useState<UserResponse | null>(null)
  const [booting, setBooting] = useState(true)

  const navigate = (p: Page) => {
    window.location.hash = `#/${p}`
    setPage(p)
  }

  // Attempt silent session restore using the refresh-token httpOnly cookie.
  useEffect(() => {
    api.auth.refresh()
      .then(tok => {
        setToken(tok.access_token)
        return api.users.me()
      })
      .then(u => {
        setUser(u)
        navigate('profile')
      })
      .catch(() => {
        clearToken()
        navigate('login')
      })
      .finally(() => setBooting(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const handleLogin = (token: string, u: UserResponse) => {
    setToken(token)
    setUser(u)
    navigate('profile')
  }

  const handleLogout = async () => {
    try { await api.auth.logout() } catch { /* best-effort */ }
    clearToken()
    setUser(null)
    navigate('login')
  }

  // ── Boot splash ─────────────────────────────────────────────────────────────
  if (booting) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', background: c.bg }}>
        <span style={{ color: c.muted, fontSize: 14 }}>Loading…</span>
      </div>
    )
  }

  // ── Shell ───────────────────────────────────────────────────────────────────
  return (
    <div style={{
      minHeight: '100vh',
      background: c.bg,
      fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
      color: c.text,
    }}>
      {user && <AppHeader user={user} current={page} onNavigate={navigate} onLogout={handleLogout} />}

      <main style={{ maxWidth: 1200, margin: '0 auto', padding: '2rem 1.5rem' }}>
        {page === 'login'    && <LoginPage    onLogin={handleLogin} onGoRegister={() => navigate('register')} />}
        {page === 'register' && <RegisterPage onSuccess={() => navigate('login')}  onGoLogin={() => navigate('login')} />}
        {page === 'profile'  && user && <ProfilePage user={user} />}
        {page === 'admin'    && user?.role === 'admin'  && <AdminPage currentUserId={user.id} />}
        {page === 'admin'    && user?.role !== 'admin'  && (
          <p style={{ color: c.danger }}>Access denied — admin role required.</p>
        )}
      </main>
    </div>
  )
}

// ── Header ────────────────────────────────────────────────────────────────────
function AppHeader({
  user, current, onNavigate, onLogout,
}: {
  user: UserResponse
  current: Page
  onNavigate: (p: Page) => void
  onLogout: () => void
}) {
  return (
    <header style={{
      background: c.surface,
      borderBottom: `1px solid ${c.border}`,
      position: 'sticky',
      top: 0,
      zIndex: 10,
    }}>
      <div style={{
        maxWidth: 1200,
        margin: '0 auto',
        padding: '0 1.5rem',
        height: 56,
        display: 'flex',
        alignItems: 'center',
        gap: '1rem',
      }}>
        {/* Wordmark */}
        <span
          onClick={() => onNavigate('profile')}
          style={{ fontWeight: 700, fontSize: 17, color: c.primary, cursor: 'pointer', letterSpacing: '-0.02em', marginRight: 'auto' }}
        >
          Kleido
        </span>

        {/* Nav */}
        <nav style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
          <NavBtn active={current === 'profile'} onClick={() => onNavigate('profile')}>Profile</NavBtn>
          {user.role === 'admin' && (
            <NavBtn active={current === 'admin'} onClick={() => onNavigate('admin')}>Admin</NavBtn>
          )}
        </nav>

        {/* User pill + sign-out */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginLeft: '0.5rem' }}>
          <span style={{ fontSize: 13, color: c.muted, maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {user.email}
          </span>
          <button
            onClick={onLogout}
            style={{
              padding: '0.35rem 0.75rem',
              borderRadius: 6,
              border: `1px solid ${c.dangerBorder}`,
              background: c.dangerBg,
              color: c.danger,
              fontSize: 13,
              fontWeight: 500,
              cursor: 'pointer',
            }}
          >
            Sign out
          </button>
        </div>
      </div>
    </header>
  )
}

function NavBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      onClick={onClick}
      style={{
        padding: '0.375rem 0.875rem',
        borderRadius: 6,
        border: 'none',
        background: active ? c.primaryLight : 'transparent',
        color: active ? c.primary : c.muted,
        fontWeight: active ? 600 : 400,
        fontSize: 13,
        cursor: 'pointer',
      }}
    >
      {children}
    </button>
  )
}
