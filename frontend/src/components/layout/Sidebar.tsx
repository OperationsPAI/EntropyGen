import { NavLink, useNavigate } from 'react-router-dom'
import { IconHome, IconUserGroup, IconSetting, IconServer, IconList, IconPulse, IconExport } from '@douyinfe/semi-icons'
import { authApi } from '../../api/auth'
import styles from './Sidebar.module.css'

const NAV_ITEMS = [
  { path: '/dashboard', label: 'Dashboard', icon: <IconHome size="small" /> },
  { path: '/agents', label: 'Agents', icon: <IconUserGroup size="small" /> },
  { path: '/roles', label: 'Roles', icon: <IconSetting size="small" /> },
  { path: '/llm', label: 'LLM Models', icon: <IconServer size="small" /> },
  { path: '/audit', label: 'Audit Log', icon: <IconList size="small" /> },
  { path: '/monitor', label: 'Monitoring', icon: <IconPulse size="small" /> },
  { path: '/export', label: 'Export', icon: <IconExport size="small" /> },
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
        <div className={styles.logo}>
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
                className={({ isActive }) =>
                  `${styles.navItem} ${isActive ? styles.navItemActive : ''}`
                }
              >
                {({ isActive }) => (
                  <>
                    {isActive && <span className={styles.activeBar} />}
                    <span style={{ display: 'flex', opacity: 0.6 }}>{icon}</span>
                    {label}
                  </>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      {/* User info + Sign Out */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          padding: '8px 12px',
        }}>
          <div className={styles.avatar}>AG</div>
          <span style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontWeight: 500 }}>
            Agent Admin
          </span>
        </div>
        <button onClick={handleLogout} className={styles.signOutBtn}>
          Sign Out
        </button>
      </div>
    </aside>
  )
}
