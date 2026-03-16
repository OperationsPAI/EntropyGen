import { Outlet } from 'react-router-dom'
import Sidebar from './Sidebar'
import AlertBanner from '../alert/AlertBanner'
import ToastContainer from '../ui/Toast'
import { useWebSocket } from '../../hooks/useWebSocket'
import styles from './AppShell.module.css'

export default function AppShell() {
  useWebSocket()

  return (
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
  )
}
