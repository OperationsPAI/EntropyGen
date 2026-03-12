import { useEffect, useState } from 'react'
import { llmApi, type LLMModel, type CreateModelDto } from '../../api/llm'
import { PageHeader, Card, Table, Button, Input, Modal, EmptyState } from '../../components/ui'
import { useToast } from '../../hooks/useToast'

export default function LLMPage() {
  const [models, setModels] = useState<LLMModel[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<LLMModel | null>(null)
  const [healthStatus, setHealthStatus] = useState<Record<string, string>>({})
  const [form, setForm] = useState<Partial<CreateModelDto>>({ provider: 'openai', rpm: 60, tpm: 100000 })
  const [creating, setCreating] = useState(false)
  const toast = useToast()

  useEffect(() => {
    setLoading(true)
    llmApi.getModels()
      .then(setModels)
      .catch(() => { setError(true) })
      .finally(() => setLoading(false))
  }, [])

  const handleHealth = async (id: string) => {
    setHealthStatus((p) => ({ ...p, [id]: 'checking...' }))
    try {
      const result = await llmApi.checkHealth(id)
      setHealthStatus((p) => ({ ...p, [id]: result.status }))
      toast.success('Health check complete', `Status: ${result.status}`)
    } catch {
      setHealthStatus((p) => ({ ...p, [id]: 'unhealthy' }))
      toast.error('Health check failed', 'Could not connect to the model.')
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await llmApi.deleteModel(deleteTarget.id)
      setModels((p) => p.filter((x) => x.id !== deleteTarget.id))
      toast.success('Model deleted', `${deleteTarget.name} has been removed.`)
    } catch {
      toast.error('Delete failed', 'Could not delete the model.')
    } finally {
      setDeleteTarget(null)
    }
  }

  const handleCreate = async () => {
    setCreating(true)
    try {
      const m = await llmApi.createModel(form as CreateModelDto)
      setModels((p) => [...p, m])
      toast.success('Model added', `${m.name} has been configured.`)
      setModalOpen(false)
      setForm({ provider: 'openai', rpm: 60, tpm: 100000 })
    } catch {
      toast.error('Creation failed', 'Could not add the model.')
    } finally {
      setCreating(false)
    }
  }

  const statusBadge = (status: string) => {
    const map: Record<string, { bg: string; color: string }> = {
      healthy: { bg: '#dce5dc', color: '#2a402a' },
      unhealthy: { bg: '#e5dcdc', color: '#402a2a' },
      unknown: { bg: '#e6e1dc', color: '#5c5752' },
    }
    const s = map[status] ?? map.unknown
    return (
      <span style={{
        padding: '3px 10px',
        borderRadius: '999px',
        backgroundColor: s.bg,
        color: s.color,
        fontSize: '0.75rem',
        fontWeight: 600,
      }}>
        {status}
      </span>
    )
  }

  const renderContent = () => {
    if (loading) {
      return (
        <Card>
          <div style={{ display: 'flex', justifyContent: 'center', padding: '48px', color: 'var(--text-muted)' }}>
            Loading models...
          </div>
        </Card>
      )
    }

    if (error) {
      return (
        <Card>
          <EmptyState
            title="Failed to load models"
            description="Could not fetch the model list. Please try again."
            action={
              <Button variant="secondary" onClick={() => { setError(false); setLoading(true); llmApi.getModels().then(setModels).catch(() => setError(true)).finally(() => setLoading(false)) }}>
                Retry
              </Button>
            }
          />
        </Card>
      )
    }

    if (models.length === 0) {
      return (
        <Card>
          <EmptyState
            title="No models configured"
            description="Add your first model to get started."
            action={
              <Button onClick={() => setModalOpen(true)}>Add Model</Button>
            }
          />
        </Card>
      )
    }

    return (
      <Card>
        <Table>
          <thead>
            <tr>
              {['Model Name', 'Provider', 'RPM', 'TPM', 'Status', 'Actions'].map((h) => (
                <th key={h}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {models.map((m) => (
              <tr key={m.id}>
                <td style={{ fontWeight: 600 }}>{m.name}</td>
                <td style={{ color: 'var(--text-muted)' }}>{m.provider}</td>
                <td style={{ fontVariantNumeric: 'tabular-nums' }}>{m.rpm}</td>
                <td style={{ fontVariantNumeric: 'tabular-nums' }}>{m.tpm.toLocaleString()}</td>
                <td>{statusBadge(healthStatus[m.id] ?? m.status)}</td>
                <td>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <Button variant="ghost" size="sm" onClick={() => handleHealth(m.id)}>
                      Test Connection
                    </Button>
                    <Button variant="danger" size="sm" onClick={() => setDeleteTarget(m)}>
                      Delete
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      </Card>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <PageHeader
        title="LLM Models"
        actions={
          <Button onClick={() => setModalOpen(true)}>+ Add Model</Button>
        }
      />

      {renderContent()}

      {/* Add Model Modal */}
      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title="Add Model"
        width={440}
        footer={
          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setModalOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} loading={creating}>Create</Button>
          </div>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <Input
            label="Name"
            value={form.name ?? ''}
            onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
          />
          <Input
            label="Provider"
            value={form.provider ?? ''}
            onChange={(e) => setForm((p) => ({ ...p, provider: e.target.value }))}
          />
          <Input
            label="API Key"
            type="password"
            value={form.apiKey ?? ''}
            onChange={(e) => setForm((p) => ({ ...p, apiKey: e.target.value }))}
          />
          <Input
            label="Base URL"
            value={form.baseUrl ?? ''}
            onChange={(e) => setForm((p) => ({ ...p, baseUrl: e.target.value }))}
          />
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <Input
              label="RPM"
              type="number"
              value={form.rpm ?? 0}
              onChange={(e) => setForm((p) => ({ ...p, rpm: parseInt(e.target.value) }))}
            />
            <Input
              label="TPM"
              type="number"
              value={form.tpm ?? 0}
              onChange={(e) => setForm((p) => ({ ...p, tpm: parseInt(e.target.value) }))}
            />
          </div>
        </div>
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        open={deleteTarget !== null}
        onClose={() => setDeleteTarget(null)}
        title="Delete Model"
        width={400}
        footer={
          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button variant="danger" onClick={handleDelete}>Delete</Button>
          </div>
        }
      >
        <p style={{ fontSize: '0.9rem', color: 'var(--text-muted)' }}>
          Are you sure you want to delete <strong style={{ color: 'var(--text-main)' }}>{deleteTarget?.name}</strong>?
          This action cannot be undone.
        </p>
      </Modal>
    </div>
  )
}
