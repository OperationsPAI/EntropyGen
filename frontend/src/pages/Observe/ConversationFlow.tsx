import { useRef, useEffect, useState, useCallback } from 'react'
import type { FileChangeEvent } from '../../types/observe'
import styles from './ObserveDetail.module.css'

/** Activity entry with a local receive timestamp. */
export interface ActivityEntry extends FileChangeEvent {
  timestamp: string
}

interface WorkspaceActivityProps {
  events: ActivityEntry[]
}

const ACTION_LABELS: Record<FileChangeEvent['action'], string> = {
  created: 'Created',
  modified: 'Modified',
  deleted: 'Deleted',
}

const ACTION_COLORS: Record<FileChangeEvent['action'], string> = {
  created: 'var(--status-green)',
  modified: 'var(--status-yellow)',
  deleted: 'var(--status-red)',
}

function formatTime(ts: string): string {
  try {
    return new Date(ts).toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return ts
  }
}

export default function WorkspaceActivity({ events }: WorkspaceActivityProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)
  const prevLenRef = useRef(events.length)

  // Auto-scroll when new events arrive
  useEffect(() => {
    if (autoScroll && events.length > prevLenRef.current && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
    prevLenRef.current = events.length
  }, [events.length, autoScroll])

  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current
    const atBottom = scrollHeight - scrollTop - clientHeight < 60
    setAutoScroll(atBottom)
  }, [])

  const scrollToBottom = useCallback(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
      setAutoScroll(true)
    }
  }, [])

  return (
    <div className={styles.conversationPanel}>
      <div className={styles.activityHeader}>
        <span>Workspace Activity</span>
        <span className={styles.activityCount}>{events.length} events</span>
      </div>

      <div
        ref={scrollRef}
        className={styles.conversationScroll}
        onScroll={handleScroll}
      >
        {events.length === 0 ? (
          <div className={styles.loadingState}>
            <span className={styles.waitingDot} />
            Watching for file changes...
          </div>
        ) : (
          events.map((entry, i) => (
            <div key={i} className={styles.activityRow}>
              <span className={styles.activityTime}>
                {formatTime(entry.timestamp)}
              </span>
              <span
                className={styles.activityAction}
                style={{ color: ACTION_COLORS[entry.action] }}
              >
                {ACTION_LABELS[entry.action]}
              </span>
              <span className={styles.activityPath} title={entry.path}>
                {entry.path}
              </span>
            </div>
          ))
        )}

        {!autoScroll && (
          <button className={styles.scrollToBottom} onClick={scrollToBottom}>
            Back to bottom
          </button>
        )}
      </div>
    </div>
  )
}
