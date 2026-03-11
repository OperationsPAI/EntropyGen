import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { agentsApi } from '../../api/agents'
import { auditApi } from '../../api/audit'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import MonacoEditor from '../../components/editor/MonacoEditor'
import TraceDetailPanel from '../../components/trace/TraceDetailPanel'
import type { Agent } from '../../types/agent'
import type { AuditTrace } from '../../types/trace'

const TABS = ['Overview', 'Activity Timeline', 'Config', 'Logs', 'Audit']

const MOCK_AGENT: Agent = {
  name: 'developer-1',
  spec: { role: 'developer', soul: '# Developer Agent\n\nYou are a senior developer.\n\n## Responsibilities\n- Implement assigned issues\n- Create PRs\n- Write tests', llm: { model: 'gpt-4o', temperature: 0.5, maxTokens: 8192 }, cron: { schedule: '*/10 * * * *', prompt: 'Check open issues assigned to you and implement them.' }, resources: { cpuRequest: '200m', cpuLimit: '1000m', memoryRequest: '512Mi', memoryLimit: '2Gi', workspaceSize: '10Gi' }, gitea: { repo: 'ai-team/webapp', permissions: ['read', 'write'] } },
  status: { phase: 'Error', conditions: [{ type: 'Ready', status: 'False', reason: 'CrashLoop', message: 'Back-off restarting failed container developer-1' }, { type: 'GiteaUserCreated', status: 'True' }], lastAction: { description: 'Pushed feat/auth branch', timestamp: new Date().toISOString() }, tokenUsage: { today: 45200, total: 312000 }, podName: 'developer-1-abc2', createdAt: new Date(Date.now() - 86400000 * 7).toISOString(), giteaUsername: 'developer-1' },
}

const MOCK_TIMELINE = [
  { type: 'llm', time: '13:05', title: 'LLM Inference', detail: '1200 → 350 tokens · gpt-4o · 2.1s' },
  { type: 'git_push', time: '13:04', title: 'Git Push', detail: 'feat/auth · 3 commits' },
  { type: 'pr', time: '13:03', title: 'PR #8 Created', detail: 'Add authentication · feat/auth → main' },
  { type: 'issue_comment', time: '13:01', title: 'Issue #15 Comment', detail: '"I\'m working on this"' },
  { type: 'cron', time: '12:50', title: 'Cron Triggered', detail: 'Check open issues' },
]

const ICON_MAP: Record<string, string> = { llm: '🧠', git_push: '📤', git_clone: '📥', pr: '🔀', issue_comment: '💬', issue: '📋', cron: '⏰', alert: '⚠️' }

export default function AgentDetail() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const [agent, setAgent] = useState<Agent>(MOCK_AGENT)
  const [activeTab, setActiveTab] = useState(0)
  const [soul, setSoul] = useState(MOCK_AGENT.spec.soul)
  const [soulConfirm, setSoulConfirm] = useState(false)
  const [traces, setTraces] = useState<AuditTrace[]>([])
  const [selectedTrace, setSelectedTrace] = useState<AuditTrace | null>(null)
  const [logs, setLogs] = useState('')
  const [logTab, setLogTab] = useState<'events' | 'pod'>('events')
  const [assignModal, setAssignModal] = useState(false)
  const [assignForm, setAssignForm] = useState({ title: '', description: '', priority: 'medium' as 'low' | 'medium' | 'high' })

  useEffect(() => {
    if (!name) return
    agentsApi.getAgent(name).then((a) => { setAgent(a); setSoul(a.spec.soul) }).catch(() => {})
    auditApi.getTraces({ agent_id: [name], limit: 20 }).then((r) => setTraces(r.items)).catch(() => {})
    agentsApi.getAgentLogs(name).then(setLogs).catch(() => {})
  }, [name])

  const labelStyle = { fontSize: '0.7rem', textTransform: 'uppercase' as const, letterSpacing: '0.05em', color: 'var(--text-muted)', fontWeight: 600 }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        <button onClick={() => navigate('/agents')} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-muted)', fontSize: '0.875rem', fontFamily: 'inherit' }}>← Agents</button>
        <span style={{ color: 'var(--line-subtle)' }}>|</span>
        <span style={{ fontWeight: 600 }}>{name}</span>
        <AgentPhaseTag phase={agent.status.phase} />
      </div>

      <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', overflow: 'hidden' }}>
        <div style={{ display: 'flex', borderBottom: '1px solid var(--line-subtle)' }}>
          {TABS.map((tab, i) => (
            <button key={tab} onClick={() => setActiveTab(i)} style={{ padding: '16px 24px', background: 'none', border: 'none', cursor: 'pointer', fontFamily: 'inherit', fontSize: '0.875rem', fontWeight: activeTab === i ? 600 : 500, color: activeTab === i ? 'var(--text-main)' : 'var(--text-muted)', borderBottom: activeTab === i ? '2px solid var(--text-main)' : '2px solid transparent', marginBottom: '-1px' }}>{tab}</button>
          ))}
        </div>

        <div style={{ padding: '24px' }}>
          {activeTab === 0 && (
            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: '24px' }}>
              <div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '24px' }}>
                  {[['Role', agent.spec.role], ['Model', agent.spec.llm.model], ['Cron', agent.spec.cron.schedule], ['Gitea User', agent.status.giteaUsername ?? '—'], ['Phase', agent.status.phase], ['Pod', agent.status.podName ?? '—']].map(([k, v]) => (
                    <div key={k}>
                      <div style={labelStyle}>{k}</div>
                      <div style={{ fontWeight: 500, marginTop: '2px' }}>{v}</div>
                    </div>
                  ))}
                </div>
                <div style={labelStyle}>Conditions</div>
                <div style={{ marginTop: '8px', display: 'flex', flexDirection: 'column', gap: '6px' }}>
                  {(agent.status.conditions ?? []).map((c, i) => (
                    <div key={i} style={{ display: 'flex', gap: '8px', fontSize: '0.85rem', padding: '8px 12px', borderRadius: '8px', backgroundColor: c.status === 'True' ? '#dce5dc' : '#e5dcdc' }}>
                      <span>{c.status === 'True' ? '✓' : '✗'}</span>
                      <span style={{ fontWeight: 600 }}>{c.type}</span>
                      {c.message && <span style={{ color: 'var(--text-muted)' }}>{c.message}</span>}
                    </div>
                  ))}
                </div>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
                <button onClick={() => setAssignModal(true)} style={{ padding: '10px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>Assign Task</button>
                <button onClick={() => agentsApi.pauseAgent(agent.name).catch(() => {})} style={{ padding: '10px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit' }}>{agent.status.phase === 'Paused' ? 'Resume' : 'Pause'}</button>
                <button onClick={() => agentsApi.resetMemory(agent.name).catch(() => {})} style={{ padding: '10px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit' }}>Reset Memory</button>
                <div style={{ marginTop: '8px', borderTop: '1px solid var(--line-subtle)', paddingTop: '16px' }}>
                  <div style={labelStyle}>Today's Tokens</div>
                  <div style={{ fontSize: '1.5rem', fontWeight: 600, letterSpacing: '-0.04em', fontVariantNumeric: 'tabular-nums', marginTop: '4px' }}>{(agent.status.tokenUsage?.today ?? 0).toLocaleString()}</div>
                </div>
              </div>
            </div>
          )}

          {activeTab === 1 && (
            <div style={{ display: 'flex', flexDirection: 'column' }}>
              {MOCK_TIMELINE.map((item, i) => (
                <div key={i} style={{ display: 'flex', gap: '16px', paddingBottom: '20px' }}>
                  {item.type === 'cron' ? (
                    <div style={{ width: '100%', padding: '8px 16px', backgroundColor: '#e8f0fa', borderRadius: '8px', display: 'flex', gap: '12px', alignItems: 'center' }}>
                      <span>{ICON_MAP[item.type]}</span>
                      <span style={{ fontWeight: 600, fontSize: '0.85rem' }}>{item.time}</span>
                      <span style={{ fontSize: '0.85rem' }}>{item.title}</span>
                      <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>{item.detail}</span>
                    </div>
                  ) : (
                    <>
                      <div style={{ flexShrink: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '4px' }}>
                        <div style={{ width: '28px', height: '28px', borderRadius: '50%', backgroundColor: 'var(--bg-canvas)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '0.9rem' }}>{ICON_MAP[item.type] ?? '●'}</div>
                        {i < MOCK_TIMELINE.length - 1 && <div style={{ width: '1px', flex: 1, backgroundColor: 'var(--line-subtle)', minHeight: '16px' }} />}
                      </div>
                      <div style={{ paddingTop: '4px' }}>
                        <div style={{ display: 'flex', gap: '8px', alignItems: 'baseline' }}>
                          <span style={{ color: 'var(--text-muted)', fontSize: '0.8rem' }}>{item.time}</span>
                          <span style={{ fontWeight: 600, fontSize: '0.875rem' }}>{item.title}</span>
                        </div>
                        <div style={{ color: 'var(--text-muted)', fontSize: '0.8rem', marginTop: '2px' }}>{item.detail}</div>
                      </div>
                    </>
                  )}
                </div>
              ))}
            </div>
          )}

          {activeTab === 2 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                  <div style={{ fontWeight: 600 }}>SOUL.md</div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--accent-orange)' }}>⚠ Changes will trigger a Pod restart</div>
                </div>
                <div style={{ borderRadius: '12px', overflow: 'hidden', border: '1px solid var(--line-subtle)' }}>
                  <MonacoEditor value={soul} onChange={setSoul} language="markdown" height="300px" />
                </div>
                <button onClick={() => setSoulConfirm(true)} style={{ marginTop: '12px', padding: '10px 20px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', borderRadius: '8px', cursor: 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>Save SOUL.md</button>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '12px' }}>
                {[['Model', agent.spec.llm.model], ['Temperature', String(agent.spec.llm.temperature)], ['Max Tokens', String(agent.spec.llm.maxTokens)]].map(([k, v]) => (
                  <div key={k}>
                    <div style={labelStyle}>{k}</div>
                    <div style={{ fontWeight: 500, marginTop: '4px' }}>{v}</div>
                  </div>
                ))}
              </div>
              <div>
                <div style={labelStyle}>Cron Expression</div>
                <div style={{ fontWeight: 500, marginTop: '4px', fontFamily: 'monospace' }}>{agent.spec.cron.schedule}</div>
              </div>
            </div>
          )}

          {activeTab === 3 && (
            <div>
              <div style={{ display: 'flex', gap: '8px', marginBottom: '16px' }}>
                {(['events', 'pod'] as const).map((t) => (
                  <button key={t} onClick={() => setLogTab(t)} style={{ padding: '8px 16px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: logTab === t ? 'var(--text-main)' : 'none', color: logTab === t ? 'white' : 'var(--text-muted)', cursor: 'pointer', fontFamily: 'inherit', fontSize: '0.875rem' }}>
                    {t === 'events' ? 'Live Event Stream' : 'Pod stdout'}
                  </button>
                ))}
              </div>
              {logTab === 'events' ? (
                <div style={{ backgroundColor: '#111', color: '#ccc', borderRadius: '12px', padding: '16px', fontFamily: 'monospace', fontSize: '0.8rem', minHeight: '200px' }}>
                  <div style={{ color: '#4a9', marginBottom: '8px' }}>● Connecting...</div>
                  <div>Waiting for events...</div>
                </div>
              ) : (
                <div style={{ backgroundColor: '#111', color: '#ccc', borderRadius: '12px', padding: '16px', fontFamily: 'monospace', fontSize: '0.8rem', minHeight: '200px', whiteSpace: 'pre-wrap' }}>
                  {logs || '[INFO] Waiting for logs…'}
                </div>
              )}
            </div>
          )}

          {activeTab === 4 && (
            <div style={{ position: 'relative' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--line-subtle)' }}>
                    {['Trace ID', 'Type', 'Path', 'Status', 'Token', 'Latency', 'Time'].map((h) => (
                      <th key={h} style={{ textAlign: 'left', padding: '8px 12px 12px', color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.03em' }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {traces.length === 0 ? (
                    <tr><td colSpan={7} style={{ padding: '20px', color: 'var(--text-muted)', textAlign: 'center' }}>No audit records</td></tr>
                  ) : traces.map((t) => (
                    <tr key={t.trace_id} onClick={() => setSelectedTrace(t)} style={{ borderBottom: '1px solid var(--line-subtle)', cursor: 'pointer' }}>
                      <td style={{ padding: '10px 12px', fontFamily: 'monospace', fontSize: '0.75rem' }}>{t.trace_id.slice(0, 8)}…</td>
                      <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{t.request_type}</td>
                      <td style={{ padding: '10px 12px', maxWidth: '150px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.path}</td>
                      <td style={{ padding: '10px 12px' }}><span style={{ color: t.status_code >= 400 ? '#e5502b' : '#2a402a' }}>{t.status_code}</span></td>
                      <td style={{ padding: '10px 12px' }}>{t.tokens_in != null ? `${t.tokens_in}+${t.tokens_out}` : '—'}</td>
                      <td style={{ padding: '10px 12px' }}>{t.latency_ms}ms</td>
                      <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{new Date(t.created_at).toLocaleTimeString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <TraceDetailPanel trace={selectedTrace} onClose={() => setSelectedTrace(null)} />
            </div>
          )}
        </div>
      </div>

      {soulConfirm && (
        <div style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 200 }}>
          <div style={{ backgroundColor: 'white', borderRadius: '24px', padding: '32px', width: '400px' }}>
            <h3 style={{ marginBottom: '8px' }}>Confirm Save SOUL.md</h3>
            <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginBottom: '24px' }}>Saving will trigger a rolling restart. Agent will be temporarily unavailable.</p>
            <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
              <button onClick={() => setSoulConfirm(false)} style={{ padding: '10px 20px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit' }}>Cancel</button>
              <button onClick={async () => { await agentsApi.updateAgent(agent.name, { spec: { soul } }).catch(() => {}); setSoulConfirm(false) }} style={{ padding: '10px 20px', borderRadius: '8px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', cursor: 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>Confirm Save</button>
            </div>
          </div>
        </div>
      )}

      {assignModal && (
        <div style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 200 }}>
          <div style={{ backgroundColor: 'white', borderRadius: '24px', padding: '32px', width: '480px' }}>
            <h3 style={{ marginBottom: '20px' }}>Assign Task to {name}</h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <label><span style={labelStyle}>Title</span><input value={assignForm.title} onChange={(e) => setAssignForm((p) => ({ ...p, title: e.target.value }))} style={{ width: '100%', marginTop: '6px', padding: '10px 14px', border: '1px solid var(--line-subtle)', borderRadius: '8px', fontSize: '0.9rem', fontFamily: 'inherit' }} /></label>
              <label><span style={labelStyle}>Description</span><textarea value={assignForm.description} onChange={(e) => setAssignForm((p) => ({ ...p, description: e.target.value }))} rows={4} style={{ width: '100%', marginTop: '6px', padding: '10px 14px', border: '1px solid var(--line-subtle)', borderRadius: '8px', fontSize: '0.9rem', fontFamily: 'inherit', resize: 'vertical' }} /></label>
              <div>
                <span style={labelStyle}>Priority</span>
                <div style={{ display: 'flex', gap: '12px', marginTop: '8px' }}>
                  {(['low', 'medium', 'high'] as const).map((p) => (
                    <label key={p} style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer', fontSize: '0.875rem' }}>
                      <input type="radio" checked={assignForm.priority === p} onChange={() => setAssignForm((prev) => ({ ...prev, priority: p }))} />
                      {p}
                    </label>
                  ))}
                </div>
              </div>
            </div>
            <div style={{ marginTop: '20px', padding: '12px', backgroundColor: '#f5f5f5', borderRadius: '8px', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
              ℹ Creates a Gitea issue and assigns it to @{agent.status.giteaUsername}
            </div>
            <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', marginTop: '20px' }}>
              <button onClick={() => setAssignModal(false)} style={{ padding: '10px 20px', borderRadius: '8px', border: '1px solid var(--line-subtle)', background: 'none', cursor: 'pointer', fontFamily: 'inherit' }}>Cancel</button>
              <button onClick={async () => {
                if (!name) return
                await agentsApi.assignTask(name, { repo: agent.spec.gitea.repo, title: assignForm.title, description: assignForm.description, labels: [], priority: assignForm.priority }).catch(() => {})
                setAssignModal(false)
              }} style={{ padding: '10px 20px', borderRadius: '8px', backgroundColor: 'var(--text-main)', color: 'white', border: 'none', cursor: 'pointer', fontFamily: 'inherit', fontWeight: 600 }}>Create & Assign</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
