import { NavLink, useNavigate } from 'react-router-dom'
import { authApi } from '../../api/auth'

const NAV_ITEMS = [
  { path: '/dashboard', label: 'Dashboard', icon: '◉' },
  { path: '/agents', label: 'Agents', icon: '◈' },
  { path: '/llm', label: 'LLM Models', icon: '◇' },
  { path: '/audit', label: 'Audit Log', icon: '◫' },
  { path: '/monitor', label: 'Monitoring', icon: '◎' },
  { path: '/export', label: 'Export', icon: '◱' },
]

export default function Sidebar() {
  const navigate = useNavigate()

  const handleLogout = async () => {
    try { await authApi.logout() } catch { /* ignore */ }
    localStorage.removeItem('jwt_token')
    navigate('/login')
  }

  return (
    <aside style={{
      width: '220px',
      flexShrink: 0,
      backgroundColor: 'var(--bg-surface)',
      borderRadius: '24px',
      padding: '24px',
      display: 'flex',
      flexDirection: 'column',
      gap: '32px',
    }}>
      {/* Logo */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontWeight: 700, fontSize: '1rem', letterSpacing: '-0.02em' }}>
        <div style={{
          width: '22px', height: '22px',
          backgroundColor: 'var(--text-main)',
          borderRadius: '50%',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
          <div style={{ width: '10px', height: '10px', backgroundColor: 'white', borderRadius: '50%' }} />
        </div>
        EntropyGen
      </div>

      {/* Nav */}
      <nav style={{ flex: 1 }}>
        <ul style={{ listStyle: 'none', display: 'flex', flexDirection: 'column', gap: '2px' }}>
          {NAV_ITEMS.map(({ path, label, icon }) => (
            <li key={path}>
              <NavLink
                to={path}
                style={({ isActive }) => ({
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  padding: '10px 12px',
                  borderRadius: '8px',
                  textDecoration: 'none',
                  fontSize: '0.875rem',
                  fontWeight: isActive ? 600 : 500,
                  color: isActive ? 'var(--text-main)' : 'var(--text-muted)',
                  backgroundColor: isActive ? 'rgba(0,0,0,0.04)' : 'transparent',
                  position: 'relative',
                  transition: 'all 0.15s',
                })}
              >
                {({ isActive }) => (
                  <>
                    {isActive && (
                      <span style={{
                        position: 'absolute', left: '-12px',
                        width: '4px', height: '4px',
                        borderRadius: '50%', backgroundColor: 'var(--text-main)',
                      }} />
                    )}
                    <span style={{ fontSize: '0.8rem', opacity: 0.6 }}>{icon}</span>
                    {label}
                  </>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      {/* Logout */}
      <button
        onClick={handleLogout}
        style={{
          background: 'none', border: 'none', cursor: 'pointer',
          textAlign: 'left', padding: '8px 12px',
          borderRadius: '8px', fontSize: '0.875rem',
          color: 'var(--text-muted)', fontWeight: 500,
        }}
      >
        Sign Out
      </button>
    </aside>
  )
}
