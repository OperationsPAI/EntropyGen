import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { agentsApi } from '../../api/agents'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import type { Agent, AgentRole, AgentPhase } from '../../types/agent'

const MOCK_AGENTS: Agent[] = [
  { name: 'observer-1', spec: { role: 'observer', soul: '# Observer\nMonitor and report.', llm: { model: 'gpt-4o', temperature: 0.7, maxTokens: 4096 }, cron: { schedule: '*/5 * * * *', prompt: 'Check open issues' }, resources: { cpuRequest: '100m', cpuLimit: '500m', memoryRequest: '256Mi', memoryLimit: '1Gi', workspaceSize: '5Gi' }, gitea: { repo: 'ai-team/webapp', permissions: ['read'] } }, status: { phase: 'Running', conditions: [{ type: 'Ready', status: 'True' }], lastAction: { description: 'Scanned 3 open issues', timestamp: new Date().toISOString() }, tokenUsage: { today: 12400, total: 98000 }, podName: 'observer-1-7d9f', createdAt: new Date(Date.now() - 86400000 * 3).toISOString(), giteaUsername: 'observer-1' } },
  { name: 'developer-1', spec: { role: 'developer', soul: '# Developer\nImplement features.', llm: { model: 'gpt-4o', temperature: 0.5, maxTokens: 8192 }, cron: { schedule: '*/10 * * * *', prompt: 'Check assigned issues and implement' }, resources: { cpuRequest: '200m', cpuLimit: '1000m', memoryRequest: '512Mi', memoryLimit: '2Gi', workspaceSize: '10Gi' }, gitea: { repo: 'ai-team/webapp', permissions: ['read', 'write'] } }, status: { phase: 'Error', conditions: [{ type: 'Ready', status: 'False', reason: 'CrashLoop', message: 'Pod restarted 6 times' }], lastAction: { description: 'Pushed feat/auth', timestamp: new Date().toISOString() }, tokenUsage: { today: 45200, total: 312000 }, podName: 'developer-1-abc2', createdAt: new Date(Date.now() - 86400000 * 7).toISOString(), giteaUsername: 'developer-1' } },
]

const ROLES: AgentRole[] = ['observer', 'developer', 'reviewer', 'sre']
const PHASES: AgentPhase[] = ['Pending', 'Initializing', 'Running', 'Paused', 'Error']

function getAge(createdAt: string) {
  const days = Math.floor((Date.now() - new Date(createdAt).getTime()) / 86400000)
  return days === 0 ? 'today' : `${days}d ago`
}

export default function AgentList() {
  const navigate = useNavigate()
  const [agents, setAgents] = useState<Agent[]>(MOCK_AGENTS)
  const [roleFilter, setRoleFilter] = useState('')
  const [phaseFilter, setPhaseFilter] = useState('')
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Agent | null>(null)
  const [deleteInput, setDeleteInput] = useState('')
  const [step, setStep] = useState(1)
  const [form, setForm] = useState({
    name: '', role: 'observer' as AgentRole,
    model: 'gpt-4o', temperature: 0.7, maxTokens: 4096,
    schedule: '*/5 * * * *', prompt: '',
    cpuRequest: '100m', cpuLimit: '500m',
    memoryRequest: '256Mi', memoryLimit: '1Gi',
    workspaceSize: '5Gi', repo: '',
    permissions: ['read'] as ('read' | 'write' | 'review' | 'merge')[],
  })

  useEffect(() => { agentsApi.getAgents().then(setAgents).catch(() => {}) }, [])

  const filtered = agents.filter((a) => (!roleFilter || a.spec.role === roleFilter) && (!phaseFilter || a.status.phase === phaseFilter))

  const handleTogglePause = async (agent: Agent) => {
    const fn = agent.status.phase === 'Paused' ? agentsApi.resumeAgent : agentsApi.pauseAgent
    await fn(agent.name).catch(() => {})
    setAgents((prev) => prev.map((a) => a.name === agent.name ? { ...a, status: { ...a.status, phase: a.status.phase === 'Paused' ? 'Running' as AgentPhase : 'Paused' as AgentPhase } } : a))
  }

  const handleDelete = async () => {
    if (!deleteTarget || deleteInput !== deleteTarget.name) return
    await agentsApi.deleteAgent(deleteTarget.name).catch(() => {})
    setAgents((prev) => prev.filter((a) => a.name !== deleteTarget.name))
    setDeleteTarget(null)
    setDeleteInput('')
  }

  const sel = (label: string, value: string, onChange: (v: string) => void, opts: string[]) => (
    <select value={value} onChange={(e) => onChange(e.target.value)} style={{ padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--line-subtle)', fontSize: '0.85rem', fontFamily: 'inherit', backgroundColor: 'white', cursor: 'pointer' }}>
      <option value="">{label}</option>
      {opts.map((o) => <option key={o} value={o}>{o}</option>)}
    </select>
  )

  const labelStyle = { fontSize: '0.75rem', fontWeight: 600 as const, textTransform: 'uppercase' as const, letterSpacing: '0.05em', color: 'var(--text-muted)', display: 'block', marginBottom: '6px' }
  const inputStyle = { width: '100%', padding: '10px 14px', border: '1px solid var(--line-subtle)', borderRadius: '8px', fontSize: '0.9rem', fontFamily: 'inherit' }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ display: 'flex', gap: '8px' }}>
          {sel('All Roles', roleFilter, setRoleFilter, ROLES)}
          {sel('All Status', phaseFilter, setPhaseFilter, PHASES)}
        </div>
        <button onClick={() => setDrawerOpen(true)} style={{ padding: '10px 20px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', borderRadius: '8px', fontWeight: 600, fontSize: '0.875rem', cursor: 'pointer', fontFamily: 'inherit' }}>
          + New Agent
        </button>
      </div>

      <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', padding: '24px' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--line-subtle)' }}>
              {['Name', 'Role', 'Status', 'Model', 'Last Action', 'Token/Today', 'Age', 'Actions'].map((h) => (
                <th key={h} style={{ textAlign: 'left', padding: '8px 12px 12px', color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.03em' }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.map((agent) => (
              <tr key={agent.name} style={{ borderBottom: '1px solid var(--line-subtle)', backgroundColor: agent.status.phase === 'Error' ? 'rgba(229,80,43,0.04)' : undefined }}>
                <td style={{ padding: '12px', fontWeight: 600 }}>{agent.name}</td>
                <td style={{ padding: '12px', color: 'var(--text-muted)' }}>{agent.spec.role}</td>
                <td style={{ padding: '12px' }}><AgentPhaseTag phase={agent.status.phase} /></td>
                <td style={{ padding: '12px', color: 'var(--text-muted)' }}>{agent.spec.llm.model}</td>
                <td style={{ padding: '12px', color: 'var(--text-muted)', maxWidth: '180px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{agent.status.lastAction?.description ?? '—'}</td>
                <td style={{ padding: '12px', fontVariantNumeric: 'tabular-nums' }}>{(agent.status.tokenUsage?.today ?? 0).toLocaleString()}</td>
                <td style={{ padding: '12px', color: 'var(--text-muted)' }}>{getAge(agent.status.createdAt)}</td>
                <td style={{ padding: '12px' }}>
                  <div style={{ display: 'flex', gap: '6px' }}>
                    <button onClick={() => handleTogglePause(agent)} style={{ padding: '5px 10px', borderRadius: '6px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontSize: '0.8rem', fontFamily: 'inherit' }}>{agent.status.phase === 'Paused' ? 'Resume' : 'Pause'}</button>
                    <button onClick={() => navigate(`/agents/${agent.name}`)} style={{ padding: '5px 10px', borderRadius: '6px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontSize: '0.8rem', fontFamily: 'inherit' }}>Detail</button>
                    <button onClick={() => setDeleteTarget(agent)} style={{ padding: '5px 10px', borderRadius: '6px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontSize: '0.8rem', color: '#e5502b', fontFamily: 'inherit' }}>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {deleteTarget && (
        <div style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 200 }}>
          <div style={{ backgroundColor: 'white', borderRadius: '24px', padding: '32px', width: '400px' }}>
            <h3 style={{ marginBottom: '8px' }}>Delete Agent</h3>
            <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginBottom: '20px' }}>This action is irreversible. Type <strong>{deleteTarget.name}</strong> to confirm.</p>
            <input value={deleteInput} onChange={(e) => setDeleteInput(e.target.value)} placeholder={deleteTarget.name} style={{ ...inputStyle, marginBottom: '16px' }} />
            <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
              <button onClick={() => { setDeleteTarget(null); setDeleteInput('') }} style={{ padding: '10px 20px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit' }}>Cancel</button>
              <button onClick={handleDelete} disabled={deleteInput !== deleteTarget.name} style={{ padding: '10px 20px', borderRadius: '8px', backgroundColor: deleteInput === deleteTarget.name ? '#e5502b' : '#ccc', color: 'white', border: 'none', cursor: deleteInput === deleteTarget.name ? 'pointer' : 'not-allowed', fontFamily: 'inherit', fontWeight: 600 }}>Confirm Delete</button>
            </div>
          </div>
        </div>
      )}

      {drawerOpen && (
        <div style={{ position: 'fixed', right: 0, top: 0, bottom: 0, width: '480px', backgroundColor: 'white', boxShadow: '-4px 0 24px rgba(0,0,0,0.1)', zIndex: 200, display: 'flex', flexDirection: 'column' }}>
          <div style={{ padding: '24px', borderBottom: '1px solid var(--line-subtle)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <div style={{ fontWeight: 600, fontSize: '1rem' }}>New Agent</div>
              <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>Step {step}/3</div>
            </div>
            <button onClick={() => { setDrawerOpen(false); setStep(1) }} style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '1.3rem', color: 'var(--text-muted)' }}>×</button>
          </div>
          <div style={{ padding: '16px 24px 0', display: 'flex', gap: '8px' }}>
            {[1, 2, 3].map((s) => <div key={s} style={{ flex: 1, height: '3px', borderRadius: '2px', backgroundColor: s <= step ? 'var(--text-main)' : 'var(--line-subtle)' }} />)}
          </div>
          <div style={{ flex: 1, overflowY: 'auto', padding: '24px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
            {step === 1 && (
              <>
                <label><span style={labelStyle}>Name</span><input value={form.name} onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))} style={inputStyle} /></label>
                <label><span style={labelStyle}>Role</span>
                  <select value={form.role} onChange={(e) => setForm((p) => ({ ...p, role: e.target.value as AgentRole }))} style={{ ...inputStyle, backgroundColor: 'white' }}>
                    {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
                  </select>
                </label>
                <label><span style={labelStyle}>LLM Model</span><input value={form.model} onChange={(e) => setForm((p) => ({ ...p, model: e.target.value }))} style={inputStyle} /></label>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                  <label><span style={labelStyle}>Temperature</span><input type="number" min={0} max={2} step={0.1} value={form.temperature} onChange={(e) => setForm((p) => ({ ...p, temperature: parseFloat(e.target.value) }))} style={inputStyle} /></label>
                  <label><span style={labelStyle}>Max Tokens</span><input type="number" value={form.maxTokens} onChange={(e) => setForm((p) => ({ ...p, maxTokens: parseInt(e.target.value) }))} style={inputStyle} /></label>
                </div>
              </>
            )}
            {step === 2 && (
              <>
                <label>
                  <span style={labelStyle}>Cron Expression</span>
                  <input value={form.schedule} onChange={(e) => setForm((p) => ({ ...p, schedule: e.target.value }))} style={inputStyle} />
                </label>
                <label><span style={labelStyle}>Trigger Prompt</span><textarea value={form.prompt} onChange={(e) => setForm((p) => ({ ...p, prompt: e.target.value }))} rows={5} style={{ ...inputStyle, resize: 'vertical' }} /></label>
              </>
            )}
            {step === 3 && (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                  {(['cpuRequest', 'cpuLimit', 'memoryRequest', 'memoryLimit'] as const).map((k) => (
                    <label key={k}><span style={labelStyle}>{k}</span><input value={form[k]} onChange={(e) => setForm((p) => ({ ...p, [k]: e.target.value }))} style={inputStyle} /></label>
                  ))}
                </div>
                <label><span style={labelStyle}>Gitea Repo</span><input value={form.repo} onChange={(e) => setForm((p) => ({ ...p, repo: e.target.value }))} style={inputStyle} /></label>
                <div>
                  <span style={labelStyle}>Gitea Permissions</span>
                  <div style={{ display: 'flex', gap: '12px', marginTop: '8px' }}>
                    {(['read', 'write', 'review', 'merge'] as const).map((p) => (
                      <label key={p} style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.875rem', cursor: 'pointer' }}>
                        <input type="checkbox" checked={form.permissions.includes(p)} onChange={(e) => setForm((prev) => ({ ...prev, permissions: e.target.checked ? [...prev.permissions, p] : prev.permissions.filter((x) => x !== p) }))} />
                        {p}
                      </label>
                    ))}
                  </div>
                </div>
              </>
            )}
          </div>
          <div style={{ padding: '20px 24px', borderTop: '1px solid var(--line-subtle)', display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            <button onClick={() => { setDrawerOpen(false); setStep(1) }} style={{ padding: '10px 20px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit' }}>Cancel</button>
            {step > 1 && <button onClick={() => setStep((s) => s - 1)} style={{ padding: '10px 20px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit' }}>Back</button>}
            {step < 3 ? (
              <button onClick={() => setStep((s) => s + 1)} style={{ padding: '10px 20px', borderRadius: '8px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', cursor: 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>Next</button>
            ) : (
              <button onClick={async () => {
                await agentsApi.createAgent({ name: form.name, spec: { role: form.role, llm: { model: form.model, temperature: form.temperature, maxTokens: form.maxTokens }, cron: { schedule: form.schedule, prompt: form.prompt }, resources: { cpuRequest: form.cpuRequest, cpuLimit: form.cpuLimit, memoryRequest: form.memoryRequest, memoryLimit: form.memoryLimit, workspaceSize: form.workspaceSize }, gitea: { repo: form.repo, permissions: form.permissions } } }).catch(() => {})
                setDrawerOpen(false); setStep(1)
              }} style={{ padding: '10px 20px', borderRadius: '8px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', cursor: 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>Create Agent</button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
