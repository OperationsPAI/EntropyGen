import { useState, useEffect, useRef, useCallback, useMemo, type ReactNode } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { IconLoading, IconPause, IconAlertTriangle, IconWrench, IconComment, IconBolt, IconRefresh, IconPlay } from '@douyinfe/semi-icons'
import PageHeader from '../../components/ui/PageHeader'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import SplitPane from '../../components/ui/SplitPane'
import FileExplorer from './FileExplorer'
import EditorPanel from './CodePanel'
import ConversationFlow from './ConversationFlow'
import StatusFooter from './StatusFooter'
import { agentsApi } from '../../api/agents'
import { observeApi, buildObserveWsUrl } from '../../api/observe'
import { computeTokenUsage } from './tokenUtils'
import { usePlatformConfig } from '../../hooks/usePlatformConfig'
import type { Agent } from '../../types/agent'
import type { JsonlMessage, MessageEnvelope, SessionInfo, DiffResultResponse, SidecarWsEvent } from '../../types/observe'
import styles from './ObserveDetail.module.css'

export default function ObserveDetail() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const platformConfig = usePlatformConfig()

  const [agent, setAgent] = useState<Agent | null>(null)
  const [messages, setMessages] = useState<JsonlMessage[]>([])
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [viewingHistoryId, setViewingHistoryId] = useState<string | null>(null)
  const [activePath, setActivePath] = useState('')
  const [activeTab, setActiveTab] = useState<'code' | 'diff'>('code')
  const [followMode, setFollowMode] = useState(true)
  const [diffData, setDiffData] = useState<DiffResultResponse | null>(null)
  const [treeRefreshKey, setTreeRefreshKey] = useState(0)
  const [contentRefreshKey, setContentRefreshKey] = useState(0)
  const [loading, setLoading] = useState(true)

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

  // Load sessions
  useEffect(() => {
    if (!name) return
    let cancelled = false
    const load = async () => {
      try {
        const data = await observeApi.getSessions(name)
        if (!cancelled) setSessions(data ?? [])
      } catch {
        // ignore
      }
    }
    load()
    const timer = setInterval(load, 30_000)
    return () => { cancelled = true; clearInterval(timer) }
  }, [name])

  // Load current session
  useEffect(() => {
    if (!name || viewingHistoryId) return
    let cancelled = false
    setLoading(true)
    observeApi.getCurrentSession(name)
      .then((msgs) => {
        if (!cancelled) setMessages(msgs)
      })
      .catch(() => {
        if (!cancelled) setMessages([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [name, viewingHistoryId])

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

  // WebSocket for live updates
  useEffect(() => {
    if (!name || viewingHistoryId) return
    const url = buildObserveWsUrl(name)
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onmessage = (evt) => {
      try {
        const event: SidecarWsEvent = JSON.parse(evt.data as string)
        if (event.type === 'jsonl') {
          setMessages((prev) => [...prev, event.data])

          // Auto-follow: if the latest message is a toolCall with a file path, switch to it
          if (followModeRef.current && event.data.type === 'message') {
            const envelope = event.data as MessageEnvelope
            if (envelope.message.role === 'assistant') {
              const blocks = envelope.message.content
              if (Array.isArray(blocks)) {
                const lastBlock = blocks[blocks.length - 1]
                if (lastBlock?.type === 'toolCall') {
                  const filePath = lastBlock.arguments.file_path ?? lastBlock.arguments.path ?? lastBlock.arguments.filePath
                  if (filePath && typeof filePath === 'string') {
                    setActivePath(filePath)
                  }
                }
              }
            }
          }
        }
        if (event.type === 'file_change') {
          setTreeRefreshKey((k) => k + 1)
          if (event.path === activePathRef.current) {
            setContentRefreshKey((k) => k + 1)
          }
        }
      } catch {
        // ignore
      }
    }

    ws.onclose = () => {
      wsRef.current = null
    }

    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [name, viewingHistoryId])

  // Load history session
  const handleSessionSelect = useCallback(async (sessionId: string | null) => {
    if (!name) return
    if (!sessionId) {
      setViewingHistoryId(null)
      return
    }
    setViewingHistoryId(sessionId)
    setLoading(true)
    try {
      const msgs = await observeApi.getSession(name, sessionId)
      setMessages(msgs)
    } catch {
      setMessages([])
    } finally {
      setLoading(false)
    }
  }, [name])

  // Handle toolCall click -> jump to file
  const handleToolCallClick = useCallback((_toolName: string, args: Record<string, unknown>) => {
    const filePath = args.file_path ?? args.path ?? args.filePath
    if (filePath && typeof filePath === 'string') {
      setActivePath(filePath)
      setActiveTab('code')
    }
  }, [])

  const handleFileSelected = useCallback((path: string) => {
    setActivePath(path)
    setActiveTab('code')
  }, [])

  const handleToggleFollow = useCallback(() => {
    setFollowMode((prev) => !prev)
  }, [])

  const tokenUsage = useMemo(() => computeTokenUsage(messages), [messages])
  const statusInfo = deriveStatus(agent, messages)

  // Fallback: if JSONL aggregation is 0 but agent has tokenUsage.today, use that
  const effectiveTokenUsage = useMemo(() => {
    if (tokenUsage.totalTokens > 0) return tokenUsage
    const today = agent?.status.tokenUsage?.today ?? 0
    if (today > 0) {
      return { inputTokens: today, outputTokens: 0, totalTokens: today, messageCount: 0 }
    }
    return tokenUsage
  }, [tokenUsage, agent])

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
          defaultLeftWidth={18}
          minLeftWidth={180}
          minRightWidth={650}
          left={
            <FileExplorer
              agentName={name ?? ''}
              activePath={activePath}
              onFileSelected={handleFileSelected}
              followMode={followMode}
              onToggleFollow={handleToggleFollow}
              sessions={sessions}
              activeSessionId={viewingHistoryId ?? undefined}
              onSessionSelect={handleSessionSelect}
              treeRefreshKey={treeRefreshKey}
            />
          }
          right={
            <SplitPane
              defaultLeftWidth={55}
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
                loading ? (
                  <div className={styles.loadingState}>Loading session...</div>
                ) : (
                  <ConversationFlow
                    messages={messages}
                    isLive={!viewingHistoryId}
                    historySessionId={viewingHistoryId ?? undefined}
                    onReturnToLive={() => handleSessionSelect(null)}
                    onToolCallClick={handleToolCallClick}
                  />
                )
              }
            />
          }
        />
      </div>

      {/* Status footer */}
      <StatusFooter
        statusIcon={statusInfo.icon}
        statusText={statusInfo.text}
        tokenUsage={effectiveTokenUsage}
        sessionCount={sessions.length}
        repo={agent?.spec.gitea.repo}
        currentTask={agent?.status.currentTask}
        giteaBaseUrl={platformConfig?.gitea_base_url}
      />
    </div>
  )
}

function deriveStatus(
  agent: Agent | null,
  messages: JsonlMessage[],
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

  if (messages.length > 0) {
    const last = messages[messages.length - 1]
    if (last.type === 'message') {
      const { message: payload } = last as MessageEnvelope
      if (payload.role === 'assistant' && Array.isArray(payload.content)) {
        const lastBlock = payload.content[payload.content.length - 1]
        if (lastBlock?.type === 'toolCall') {
          return { icon: <IconWrench size="small" />, text: `Executing ${lastBlock.name}...` }
        }
        if (lastBlock?.type === 'text') {
          return { icon: <IconComment size="small" />, text: 'Composing response...' }
        }
        if (lastBlock?.type === 'thinking') {
          return { icon: <IconBolt size="small" />, text: 'Thinking...' }
        }
      }
      if (payload.role === 'toolResult') {
        return { icon: <IconLoading size="small" />, text: 'Processing tool result...' }
      }
    }
    if (last.type === 'session') {
      return { icon: <IconRefresh size="small" />, text: 'Session started' }
    }
  }

  return { icon: <IconPlay size="small" />, text: 'Waiting for next cron cycle' }
}
