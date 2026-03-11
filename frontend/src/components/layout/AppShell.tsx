import { Outlet, Navigate } from 'react-router-dom'
import Sidebar from './Sidebar'
import AlertBanner from '../alert/AlertBanner'
import { useWebSocket } from '../../hooks/useWebSocket'

function AuthGuard({ children }: { children: React.ReactNode }) {
  const token = localStorage.getItem('jwt_token')
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function AppShell() {
  useWebSocket()

  return (
    <AuthGuard>
      <div style={{
        display: 'flex',
        minHeight: '100vh',
        padding: '16px',
        gap: '16px',
        backgroundColor: 'var(--bg-canvas)',
      }}>
        <Sidebar />
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '12px', minWidth: 0 }}>
          <AlertBanner />
          <main style={{ flex: 1 }}>
            <Outlet />
          </main>
        </div>
      </div>
    </AuthGuard>
  )
}
