import { useState } from 'react'
import { auditApi } from '../../api/audit'
import { PageHeader, Card, Input, Select, Button } from '../../components/ui'
import { useToast } from '../../hooks/useToast'
import type { TraceFilter, RequestType } from '../../types/trace'

export default function Export() {
  const [filter, setFilter] = useState<TraceFilter>({})
  const [estimated, setEstimated] = useState<number | null>(null)
  const [estimating, setEstimating] = useState(false)
  const toast = useToast()

  const handleEstimate = async () => {
    setEstimating(true)
    const r = await auditApi.getTraces({ ...filter, limit: 1 }).catch(() => null)
    setEstimated(r?.total ?? null)
    setEstimating(false)
  }

  const handleExport = () => {
    auditApi.exportTraces(filter)
    toast.success('Export started', 'Your JSONL file will download shortly.')
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px', maxWidth: '600px' }}>
      <PageHeader title="Training Data Export" />

      <Card>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <Input
              type="text"
              placeholder="YYYY-MM-DD"
              label="Start Date"
              onBlur={(e) => setFilter((p) => ({ ...p, start_time: e.target.value || undefined }))}
            />
            <Input
              type="text"
              placeholder="YYYY-MM-DD"
              label="End Date"
              onBlur={(e) => setFilter((p) => ({ ...p, end_time: e.target.value || undefined }))}
            />
          </div>
          <Input
            label="Agent (empty = all)"
            placeholder="agent-name"
            onChange={(e) => setFilter((p) => ({ ...p, agent_id: e.target.value ? [e.target.value] : undefined }))}
          />
          <Select
            label="Request Type (empty = all)"
            onChange={(e) => setFilter((p) => ({ ...p, request_type: (e.target.value as RequestType) || undefined }))}
          >
            <option value="">All</option>
            <option value="llm_inference">llm_inference</option>
            <option value="gitea_api">gitea_api</option>
            <option value="git_http">git_http</option>
            <option value="heartbeat">heartbeat</option>
          </Select>

          {estimated !== null && (
            <div style={{
              padding: '12px 16px',
              backgroundColor: 'rgba(17,17,17,0.03)',
              borderRadius: '8px',
              fontSize: '0.875rem',
            }}>
              Estimated records: ~<strong>{estimated.toLocaleString()}</strong>
            </div>
          )}

          <div style={{ display: 'flex', gap: '12px' }}>
            <Button
              variant="secondary"
              fullWidth
              onClick={handleEstimate}
              loading={estimating}
            >
              Estimate Count
            </Button>
            <Button
              fullWidth
              onClick={handleExport}
            >
              Export JSONL
            </Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
