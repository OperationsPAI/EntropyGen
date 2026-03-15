import { Outlet, Navigate } from 'react-router-dom'
import Sidebar from './Sidebar'
import AlertBanner from '../alert/AlertBanner'
import ToastContainer from '../ui/Toast'
import { useWebSocket } from '../../hooks/useWebSocket'
import styles from './AppShell.module.css'

function AuthGuard({ children }: { children: React.ReactNode }) {
  const token = localStorage.getItem('jwt_token')
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function AppShell() {
  useWebSocket()

  return (
    <AuthGuard>
      <div className={styles.shell}>
        <Sidebar />
        <div className={styles.content}>
          <AlertBanner />
          <main className={styles.main}>
            <Outlet />
          </main>
        </div>
        <ToastContainer />
      </div>
    </AuthGuard>
  )
}
