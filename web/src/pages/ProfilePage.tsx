import { UserResponse } from '../api/client'
import { c, panelStyle, PageHeader, RoleBadge, StatusBadge, StatCard } from '../ui'

interface Props {
  user: UserResponse
}

export default function ProfilePage({ user }: Props) {
  const joined = new Date(user.created_at).toLocaleDateString('en-US', {
    year: 'numeric', month: 'long', day: 'numeric',
  })

  return (
    <div style={{ maxWidth: 640 }}>
      <PageHeader title="My profile" subtitle="Your account details" />

      {/* Avatar + identity card */}
      <div style={{ ...panelStyle, display: 'flex', alignItems: 'center', gap: '1.25rem', marginBottom: '1.5rem' }}>
        <div style={{
          width: 52,
          height: 52,
          borderRadius: '50%',
          background: `linear-gradient(135deg, ${c.primary}, #7c3aed)`,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: '#fff',
          fontSize: 20,
          fontWeight: 700,
          flexShrink: 0,
          userSelect: 'none',
        }}>
          {user.email.charAt(0).toUpperCase()}
        </div>
        <div>
          <div style={{ fontWeight: 600, fontSize: 16, color: c.text }}>{user.email}</div>
          <div style={{ marginTop: '0.35rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <RoleBadge role={user.role} />
            <StatusBadge active={user.is_active} />
          </div>
        </div>
      </div>

      {/* Stat row */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>
        <StatCard label="Role"         value={user.role} />
        <StatCard label="Status"       value={user.is_active ? 'Active' : 'Inactive'} />
        <StatCard label="Member since" value={joined} />
      </div>

      {/* Detail rows */}
      <div style={panelStyle}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <tbody>
            {([
              ['User ID',  user.id],
              ['Email',    user.email],
              ['Role',     user.role],
              ['Active',   user.is_active ? 'Yes' : 'No'],
              ['Joined',   joined],
            ] as [string, string][]).map(([label, value]) => (
              <tr key={label} style={{ borderBottom: `1px solid ${c.border}` }}>
                <td style={{ padding: '0.75rem 0', fontWeight: 500, color: c.muted, width: 130, verticalAlign: 'top' }}>
                  {label}
                </td>
                <td style={{ padding: '0.75rem 0', color: c.text, wordBreak: 'break-all' }}>
                  {value}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
