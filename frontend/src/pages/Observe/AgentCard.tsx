import { useNavigate } from 'react-router-dom'
import type { Agent, AgentPhase } from '../../types/agent'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import styles from './Observe.module.css'

interface AgentCardProps {
  agent: Agent
}

export default function AgentCard({ agent }: AgentCardProps) {
  const navigate = useNavigate()
  const { name, spec, status } = agent

  const activityInfo = getActivityInfo(status.phase, status.lastAction?.description)
  const { percent, label, color } = getActivityBar(status.lastAction?.timestamp)

  return (
    <div
      className={styles.agentCard}
      onClick={() => navigate(`/observe/${name}`)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter') navigate(`/observe/${name}`)
      }}
    >
      <div className={styles.cardHeader}>
        <span className={styles.cardName}>{name}</span>
        <AgentPhaseTag phase={status.phase} />
      </div>

      <div className={styles.cardRole}>Role: {spec.role}</div>

      <div className={styles.cardActivity}>
        <div className={styles.activityMain}>
          <span className={styles.activityIcon}>{activityInfo.icon}</span>
          {activityInfo.text}
        </div>
        {activityInfo.detail && (
          <div className={styles.activityDetail}>{activityInfo.detail}</div>
        )}
      </div>

      <div className={styles.cardMeta}>
        <span className={styles.lastAction} title={status.lastAction?.description}>
          {status.lastAction ? `Latest: ${status.lastAction.description}` : 'No recent actions'}
        </span>
        <span className={styles.tokenCount}>
          Token: {formatNumber(status.tokenUsage.today)}
        </span>
      </div>

      <div className={styles.activityBarWrapper}>
        <div className={styles.activityBar}>
          <div
            className={styles.activityBarFill}
            style={{ width: `${percent}%`, backgroundColor: color }}
          />
        </div>
        <span className={styles.activityBarLabel}>{label}</span>
      </div>
    </div>
  )
}

function getActivityInfo(phase: AgentPhase, lastDescription?: string) {
  if (phase === 'Paused') {
    return { icon: '⏸', text: 'Paused', detail: 'Manually paused by admin' }
  }
  if (phase === 'Error') {
    return { icon: '⚠', text: 'Error', detail: 'Check agent status' }
  }
  if (phase === 'Initializing' || phase === 'Pending') {
    return { icon: '⏳', text: 'Initializing...', detail: undefined }
  }
  if (lastDescription) {
    return { icon: '▶', text: lastDescription, detail: undefined }
  }
  return { icon: '▶', text: 'Waiting for next cron cycle', detail: 'Idle...' }
}

function getActivityBar(timestamp?: string): { percent: number; label: string; color: string } {
  if (!timestamp) return { percent: 5, label: 'no data', color: 'var(--text-muted)' }
  const minutes = (Date.now() - new Date(timestamp).getTime()) / 60000
  if (minutes < 1) return { percent: 100, label: 'just now', color: 'var(--status-green)' }
  if (minutes < 5) return { percent: 75, label: `${Math.round(minutes)}m ago`, color: 'var(--status-green)' }
  if (minutes < 15) return { percent: 50, label: `${Math.round(minutes)}m ago`, color: 'var(--status-yellow)' }
  if (minutes < 60) return { percent: 25, label: `${Math.round(minutes)}m ago`, color: 'var(--text-muted)' }
  const hours = Math.round(minutes / 60)
  return { percent: 8, label: `${hours}h ago`, color: 'var(--text-muted)' }
}

function formatNumber(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}
