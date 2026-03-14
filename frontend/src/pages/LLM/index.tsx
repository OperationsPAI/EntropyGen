import { useEffect, useState } from 'react'
import { llmApi, type LLMModel, type CreateModelDto } from '../../api/llm'
import { PageHeader, Card, Table, Button, Input, Modal, EmptyState } from '../../components/ui'
import { useToast } from '../../hooks/useToast'

const FIELD_HINTS: Record<string, string> = {
  name: 'LiteLLM model alias. Use any identifier, e.g. "gpt-4o" or "my-claude". This is the name you select when creating an agent.',
  provider: 'LLM provider key used by LiteLLM routing, e.g. "openai", "anthropic", "azure", "ollama".',
  apiKey: 'Your provider API key. For OpenAI: sk-xxx, for Anthropic: sk-ant-xxx. Stored securely in LiteLLM.',
  baseUrl: 'Custom API endpoint. Leave empty for default provider URLs. Use for proxies, Azure endpoints, or self-hosted models (e.g. http://ollama:11434).',
  rpm: 'Requests per minute rate limit. Set to match your provider plan to avoid 429 errors.',
  tpm: 'Tokens per minute rate limit. Set to match your provider plan.',
}

function HintIcon({ field }: { field: string }) {
  const [open, setOpen] = useState(false)
  const hint = FIELD_HINTS[field]
  if (!hint) return null

  return (
    <span
      style={{ position: 'relative', display: 'inline-block', marginLeft: '6px', cursor: 'help' }}
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      <span style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: '16px',
        height: '16px',
        borderRadius: '50%',
        backgroundColor: 'var(--line-subtle, #e0e0e0)',
        color: 'var(--text-muted)',
        fontSize: '0.65rem',
        fontWeight: 700,
        lineHeight: 1,
        verticalAlign: 'middle',
      }}>?</span>
      {open && (
        <span style={{
          position: 'absolute',
          bottom: '100%',
          left: '50%',
          transform: 'translateX(-50%)',
          marginBottom: '6px',
          padding: '8px 12px',
          backgroundColor: 'var(--bg-elevated, #333)',
          color: 'var(--text-on-elevated, #fff)',
          fontSize: '0.75rem',
          lineHeight: 1.5,
          borderRadius: '6px',
          width: '260px',
          zIndex: 100,
          boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
          pointerEvents: 'none',
        }}>
          {hint}
        </span>
      )}
    </span>
  )
}

function FieldLabel({ label, field }: { label: string; field: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', marginBottom: '4px' }}>
      <span style={{ fontSize: '0.8rem', fontWeight: 500, color: 'var(--text-main)' }}>{label}</span>
      <HintIcon field={field} />
    </div>
  )
}

export default function LLMPage() {
  const [models, setModels] = useState<LLMModel[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<LLMModel | null>(null)
  const [healthStatus, setHealthStatus] = useState<Record<string, string>>({})
  const [form, setForm] = useState<Partial<CreateModelDto>>({ provider: 'openai', rpm: 60, tpm: 100000 })
  const [creating, setCreating] = useState(false)
  const [serviceStatus, setServiceStatus] = useState<'idle' | 'checking' | 'healthy' | 'unhealthy'>('idle')
  const [serviceDetail, setServiceDetail] = useState('')
  const [chatModel, setChatModel] = useState<string | null>(null)
  const [chatMessage, setChatMessage] = useState('Say "hello" in one sentence.')
  const [chatResult, setChatResult] = useState<{ reply: string; model: string; latencyMs: number } | null>(null)
  const [chatError, setChatError] = useState('')
  const [chatLoading, setChatLoading] = useState(false)
  const toast = useToast()

  const checkService = async () => {
    setServiceStatus('checking')
    setServiceDetail('')
    try {
      const data = await llmApi.checkServiceHealth()
      const isHealthy =
        data?.status === 'healthy' ||
        data?.['healthy_endpoints'] !== undefined ||
        (typeof data === 'object' && data !== null)
      setServiceStatus(isHealthy ? 'healthy' : 'unhealthy')
      setServiceDetail(
        isHealthy
          ? `Connected${data?.['healthy_count'] !== undefined ? ` \u00b7 ${data['healthy_count']} healthy endpoint(s)` : ''}`
          : JSON.stringify(data).slice(0, 120)
      )
    } catch (err: unknown) {
      setServiceStatus('unhealthy')
      setServiceDetail(err instanceof Error ? err.message : 'Could not reach LiteLLM service')
    }
  }

  useEffect(() => {
    setLoading(true)
    llmApi.getModels()
      .then(setModels)
      .catch(() => { setError(true) })
      .finally(() => setLoading(false))
    checkService()
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

  const handleChatTest = async () => {
    if (!chatModel || !chatMessage.trim()) return
    setChatLoading(true)
    setChatResult(null)
    setChatError('')
    try {
      const result = await llmApi.chatTest(chatModel, chatMessage)
      setChatResult(result)
    } catch (err: unknown) {
      setChatError(err instanceof Error ? err.message : 'Chat request failed')
    } finally {
      setChatLoading(false)
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

  const serviceBadge = () => {
    const map: Record<string, { bg: string; color: string; label: string }> = {
      idle: { bg: '#e6e1dc', color: '#5c5752', label: 'Not checked' },
      checking: { bg: '#dce1e5', color: '#2a3640', label: 'Checking...' },
      healthy: { bg: '#dce5dc', color: '#2a402a', label: 'Healthy' },
      unhealthy: { bg: '#e5dcdc', color: '#402a2a', label: 'Unhealthy' },
    }
    const s = map[serviceStatus]
    return (
      <span style={{
        padding: '3px 10px',
        borderRadius: '999px',
        backgroundColor: s.bg,
        color: s.color,
        fontSize: '0.75rem',
        fontWeight: 600,
      }}>
        {s.label}
      </span>
    )
  }

  const renderServicePanel = () => (
    <Card>
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '4px 0',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <span style={{ fontSize: '0.8rem', fontWeight: 600 }}>LiteLLM Service</span>
          {serviceBadge()}
          {serviceDetail && (
            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
              {serviceDetail}
            </span>
          )}
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={checkService}
          disabled={serviceStatus === 'checking'}
        >
          Test Connection
        </Button>
      </div>
    </Card>
  )

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
                    <Button variant="ghost" size="sm" onClick={() => {
                      setChatModel(m.id)
                      setChatResult(null)
                      setChatError('')
                    }}>
                      Send Test
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => handleHealth(m.id)}>
                      Health
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

      {renderServicePanel()}
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
          <div>
            <FieldLabel label="Name" field="name" />
            <Input
              value={form.name ?? ''}
              onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
              placeholder="gpt-4o"
            />
          </div>
          <div>
            <FieldLabel label="Provider" field="provider" />
            <Input
              value={form.provider ?? ''}
              onChange={(e) => setForm((p) => ({ ...p, provider: e.target.value }))}
              placeholder="openai"
            />
          </div>
          <div>
            <FieldLabel label="API Key" field="apiKey" />
            <Input
              type="password"
              value={form.apiKey ?? ''}
              onChange={(e) => setForm((p) => ({ ...p, apiKey: e.target.value }))}
              placeholder="sk-..."
            />
          </div>
          <div>
            <FieldLabel label="Base URL" field="baseUrl" />
            <Input
              value={form.baseUrl ?? ''}
              onChange={(e) => setForm((p) => ({ ...p, baseUrl: e.target.value }))}
              placeholder="https://api.openai.com/v1 (leave empty for default)"
            />
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
            <div>
              <FieldLabel label="RPM" field="rpm" />
              <Input
                type="number"
                value={form.rpm ?? 0}
                onChange={(e) => setForm((p) => ({ ...p, rpm: parseInt(e.target.value) }))}
              />
            </div>
            <div>
              <FieldLabel label="TPM" field="tpm" />
              <Input
                type="number"
                value={form.tpm ?? 0}
                onChange={(e) => setForm((p) => ({ ...p, tpm: parseInt(e.target.value) }))}
              />
            </div>
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

      {/* Chat Test Modal */}
      <Modal
        open={chatModel !== null}
        onClose={() => setChatModel(null)}
        title={`Test Model: ${chatModel ?? ''}`}
        width={520}
        footer={
          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setChatModel(null)}>Close</Button>
            <Button onClick={handleChatTest} loading={chatLoading}>Send</Button>
          </div>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '6px' }}>
              Send a test message to verify the full chain: Backend &rarr; LiteLLM &rarr; Provider &rarr; Response
            </div>
            <Input
              label="Message"
              value={chatMessage}
              onChange={(e) => setChatMessage(e.target.value)}
              placeholder='Say "hello" in one sentence.'
            />
          </div>

          {chatError && (
            <div style={{
              padding: '10px 14px',
              backgroundColor: '#e5dcdc',
              borderRadius: '6px',
              fontSize: '0.8rem',
              color: '#402a2a',
              wordBreak: 'break-word',
            }}>
              {chatError}
            </div>
          )}

          {chatResult && (
            <div style={{
              padding: '12px 14px',
              backgroundColor: '#dce5dc',
              borderRadius: '6px',
              fontSize: '0.8rem',
              display: 'flex',
              flexDirection: 'column',
              gap: '8px',
            }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', color: '#5c6e5c' }}>
                <span>Model: {chatResult.model}</span>
                <span>{chatResult.latencyMs}ms</span>
              </div>
              <div style={{ color: '#2a402a', whiteSpace: 'pre-wrap', lineHeight: 1.5 }}>
                {chatResult.reply}
              </div>
            </div>
          )}
        </div>
      </Modal>
    </div>
  )
}
