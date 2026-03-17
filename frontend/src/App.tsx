import { createBrowserRouter, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import { AuthProvider } from './contexts/AuthContext'
import AppShell from './components/layout/AppShell'
import RequireAuth from './components/auth/RequireAuth'
import Login from './pages/Login'

const Dashboard = lazy(() => import('./pages/Dashboard'))
const AgentList = lazy(() => import('./pages/Agents'))
const AgentDetail = lazy(() => import('./pages/Agents/Detail'))
const NewAgent = lazy(() => import('./pages/Agents/NewAgent'))
const RoleList = lazy(() => import('./pages/Roles'))
const NewRole = lazy(() => import('./pages/Roles/NewRole'))
const RoleEditor = lazy(() => import('./pages/Roles/Editor'))
const LLM = lazy(() => import('./pages/LLM'))
const Audit = lazy(() => import('./pages/Audit'))
const Monitor = lazy(() => import('./pages/Monitor'))
const ObserveDetail = lazy(() => import('./pages/Observe/ObserveDetail'))
const Export = lazy(() => import('./pages/Export'))
const Users = lazy(() => import('./pages/Users'))

const Loading = () => (
  <div style={{
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    height: '100%',
    color: 'var(--text-muted)',
    gap: '12px',
    padding: '48px',
  }}>
    <span style={{
      width: '20px',
      height: '20px',
      border: '2px solid var(--line-subtle)',
      borderTopColor: 'var(--text-muted)',
      borderRadius: '50%',
      animation: 'spin 0.8s linear infinite',
    }} />
    <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
  </div>
)

const withAuth = (element: React.ReactNode) => (
  <AuthProvider>{element}</AuthProvider>
)

const authed = (element: React.ReactNode) => (
  <RequireAuth><Suspense fallback={<Loading />}>{element}</Suspense></RequireAuth>
)

export const router = createBrowserRouter([
  {
    path: '/login',
    element: withAuth(<Login />),
  },
  {
    path: '/',
    element: withAuth(<AppShell />),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      // Guest-accessible (read-only)
      { path: 'dashboard', element: <Suspense fallback={<Loading />}><Dashboard /></Suspense> },
      { path: 'agents', element: <Suspense fallback={<Loading />}><AgentList /></Suspense> },
      { path: 'agents/:name', element: <Suspense fallback={<Loading />}><AgentDetail /></Suspense> },
      // Requires login (member+)
      { path: 'agents/new', element: authed(<NewAgent />) },
      { path: 'observe/:name', element: authed(<ObserveDetail />) },
      { path: 'roles', element: authed(<RoleList />) },
      { path: 'roles/new', element: authed(<NewRole />) },
      { path: 'roles/:name', element: authed(<RoleEditor />) },
      { path: 'llm', element: authed(<LLM />) },
      { path: 'audit', element: authed(<Audit />) },
      { path: 'monitor', element: authed(<Monitor />) },
      { path: 'export', element: authed(<Export />) },
      { path: 'users', element: authed(<Users />) },
    ],
  },
])
