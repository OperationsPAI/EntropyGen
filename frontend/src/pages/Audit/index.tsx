import { useEffect, useState, useCallback } from 'react'
import { auditApi } from '../../api/audit'
import TraceDetailPanel from '../../components/trace/TraceDetailPanel'
import type { AuditTrace, TraceFilter, RequestType } from '../../types/trace'

const REQUEST_TYPES: RequestType[] = ['llm_inference', 'gitea_api', 'git_http', 'heartbeat']

export default function AuditPage() {
  const [traces, setTraces] = useState<AuditTrace[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [filter, setFilter] = useState<TraceFilter>({ limit: 20, page: 1 })
  const [selected, setSelected] = useState<AuditTrace | null>(null)
  const [loading, setLoading] = useState(false)

  const loadTraces = useCallback((f: TraceFilter) => {
    setLoading(true)
    auditApi.getTraces(f).then((r) => { setTraces(r.items); setTotal(r.total) }).catch(() => {}).finally(() => setLoading(false))
  }, [])

  useEffect(() => { loadTraces(filter) }, [filter, loadTraces])

  const update = (partial: Partial<TraceFilter>) => {
    const next = { ...filter, ...partial, page: 1 }
    setFilter(next)
    setPage(1)
  }

  const totalPages = Math.max(1, Math.ceil(total / 20))

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', position: 'relative' }}>
      <h2 style={{ fontSize: '1.1rem', fontWeight: 600 }}>Audit Log</h2>

      <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '16px', padding: '16px 20px', display: 'flex', gap: '12px', alignItems: 'center', flexWrap: 'wrap' }}>
        <select onChange={(e) => update({ request_type: (e.target.value as RequestType) || undefined })} style={{ padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--line-subtle)', fontSize: '0.85rem', fontFamily: 'inherit', backgroundColor: 'white' }}>
          <option value="">All Types</option>
          {REQUEST_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
        </select>
        <select onChange={(e) => update({ status: (e.target.value as 'success' | 'error') || undefined })} style={{ padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--line-subtle)', fontSize: '0.85rem', fontFamily: 'inherit', backgroundColor: 'white' }}>
          <option value="">All Status</option>
          <option value="success">Success</option>
          <option value="error">Error</option>
        </select>
        <input type="date" onChange={(e) => update({ start_time: e.target.value || undefined })} style={{ padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--line-subtle)', fontSize: '0.85rem', fontFamily: 'inherit' }} />
        <input type="date" onChange={(e) => update({ end_time: e.target.value || undefined })} style={{ padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--line-subtle)', fontSize: '0.85rem', fontFamily: 'inherit' }} />
        {loading && <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>Loading...</span>}
        <span style={{ marginLeft: 'auto', color: 'var(--text-muted)', fontSize: '0.8rem' }}>{total} records total</span>
      </div>

      <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', padding: '24px' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--line-subtle)' }}>
              {['Trace ID', 'Agent', 'Type', 'Method + Path', 'Status', 'Model', 'Token In/Out', 'Latency', 'Time'].map((h) => (
                <th key={h} style={{ textAlign: 'left', padding: '8px 12px 12px', color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.03em' }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {traces.length === 0 ? (
              <tr><td colSpan={9} style={{ padding: '32px', color: 'var(--text-muted)', textAlign: 'center' }}>No data</td></tr>
            ) : traces.map((t) => (
              <tr key={t.trace_id} onClick={() => setSelected(t)} style={{ borderBottom: '1px solid var(--line-subtle)', cursor: 'pointer' }}>
                <td style={{ padding: '10px 12px', fontFamily: 'monospace', fontSize: '0.75rem' }}>{t.trace_id.slice(0, 8)}…</td>
                <td style={{ padding: '10px 12px' }}>{t.agent_id}</td>
                <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{t.request_type}</td>
                <td style={{ padding: '10px 12px', maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}><span style={{ color: 'var(--text-muted)' }}>{t.method}</span> {t.path}</td>
                <td style={{ padding: '10px 12px' }}>
                  <span style={{ padding: '2px 8px', borderRadius: '6px', backgroundColor: t.status_code >= 400 ? '#e5dcdc' : '#dce5dc', color: t.status_code >= 400 ? '#402a2a' : '#2a402a', fontSize: '0.75rem', fontWeight: 600 }}>{t.status_code}</span>
                </td>
                <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{t.model ?? '—'}</td>
                <td style={{ padding: '10px 12px', fontVariantNumeric: 'tabular-nums' }}>{t.tokens_in != null ? `${t.tokens_in} / ${t.tokens_out}` : '—'}</td>
                <td style={{ padding: '10px 12px' }}>{t.latency_ms}ms</td>
                <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{new Date(t.created_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <div style={{ display: 'flex', justifyContent: 'center', gap: '8px', marginTop: '16px' }}>
          <button disabled={page === 1} onClick={() => { const p = page - 1; setPage(p); setFilter((f) => ({ ...f, page: p })) }} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--line-subtle)', background: 'none', cursor: page === 1 ? 'not-allowed' : 'pointer', opacity: page === 1 ? 0.4 : 1, fontFamily: 'inherit' }}>Previous</button>
          <span style={{ padding: '6px 14px', fontSize: '0.85rem', color: 'var(--text-muted)' }}>Page {page} of {totalPages}</span>
          <button disabled={page >= totalPages} onClick={() => { const p = page + 1; setPage(p); setFilter((f) => ({ ...f, page: p })) }} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--line-subtle)', background: 'none', cursor: page >= totalPages ? 'not-allowed' : 'pointer', opacity: page >= totalPages ? 0.4 : 1, fontFamily: 'inherit' }}>Next</button>
        </div>
      </div>
      <TraceDetailPanel trace={selected} onClose={() => setSelected(null)} />
    </div>
  )
}
