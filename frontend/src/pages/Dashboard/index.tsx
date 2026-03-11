import { useEffect, useState } from 'react'
import ReactECharts from 'echarts-for-react'
import { agentsApi } from '../../api/agents'
import { useAgentStore } from '../../stores/agentStore'
import { useEventStore } from '../../stores/eventStore'
import { useAlertStore } from '../../stores/alertStore'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import type { Agent } from '../../types/agent'

const MOCK_AGENTS: Agent[] = [
  { name: 'observer-1', spec: { role: 'observer', soul: '', llm: { model: 'gpt-4o', temperature: 0.7, maxTokens: 4096 }, cron: { schedule: '*/5 * * * *', prompt: '' }, resources: { cpuRequest: '100m', cpuLimit: '500m', memoryRequest: '256Mi', memoryLimit: '1Gi', workspaceSize: '5Gi' }, gitea: { repo: 'ai-team/webapp', permissions: ['read'] } }, status: { phase: 'Running', conditions: [], lastAction: { description: 'Scanned 3 open issues', timestamp: new Date().toISOString() }, tokenUsage: { today: 12400, total: 98000 }, createdAt: new Date(Date.now() - 86400000 * 3).toISOString() } },
  { name: 'developer-1', spec: { role: 'developer', soul: '', llm: { model: 'gpt-4o', temperature: 0.5, maxTokens: 8192 }, cron: { schedule: '*/10 * * * *', prompt: '' }, resources: { cpuRequest: '200m', cpuLimit: '1000m', memoryRequest: '512Mi', memoryLimit: '2Gi', workspaceSize: '10Gi' }, gitea: { repo: 'ai-team/webapp', permissions: ['read', 'write'] } }, status: { phase: 'Error', conditions: [{ type: 'Ready', status: 'False', reason: 'CrashLoop', message: 'Pod restarted 6 times' }], lastAction: { description: 'Pushed feat/auth', timestamp: new Date().toISOString() }, tokenUsage: { today: 45200, total: 312000 }, createdAt: new Date(Date.now() - 86400000 * 7).toISOString() } },
  { name: 'reviewer-1', spec: { role: 'reviewer', soul: '', llm: { model: 'claude-3-5-sonnet', temperature: 0.3, maxTokens: 4096 }, cron: { schedule: '*/15 * * * *', prompt: '' }, resources: { cpuRequest: '100m', cpuLimit: '500m', memoryRequest: '256Mi', memoryLimit: '1Gi', workspaceSize: '5Gi' }, gitea: { repo: 'ai-team/webapp', permissions: ['read', 'review'] } }, status: { phase: 'Running', conditions: [], lastAction: { description: 'Reviewed PR #8', timestamp: new Date().toISOString() }, tokenUsage: { today: 8900, total: 67000 }, createdAt: new Date(Date.now() - 86400000 * 5).toISOString() } },
  { name: 'sre-1', spec: { role: 'sre', soul: '', llm: { model: 'gpt-4o-mini', temperature: 0.2, maxTokens: 2048 }, cron: { schedule: '*/30 * * * *', prompt: '' }, resources: { cpuRequest: '100m', cpuLimit: '500m', memoryRequest: '256Mi', memoryLimit: '1Gi', workspaceSize: '5Gi' }, gitea: { repo: 'ai-team/infra', permissions: ['read'] } }, status: { phase: 'Paused', conditions: [], lastAction: { description: 'Checked pod metrics', timestamp: new Date().toISOString() }, tokenUsage: { today: 0, total: 23000 }, createdAt: new Date(Date.now() - 86400000 * 14).toISOString() } },
]

function genHourlyData() {
  const hours = Array.from({ length: 24 }, (_, i) => `${i}:00`)
  const agents = ['observer-1', 'developer-1', 'reviewer-1']
  return { xAxis: hours, series: agents.map((name) => ({ name, data: Array.from({ length: 24 }, () => Math.floor(Math.random() * 3000)) })) }
}

export default function Dashboard() {
  const { agents, setAgents } = useAgentStore()
  const { events } = useEventStore()
  const { alerts } = useAlertStore()
  const [display, setDisplay] = useState<Agent[]>(MOCK_AGENTS)

  useEffect(() => {
    agentsApi.getAgents().then((data) => { setAgents(data); setDisplay(data) }).catch(() => { setAgents(MOCK_AGENTS); setDisplay(MOCK_AGENTS) })
  }, [setAgents])

  useEffect(() => { if (agents.length > 0) setDisplay(agents) }, [agents])

  const running = display.filter((a) => a.status.phase === 'Running').length
  const todayTokens = display.reduce((s, a) => s + (a.status.tokenUsage?.today ?? 0), 0)
  const chartData = genHourlyData()

  const chartOption = {
    tooltip: { trigger: 'axis' },
    legend: { data: chartData.series.map((s) => s.name), bottom: 0 },
    grid: { top: 16, bottom: 40, left: 16, right: 16, containLabel: true },
    xAxis: { type: 'category', data: chartData.xAxis, axisLine: { show: false }, axisTick: { show: false }, axisLabel: { fontSize: 10, color: '#665f58' } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: 'rgba(17,17,17,0.08)' } }, axisLabel: { fontSize: 10, color: '#665f58' } },
    series: chartData.series.map((s) => ({ name: s.name, type: 'line', smooth: true, data: s.data, symbol: 'none', lineStyle: { width: 2 } })),
  }

  const statCard = (label: string, value: string | number, sub?: string) => (
    <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '16px', padding: '20px 24px' }}>
      <div style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', fontWeight: 600, marginBottom: '8px' }}>{label}</div>
      <div style={{ fontSize: '2rem', fontWeight: 600, letterSpacing: '-0.04em', fontVariantNumeric: 'tabular-nums' }}>{value}</div>
      {sub && <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', marginTop: '4px' }}>{sub}</div>}
    </div>
  )

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '12px' }}>
        {statCard('Running Agents', `${running}/${display.length}`)}
        {statCard("Today's Tokens", todayTokens.toLocaleString())}
        {statCard('Gitea Events', events.filter((e) => e.event_type.startsWith('gitea.')).length || '—')}
        {statCard('Alerts', alerts.length, alerts.length > 0 ? 'Attention needed' : 'Normal')}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '3fr 2fr', gap: '12px', alignItems: 'start' }}>
        <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', padding: '24px' }}>
          <div style={{ fontWeight: 600, fontSize: '1rem', letterSpacing: '-0.02em', marginBottom: '16px' }}>Agent Status</div>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid var(--line-subtle)' }}>
                {['Name', 'Role', 'Status', 'Last Action', 'Token/Today'].map((h) => (
                  <th key={h} style={{ textAlign: 'left', padding: '8px 12px 12px', color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.03em' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {display.map((agent) => (
                <tr key={agent.name} style={{ borderBottom: '1px solid var(--line-subtle)', backgroundColor: agent.status.phase === 'Error' ? 'rgba(229,80,43,0.04)' : undefined }}>
                  <td style={{ padding: '12px', fontWeight: 600 }}>{agent.name}</td>
                  <td style={{ padding: '12px', color: 'var(--text-muted)' }}>{agent.spec.role}</td>
                  <td style={{ padding: '12px' }}><AgentPhaseTag phase={agent.status.phase} /></td>
                  <td style={{ padding: '12px', color: 'var(--text-muted)', maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{agent.status.lastAction?.description ?? '—'}</td>
                  <td style={{ padding: '12px', fontVariantNumeric: 'tabular-nums', fontWeight: 500 }}>{(agent.status.tokenUsage?.today ?? 0).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', padding: '24px' }}>
            <div style={{ fontWeight: 600, fontSize: '0.9rem', letterSpacing: '-0.02em', marginBottom: '12px' }}>Token Trend (Today)</div>
            <ReactECharts option={chartOption} style={{ height: '180px' }} />
          </div>
          <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', padding: '24px' }}>
            <div style={{ fontWeight: 600, fontSize: '0.9rem', letterSpacing: '-0.02em', marginBottom: '12px' }}>Live Event Stream</div>
            {events.length === 0 ? (
              <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>No events yet</div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '200px', overflowY: 'auto' }}>
                {events.map((ev, i) => (
                  <div key={i} style={{ fontSize: '0.8rem', display: 'flex', gap: '8px' }}>
                    <span style={{ color: 'var(--text-muted)', flexShrink: 0 }}>{new Date(ev.timestamp).toLocaleTimeString()}</span>
                    <span style={{ fontWeight: 500 }}>{ev.agent_id}</span>
                    <span style={{ color: 'var(--text-muted)' }}>{ev.event_type}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
