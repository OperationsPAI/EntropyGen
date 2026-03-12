import { useState } from 'react'
import type { AuditTrace } from '../../types/trace'
import MonacoEditor from '../editor/MonacoEditor'
import styles from './TraceDetail.module.css'

function formatJson(str?: string): string {
  if (!str) return ''
  try {
    return JSON.stringify(JSON.parse(str), null, 2)
  } catch {
    return str
  }
}

interface TraceDetailProps {
  trace: AuditTrace
  onClose: () => void
}

const TABS = ['Request', 'Response'] as const

export default function TraceDetail({ trace, onClose }: TraceDetailProps) {
  const [tab, setTab] = useState<(typeof TABS)[number]>('Request')

  const content = tab === 'Request' ? formatJson(trace.request_body) : formatJson(trace.response_body)

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <div>
          <div className={styles.headerTitle}>{trace.trace_id.slice(0, 8)}&hellip;</div>
          <div className={styles.headerSub}>
            {trace.agent_id} &middot; {trace.request_type}
          </div>
        </div>
        <button className={styles.closeBtn} onClick={onClose}>
          &times;
        </button>
      </div>

      <div className={styles.metaBar}>
        <span>
          Status{' '}
          <strong className={trace.status_code >= 400 ? styles.metaValueErr : styles.metaValue}>
            {trace.status_code}
          </strong>
        </span>
        {trace.model && (
          <span>
            Model <strong className={styles.metaValue}>{trace.model}</strong>
          </span>
        )}
        {trace.tokens_in != null && (
          <span>
            In <strong className={styles.metaValue}>{trace.tokens_in}</strong>
          </span>
        )}
        {trace.tokens_out != null && (
          <span>
            Out <strong className={styles.metaValue}>{trace.tokens_out}</strong>
          </span>
        )}
        <span>
          Latency <strong className={styles.metaValue}>{trace.latency_ms}ms</strong>
        </span>
      </div>

      <div className={styles.tabs}>
        {TABS.map((t) => (
          <button
            key={t}
            className={`${styles.tab} ${tab === t ? styles.tabActive : ''}`}
            onClick={() => setTab(t)}
          >
            {t}
          </button>
        ))}
      </div>

      <div className={styles.body}>
        <div className={styles.editorWrap}>
          <MonacoEditor value={content} language="json" readOnly height="100%" />
        </div>
      </div>
    </div>
  )
}
