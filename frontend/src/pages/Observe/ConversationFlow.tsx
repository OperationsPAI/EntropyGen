import { useRef, useEffect, useState, useCallback } from 'react'
import type { JsonlMessage } from '../../types/observe'
import MessageBubble from './MessageBubble'
import styles from './ObserveDetail.module.css'

interface ConversationFlowProps {
  messages: JsonlMessage[]
  isLive: boolean
  historySessionId?: string
  onReturnToLive?: () => void
  onToolCallClick?: (toolName: string, args: Record<string, unknown>) => void
}

export default function ConversationFlow({
  messages,
  isLive,
  historySessionId,
  onReturnToLive,
  onToolCallClick,
}: ConversationFlowProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)
  const prevLenRef = useRef(messages.length)

  // Auto-scroll when new messages arrive and autoScroll is on
  useEffect(() => {
    if (autoScroll && messages.length > prevLenRef.current && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
    prevLenRef.current = messages.length
  }, [messages.length, autoScroll])

  // Detect user scroll to pause auto-scroll
  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current
    const atBottom = scrollHeight - scrollTop - clientHeight < 60
    setAutoScroll(atBottom)
  }, [])

  const scrollToBottom = useCallback(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
      setAutoScroll(true)
    }
  }, [])

  return (
    <div className={styles.conversationPanel}>
      {historySessionId && (
        <div className={styles.historyBanner}>
          <span>Viewing session {historySessionId.slice(0, 8)}</span>
          <button className={styles.historyBannerLink} onClick={onReturnToLive}>
            Return to live
          </button>
        </div>
      )}

      <div
        ref={scrollRef}
        className={styles.conversationScroll}
        onScroll={handleScroll}
      >
        {messages.length === 0 ? (
          <div className={styles.loadingState}>
            {isLive && <span className={styles.waitingDot} />}
            {isLive ? 'Waiting for messages...' : 'No messages in this session'}
          </div>
        ) : (
          messages.map((msg, i) => (
            <MessageBubble
              key={i}
              message={msg}
              onToolCallClick={onToolCallClick}
            />
          ))
        )}

        {!autoScroll && (
          <button className={styles.scrollToBottom} onClick={scrollToBottom}>
            ↓ Back to bottom
          </button>
        )}
      </div>
    </div>
  )
}
