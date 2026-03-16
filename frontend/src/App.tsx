import { createBrowserRouter, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import { AuthProvider } from './contexts/AuthContext'
import AppShell from './components/layout/AppShell'
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
      { path: 'dashboard', element: <Suspense fallback={<Loading />}><Dashboard /></Suspense> },
      { path: 'agents', element: <Suspense fallback={<Loading />}><AgentList /></Suspense> },
      { path: 'agents/new', element: <Suspense fallback={<Loading />}><NewAgent /></Suspense> },
      { path: 'agents/:name', element: <Suspense fallback={<Loading />}><AgentDetail /></Suspense> },
      { path: 'observe/:name', element: <Suspense fallback={<Loading />}><ObserveDetail /></Suspense> },
      { path: 'roles', element: <Suspense fallback={<Loading />}><RoleList /></Suspense> },
      { path: 'roles/new', element: <Suspense fallback={<Loading />}><NewRole /></Suspense> },
      { path: 'roles/:name', element: <Suspense fallback={<Loading />}><RoleEditor /></Suspense> },
      { path: 'llm', element: <Suspense fallback={<Loading />}><LLM /></Suspense> },
      { path: 'audit', element: <Suspense fallback={<Loading />}><Audit /></Suspense> },
      { path: 'monitor', element: <Suspense fallback={<Loading />}><Monitor /></Suspense> },
      { path: 'export', element: <Suspense fallback={<Loading />}><Export /></Suspense> },
      { path: 'users', element: <Suspense fallback={<Loading />}><Users /></Suspense> },
    ],
  },
])
