import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { IconCode, IconGithubLogo, IconComment, IconList, IconClock, IconAlertTriangle } from '@douyinfe/semi-icons'
import { agentsApi } from '../../api/agents'
import { rolesApi } from '../../api/roles'
import { auditApi } from '../../api/audit'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import FileEditor from '../../components/agent/FileEditor'
import TraceDetail from '../../components/trace/TraceDetail'
import { PageHeader, Card, Button, Modal, Input, Textarea, EmptyState, SplitPane } from '../../components/ui'
import { useToast } from '../../hooks/useToast'
import { usePlatformConfig } from '../../hooks/usePlatformConfig'
import { giteaRepoUrl, giteaIssuesUrl, giteaPRsUrl } from '../../utils/giteaLinks'
import type { Agent, RoleFile } from '../../types/agent'
import type { AgentFile } from '../../components/agent/FileEditor'
import type { AuditTrace } from '../../types/trace'
import styles from './Detail.module.css'

const TABS = ['Overview', 'Activity Timeline', 'Files', 'Logs', 'Audit']

interface TimelineItem {
  type: string
  time: string
  title: string
  detail: string
}

const ICON_MAP: Record<string, { icon: React.ReactNode; className: string }> = {
  llm: { icon: <IconCode size="small" />, className: styles.dotLlm },
  git_push: { icon: <IconGithubLogo size="small" />, className: styles.dotGit },
  git_clone: { icon: <IconGithubLogo size="small" />, className: styles.dotGit },
  pr: { icon: <IconGithubLogo size="small" />, className: styles.dotPr },
  issue_comment: { icon: <IconComment size="small" />, className: styles.dotComment },
  issue: { icon: <IconList size="small" />, className: styles.dotIssue },
  cron: { icon: <IconClock size="small" />, className: styles.dotDefault },
  alert: { icon: <IconAlertTriangle size="small" />, className: styles.dotAlert },
}

function getIconInfo(type: string) {
  return ICON_MAP[type] ?? { icon: <IconList size="small" />, className: styles.dotDefault }
}

function AgentDetailInner({ agent: initialAgent }: { agent: Agent }) {
  const toast = useToast()
  const config = usePlatformConfig()
  const [agent, setAgent] = useState(initialAgent)
  const [activeTab, setActiveTab] = useState(0)
  const [traces, setTraces] = useState<AuditTrace[]>([])
  const [selectedTrace, setSelectedTrace] = useState<AuditTrace | null>(null)
  const [logs, setLogs] = useState('')
  const [logTab, setLogTab] = useState<'events' | 'pod'>('events')
  const [timeline] = useState<TimelineItem[]>([])
  const [assignModal, setAssignModal] = useState(false)
  const [assignLoading, setAssignLoading] = useState(false)
  const [assignForm, setAssignForm] = useState({ title: '', description: '', priority: 'medium' as 'low' | 'medium' | 'high' })
  const [roleFiles, setRoleFiles] = useState<AgentFile[]>([])
  const [roleFilesActive, setRoleFilesActive] = useState('')

  // Edit settings state
  const [editModal, setEditModal] = useState(false)
  const [editSaving, setEditSaving] = useState(false)
  const [editForm, setEditForm] = useState({
    model: '',
    temperature: 0.7,
    maxTokens: 65536,
    runtimeType: 'openclaw',
    repo: '',
    permissions: ['read'] as ('read' | 'write' | 'review' | 'merge')[],
  })

  useEffect(() => {
    auditApi.getTraces({ agent_id: [agent.name], limit: 20 })
      .then((r) => setTraces(r.items))
      .catch(() => {})
    agentsApi.getAgentLogs(agent.name)
      .then(setLogs)
      .catch(() => {})
  }, [agent.name])

  useEffect(() => {
    if (!agent.spec.role) return
    rolesApi.getRole(agent.spec.role)
      .then((role) => {
        if (!role) return
        const files: AgentFile[] = (role.files ?? []).map((f: RoleFile) => ({
          name: f.name,
          language: 'markdown',
          content: f.content,
          readOnly: true,
          description: `Inherited from role: ${agent.spec.role}`,
        }))
        setRoleFiles(files)
        if (files.length > 0) setRoleFilesActive(files[0].name)
      })
      .catch(() => {})
  }, [agent.spec.role])

  const handleTogglePause = async () => {
    const isPaused = agent.status.phase === 'Paused'
    const fn = isPaused ? agentsApi.resumeAgent : agentsApi.pauseAgent
    try {
      await fn(agent.name)
      setAgent({ ...agent, status: { ...agent.status, phase: isPaused ? 'Running' : 'Paused' } })
      toast.success(isPaused ? 'Agent resumed' : 'Agent paused')
    } catch {
      toast.error('Operation failed', `Could not ${isPaused ? 'resume' : 'pause'} agent`)
    }
  }

  const handleResetMemory = async () => {
    try {
      await agentsApi.resetMemory(agent.name)
      toast.success('Memory reset', 'Agent memory has been cleared.')
    } catch {
      toast.error('Reset failed', 'Could not reset agent memory')
    }
  }

  const handleAssignTask = async () => {
    setAssignLoading(true)
    try {
      await agentsApi.assignTask(agent.name, {
        repo: agent.spec.gitea.repo,
        title: assignForm.title,
        description: assignForm.description,
        labels: [],
        priority: assignForm.priority,
      })
      toast.success('Task assigned', `Issue created and assigned to @${agent.status.giteaUsername}`)
      setAssignModal(false)
      setAssignForm({ title: '', description: '', priority: 'medium' })
    } catch {
      toast.error('Assign failed', 'Could not create the issue')
    } finally {
      setAssignLoading(false)
    }
  }

  const openEditModal = () => {
    setEditForm({
      model: agent.spec.llm.model,
      temperature: agent.spec.llm.temperature,
      maxTokens: agent.spec.llm.maxTokens,
      runtimeType: agent.spec.runtime?.type || 'openclaw',
      repo: agent.spec.gitea.repo,
      permissions: [...agent.spec.gitea.permissions],
    })
    setEditModal(true)
  }

  const toggleEditPermission = (perm: 'read' | 'write' | 'review' | 'merge') =>
    setEditForm((prev) => ({
      ...prev,
      permissions: prev.permissions.includes(perm)
        ? prev.permissions.filter((p) => p !== perm)
        : [...prev.permissions, perm],
    }))

  const handleSaveSettings = async () => {
    setEditSaving(true)
    try {
      const updated = await agentsApi.updateAgent(agent.name, {
        role: agent.spec.role,
        llm: { model: editForm.model, temperature: editForm.temperature, maxTokens: editForm.maxTokens },
        runtime: { type: editForm.runtimeType },
        resources: agent.spec.resources,
        gitea: { repo: editForm.repo, repos: editForm.repo ? [editForm.repo] : [], permissions: editForm.permissions },
      })
      setAgent(updated)
      setEditModal(false)
      toast.success('Settings saved', 'Agent configuration updated')
    } catch {
      toast.error('Save failed', 'Could not update agent settings')
    } finally {
      setEditSaving(false)
    }
  }

  const renderOverview = () => (
    <div className={styles.overviewGrid}>
      <div>
        <div className={styles.defGrid}>
          <div>
            <div className={styles.defLabel}>Role</div>
            <div className={styles.defValue}>
              <Link to={`/roles/${agent.spec.role}`} className={styles.roleLink}>
                {agent.spec.role}
              </Link>
            </div>
          </div>
          {([
            ['Model', agent.spec.llm.model],
            ['Temperature', String(agent.spec.llm.temperature)],
            ['Max Tokens', String(agent.spec.llm.maxTokens)],
            ['Runtime Type', agent.spec.runtime?.type || 'openclaw'],
            ['Repo', agent.spec.gitea.repo || '\u2014'],
            ['Permissions', agent.spec.gitea.permissions.join(', ') || '\u2014'],
            ['Gitea User', agent.status.giteaUsername ?? '\u2014'],
            ['Phase', agent.status.phase],
            ['Pod', agent.status.podName ?? '\u2014'],
          ] as const).map(([k, v]) => (
            <div key={k}>
              <div className={styles.defLabel}>{k}</div>
              <div className={styles.defValue}>{v}</div>
            </div>
          ))}
          {agent.status.currentTask && (() => {
            const task = agent.status.currentTask!
            const base = config?.gitea_base_url ?? ''
            const repo = task.repo || agent.spec.gitea.repo
            const path = task.type === 'pr' ? 'pulls' : 'issues'
            const url = base ? `${base}/${repo}/${path}/${task.number}` : ''
            return (
              <div>
                <div className={styles.defLabel}>Current Task</div>
                <div className={styles.defValue}>
                  {url ? (
                    <a href={url} target="_blank" rel="noopener noreferrer" className={styles.currentTaskLink}>
                      {task.type === 'pr' ? 'PR' : 'Issue'} #{task.number}
                      {task.title && ` · ${task.title}`}
                    </a>
                  ) : (
                    <span>{task.type === 'pr' ? 'PR' : 'Issue'} #{task.number}{task.title && ` · ${task.title}`}</span>
                  )}
                </div>
              </div>
            )
          })()}
        </div>
        <div className={styles.defLabel}>Conditions</div>
        <div className={styles.conditionList}>
          {(agent.status.conditions ?? []).map((c, i) => (
            <div key={i} className={`${styles.condition} ${c.status === 'True' ? styles.conditionOk : styles.conditionFail}`}>
              <span className={styles.conditionIcon}>{c.status === 'True' ? '\u2713' : '\u2717'}</span>
              <span className={styles.conditionType}>{c.type}</span>
              {c.message && <span className={styles.conditionMsg}>{c.message}</span>}
            </div>
          ))}
        </div>
      </div>
      <div className={styles.sidebar}>
        <Button variant="secondary" fullWidth onClick={() => setAssignModal(true)}>Assign Task</Button>
        <Button variant="secondary" fullWidth onClick={handleTogglePause}>
          {agent.status.phase === 'Paused' ? 'Resume' : 'Pause'}
        </Button>
        <Button variant="secondary" fullWidth onClick={handleResetMemory}>Reset Memory</Button>
        <Button variant="secondary" fullWidth onClick={openEditModal}>Edit Settings</Button>
        {config?.gitea_base_url && agent.spec.gitea.repo && (
          <div className={styles.quickLinks}>
            <div className={styles.quickLinksLabel}>Open in Gitea</div>
            <a href={giteaRepoUrl(config.gitea_base_url, agent.spec.gitea.repo)} target="_blank" rel="noopener noreferrer" className={styles.quickLink}>
              <span>Repository</span>
              <span className={styles.quickLinkExtIcon}>&#8599;</span>
            </a>
            <a href={giteaIssuesUrl(config.gitea_base_url, agent.spec.gitea.repo, agent.status.giteaUsername)} target="_blank" rel="noopener noreferrer" className={styles.quickLink}>
              <span>Issues</span>
              <span className={styles.quickLinkExtIcon}>&#8599;</span>
            </a>
            <a href={giteaPRsUrl(config.gitea_base_url, agent.spec.gitea.repo, agent.status.giteaUsername)} target="_blank" rel="noopener noreferrer" className={styles.quickLink}>
              <span>Pull Requests</span>
              <span className={styles.quickLinkExtIcon}>&#8599;</span>
            </a>
          </div>
        )}
        <div className={styles.tokenBlock}>
          <div className={styles.defLabel}>Today's Tokens</div>
          <div className={styles.tokenValue}>{(agent.status.tokenUsage?.today ?? 0).toLocaleString()}</div>
        </div>
      </div>
    </div>
  )

  const renderTimeline = () => {
    if (timeline.length === 0) {
      return <EmptyState title="No activity yet" description="Events will appear here as the agent runs." />
    }

    return (
      <div className={styles.timeline}>
        {timeline.map((item, i) => {
          const info = getIconInfo(item.type)
          if (item.type === 'cron') {
            return (
              <div key={i} className={styles.timelineItem}>
                <div className={styles.timelineCron}>
                  <div className={styles.timelineCronIcon}><IconClock size="small" /></div>
                  <span className={styles.timelineCronTime}>{item.time}</span>
                  <span className={styles.timelineCronTitle}>{item.title}</span>
                  <span className={styles.timelineCronDetail}>{item.detail}</span>
                </div>
              </div>
            )
          }
          return (
            <div key={i} className={styles.timelineItem}>
              <div className={styles.timelineDot}>
                <div className={`${styles.timelineDotIcon} ${info.className}`}>{info.icon}</div>
                {i < timeline.length - 1 && <div className={styles.timelineLine} />}
              </div>
              <div className={styles.timelineBody}>
                <div className={styles.timelineMeta}>
                  <span className={styles.timelineTime}>{item.time}</span>
                  <span className={styles.timelineTitle}>{item.title}</span>
                </div>
                <div className={styles.timelineDetail}>{item.detail}</div>
              </div>
            </div>
          )
        })}
      </div>
    )
  }

  const renderFiles = () => (
    <div>
      <div className={styles.filesHeader}>
        <span className={styles.filesInherited}>Inherited from role: {agent.spec.role}</span>
        <Link to={`/roles/${agent.spec.role}`} className={styles.filesEditLink}>
          Edit in Role Editor &rarr;
        </Link>
      </div>
      <div className={styles.filesWrap}>
        {roleFiles.length > 0 ? (
          <FileEditor
            files={roleFiles}
            activeFile={roleFilesActive}
            onSelectFile={setRoleFilesActive}
            onChangeContent={() => {}}
            onSave={() => {}}
            saving={false}
            dirty={{}}
          />
        ) : (
          <EmptyState title="No files" description="This role has no files yet." />
        )}
      </div>
    </div>
  )

  const renderLogs = () => (
    <div>
      <div className={styles.logToggle}>
        {(['events', 'pod'] as const).map((t) => (
          <Button key={t} variant={logTab === t ? 'primary' : 'secondary'} size="sm" onClick={() => setLogTab(t)}>
            {t === 'events' ? 'Live Event Stream' : 'Pod stdout'}
          </Button>
        ))}
      </div>
      {logTab === 'events' ? (
        <div className={styles.logTerminal}>
          <div className={styles.logConnecting}>&bull; Connecting...</div>
          <div>Waiting for events...</div>
        </div>
      ) : (
        <div className={styles.logTerminal}>
          {logs || '[INFO] Waiting for logs...'}
        </div>
      )}
    </div>
  )

  const renderAudit = () => (
    <div className={styles.auditSplitWrap}>
      {traces.length === 0 ? (
        <EmptyState title="No audit records" description="Trace records will appear here once the agent starts processing requests." />
      ) : (
        <SplitPane
          left={
            <div style={{ height: '100%', overflow: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--line-subtle)' }}>
                    {['Trace ID', 'Type', 'Path', 'Status', 'Token', 'Latency', 'Time'].map((h) => (
                      <th key={h} style={{ textAlign: 'left', padding: '8px 12px 12px', color: 'var(--text-muted)', fontWeight: 600, fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.03em' }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {traces.map((t) => (
                    <tr
                      key={t.trace_id}
                      onClick={() => setSelectedTrace(t)}
                      style={{
                        borderBottom: '1px solid var(--line-subtle)',
                        cursor: 'pointer',
                        backgroundColor: selectedTrace?.trace_id === t.trace_id ? 'rgba(0,0,0,0.04)' : undefined,
                      }}
                    >
                      <td style={{ padding: '10px 12px' }} className={styles.traceId}>{t.trace_id.slice(0, 8)}...</td>
                      <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{t.request_type}</td>
                      <td style={{ padding: '10px 12px', maxWidth: '150px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.path}</td>
                      <td style={{ padding: '10px 12px' }}>
                        <span className={t.status_code >= 400 ? styles.statusErr : styles.statusOk}>{t.status_code}</span>
                      </td>
                      <td style={{ padding: '10px 12px' }}>{t.tokens_in != null ? `${t.tokens_in}+${t.tokens_out}` : '\u2014'}</td>
                      <td style={{ padding: '10px 12px' }}>{t.latency_ms}ms</td>
                      <td style={{ padding: '10px 12px', color: 'var(--text-muted)' }}>{new Date(t.created_at).toLocaleTimeString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          }
          right={selectedTrace ? <TraceDetail trace={selectedTrace} onClose={() => setSelectedTrace(null)} /> : null}
          defaultLeftWidth={55}
          minLeftWidth={350}
          minRightWidth={320}
        />
      )}
    </div>
  )

  const tabContent = [renderOverview, renderTimeline, renderFiles, renderLogs, renderAudit]

  return (
    <>
      <Card>
        <div className={styles.tabBar}>
          {TABS.map((tab, i) => (
            <button
              key={tab}
              onClick={() => setActiveTab(i)}
              className={`${styles.tab} ${activeTab === i ? styles.tabActive : ''}`}
            >
              {tab}
            </button>
          ))}
        </div>
        <div className={styles.tabContent}>
          {tabContent[activeTab]()}
        </div>
      </Card>

      <Modal
        open={assignModal}
        onClose={() => setAssignModal(false)}
        title={`Assign Task to ${agent.name}`}
        width={480}
        footer={
          <>
            <Button variant="secondary" onClick={() => setAssignModal(false)}>Cancel</Button>
            <Button onClick={handleAssignTask} loading={assignLoading} disabled={!assignForm.title.trim()}>Create & Assign</Button>
          </>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
          <Input label="Title" value={assignForm.title} onChange={(e) => setAssignForm((p) => ({ ...p, title: e.target.value }))} />
          <Textarea label="Description" value={assignForm.description} onChange={(e) => setAssignForm((p) => ({ ...p, description: e.target.value }))} rows={4} />
          <div>
            <div className={styles.defLabel} style={{ marginBottom: '8px' }}>Priority</div>
            <div style={{ display: 'flex', gap: 'var(--space-md)' }}>
              {(['low', 'medium', 'high'] as const).map((p) => (
                <label key={p} style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer', fontSize: '0.875rem' }}>
                  <input type="radio" checked={assignForm.priority === p} onChange={() => setAssignForm((prev) => ({ ...prev, priority: p }))} />
                  {p}
                </label>
              ))}
            </div>
          </div>
        </div>
        <div className={styles.assignInfo}>
          Creates a Gitea issue and assigns it to @{agent.status.giteaUsername}
        </div>
      </Modal>

      <Modal
        open={editModal}
        onClose={() => setEditModal(false)}
        title={`Edit Settings: ${agent.name}`}
        width={520}
        footer={
          <>
            <Button variant="secondary" onClick={() => setEditModal(false)}>Cancel</Button>
            <Button onClick={handleSaveSettings} loading={editSaving}>Save</Button>
          </>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-md)' }}>
          <div className={styles.defLabel}>LLM</div>
          <Input label="Model" value={editForm.model} onChange={(e) => setEditForm((p) => ({ ...p, model: e.target.value }))} />
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 'var(--space-md)' }}>
            <Input label="Temperature" type="number" min={0} max={2} step={0.1} value={editForm.temperature} onChange={(e) => setEditForm((p) => ({ ...p, temperature: parseFloat(e.target.value) }))} />
            <Input label="Max Tokens" type="number" value={editForm.maxTokens} onChange={(e) => setEditForm((p) => ({ ...p, maxTokens: parseInt(e.target.value, 10) }))} />
          </div>

          <div className={styles.defLabel}>Runtime</div>
          <Input label="Runtime Type" value={editForm.runtimeType} onChange={(e) => setEditForm((p) => ({ ...p, runtimeType: e.target.value }))} />

          <div className={styles.defLabel}>Gitea</div>
          <Input label="Repository" value={editForm.repo} onChange={(e) => setEditForm((p) => ({ ...p, repo: e.target.value }))} placeholder="org/repo" />
          <div>
            <div className={styles.defLabel} style={{ marginBottom: '8px' }}>Permissions</div>
            <div style={{ display: 'flex', gap: 'var(--space-lg)' }}>
              {(['read', 'write', 'review', 'merge'] as const).map((perm) => (
                <label key={perm} style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer', fontSize: '0.875rem' }}>
                  <input type="checkbox" checked={editForm.permissions.includes(perm)} onChange={() => toggleEditPermission(perm)} />
                  {perm}
                </label>
              ))}
            </div>
          </div>
        </div>
      </Modal>
    </>
  )
}

export default function AgentDetail() {
  const { name } = useParams<{ name: string }>()
  const toast = useToast()
  const config = usePlatformConfig()
  const [agent, setAgent] = useState<Agent | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!name) return
    agentsApi.getAgent(name)
      .then(setAgent)
      .catch(() => toast.error('Failed to load agent', name))
      .finally(() => setLoading(false))
  }, [name])

  if (loading) {
    return (
      <div className={styles.page}>
        <PageHeader
          breadcrumbs={[{ label: 'Agents', path: '/agents' }, { label: name ?? '' }]}
          title={name ?? ''}
        />
        <Card>
          <div className={`${styles.skeleton} ${styles.skeletonBlock}`} />
        </Card>
      </div>
    )
  }

  if (!agent) {
    return (
      <div className={styles.page}>
        <PageHeader
          breadcrumbs={[{ label: 'Agents', path: '/agents' }, { label: name ?? '' }]}
          title={name ?? ''}
        />
        <Card>
          <EmptyState title="Agent not found" description={`No agent with name "${name}" exists.`} />
        </Card>
      </div>
    )
  }

  return (
    <div className={styles.page}>
      <PageHeader
        breadcrumbs={[{ label: 'Agents', path: '/agents' }, { label: agent.name }]}
        title={
          <span style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-sm)' }}>
            {agent.name}
            <AgentPhaseTag phase={agent.status.phase} />
          </span>
        }
        actions={
          config && agent.spec.gitea.repo ? (
            <div className={styles.giteaLinks}>
              <a href={giteaRepoUrl(config.gitea_base_url, agent.spec.gitea.repo)} target="_blank" rel="noopener noreferrer" className={styles.giteaLink}>
                <IconGithubLogo size="small" /> {agent.spec.gitea.repo}
              </a>
              <a href={giteaIssuesUrl(config.gitea_base_url, agent.spec.gitea.repo, agent.status.giteaUsername)} target="_blank" rel="noopener noreferrer" className={styles.giteaLink}>
                Issues &#8599;
              </a>
              <a href={giteaPRsUrl(config.gitea_base_url, agent.spec.gitea.repo, agent.status.giteaUsername)} target="_blank" rel="noopener noreferrer" className={styles.giteaLink}>
                PRs &#8599;
              </a>
            </div>
          ) : undefined
        }
      />
      <AgentDetailInner agent={agent} />
    </div>
  )
}
