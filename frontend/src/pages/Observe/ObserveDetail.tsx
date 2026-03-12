import { useState, useEffect, useRef, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import PageHeader from '../../components/ui/PageHeader'
import AgentPhaseTag from '../../components/agent/AgentPhaseTag'
import SplitPane from '../../components/ui/SplitPane'
import ConversationFlow from './ConversationFlow'
import CodePanel from './CodePanel'
import SessionHistory from './SessionHistory'
import { agentsApi } from '../../api/agents'
import { observeApi, buildObserveWsUrl } from '../../api/observe'
import type { Agent } from '../../types/agent'
import type { JsonlMessage, SidecarWsEvent } from '../../types/observe'
import styles from './ObserveDetail.module.css'

export default function ObserveDetail() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()

  const [agent, setAgent] = useState<Agent | null>(null)
  const [messages, setMessages] = useState<JsonlMessage[]>([])
  const [viewingHistoryId, setViewingHistoryId] = useState<string | null>(null)
  const [selectedFile, setSelectedFile] = useState<string | undefined>()
  const [loading, setLoading] = useState(true)

  const wsRef = useRef<WebSocket | null>(null)

  // Load agent info
  useEffect(() => {
    if (!name) return
    let cancelled = false
    agentsApi.getAgent(name)
      .then((data) => { if (!cancelled) setAgent(data) })
      .catch(() => {})
    return () => { cancelled = true }
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
        }
        // file_change events are handled by CodePanel's polling
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
      // Return to live
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

  // Handle toolCall click → jump to file in code panel
  const handleToolCallClick = useCallback((_toolName: string, args: Record<string, unknown>) => {
    const filePath = args.file_path ?? args.path ?? args.filePath
    if (filePath && typeof filePath === 'string') {
      setSelectedFile(filePath)
    }
  }, [])

  const statusInfo = deriveStatus(agent, messages)

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

      {/* Status banner */}
      <div className={styles.statusBanner}>
        <span className={styles.statusIcon}>{statusInfo.icon}</span>
        <span className={styles.statusText}>{statusInfo.text}</span>
        <div className={styles.statusMeta}>
          {agent?.spec.gitea.repo && <span>Repo: {agent.spec.gitea.repo}</span>}
          {agent?.status.tokenUsage && (
            <span>Token today: {agent.status.tokenUsage.today.toLocaleString()}</span>
          )}
        </div>
      </div>

      {/* Main content: conversation + code */}
      <div className={styles.mainArea}>
        <SplitPane
          defaultLeftWidth={55}
          minLeftWidth={400}
          minRightWidth={300}
          left={
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
          right={
            <CodePanel
              agentName={name ?? ''}
              selectedFile={selectedFile}
              onFileSelected={setSelectedFile}
            />
          }
        />
      </div>

      {/* Session history */}
      <SessionHistory
        agentName={name ?? ''}
        activeSessionId={viewingHistoryId ?? undefined}
        onSessionSelect={handleSessionSelect}
      />
    </div>
  )
}

function deriveStatus(
  agent: Agent | null,
  messages: JsonlMessage[],
): { icon: string; text: string } {
  if (!agent) return { icon: '⏳', text: 'Loading...' }

  if (agent.status.phase === 'Paused') {
    return { icon: '⏸', text: 'Paused — manually paused by admin' }
  }
  if (agent.status.phase === 'Error') {
    return { icon: '⚠', text: 'Error — check agent status' }
  }
  if (agent.status.phase === 'Initializing' || agent.status.phase === 'Pending') {
    return { icon: '⏳', text: 'Initializing...' }
  }

  // Derive from latest message
  if (messages.length > 0) {
    const last = messages[messages.length - 1]
    if (last.type === 'message') {
      if (last.role === 'assistant' && Array.isArray(last.content)) {
        const lastBlock = last.content[last.content.length - 1]
        if (lastBlock?.type === 'toolCall') {
          return { icon: '🔧', text: `Executing ${lastBlock.name}...` }
        }
        if (lastBlock?.type === 'text') {
          return { icon: '💬', text: 'Composing response...' }
        }
        if (lastBlock?.type === 'thinking') {
          return { icon: '🧠', text: 'Thinking...' }
        }
      }
      if (last.role === 'toolResult') {
        return { icon: '⏳', text: 'Processing tool result...' }
      }
    }
    if (last.type === 'session') {
      return { icon: '🔄', text: 'Session started' }
    }
  }

  return { icon: '▶', text: 'Waiting for next cron cycle' }
}
