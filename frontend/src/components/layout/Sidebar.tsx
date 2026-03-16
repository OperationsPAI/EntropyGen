import { useState, useEffect } from 'react'
import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { IconHome, IconUserGroup, IconSetting, IconServer, IconList, IconPulse, IconExport, IconChevronLeft, IconChevronRight, IconUser, IconShield } from '@douyinfe/semi-icons'
import { authApi } from '../../api/auth'
import { useAuth } from '../../contexts/AuthContext'
import styles from './Sidebar.module.css'

const NAV_ITEMS = [
  { path: '/dashboard', label: 'Dashboard', icon: <IconHome size="small" /> },
  { path: '/agents', label: 'Agents', icon: <IconUserGroup size="small" /> },
  { path: '/roles', label: 'Roles', icon: <IconSetting size="small" /> },
  { path: '/llm', label: 'LLM Models', icon: <IconServer size="small" /> },
  { path: '/audit', label: 'Audit Log', icon: <IconList size="small" /> },
  { path: '/monitor', label: 'Data Dashboard', icon: <IconPulse size="small" /> },
  { path: '/export', label: 'Export', icon: <IconExport size="small" /> },
]

const ADMIN_NAV_ITEMS = [
  { path: '/users', label: 'User Management', icon: <IconShield size="small" /> },
]

/** Routes that auto-collapse the sidebar for maximum content space */
const AUTO_COLLAPSE_ROUTES = ['/observe/']

export default function Sidebar() {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, isGuest, isAdmin, logout } = useAuth()
  const [collapsed, setCollapsed] = useState(false)

  // Auto-collapse when entering observe detail pages, auto-expand when leaving
  useEffect(() => {
    const shouldCollapse = AUTO_COLLAPSE_ROUTES.some((r) => location.pathname.startsWith(r))
    setCollapsed(shouldCollapse)
  }, [location.pathname])

  const handleLogout = async () => {
    try { await authApi.logout() } catch { /* ignore */ }
    logout()
    navigate('/login')
  }

  const handleLogin = () => navigate('/login')

  const avatarText = user ? user.username.slice(0, 2).toUpperCase() : 'GU'
  const displayName = user ? user.username : 'Guest'
  const roleLabel = user?.role === 'admin' ? 'Admin' : user?.role === 'member' ? 'Member' : ''

  return (
    <aside className={`${styles.sidebar} ${collapsed ? styles.collapsed : ''}`}>
      {/* Logo row */}
      <div className={styles.logoRow}>
        <div className={styles.logo}>
          <div className={styles.logoDot} />
        </div>
        {!collapsed && <span className={styles.logoText}>EntropyGen</span>}
        <button
          className={styles.collapseBtn}
          onClick={() => setCollapsed((p) => !p)}
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? <IconChevronRight size="extra-small" /> : <IconChevronLeft size="extra-small" />}
        </button>
      </div>

      {/* Nav */}
      <nav className={styles.nav}>
        <ul className={styles.navList}>
          {NAV_ITEMS.map(({ path, label, icon }) => (
            <li key={path}>
              <NavLink
                to={path}
                className={({ isActive }) =>
                  `${styles.navItem} ${isActive ? styles.navItemActive : ''} ${collapsed ? styles.navItemCollapsed : ''}`
                }
                title={collapsed ? label : undefined}
              >
                {({ isActive }) => (
                  <>
                    {isActive && <span className={styles.activeBar} />}
                    <span className={styles.navIcon}>{icon}</span>
                    {!collapsed && label}
                  </>
                )}
              </NavLink>
            </li>
          ))}
          {isAdmin && ADMIN_NAV_ITEMS.map(({ path, label, icon }) => (
            <li key={path}>
              <NavLink
                to={path}
                className={({ isActive }) =>
                  `${styles.navItem} ${isActive ? styles.navItemActive : ''} ${collapsed ? styles.navItemCollapsed : ''}`
                }
                title={collapsed ? label : undefined}
              >
                {({ isActive }) => (
                  <>
                    {isActive && <span className={styles.activeBar} />}
                    <span className={styles.navIcon}>{icon}</span>
                    {!collapsed && label}
                  </>
                )}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      {/* User section */}
      <div className={styles.userSection}>
        {isGuest ? (
          <button
            onClick={handleLogin}
            className={`${styles.signOutBtn} ${collapsed ? styles.signOutBtnCollapsed : ''}`}
            title={collapsed ? 'Sign In' : undefined}
            style={{ display: 'flex', alignItems: 'center', gap: '8px' }}
          >
            <IconUser size="small" />
            {!collapsed && 'Sign In'}
          </button>
        ) : (
          <>
            <div className={`${styles.userRow} ${collapsed ? styles.userRowCollapsed : ''}`}>
              <div className={styles.avatar}>{avatarText}</div>
              {!collapsed && (
                <div style={{ overflow: 'hidden' }}>
                  <span className={styles.userName}>{displayName}</span>
                  {roleLabel && (
                    <div style={{ fontSize: '0.7rem', color: 'var(--text-muted)', opacity: 0.7, marginTop: '1px' }}>
                      {roleLabel}
                    </div>
                  )}
                </div>
              )}
            </div>
            <button
              onClick={handleLogout}
              className={`${styles.signOutBtn} ${collapsed ? styles.signOutBtnCollapsed : ''}`}
              title={collapsed ? 'Sign Out' : undefined}
            >
              {collapsed ? <span className={styles.signOutShort}>Out</span> : 'Sign Out'}
            </button>
          </>
        )}
      </div>
    </aside>
  )
}
