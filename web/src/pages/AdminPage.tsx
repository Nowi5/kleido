import { useEffect, useState } from 'react'
import { api, UserResponse, UsersListResponse } from '../api/client'
import {
  c, panelStyle,
  PageHeader, Spinner, StatCard,
  RoleBadge, StatusBadge,
  ErrorBanner,
  dangerBtnStyle, ghostBtnStyle,
} from '../ui'

const PER_PAGE = 20

interface Props {
  currentUserId: string
}

export default function AdminPage({ currentUserId }: Props) {
  const [data,        setData]        = useState<UsersListResponse | null>(null)
  const [page,        setPage]        = useState(1)
  const [loading,     setLoading]     = useState(true)
  const [error,       setError]       = useState<string | null>(null)
  const [confirmId,   setConfirmId]   = useState<string | null>(null)
  const [deleting,    setDeleting]    = useState<string | null>(null)

  const fetchPage = async (p: number) => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.users.list(p, PER_PAGE)
      setData(res)
      setPage(p)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchPage(1) }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const handleDelete = async (id: string) => {
    setDeleting(id)
    setConfirmId(null)
    try {
      await api.users.delete(id)
      // Refresh current page; if it becomes empty and we're past page 1, go back.
      const nextPage = (data && data.data.length === 1 && page > 1) ? page - 1 : page
      await fetchPage(nextPage)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setDeleting(null)
    }
  }

  const totalPages = data ? Math.ceil(data.total / PER_PAGE) : 0
  const activeCount   = data?.data.filter(u => u.is_active).length ?? 0
  const adminCount    = data?.data.filter(u => u.role === 'admin').length ?? 0

  return (
    <div>
      <PageHeader title="Admin dashboard" subtitle="Manage users and monitor account health" />

      {/* Stat row */}
      {data && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>
          <StatCard label="Total users"   value={data.total}   sub="all accounts" />
          <StatCard label="Active"        value={activeCount}  sub={`on page ${page}`} />
          <StatCard label="Admins"        value={adminCount}   sub={`on page ${page}`} />
          <StatCard label="Page"          value={`${page} / ${totalPages || 1}`} sub={`${PER_PAGE} per page`} />
        </div>
      )}

      {error && <ErrorBanner message={error} />}

      {/* User table */}
      <div style={panelStyle}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <h2 style={{ margin: 0, fontSize: 15, fontWeight: 600, color: c.text }}>Users</h2>
          <button
            onClick={() => fetchPage(page)}
            disabled={loading}
            style={{ ...ghostBtnStyle(true), fontSize: 12 }}
          >
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        {loading && !data ? (
          <Spinner />
        ) : (
          <>
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                <thead>
                  <tr style={{ borderBottom: `2px solid ${c.border}` }}>
                    {['Email', 'Role', 'Status', 'Joined', 'Actions'].map(h => (
                      <th key={h} style={{
                        padding: '0.625rem 0.75rem',
                        textAlign: 'left',
                        fontWeight: 600,
                        color: c.muted,
                        fontSize: 11,
                        textTransform: 'uppercase',
                        letterSpacing: '0.05em',
                        whiteSpace: 'nowrap',
                      }}>
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {data?.data.map(user => (
                    <UserRow
                      key={user.id}
                      user={user}
                      isSelf={user.id === currentUserId}
                      isConfirming={confirmId === user.id}
                      isDeleting={deleting === user.id}
                      onConfirm={() => setConfirmId(user.id)}
                      onCancel={() => setConfirmId(null)}
                      onDelete={() => handleDelete(user.id)}
                    />
                  ))}
                  {data?.data.length === 0 && (
                    <tr>
                      <td colSpan={5} style={{ padding: '2rem', textAlign: 'center', color: c.muted }}>
                        No users found.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: '1rem', paddingTop: '1rem', borderTop: `1px solid ${c.border}` }}>
                <span style={{ fontSize: 12, color: c.muted }}>
                  Page {page} of {totalPages} &middot; {data?.total} total users
                </span>
                <div style={{ display: 'flex', gap: '0.5rem' }}>
                  <button
                    onClick={() => fetchPage(page - 1)}
                    disabled={page <= 1 || loading}
                    style={ghostBtnStyle(true)}
                  >
                    ← Prev
                  </button>
                  <button
                    onClick={() => fetchPage(page + 1)}
                    disabled={page >= totalPages || loading}
                    style={ghostBtnStyle(true)}
                  >
                    Next →
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

// ── Row component ─────────────────────────────────────────────────────────────
function UserRow({
  user, isSelf,
  isConfirming, isDeleting,
  onConfirm, onCancel, onDelete,
}: {
  user: UserResponse
  isSelf: boolean
  isConfirming: boolean
  isDeleting: boolean
  onConfirm: () => void
  onCancel: () => void
  onDelete: () => void
}) {
  const joined = new Date(user.created_at).toLocaleDateString('en-US', {
    year: 'numeric', month: 'short', day: 'numeric',
  })

  return (
    <tr style={{
      borderBottom: `1px solid ${c.border}`,
      background: !user.is_active ? '#fafafa' : undefined,
      opacity: isDeleting ? 0.5 : 1,
      transition: 'opacity 0.2s',
    }}>
      {/* Email */}
      <td style={{ padding: '0.75rem', color: c.text, maxWidth: 240 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <div style={{
            width: 28,
            height: 28,
            borderRadius: '50%',
            background: `linear-gradient(135deg, ${c.primary}, #7c3aed)`,
            color: '#fff',
            fontSize: 11,
            fontWeight: 700,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}>
            {user.email.charAt(0).toUpperCase()}
          </div>
          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {user.email}
            {isSelf && (
              <span style={{ marginLeft: '0.35rem', fontSize: 10, background: c.primaryLight, color: c.primary, borderRadius: 4, padding: '1px 5px', fontWeight: 600 }}>
                you
              </span>
            )}
          </span>
        </div>
      </td>

      {/* Role */}
      <td style={{ padding: '0.75rem' }}>
        <RoleBadge role={user.role} />
      </td>

      {/* Status */}
      <td style={{ padding: '0.75rem' }}>
        <StatusBadge active={user.is_active} />
      </td>

      {/* Joined */}
      <td style={{ padding: '0.75rem', color: c.muted, whiteSpace: 'nowrap' }}>
        {joined}
      </td>

      {/* Actions */}
      <td style={{ padding: '0.75rem' }}>
        {isSelf ? (
          <span style={{ fontSize: 12, color: c.muted }}>—</span>
        ) : isConfirming ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
            <span style={{ fontSize: 12, color: c.danger, fontWeight: 500 }}>Deactivate?</span>
            <button onClick={onDelete}  style={dangerBtnStyle(true)}>Yes</button>
            <button onClick={onCancel}  style={ghostBtnStyle(true)}>No</button>
          </div>
        ) : (
          <button
            onClick={onConfirm}
            disabled={isDeleting || !user.is_active}
            title={!user.is_active ? 'Already inactive' : 'Deactivate user'}
            style={{
              ...dangerBtnStyle(true),
              opacity: (!user.is_active || isDeleting) ? 0.4 : 1,
              cursor: (!user.is_active || isDeleting) ? 'not-allowed' : 'pointer',
            }}
          >
            Deactivate
          </button>
        )}
      </td>
    </tr>
  )
}
