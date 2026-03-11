import { useState } from 'react'
import { auditApi } from '../../api/audit'
import type { TraceFilter, RequestType } from '../../types/trace'

export default function Export() {
  const [filter, setFilter] = useState<TraceFilter>({})
  const [estimated, setEstimated] = useState<number | null>(null)
  const [estimating, setEstimating] = useState(false)

  const handleEstimate = async () => {
    setEstimating(true)
    const r = await auditApi.getTraces({ ...filter, limit: 1 }).catch(() => null)
    setEstimated(r?.total ?? null)
    setEstimating(false)
  }

  const labelStyle = { fontSize: '0.75rem', fontWeight: 600 as const, textTransform: 'uppercase' as const, letterSpacing: '0.05em', color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }
  const inputStyle = { width: '100%', padding: '10px 14px', border: '1px solid var(--line-subtle)', borderRadius: '8px', fontSize: '0.9rem', fontFamily: 'inherit' }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', maxWidth: '600px' }}>
      <h2 style={{ fontSize: '1.1rem', fontWeight: 600 }}>Training Data Export</h2>
      <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', padding: '32px' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <label>
              <span style={labelStyle}>Start Date</span>
              <input type="date" onChange={(e) => setFilter((p) => ({ ...p, start_time: e.target.value || undefined }))} style={inputStyle} />
            </label>
            <label>
              <span style={labelStyle}>End Date</span>
              <input type="date" onChange={(e) => setFilter((p) => ({ ...p, end_time: e.target.value || undefined }))} style={inputStyle} />
            </label>
          </div>
          <label>
            <span style={labelStyle}>Agent (empty = all)</span>
            <input placeholder="agent-name" onChange={(e) => setFilter((p) => ({ ...p, agent_id: e.target.value ? [e.target.value] : undefined }))} style={inputStyle} />
          </label>
          <label>
            <span style={labelStyle}>Request Type (empty = all)</span>
            <select onChange={(e) => setFilter((p) => ({ ...p, request_type: (e.target.value as RequestType) || undefined }))} style={{ ...inputStyle, backgroundColor: 'white' }}>
              <option value="">All</option>
              <option value="llm_inference">llm_inference</option>
              <option value="gitea_api">gitea_api</option>
              <option value="git_http">git_http</option>
              <option value="heartbeat">heartbeat</option>
            </select>
          </label>

          {estimated !== null && (
            <div style={{ padding: '12px 16px', backgroundColor: '#f5f5f5', borderRadius: '8px', fontSize: '0.875rem' }}>
              Estimated records: ~<strong>{estimated.toLocaleString()}</strong>
            </div>
          )}

          <div style={{ display: 'flex', gap: '12px' }}>
            <button onClick={handleEstimate} disabled={estimating} style={{ flex: 1, padding: '12px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: estimating ? 'not-allowed' : 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>
              {estimating ? 'Estimating...' : 'Estimate Count'}
            </button>
            <button onClick={() => auditApi.exportTraces(filter)} style={{ flex: 1, padding: '12px', borderRadius: '8px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', cursor: 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>
              Export JSONL
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
