import { createBrowserRouter, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import AppShell from './components/layout/AppShell'
import Login from './pages/Login'

const Dashboard = lazy(() => import('./pages/Dashboard'))
const AgentList = lazy(() => import('./pages/Agents'))
const AgentDetail = lazy(() => import('./pages/Agents/Detail'))
const LLM = lazy(() => import('./pages/LLM'))
const Audit = lazy(() => import('./pages/Audit'))
const Monitor = lazy(() => import('./pages/Monitor'))
const Export = lazy(() => import('./pages/Export'))

const Loading = () => (
  <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%', color: 'var(--text-muted)' }}>
    Loading...
  </div>
)

export const router = createBrowserRouter([
  {
    path: '/login',
    element: <Login />,
  },
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: 'dashboard', element: <Suspense fallback={<Loading />}><Dashboard /></Suspense> },
      { path: 'agents', element: <Suspense fallback={<Loading />}><AgentList /></Suspense> },
      { path: 'agents/:name', element: <Suspense fallback={<Loading />}><AgentDetail /></Suspense> },
      { path: 'llm', element: <Suspense fallback={<Loading />}><LLM /></Suspense> },
      { path: 'audit', element: <Suspense fallback={<Loading />}><Audit /></Suspense> },
      { path: 'monitor', element: <Suspense fallback={<Loading />}><Monitor /></Suspense> },
      { path: 'export', element: <Suspense fallback={<Loading />}><Export /></Suspense> },
    ],
  },
])
