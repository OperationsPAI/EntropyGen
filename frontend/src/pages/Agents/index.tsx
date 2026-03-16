import { useEffect, useState, useMemo, useCallback, useRef } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { IconPause, IconPlay, IconEyeOpened, IconDelete, IconList, IconGridSquare } from '@douyinfe/semi-icons'
import { agentsApi } from '../../api/agents'
import { llmApi, type LLMModel } from '../../api/llm'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import AgentCard from '../Observe/AgentCard'
import { PageHeader, Card, Table, Button, Input, Modal, EmptyState } from '../../components/ui'
import { useToast } from '../../hooks/useToast'
import type { Agent, AgentPhase } from '../../types/agent'
import styles from './Agents.module.css'

const PHASES: AgentPhase[] = ['Pending', 'Initializing', 'Running', 'Paused', 'Error']
const VIEW_KEY = 'agents_view_mode'

function getAge(createdAt: string) {
  const days = Math.floor((Date.now() - new Date(createdAt).getTime()) / 86400000)
  return days === 0 ? 'today' : `${days}d ago`
}

type ViewMode = 'table' | 'card'

export default function AgentList() {
  const navigate = useNavigate()
  const toast = useToast()
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [roleFilter, setRoleFilter] = useState('')
  const [phaseFilter, setPhaseFilter] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<Agent | null>(null)
  const [deleteInput, setDeleteInput] = useState('')
  const [viewMode, setViewMode] = useState<ViewMode>(
    () => (localStorage.getItem(VIEW_KEY) as ViewMode) || 'table',
  )
  const [modelMap, setModelMap] = useState<Map<string, string>>(new Map())
  const cancelledRef = useRef(false)

  const switchView = (mode: ViewMode) => {
    setViewMode(mode)
    localStorage.setItem(VIEW_KEY, mode)
  }

  const loadData = useCallback(async () => {
    try {
      const [agentData, models] = await Promise.all([
        agentsApi.getAgents(),
        llmApi.getModels().catch(() => [] as LLMModel[]),
      ])
      if (!cancelledRef.current) {
        setAgents(agentData)
        const map = new Map<string, string>()
        for (const m of models) map.set(m.id, m.name)
        setModelMap(map)
      }
    } catch {
      if (!cancelledRef.current) setError('Failed to load agents')
    } finally {
      if (!cancelledRef.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    cancelledRef.current = false
    loadData()
    const timer = setInterval(loadData, 10_000)
    return () => {
      cancelledRef.current = true
      clearInterval(timer)
    }
  }, [loadData])

  const uniqueRoles = useMemo(
    () => [...new Set(agents.map((a) => a.spec.role))].sort(),
    [agents],
  )

  const filtered = agents.filter(
    (a) => (!roleFilter || a.spec.role === roleFilter) && (!phaseFilter || a.status.phase === phaseFilter),
  )

  const onlineCount = agents.filter(
    (a) => a.status.phase === 'Running' || a.status.phase === 'Initializing',
  ).length

  const handleTogglePause = async (agent: Agent) => {
    const isPaused = agent.status.phase === 'Paused'
    const fn = isPaused ? agentsApi.resumeAgent : agentsApi.pauseAgent
    try {
      await fn(agent.name)
      setAgents((prev) =>
        prev.map((a) =>
          a.name === agent.name
            ? { ...a, status: { ...a.status, phase: (isPaused ? 'Running' : 'Paused') as AgentPhase } }
            : a,
        ),
      )
      toast.success(isPaused ? 'Agent resumed' : 'Agent paused', agent.name)
    } catch {
      toast.error('Operation failed', `Could not ${isPaused ? 'resume' : 'pause'} ${agent.name}`)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget || deleteInput !== deleteTarget.name) return
    try {
      await agentsApi.deleteAgent(deleteTarget.name)
      setAgents((prev) => prev.filter((a) => a.name !== deleteTarget.name))
      toast.success('Agent deleted', deleteTarget.name)
    } catch {
      toast.error('Delete failed', `Could not delete ${deleteTarget.name}`)
    }
    setDeleteTarget(null)
    setDeleteInput('')
  }

  const toggleRole = (role: string) => setRoleFilter((prev) => (prev === role ? '' : role))
  const togglePhase = (phase: string) => setPhaseFilter((prev) => (prev === phase ? '' : phase))

  const renderFilters = () => (
    <div className={styles.toolbar}>
      {uniqueRoles.map((r) => (
        <button
          key={r}
          onClick={() => toggleRole(r)}
          className={`${styles.filterPill} ${roleFilter === r ? styles.filterPillActive : ''}`}
        >
          {r}
        </button>
      ))}
      {uniqueRoles.length > 0 && <div className={styles.filterDivider} />}
      {PHASES.map((p) => (
        <button
          key={p}
          onClick={() => togglePhase(p)}
          className={`${styles.filterPill} ${phaseFilter === p ? styles.filterPillActive : ''}`}
        >
          {p}
        </button>
      ))}
    </div>
  )

  const renderTable = () => (
    <Table>
      <thead>
        <tr>
          {['Name', 'Role', 'Status', 'Model', 'Last Action', 'Token/Today', 'Age', 'Actions'].map((h) => (
            <th key={h}>{h}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {filtered.map((agent) => (
          <tr key={agent.name} className={agent.status.phase === 'Error' ? styles.errorRow : undefined}>
            <td className={styles.nameCell}>
              <Link to={`/agents/${agent.name}`} style={{ color: 'inherit', textDecoration: 'none' }}>
                {agent.name}
              </Link>
            </td>
            <td>
              <Link to={`/roles/${agent.spec.role}`} className={styles.roleLink}>
                {agent.spec.role}
              </Link>
            </td>
            <td><AgentPhaseTag phase={agent.status.phase} /></td>
            <td className={styles.mutedCell}>{modelMap.get(agent.spec.llm.model) ?? agent.spec.llm.model}</td>
            <td className={styles.lastActionCell}>{agent.status.lastAction?.description ?? '\u2014'}</td>
            <td className={styles.tokenCell}>{(agent.status.tokenUsage?.today ?? 0).toLocaleString()}</td>
            <td className={styles.mutedCell}>{getAge(agent.status.createdAt)}</td>
            <td>
              <div className={styles.actionsCell}>
                <Button variant="ghost" size="sm" onClick={() => handleTogglePause(agent)}>
                  {agent.status.phase === 'Paused' ? <IconPlay size="small" /> : <IconPause size="small" />}
                </Button>
                <Button variant="ghost" size="sm" onClick={() => navigate(`/observe/${agent.name}`)}>
                  <IconEyeOpened size="small" />
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(agent)}>
                  <IconDelete size="small" style={{ color: 'var(--accent-orange)' }} />
                </Button>
              </div>
            </td>
          </tr>
        ))}
      </tbody>
    </Table>
  )

  const renderCards = () => (
    <div className={styles.cardGrid}>
      {filtered.map((agent) => (
        <AgentCard key={agent.name} agent={agent} />
      ))}
    </div>
  )

  const renderContent = () => {
    if (loading) {
      return (
        <div>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className={`${styles.skeleton} ${styles.skeletonRow}`} />
          ))}
        </div>
      )
    }

    if (error) {
      return <EmptyState title="Error loading agents" description={error} />
    }

    if (filtered.length === 0) {
      return (
        <EmptyState
          title="No agents found"
          description={agents.length === 0 ? 'Create your first agent to get started.' : 'Try adjusting your filters.'}
          action={agents.length === 0 ? <Button onClick={() => navigate('/agents/new')}>New Agent</Button> : undefined}
        />
      )
    }

    return viewMode === 'table' ? renderTable() : renderCards()
  }

  return (
    <div className={styles.page}>
      <PageHeader
        title="Agents"
        description={
          <div className={styles.statusBar}>
            <span className={styles.onlineCount}>
              <span className={styles.onlineDot} />
              {onlineCount}/{agents.length} Online
            </span>
            <span>Auto-refresh: ON</span>
          </div>
        }
        actions={
          <div className={styles.headerActions}>
            <div className={styles.viewToggle}>
              <button
                className={`${styles.viewBtn} ${viewMode === 'table' ? styles.viewBtnActive : ''}`}
                onClick={() => switchView('table')}
                title="Table view"
              >
                <IconList size="small" />
              </button>
              <button
                className={`${styles.viewBtn} ${viewMode === 'card' ? styles.viewBtnActive : ''}`}
                onClick={() => switchView('card')}
                title="Card view"
              >
                <IconGridSquare size="small" />
              </button>
            </div>
            <Button onClick={() => navigate('/agents/new')}>+ New Agent</Button>
          </div>
        }
      />

      <Card>
        {renderFilters()}
        {renderContent()}
      </Card>

      <Modal
        open={!!deleteTarget}
        onClose={() => { setDeleteTarget(null); setDeleteInput('') }}
        title="Delete Agent"
        footer={
          <>
            <Button variant="secondary" onClick={() => { setDeleteTarget(null); setDeleteInput('') }}>Cancel</Button>
            <Button variant="danger" onClick={handleDelete} disabled={deleteInput !== deleteTarget?.name}>
              Confirm Delete
            </Button>
          </>
        }
      >
        <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginBottom: 'var(--space-md)' }}>
          This action is irreversible. Type <strong>{deleteTarget?.name}</strong> to confirm.
        </p>
        <Input
          value={deleteInput}
          onChange={(e) => setDeleteInput(e.target.value)}
          placeholder={deleteTarget?.name}
        />
      </Modal>
    </div>
  )
}
