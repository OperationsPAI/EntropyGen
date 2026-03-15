import { useState } from 'react'
import { IconWrench, IconTickCircle, IconMinusCircle, IconTreeTriangleDown, IconTreeTriangleRight } from '@douyinfe/semi-icons'
import type {
  JsonlMessage,
  MessageEnvelope,
  UserMessagePayload,
  AssistantMessagePayload,
  ToolResultMessagePayload,
  SessionMessage,
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
      return <BadgeMessage text={`Model: ${message.modelId}`} />
    case 'thinking_level_change':
      return <BadgeMessage text={`Thinking: ${message.thinkingLevel}`} />
    case 'custom':
      return null
    case 'message': {
      const { message: payload } = message as MessageEnvelope
      if (payload.role === 'user') return <UserBubble payload={payload} />
      if (payload.role === 'assistant') return <AssistantBubble payload={payload} onToolCallClick={onToolCallClick} />
      if (payload.role === 'toolResult') return <ToolResultBubble payload={payload} />
      return null
    }
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

function UserBubble({ payload }: { payload: UserMessagePayload }) {
  const text = extractText(payload.content)
  return (
    <div className={styles.userBubble}>
      <div className={styles.userLabel}>User</div>
      <div className={styles.userBubbleInner}>
        {truncateText(text, 500)}
      </div>
    </div>
  )
}

function AssistantBubble({
  payload,
  onToolCallClick,
}: {
  payload: AssistantMessagePayload
  onToolCallClick?: (toolName: string, args: Record<string, unknown>) => void
}) {
  return (
    <div className={styles.assistantBubble}>
      <div className={styles.assistantLabel}>Assistant</div>
      <div className={styles.assistantBubbleInner}>
        {Array.isArray(payload.content) ? (
          payload.content.map((block, i) => (
            <ContentBlockView
              key={i}
              block={block}
              onToolCallClick={onToolCallClick}
            />
          ))
        ) : (
          <div className={styles.assistantText}>
            {String(payload.content)}
          </div>
        )}
        {payload.usage && (
          <div className={styles.assistantMeta}>
            {payload.model && <span>{payload.model}</span>}
            <span>
              {payload.usage.input}+{payload.usage.output} tokens
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
          {expanded ? <IconTreeTriangleDown size="small" /> : <IconTreeTriangleRight size="small" />} Thinking...
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
          <IconWrench size="small" /> {block.name}
        </div>
        {argsSummary && (
          <div className={styles.toolCallArgs}>{argsSummary}</div>
        )}
      </div>
    )
  }

  return null
}

function ToolResultBubble({ payload }: { payload: ToolResultMessagePayload }) {
  const [expanded, setExpanded] = useState(!!payload.isError)
  const isError = !!payload.isError
  const text = extractText(payload.content)
  return (
    <div className={styles.toolResult}>
      <div
        className={`${styles.toolResultInner} ${isError ? styles.toolResultError : styles.toolResultSuccess}`}
      >
        <div>{isError ? <IconMinusCircle size="small" /> : <IconTickCircle size="small" />} {payload.toolName ?? 'tool result'}</div>
        {text && (
          <>
            <button
              className={styles.toolResultToggle}
              onClick={() => setExpanded(!expanded)}
              style={{ color: isError ? 'var(--status-red)' : 'var(--status-green)' }}
            >
              {expanded ? <><IconTreeTriangleDown size="small" /> Collapse</> : <><IconTreeTriangleRight size="small" /> Expand</>}
            </button>
            {expanded && (
              <div className={styles.toolResultContent}>
                {truncateText(text, 1000)}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

/** Extract plain text from a ContentBlock array. */
function extractText(blocks: ContentBlock[]): string {
  if (!Array.isArray(blocks)) return String(blocks)
  return blocks
    .filter((b): b is { type: 'text'; text: string } => b.type === 'text')
    .map((b) => b.text)
    .join('\n')
}

function summarizeToolArgs(name: string, args: Record<string, unknown>): string {
  if (!args) return ''
  const filePath = args.file_path ?? args.path ?? args.filePath
  if (filePath && typeof filePath === 'string') return filePath
  if (name.toLowerCase().includes('bash') && args.command) {
    return truncateText(String(args.command), 60)
  }
  if (args.pattern && typeof args.pattern === 'string') return args.pattern
  const firstVal = Object.values(args).find((v) => typeof v === 'string')
  return firstVal ? truncateText(String(firstVal), 60) : ''
}

function truncateText(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen) + '...'
}
