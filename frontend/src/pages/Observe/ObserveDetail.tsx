import { useState, useEffect, useRef, useCallback, useMemo, type ReactNode } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { IconLoading, IconPause, IconAlertTriangle, IconPlay } from '@douyinfe/semi-icons'
import PageHeader from '../../components/ui/PageHeader'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import SplitPane from '../../components/ui/SplitPane'
import FileExplorer from './FileExplorer'
import EditorPanel from './CodePanel'
import WorkspaceActivity from './ConversationFlow'
import type { ActivityEntry } from './ConversationFlow'
import StatusFooter from './StatusFooter'
import { agentsApi } from '../../api/agents'
import { observeApi, buildObserveWsUrl } from '../../api/observe'
import { usePlatformConfig } from '../../hooks/usePlatformConfig'
import type { Agent } from '../../types/agent'
import type { DiffResultResponse, SidecarWsEvent } from '../../types/observe'
import type { AggregatedTokenUsage } from './tokenUtils'
import styles from './ObserveDetail.module.css'

export default function ObserveDetail() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const platformConfig = usePlatformConfig()

  const [agent, setAgent] = useState<Agent | null>(null)
  const [activityEvents, setActivityEvents] = useState<ActivityEntry[]>([])
  const [activePath, setActivePath] = useState('')
  const [activeTab, setActiveTab] = useState<'code' | 'diff'>('code')
  const [followMode, setFollowMode] = useState(true)
  const [diffData, setDiffData] = useState<DiffResultResponse | null>(null)
  const [treeRefreshKey, setTreeRefreshKey] = useState(0)
  const [contentRefreshKey, setContentRefreshKey] = useState(0)

  const wsRef = useRef<WebSocket | null>(null)
  const activePathRef = useRef(activePath)
  activePathRef.current = activePath

  const followModeRef = useRef(followMode)
  followModeRef.current = followMode

  // Load agent info
  useEffect(() => {
    if (!name) return
    let cancelled = false
    agentsApi.getAgent(name)
      .then((data) => { if (!cancelled) setAgent(data) })
      .catch(() => {})
    return () => { cancelled = true }
  }, [name])

  // Load diff
  useEffect(() => {
    if (!name) return
    let cancelled = false
    const loadDiff = async () => {
      try {
        const d = await observeApi.getWorkspaceDiff(name)
        if (!cancelled) setDiffData(d)
      } catch {
        // ignore
      }
    }
    loadDiff()
    const timer = setInterval(loadDiff, 15_000)
    return () => { cancelled = true; clearInterval(timer) }
  }, [name])

  // WebSocket for live file change events
  useEffect(() => {
    if (!name) return
    const url = buildObserveWsUrl(name)
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onmessage = (evt) => {
      try {
        const event: SidecarWsEvent = JSON.parse(evt.data as string)
        if (event.type === 'file_change') {
          const entry: ActivityEntry = {
            ...event,
            timestamp: new Date().toISOString(),
          }
          setActivityEvents((prev) => [...prev, entry])
          setTreeRefreshKey((k) => k + 1)

          // If the changed file is currently viewed, refresh its content
          if (event.path === activePathRef.current) {
            setContentRefreshKey((k) => k + 1)
          }

          // Auto-follow: switch to the changed file
          if (followModeRef.current && event.action !== 'deleted') {
            setActivePath(event.path)
          }
        }
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      wsRef.current = null
    }

    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [name])

  const handleFileSelected = useCallback((path: string) => {
    setActivePath(path)
    setActiveTab('code')
  }, [])

  const handleToggleFollow = useCallback(() => {
    setFollowMode((prev) => !prev)
  }, [])

  // Token usage from agent status (no longer aggregated from JSONL)
  const tokenUsage: AggregatedTokenUsage = useMemo(() => {
    const today = agent?.status.tokenUsage?.today ?? 0
    if (today > 0) {
      return { inputTokens: today, outputTokens: 0, totalTokens: today }
    }
    return { inputTokens: 0, outputTokens: 0, totalTokens: 0 }
  }, [agent])

  const statusInfo = deriveStatus(agent)

  return (
    <div className={styles.detailContainer}>
      <PageHeader
        title={name ?? 'Agent Observe'}
        breadcrumbs={[
          { label: 'Observe', path: '/observe' },
          { label: name ?? '' },
        ]}
        actions={
          agent && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
              <AgentPhaseTag phase={agent.status.phase} />
              <button
                onClick={() => navigate(`/agents/${name}`)}
                style={{
                  background: 'none', border: '1px solid var(--line-subtle)',
                  borderRadius: 'var(--radius-sm)', padding: '6px 14px',
                  fontSize: '0.8125rem', cursor: 'pointer', color: 'var(--text-muted)',
                }}
              >
                Manage
              </button>
            </div>
          )
        }
      />

      {/* Main 3-panel area */}
      <div className={styles.mainArea}>
        <SplitPane
          defaultLeftWidth={15}
          minLeftWidth={160}
          minRightWidth={500}
          left={
            <FileExplorer
              agentName={name ?? ''}
              activePath={activePath}
              onFileSelected={handleFileSelected}
              followMode={followMode}
              onToggleFollow={handleToggleFollow}
              treeRefreshKey={treeRefreshKey}
            />
          }
          right={
            <SplitPane
              defaultLeftWidth={65}
              minLeftWidth={300}
              minRightWidth={280}
              left={
                <EditorPanel
                  agentName={name ?? ''}
                  activePath={activePath}
                  activeTab={activeTab}
                  onTabChange={setActiveTab}
                  diffData={diffData}
                  followMode={followMode}
                  contentRefreshKey={contentRefreshKey}
                />
              }
              right={
                <WorkspaceActivity events={activityEvents} />
              }
            />
          }
        />
      </div>

      {/* Status footer */}
      <StatusFooter
        statusIcon={statusInfo.icon}
        statusText={statusInfo.text}
        tokenUsage={tokenUsage}
        repo={agent?.spec.gitea.repo}
        currentTask={agent?.status.currentTask}
        giteaBaseUrl={platformConfig?.gitea_base_url}
      />
    </div>
  )
}

function deriveStatus(
  agent: Agent | null,
): { icon: ReactNode; text: string } {
  if (!agent) return { icon: <IconLoading size="small" />, text: 'Loading...' }

  if (agent.status.phase === 'Paused') {
    return { icon: <IconPause size="small" />, text: 'Paused — manually paused by admin' }
  }
  if (agent.status.phase === 'Error') {
    return { icon: <IconAlertTriangle size="small" />, text: 'Error — check agent status' }
  }
  if (agent.status.phase === 'Initializing' || agent.status.phase === 'Pending') {
    return { icon: <IconLoading size="small" />, text: 'Initializing...' }
  }

  return { icon: <IconPlay size="small" />, text: 'Waiting for next cron cycle' }
}
