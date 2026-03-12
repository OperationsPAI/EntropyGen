import { useEffect, useState, useCallback, useMemo } from 'react'
import { auditApi } from '../../api/audit'
import TraceDetail from '../../components/trace/TraceDetail'
import { PageHeader, Card, Table, Select, Input, EmptyState, Button, SplitPane } from '../../components/ui'
import type { AuditTrace, TraceFilter, RequestType } from '../../types/trace'
import styles from './Audit.module.css'

const REQUEST_TYPES: RequestType[] = ['llm_inference', 'gitea_api', 'git_http', 'heartbeat']
const PAGE_SIZE = 20

export default function AuditPage() {
  const [traces, setTraces] = useState<AuditTrace[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [filter, setFilter] = useState<TraceFilter>({ limit: PAGE_SIZE, page: 1 })
  const [selected, setSelected] = useState<AuditTrace | null>(null)
  const [loading, setLoading] = useState(false)

  const loadTraces = useCallback((f: TraceFilter) => {
    setLoading(true)
    auditApi.getTraces(f)
      .then((r) => { setTraces(r?.items ?? []); setTotal(r?.total ?? 0) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { loadTraces(filter) }, [filter, loadTraces])

  const update = (partial: Partial<TraceFilter>) => {
    const next = { ...filter, ...partial, page: 1 }
    setFilter(next)
    setPage(1)
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const goToPage = (p: number) => {
    setPage(p)
    setFilter((f) => ({ ...f, page: p }))
  }

  const pageNumbers = useMemo(() => {
    const pages: (number | 'ellipsis')[] = []
    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) pages.push(i)
    } else {
      pages.push(1)
      if (page > 3) pages.push('ellipsis')
      for (let i = Math.max(2, page - 1); i <= Math.min(totalPages - 1, page + 1); i++) {
        pages.push(i)
      }
      if (page < totalPages - 2) pages.push('ellipsis')
      pages.push(totalPages)
    }
    return pages
  }, [page, totalPages])

  const tablePane = (
    <div className={styles.tablePane}>
      <div className={styles.tableScroll}>
        {traces.length === 0 && !loading ? (
          <EmptyState title="No audit records" description="Adjust your filters or check back later." />
        ) : (
          <Table>
            <thead>
              <tr>
                {['Trace ID', 'Agent', 'Type', 'Method + Path', 'Status', 'Model', 'Token In/Out', 'Latency', 'Time'].map((h) => (
                  <th key={h}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {traces.map((t) => (
                <tr
                  key={t.trace_id}
                  onClick={() => setSelected(t)}
                  style={{ cursor: 'pointer', backgroundColor: selected?.trace_id === t.trace_id ? 'rgba(0,0,0,0.04)' : undefined }}
                >
                  <td style={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>{t.trace_id.slice(0, 8)}...</td>
                  <td>{t.agent_id}</td>
                  <td style={{ color: 'var(--text-muted)' }}>{t.request_type}</td>
                  <td style={{ maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    <span style={{ color: 'var(--text-muted)' }}>{t.method}</span> {t.path}
                  </td>
                  <td>
                    <span style={{
                      padding: '2px 8px',
                      borderRadius: '6px',
                      backgroundColor: t.status_code >= 400 ? '#e5dcdc' : '#dce5dc',
                      color: t.status_code >= 400 ? '#402a2a' : '#2a402a',
                      fontSize: '0.75rem',
                      fontWeight: 600,
                    }}>{t.status_code}</span>
                  </td>
                  <td style={{ color: 'var(--text-muted)' }}>{t.model ?? '\u2014'}</td>
                  <td style={{ fontVariantNumeric: 'tabular-nums' }}>
                    {t.tokens_in != null ? `${t.tokens_in} / ${t.tokens_out}` : '\u2014'}
                  </td>
                  <td>{t.latency_ms}ms</td>
                  <td style={{ color: 'var(--text-muted)' }}>{new Date(t.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </div>
      {traces.length > 0 && (
        <div className={styles.pagination}>
          <Button variant="ghost" size="sm" disabled={page === 1} onClick={() => goToPage(page - 1)}>
            Previous
          </Button>
          {pageNumbers.map((p, i) =>
            p === 'ellipsis' ? (
              <span key={`e-${i}`} className={styles.ellipsis}>...</span>
            ) : (
              <Button key={p} variant={p === page ? 'primary' : 'ghost'} size="sm" onClick={() => goToPage(p)}>
                {p}
              </Button>
            )
          )}
          <Button variant="ghost" size="sm" disabled={page >= totalPages} onClick={() => goToPage(page + 1)}>
            Next
          </Button>
        </div>
      )}
    </div>
  )

  return (
    <div className={styles.page}>
      <PageHeader
        title="Audit Log"
        description={`${total} records total`}
      />

      <Card>
        <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-end', flexWrap: 'wrap' }}>
          <Select
            onChange={(e) => update({ request_type: (e.target.value as RequestType) || undefined })}
            label="Type"
          >
            <option value="">All Types</option>
            {REQUEST_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
          </Select>
          <Select
            onChange={(e) => update({ status: (e.target.value as 'success' | 'error') || undefined })}
            label="Status"
          >
            <option value="">All Status</option>
            <option value="success">Success</option>
            <option value="error">Error</option>
          </Select>
          <Input
            type="text"
            placeholder="YYYY-MM-DD"
            label="Start Date"
            onBlur={(e) => update({ start_time: e.target.value || undefined })}
            style={{ width: '160px' }}
          />
          <Input
            type="text"
            placeholder="YYYY-MM-DD"
            label="End Date"
            onBlur={(e) => update({ end_time: e.target.value || undefined })}
            style={{ width: '160px' }}
          />
          {loading && <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem', paddingBottom: '8px' }}>Loading...</span>}
        </div>
      </Card>

      <Card>
        <div className={styles.splitWrap}>
          <SplitPane
            left={tablePane}
            right={selected ? <TraceDetail trace={selected} onClose={() => setSelected(null)} /> : null}
            defaultLeftWidth={60}
            minLeftWidth={400}
            minRightWidth={320}
          />
        </div>
      </Card>
    </div>
  )
}
