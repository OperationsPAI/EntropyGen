import { useAlertStore } from '../../stores/alertStore'
import { useNavigate } from 'react-router-dom'

export default function AlertBanner() {
  const { banner, dismissBanner } = useAlertStore()

  const navigate = useNavigate()

  if (!banner) return null

  return (
    <div style={{
      backgroundColor: 'var(--bg-surface)',
      borderRadius: '12px',
      padding: '14px 20px',
      display: 'flex',
      alignItems: 'center',
      gap: '14px',
      border: '1px solid var(--line-strong)',
      cursor: 'pointer',
    }}
      onClick={() => navigate(`/agents/${banner.agent_id}`)}
    >
      <div style={{
        width: '28px', height: '28px',
        backgroundColor: 'var(--accent-orange)',
        borderRadius: '50%',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        color: 'white', fontSize: '0.8rem', fontWeight: 700, flexShrink: 0,
      }}>!</div>
      <div style={{ flex: 1 }}>
        <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>
          {banner.agent_id} — {banner.alert_type}
        </div>
        <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>{banner.message}</div>
      </div>
      <button
        onClick={(e) => { e.stopPropagation(); dismissBanner() }}
        style={{
          background: 'none', border: 'none', cursor: 'pointer',
          color: 'var(--text-muted)', fontSize: '1.2rem', lineHeight: 1,
        }}
      >×</button>
    </div>
  )
}
