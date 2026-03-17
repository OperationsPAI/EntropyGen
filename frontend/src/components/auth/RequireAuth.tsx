import { Navigate } from 'react-router-dom'
import { useAuth } from '../../contexts/AuthContext'

export default function RequireAuth({ children }: { children: React.ReactNode }) {
  const { isGuest } = useAuth()

  if (isGuest) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}
