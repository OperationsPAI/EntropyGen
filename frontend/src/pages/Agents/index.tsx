import { useEffect, useState, useMemo } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { IconPause, IconPlay, IconEyeOpened, IconDelete } from '@douyinfe/semi-icons'
import { agentsApi } from '../../api/agents'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import { PageHeader, Card, Table, Button, Input, Modal, EmptyState } from '../../components/ui'
import { useToast } from '../../hooks/useToast'
import type { Agent, AgentPhase } from '../../types/agent'
import styles from './Agents.module.css'

const PHASES: AgentPhase[] = ['Pending', 'Initializing', 'Running', 'Paused', 'Error']

function getAge(createdAt: string) {
  const days = Math.floor((Date.now() - new Date(createdAt).getTime()) / 86400000)
  return days === 0 ? 'today' : `${days}d ago`
}

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

  useEffect(() => {
    agentsApi.getAgents()
      .then(setAgents)
      .catch(() => setError('Failed to load agents'))
      .finally(() => setLoading(false))
  }, [])

  const uniqueRoles = useMemo(
    () => [...new Set(agents.map((a) => a.spec.role))].sort(),
    [agents],
  )

  const filtered = agents.filter(
    (a) => (!roleFilter || a.spec.role === roleFilter) && (!phaseFilter || a.status.phase === phaseFilter),
  )

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

  const renderTable = () => {
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

    return (
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
              <td className={styles.nameCell}>{agent.name}</td>
              <td>
                <Link to={`/roles/${agent.spec.role}`} className={styles.roleLink}>
                  {agent.spec.role}
                </Link>
              </td>
              <td><AgentPhaseTag phase={agent.status.phase} /></td>
              <td className={styles.mutedCell}>{agent.spec.llm.model}</td>
              <td className={styles.lastActionCell}>{agent.status.lastAction?.description ?? '\u2014'}</td>
              <td className={styles.tokenCell}>{(agent.status.tokenUsage?.today ?? 0).toLocaleString()}</td>
              <td className={styles.mutedCell}>{getAge(agent.status.createdAt)}</td>
              <td>
                <div className={styles.actionsCell}>
                  <Button variant="ghost" size="sm" onClick={() => handleTogglePause(agent)}>
                    {agent.status.phase === 'Paused' ? <IconPlay size="small" /> : <IconPause size="small" />}
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => navigate(`/agents/${agent.name}`)}>
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
  }

  return (
    <div className={styles.page}>
      <PageHeader
        title="Agents"
        actions={<Button onClick={() => navigate('/agents/new')}>+ New Agent</Button>}
      />

      <Card>
        {renderFilters()}
        {renderTable()}
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
