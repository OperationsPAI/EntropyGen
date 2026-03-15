import { useEffect, useCallback, useState } from 'react'
import ReactECharts from '../../components/ReactECharts'
import { agentsApi } from '../../api/agents'
import { monitorApi } from '../../api/monitor'
import type { AgentActivityPoint } from '../../api/monitor'
import { useAgentStore } from '../../stores/agentStore'
import { useEventStore } from '../../stores/eventStore'
import { useAlertStore } from '../../stores/alertStore'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import { Card, PageHeader, EmptyState, Table, Button } from '../../components/ui'
import type { Agent } from '../../types/agent'
import type { RealtimeEvent } from '../../types/event'
import styles from './Dashboard.module.css'

function getEventBadgeClass(eventType: string): string {
  if (eventType.startsWith('gitea.')) return `${styles.eventBadge} ${styles.eventBadgeGitea}`
  if (eventType.startsWith('llm.')) return `${styles.eventBadge} ${styles.eventBadgeLlm}`
  if (eventType.startsWith('alert.')) return `${styles.eventBadge} ${styles.eventBadgeAlert}`
  return styles.eventBadge
}

function formatEventType(eventType: string): string {
  const parts = eventType.split('.')
  return parts.length > 1 ? parts.slice(1).join('.') : eventType
}

function SkeletonStatCard() {
  return (
    <Card>
      <div className={styles.statLabel}>&nbsp;</div>
      <div className={`${styles.skeleton} ${styles.skeletonStatValue}`} />
      <div className={`${styles.skeleton} ${styles.skeletonStatSub}`} />
    </Card>
  )
}

function SkeletonTable() {
  return (
    <Card title="Agent Status">
      {Array.from({ length: 4 }, (_, i) => (
        <div key={i} className={`${styles.skeleton} ${styles.skeletonRow}`} />
      ))}
    </Card>
  )
}

function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <Card>
      <div className={styles.statLabel}>{label}</div>
      <div className={styles.statValue}>{value}</div>
      {sub && <div className={styles.statSub}>{sub}</div>}
    </Card>
  )
}

function AgentStatusTable({ agents }: { agents: Agent[] }) {
  return (
    <Card title="Agent Status">
      {agents.length === 0 ? (
        <EmptyState
          title="No agents yet"
          description="Create your first agent to get started"
        />
      ) : (
        <Table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Role</th>
              <th>Status</th>
              <th>Last Action</th>
              <th>Token/Today</th>
            </tr>
          </thead>
          <tbody>
            {agents.map((agent) => (
              <tr key={agent.name} className={agent.status.phase === 'Error' ? styles.errorRow : undefined}>
                <td className={styles.agentName}>{agent.name}</td>
                <td className={styles.agentRole}>{agent.spec.role}</td>
                <td><AgentPhaseTag phase={agent.status.phase} /></td>
                <td className={styles.lastAction}>{agent.status.lastAction?.description ?? '\u2014'}</td>
                <td className={styles.tokenCell}>{(agent.status.tokenUsage?.today ?? 0).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </Card>
  )
}

function TokenTrendChart({ agents }: { agents: Agent[] }) {
  const [activityData, setActivityData] = useState<AgentActivityPoint[]>([])

  useEffect(() => {
    monitorApi.getActivityHeatmap(1).then(setActivityData).catch(() => {})
  }, [])

  if (agents.length === 0) {
    return (
      <Card title="Token Trend (Today)">
        <EmptyState title="No data" description="Token trends will appear once agents are running" />
      </Card>
    )
  }

  const hours = Array.from({ length: 24 }, (_, i) => `${i}:00`)

  // Build hourly data per agent from activity data
  const agentHours = new Map<string, number[]>()
  for (const agent of agents) {
    agentHours.set(agent.name, new Array(24).fill(0))
  }
  for (const pt of activityData) {
    // Audit traces use "agent-<name>" as agent_id; strip prefix for matching
    const name = pt.agent_id.startsWith('agent-') ? pt.agent_id.slice(6) : pt.agent_id
    const arr = agentHours.get(name)
    if (arr && pt.hour >= 0 && pt.hour < 24) {
      arr[pt.hour] = pt.count
    }
  }

  const chartOption = {
    tooltip: { trigger: 'axis' as const },
    legend: { data: agents.map((a) => a.name), bottom: 0 },
    grid: { top: 16, bottom: 40, left: 16, right: 16, containLabel: true },
    xAxis: {
      type: 'category' as const,
      data: hours,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { fontSize: 10, color: '#665f58' },
    },
    yAxis: {
      type: 'value' as const,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: 'rgba(17,17,17,0.08)' } },
      axisLabel: { fontSize: 10, color: '#665f58' },
    },
    series: agents.map((a) => ({
      name: a.name,
      type: 'line' as const,
      smooth: true,
      data: agentHours.get(a.name) ?? hours.map(() => 0),
      symbol: 'none',
      lineStyle: { width: 2 },
    })),
  }

  return (
    <Card title="Activity Trend (Today)">
      <ReactECharts option={chartOption} style={{ height: '180px' }} />
    </Card>
  )
}

function LiveEventStream({ events }: { events: RealtimeEvent[] }) {
  const titleEl = (
    <div className={styles.liveHeader}>
      <span>Live Event Stream</span>
      <span className={styles.liveDot} />
      <span className={styles.liveLabel}>Live</span>
    </div>
  )

  return (
    <Card title={titleEl}>
      {events.length === 0 ? (
        <div className={styles.waitingText}>
          Waiting for events<span className={styles.waitingDots} />
        </div>
      ) : (
        <div className={styles.eventList}>
          {events.map((ev, i) => (
            <div key={i} className={styles.eventRow}>
              <span className={styles.eventTime}>
                {new Date(ev.timestamp).toLocaleTimeString()}
              </span>
              <span className={styles.eventAgent}>{ev.agent_id}</span>
              <span className={getEventBadgeClass(ev.event_type)}>
                {formatEventType(ev.event_type)}
              </span>
            </div>
          ))}
        </div>
      )}
    </Card>
  )
}

export default function Dashboard() {
  const { agents, setAgents, setLoading, setError } = useAgentStore()
  const loading = useAgentStore((s) => s.loading)
  const error = useAgentStore((s) => s.error)
  const { events } = useEventStore()
  const { alerts } = useAlertStore()
  const [hasFetched, setHasFetched] = useState(false)

  const fetchAgents = useCallback(() => {
    setLoading(true)
    setError(null)
    agentsApi
      .getAgents()
      .then((data) => {
        setAgents(data)
        setHasFetched(true)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to load agents')
        setHasFetched(true)
      })
      .finally(() => setLoading(false))
  }, [setAgents, setLoading, setError])

  useEffect(() => {
    fetchAgents()
  }, [fetchAgents])

  if (loading && !hasFetched) {
    return (
      <div className={styles.page}>
        <PageHeader title="Dashboard" />
        <div className={styles.statsGrid}>
          {Array.from({ length: 4 }, (_, i) => <SkeletonStatCard key={i} />)}
        </div>
        <div className={styles.mainGrid}>
          <SkeletonTable />
          <div className={styles.rightCol}>
            <Card title="Token Trend (Today)">
              <div className={`${styles.skeleton} ${styles.skeletonChart}`} />
            </Card>
            <Card title="Live Event Stream">
              <div className={styles.waitingText}>
                Waiting for events<span className={styles.waitingDots} />
              </div>
            </Card>
          </div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className={styles.page}>
        <PageHeader title="Dashboard" />
        <EmptyState
          title="Failed to load dashboard"
          description={error}
          action={
            <Button variant="secondary" onClick={fetchAgents} loading={loading}>
              Retry
            </Button>
          }
        />
      </div>
    )
  }

  const running = agents.filter((a) => a.status.phase === 'Running').length
  const todayTokens = agents.reduce((s, a) => s + (a.status.tokenUsage?.today ?? 0), 0)

  return (
    <div className={styles.page}>
      <PageHeader title="Dashboard" />
      <div className={styles.statsGrid}>
        <StatCard label="Running Agents" value={`${running}/${agents.length}`} />
        <StatCard label="Today's Tokens" value={todayTokens.toLocaleString()} />
        <StatCard label="Live Events" value={events.length || '\u2014'} sub={events.length > 0 ? 'in current session' : undefined} />
        <StatCard
          label="Alerts"
          value={alerts.length}
          sub={alerts.length > 0 ? 'Attention needed' : 'Normal'}
        />
      </div>

      <div className={styles.mainGrid}>
        <AgentStatusTable agents={agents} />
        <div className={styles.rightCol}>
          <TokenTrendChart agents={agents} />
          <LiveEventStream events={events} />
        </div>
      </div>
    </div>
  )
}
