/**
 * Shared design system — tokens, style helpers, and small reusable components.
 * Import from here to keep pages consistent without an external CSS framework.
 */
import type { CSSProperties, ReactNode } from 'react'

// ── Design tokens ─────────────────────────────────────────────────────────────
export const c = {
  primary:       '#2563eb',
  primaryDark:   '#1d4ed8',
  primaryLight:  '#eff6ff',
  bg:            '#f8fafc',
  surface:       '#ffffff',
  border:        '#e2e8f0',
  text:          '#1e293b',
  muted:         '#64748b',
  danger:        '#dc2626',
  dangerBg:      '#fef2f2',
  dangerBorder:  '#fecaca',
  successText:   '#16a34a',
  successBg:     '#f0fdf4',
  successBorder: '#bbf7d0',
} as const

// ── Layout ────────────────────────────────────────────────────────────────────
export const centeredLayout: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  minHeight: 'calc(100vh - 8rem)',
}

export const authCardStyle: CSSProperties = {
  background: c.surface,
  borderRadius: 12,
  border: `1px solid ${c.border}`,
  boxShadow: '0 1px 3px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.04)',
  padding: '2.5rem',
  width: '100%',
  maxWidth: 420,
}

export const panelStyle: CSSProperties = {
  background: c.surface,
  borderRadius: 12,
  border: `1px solid ${c.border}`,
  boxShadow: '0 1px 3px rgba(0,0,0,0.06)',
  padding: '1.5rem',
}

// ── Form elements ─────────────────────────────────────────────────────────────
export const inputStyle: CSSProperties = {
  padding: '0.5rem 0.75rem',
  borderRadius: 8,
  border: `1px solid ${c.border}`,
  fontSize: 14,
  width: '100%',
  boxSizing: 'border-box',
  background: '#f8fafc',
  color: c.text,
  outline: 'none',
}

export function primaryBtnStyle(disabled = false): CSSProperties {
  return {
    padding: '0.625rem 1rem',
    borderRadius: 8,
    border: 'none',
    background: disabled ? '#93c5fd' : c.primary,
    color: '#ffffff',
    fontWeight: 600,
    fontSize: 14,
    cursor: disabled ? 'not-allowed' : 'pointer',
    width: '100%',
    marginTop: '0.5rem',
  }
}

export const linkBtnStyle: CSSProperties = {
  background: 'none',
  border: 'none',
  color: c.primary,
  cursor: 'pointer',
  fontWeight: 500,
  fontSize: 14,
  padding: 0,
}

export function dangerBtnStyle(small = false): CSSProperties {
  return {
    padding: small ? '0.25rem 0.5rem' : '0.5rem 0.875rem',
    borderRadius: 6,
    border: `1px solid ${c.dangerBorder}`,
    background: c.dangerBg,
    color: c.danger,
    fontWeight: 500,
    fontSize: small ? 12 : 13,
    cursor: 'pointer',
    whiteSpace: 'nowrap',
  }
}

export function ghostBtnStyle(small = false): CSSProperties {
  return {
    padding: small ? '0.25rem 0.5rem' : '0.5rem 0.875rem',
    borderRadius: 6,
    border: `1px solid ${c.border}`,
    background: 'transparent',
    color: c.muted,
    fontWeight: 500,
    fontSize: small ? 12 : 13,
    cursor: 'pointer',
    whiteSpace: 'nowrap',
  }
}

// ── Components ────────────────────────────────────────────────────────────────
export function FormField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.375rem' }}>
      <label style={{ fontSize: 13, fontWeight: 500, color: '#374151' }}>{label}</label>
      {children}
    </div>
  )
}

export function ErrorBanner({ message }: { message: string }) {
  return (
    <div style={{
      background: c.dangerBg,
      border: `1px solid ${c.dangerBorder}`,
      color: c.danger,
      padding: '0.75rem 1rem',
      borderRadius: 8,
      fontSize: 13,
      marginBottom: '1rem',
    }}>
      {message}
    </div>
  )
}

export function SuccessBanner({ message }: { message: string }) {
  return (
    <div style={{
      background: c.successBg,
      border: `1px solid ${c.successBorder}`,
      color: c.successText,
      padding: '0.75rem 1rem',
      borderRadius: 8,
      fontSize: 13,
      marginBottom: '1rem',
    }}>
      {message}
    </div>
  )
}

export function RoleBadge({ role }: { role: string }) {
  const admin = role === 'admin'
  return (
    <span style={{
      display: 'inline-block',
      padding: '0.2rem 0.55rem',
      borderRadius: 20,
      fontSize: 11,
      fontWeight: 600,
      textTransform: 'uppercase',
      letterSpacing: '0.05em',
      background: admin ? '#fef3c7' : '#dbeafe',
      color: admin ? '#92400e' : '#1d4ed8',
    }}>
      {role}
    </span>
  )
}

export function StatusBadge({ active }: { active: boolean }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem', fontSize: 13 }}>
      <span style={{
        width: 7,
        height: 7,
        borderRadius: '50%',
        background: active ? '#22c55e' : '#cbd5e1',
        display: 'inline-block',
        flexShrink: 0,
      }} />
      <span style={{ color: active ? '#15803d' : c.muted }}>{active ? 'Active' : 'Inactive'}</span>
    </span>
  )
}

export function AuthCardHeader({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div style={{ textAlign: 'center', marginBottom: '2rem' }}>
      <div style={{ fontSize: 26, fontWeight: 700, color: c.primary, marginBottom: '0.5rem', letterSpacing: '-0.02em' }}>
        Kleido
      </div>
      <h1 style={{ fontSize: 19, fontWeight: 600, margin: 0, color: c.text }}>{title}</h1>
      <p style={{ color: c.muted, fontSize: 14, margin: '0.375rem 0 0' }}>{subtitle}</p>
    </div>
  )
}

export function PageHeader({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div style={{ marginBottom: '1.5rem' }}>
      <h1 style={{ fontSize: 22, fontWeight: 700, margin: 0, color: c.text }}>{title}</h1>
      {subtitle && <p style={{ margin: '0.375rem 0 0', color: c.muted, fontSize: 14 }}>{subtitle}</p>}
    </div>
  )
}

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '4rem' }}>
      <span style={{ color: c.muted, fontSize: 14 }}>{label}</span>
    </div>
  )
}

export function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div style={{
      background: c.surface,
      border: `1px solid ${c.border}`,
      borderRadius: 10,
      padding: '1.25rem 1.5rem',
    }}>
      <div style={{ fontSize: 12, fontWeight: 500, color: c.muted, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: '0.5rem' }}>
        {label}
      </div>
      <div style={{ fontSize: 28, fontWeight: 700, color: c.text, lineHeight: 1 }}>{value}</div>
      {sub && <div style={{ fontSize: 12, color: c.muted, marginTop: '0.35rem' }}>{sub}</div>}
    </div>
  )
}
