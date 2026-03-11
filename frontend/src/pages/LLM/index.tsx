import { useEffect, useState } from 'react'
import { llmApi, type LLMModel, type CreateModelDto } from '../../api/llm'

const MOCK_MODELS: LLMModel[] = [
  { id: '1', name: 'gpt-4o', provider: 'openai', rpm: 60, tpm: 100000, status: 'healthy' },
  { id: '2', name: 'claude-3-5-sonnet', provider: 'anthropic', rpm: 50, tpm: 80000, status: 'healthy' },
  { id: '3', name: 'gpt-4o-mini', provider: 'openai', rpm: 200, tpm: 200000, status: 'unknown' },
]

export default function LLMPage() {
  const [models, setModels] = useState<LLMModel[]>(MOCK_MODELS)
  const [modalOpen, setModalOpen] = useState(false)
  const [healthStatus, setHealthStatus] = useState<Record<string, string>>({})
  const [form, setForm] = useState<Partial<CreateModelDto>>({ provider: 'openai', rpm: 60, tpm: 100000 })

  useEffect(() => { llmApi.getModels().then(setModels).catch(() => {}) }, [])

  const handleHealth = async (id: string) => {
    setHealthStatus((p) => ({ ...p, [id]: 'checking…' }))
    const result = await llmApi.checkHealth(id).catch(() => ({ status: 'unhealthy' as const }))
    setHealthStatus((p) => ({ ...p, [id]: result.status }))
  }

  const statusBadge = (status: string) => {
    const map: Record<string, { bg: string; color: string }> = {
      healthy: { bg: '#dce5dc', color: '#2a402a' },
      unhealthy: { bg: '#e5dcdc', color: '#402a2a' },
      unknown: { bg: '#e6e1dc', color: '#5c5752' },
    }
    const s = map[status] ?? map.unknown
    return <span style={{ padding: '3px 10px', borderRadius: '999px', backgroundColor: s.bg, color: s.color, fontSize: '0.75rem', fontWeight: 600 }}>{status}</span>
  }

  const labelStyle = { fontSize: '0.75rem', fontWeight: 600 as const, textTransform: 'uppercase' as const, letterSpacing: '0.05em', color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }
  const inputStyle = { width: '100%', padding: '10px 14px', border: '1px solid var(--line-subtle)', borderRadius: '8px', fontSize: '0.9rem', fontFamily: 'inherit' }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2 style={{ fontSize: '1.1rem', fontWeight: 600 }}>LLM 模型配置</h2>
        <button onClick={() => setModalOpen(true)} style={{ padding: '10px 20px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', borderRadius: '8px', fontWeight: 600, fontSize: '0.875rem', cursor: 'pointer', fontFamily: 'inherit' }}>+ 新增模型</button>
      </div>

      <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', padding: '24px' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--line-subtle)' }}>
              {['模型名称', 'Provider', 'RPM', 'TPM', '状态', '操作'].map((h) => (
                <th key={h} style={{ textAlign: 'left', padding: '8px 12px 12px', color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.03em' }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {models.map((m) => (
              <tr key={m.id} style={{ borderBottom: '1px solid var(--line-subtle)' }}>
                <td style={{ padding: '12px', fontWeight: 600 }}>{m.name}</td>
                <td style={{ padding: '12px', color: 'var(--text-muted)' }}>{m.provider}</td>
                <td style={{ padding: '12px', fontVariantNumeric: 'tabular-nums' }}>{m.rpm}</td>
                <td style={{ padding: '12px', fontVariantNumeric: 'tabular-nums' }}>{m.tpm.toLocaleString()}</td>
                <td style={{ padding: '12px' }}>{statusBadge(healthStatus[m.id] ?? m.status)}</td>
                <td style={{ padding: '12px' }}>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <button onClick={() => handleHealth(m.id)} style={{ padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontSize: '0.8rem', fontFamily: 'inherit' }}>测试连通性</button>
                    <button onClick={() => llmApi.deleteModel(m.id).then(() => setModels((p) => p.filter((x) => x.id !== m.id))).catch(() => {})} style={{ padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontSize: '0.8rem', color: '#e5502b', fontFamily: 'inherit' }}>删除</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {modalOpen && (
        <div style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 200 }}>
          <div style={{ backgroundColor: 'white', borderRadius: '24px', padding: '32px', width: '440px' }}>
            <h3 style={{ marginBottom: '20px' }}>新增模型</h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              {(['name', 'provider', 'apiKey', 'baseUrl'] as const).map((k) => (
                <label key={k}>
                  <span style={labelStyle}>{k}</span>
                  <input value={String(form[k] ?? '')} onChange={(e) => setForm((p) => ({ ...p, [k]: e.target.value }))} type={k === 'apiKey' ? 'password' : 'text'} style={inputStyle} />
                </label>
              ))}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                {(['rpm', 'tpm'] as const).map((k) => (
                  <label key={k}>
                    <span style={labelStyle}>{k.toUpperCase()}</span>
                    <input type="number" value={form[k] ?? 0} onChange={(e) => setForm((p) => ({ ...p, [k]: parseInt(e.target.value) }))} style={inputStyle} />
                  </label>
                ))}
              </div>
            </div>
            <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', marginTop: '20px' }}>
              <button onClick={() => setModalOpen(false)} style={{ padding: '10px 20px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit' }}>取消</button>
              <button onClick={async () => {
                const m = await llmApi.createModel(form as CreateModelDto).catch(() => null)
                if (m) setModels((p) => [...p, m])
                setModalOpen(false)
              }} style={{ padding: '10px 20px', borderRadius: '8px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', cursor: 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>创建</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
