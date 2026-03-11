import type { AuditTrace } from '../../types/trace'
import MonacoEditor from '../editor/MonacoEditor'

function formatJson(str?: string): string {
  if (!str) return ''
  try { return JSON.stringify(JSON.parse(str), null, 2) } catch { return str }
}

interface Props {
  trace: AuditTrace | null
  onClose: () => void
}

export default function TraceDetailPanel({ trace, onClose }: Props) {
  if (!trace) return null

  return (
    <div style={{
      position: 'fixed', right: 0, top: 0, bottom: 0,
      width: '480px', backgroundColor: 'var(--bg-surface)',
      boxShadow: '-4px 0 24px rgba(0,0,0,0.08)',
      display: 'flex', flexDirection: 'column', zIndex: 100,
    }}>
      <div style={{
        padding: '20px 24px',
        borderBottom: '1px solid var(--line-subtle)',
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
      }}>
        <div>
          <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>{trace.trace_id.slice(0, 8)}…</div>
          <div style={{ color: 'var(--text-muted)', fontSize: '0.75rem' }}>
            {trace.agent_id} · {trace.request_type}
          </div>
        </div>
        <button onClick={onClose} style={{
          background: 'none', border: 'none', cursor: 'pointer',
          fontSize: '1.3rem', color: 'var(--text-muted)',
        }}>×</button>
      </div>

      <div style={{
        padding: '16px 24px', fontSize: '0.8rem', color: 'var(--text-muted)',
        display: 'flex', gap: '16px', borderBottom: '1px solid var(--line-subtle)',
        flexWrap: 'wrap',
      }}>
        <span>Status <strong style={{ color: trace.status_code >= 400 ? '#e5502b' : 'var(--text-main)' }}>{trace.status_code}</strong></span>
        {trace.model && <span>Model <strong>{trace.model}</strong></span>}
        {trace.tokens_in != null && <span>In <strong>{trace.tokens_in}</strong></span>}
        {trace.tokens_out != null && <span>Out <strong>{trace.tokens_out}</strong></span>}
        <span>Latency <strong>{trace.latency_ms}ms</strong></span>
      </div>

      <div style={{ flex: 1, overflow: 'auto', padding: '16px 24px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <div>
          <div style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', marginBottom: '8px', fontWeight: 600 }}>Request</div>
          <div style={{ borderRadius: '8px', overflow: 'hidden', border: '1px solid var(--line-subtle)' }}>
            <MonacoEditor value={formatJson(trace.request_body)} language="json" readOnly height="200px" />
          </div>
        </div>
        <div>
          <div style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', marginBottom: '8px', fontWeight: 600 }}>Response</div>
          <div style={{ borderRadius: '8px', overflow: 'hidden', border: '1px solid var(--line-subtle)' }}>
            <MonacoEditor value={formatJson(trace.response_body)} language="json" readOnly height="200px" />
          </div>
        </div>
      </div>
    </div>
  )
}
