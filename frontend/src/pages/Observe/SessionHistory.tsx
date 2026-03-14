import { useState, useEffect } from 'react'
import { observeApi } from '../../api/observe'
import type { SessionInfo } from '../../types/observe'
import styles from './ObserveDetail.module.css'

interface SessionHistoryProps {
  agentName: string
  activeSessionId?: string
  onSessionSelect: (sessionId: string | null) => void
}

export default function SessionHistory({
  agentName,
  activeSessionId,
  onSessionSelect,
}: SessionHistoryProps) {
  const [sessions, setSessions] = useState<SessionInfo[]>([])

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const data = await observeApi.getSessions(agentName)
        if (!cancelled) setSessions(data ?? [])
      } catch {
        // ignore
      }
    }
    load()
    const timer = setInterval(load, 30_000)
    return () => { cancelled = true; clearInterval(timer) }
  }, [agentName])

  return (
    <div className={styles.sessionHistory}>
      <div className={styles.sessionHistoryHeader}>
        <span>Sessions</span>
        <span style={{ fontWeight: 400, fontSize: '0.75rem', color: 'var(--text-muted)' }}>
          {sessions.length} total
        </span>
      </div>
      <div className={styles.sessionList}>
        {sessions.length === 0 ? (
          <div style={{ padding: '12px 24px', fontSize: '0.8125rem', color: 'var(--text-muted)' }}>
            No sessions found
          </div>
        ) : (
          sessions.map((session) => {
            const isActive = activeSessionId === session.id
            return (
              <div
                key={session.id || session.filename}
                className={`${styles.sessionItem} ${isActive ? styles.sessionItemActive : ''}`}
                onClick={() => onSessionSelect(session.is_current ? null : (session.id || session.filename))}
              >
                <span className={styles.sessionId}>{(session.id || session.filename).slice(0, 8)}</span>
                <span className={styles.sessionTime}>
                  {formatSessionTime(session.started_at)}
                </span>
                <span className={styles.sessionMsgCount}>
                  {session.message_count} messages
                </span>
                <span
                  className={`${styles.sessionStatus} ${
                    session.is_current ? styles.sessionStatusActive : styles.sessionStatusDone
                  }`}
                >
                  {session.is_current ? 'running' : 'completed'}
                </span>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}

function formatSessionTime(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  const now = new Date()
  const isToday = d.toDateString() === now.toDateString()
  if (isToday) {
    return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) +
    ' ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
}
