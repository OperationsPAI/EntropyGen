import { useState } from 'react'
import type {
  JsonlMessage,
  UserMessage,
  AssistantMessage,
  ToolResultMessage,
  SessionMessage,
  ModelChangeMessage,
  ThinkingLevelChangeMessage,
  ContentBlock,
} from '../../types/observe'
import styles from './ObserveDetail.module.css'

interface MessageBubbleProps {
  message: JsonlMessage
  onToolCallClick?: (toolName: string, args: Record<string, unknown>) => void
}

export default function MessageBubble({ message, onToolCallClick }: MessageBubbleProps) {
  switch (message.type) {
    case 'session':
      return <SessionDivider message={message} />
    case 'model_change':
      return <BadgeMessage text={`Model: ${(message as ModelChangeMessage).modelId}`} />
    case 'thinking_level_change':
      return <BadgeMessage text={`Thinking: ${(message as ThinkingLevelChangeMessage).level}`} />
    case 'message':
      if (message.role === 'user') return <UserBubble message={message as UserMessage} />
      if (message.role === 'assistant') return <AssistantBubble message={message as AssistantMessage} onToolCallClick={onToolCallClick} />
      if (message.role === 'toolResult') return <ToolResultBubble message={message as ToolResultMessage} />
      return null
    default:
      return null
  }
}

function SessionDivider({ message }: { message: SessionMessage }) {
  const time = new Date(message.timestamp).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  return (
    <div className={styles.sessionDivider}>
      <span className={styles.sessionDividerLine}>
        Session started {time}
      </span>
    </div>
  )
}

function BadgeMessage({ text }: { text: string }) {
  return (
    <div className={styles.badge}>
      <span className={styles.badgeTag}>{text}</span>
    </div>
  )
}

function UserBubble({ message }: { message: UserMessage }) {
  const content = typeof message.content === 'string'
    ? message.content
    : JSON.stringify(message.content)
  return (
    <div className={styles.userBubble}>
      <div className={styles.userLabel}>User</div>
      <div className={styles.userBubbleInner}>
        {truncateText(content, 500)}
      </div>
    </div>
  )
}

function AssistantBubble({
  message,
  onToolCallClick,
}: {
  message: AssistantMessage
  onToolCallClick?: (toolName: string, args: Record<string, unknown>) => void
}) {
  return (
    <div className={styles.assistantBubble}>
      <div className={styles.assistantLabel}>Assistant</div>
      <div className={styles.assistantBubbleInner}>
        {Array.isArray(message.content) ? (
          message.content.map((block, i) => (
            <ContentBlockView
              key={i}
              block={block}
              onToolCallClick={onToolCallClick}
            />
          ))
        ) : (
          <div className={styles.assistantText}>
            {String(message.content)}
          </div>
        )}
        {message.usage && (
          <div className={styles.assistantMeta}>
            {message.model && <span>{message.model}</span>}
            <span>
              {message.usage.inputTokens}+{message.usage.outputTokens} tokens
            </span>
          </div>
        )}
      </div>
    </div>
  )
}

function ContentBlockView({
  block,
  onToolCallClick,
}: {
  block: ContentBlock
  onToolCallClick?: (toolName: string, args: Record<string, unknown>) => void
}) {
  const [expanded, setExpanded] = useState(false)

  if (block.type === 'thinking') {
    return (
      <div className={styles.thinkingBlock}>
        <button
          className={styles.thinkingToggle}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? '▾' : '▸'} Thinking...
        </button>
        {expanded && (
          <div className={styles.thinkingContent}>{block.thinking}</div>
        )}
      </div>
    )
  }

  if (block.type === 'text') {
    return <div className={styles.assistantText}>{block.text}</div>
  }

  if (block.type === 'toolCall') {
    const argsSummary = summarizeToolArgs(block.name, block.arguments)
    return (
      <div
        className={styles.toolCallCard}
        onClick={() => onToolCallClick?.(block.name, block.arguments)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter') onToolCallClick?.(block.name, block.arguments)
        }}
      >
        <div className={styles.toolCallName}>
          <span>🔧</span> {block.name}
        </div>
        {argsSummary && (
          <div className={styles.toolCallArgs}>{argsSummary}</div>
        )}
      </div>
    )
  }

  return null
}

function ToolResultBubble({ message }: { message: ToolResultMessage }) {
  const [expanded, setExpanded] = useState(!!message.isError)
  const isError = !!message.isError
  return (
    <div className={styles.toolResult}>
      <div
        className={`${styles.toolResultInner} ${isError ? styles.toolResultError : styles.toolResultSuccess}`}
      >
        <div>{isError ? '✗' : '✓'} {message.name ?? 'tool result'}</div>
        {message.content && (
          <>
            <button
              className={styles.toolResultToggle}
              onClick={() => setExpanded(!expanded)}
              style={{ color: isError ? 'var(--status-red)' : 'var(--status-green)' }}
            >
              {expanded ? '▾ Collapse' : '▸ Expand'}
            </button>
            {expanded && (
              <div className={styles.toolResultContent}>
                {truncateText(message.content, 1000)}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function summarizeToolArgs(name: string, args: Record<string, unknown>): string {
  if (!args) return ''
  // Show file path for file operations
  const filePath = args.file_path ?? args.path ?? args.filePath
  if (filePath && typeof filePath === 'string') return filePath
  // Show command for bash
  if (name.toLowerCase().includes('bash') && args.command) {
    return truncateText(String(args.command), 60)
  }
  // Show pattern for search
  if (args.pattern && typeof args.pattern === 'string') return args.pattern
  // Fallback: first string arg
  const firstVal = Object.values(args).find((v) => typeof v === 'string')
  return firstVal ? truncateText(String(firstVal), 60) : ''
}

function truncateText(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen) + '...'
}
