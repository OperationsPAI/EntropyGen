import { useState, useEffect } from 'react'
import PageHeader from '../../components/ui/PageHeader'
import { agentsApi } from '../../api/agents'
import type { Agent } from '../../types/agent'
import AgentCard from './AgentCard'
import styles from './Observe.module.css'

export default function ObserveOverview() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const data = await agentsApi.getAgents()
        if (!cancelled) setAgents(data)
      } catch {
        // ignore
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()

    // Auto-refresh every 10 seconds
    const timer = setInterval(load, 10_000)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [])

  const onlineCount = agents.filter(
    (a) => a.status.phase === 'Running' || a.status.phase === 'Initializing'
  ).length

  return (
    <div>
      <PageHeader
        title="Agent Observe"
        description={
          <div className={styles.statusBar}>
            <span className={styles.onlineCount}>
              <span className={styles.onlineDot} />
              {onlineCount}/{agents.length} Online
            </span>
            <span>Auto-refresh: ON</span>
          </div>
        }
      />

      {loading ? (
        <div style={{ padding: 'var(--space-xl)', color: 'var(--text-muted)', textAlign: 'center' }}>
          Loading agents...
        </div>
      ) : agents.length === 0 ? (
        <div style={{ padding: 'var(--space-xl)', color: 'var(--text-muted)', textAlign: 'center' }}>
          No agents found
        </div>
      ) : (
        <div className={styles.grid}>
          {agents.map((agent) => (
            <AgentCard key={agent.name} agent={agent} />
          ))}
        </div>
      )}
    </div>
  )
}
