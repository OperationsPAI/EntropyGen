// --- JSONL message types from OpenClaw completions ---
// Aligned with actual backend NDJSON structure.

// Every JSONL line has these common envelope fields.
interface BaseEnvelope {
  id?: string
  parentId?: string | null
  timestamp: string
}

export interface SessionMessage extends BaseEnvelope {
  type: 'session'
  version?: number
  cwd: string
}

export interface ModelChangeMessage extends BaseEnvelope {
  type: 'model_change'
  provider: string
  modelId: string
}

export interface ThinkingLevelChangeMessage extends BaseEnvelope {
  type: 'thinking_level_change'
  thinkingLevel: string
}

export interface CustomMessage extends BaseEnvelope {
  type: 'custom'
  customType: string
  data: Record<string, unknown>
}

// --- Content blocks inside assistant messages ---

export interface ThinkingBlock {
  type: 'thinking'
  thinking: string
}

export interface TextBlock {
  type: 'text'
  text: string
}

export interface ToolCallBlock {
  type: 'toolCall'
  id?: string
  name: string
  arguments: Record<string, unknown>
}

export type ContentBlock = ThinkingBlock | TextBlock | ToolCallBlock

// --- Usage (backend shape) ---

export interface TokenUsage {
  input: number
  output: number
  cacheRead?: number
  cacheWrite?: number
  totalTokens?: number
  cost?: {
    input: number
    output: number
    cacheRead: number
    cacheWrite: number
    total: number
  }
}

// --- Message envelope: type="message", inner payload in .message ---

export interface UserMessagePayload {
  role: 'user'
  content: ContentBlock[]
  timestamp?: number
}

export interface AssistantMessagePayload {
  role: 'assistant'
  content: ContentBlock[]
  model?: string
  provider?: string
  usage?: TokenUsage
  stopReason?: string
  timestamp?: number
}

export interface ToolResultMessagePayload {
  role: 'toolResult'
  toolCallId?: string
  toolName: string
  content: ContentBlock[]
  isError?: boolean
  timestamp?: number
}

export type MessagePayload =
  | UserMessagePayload
  | AssistantMessagePayload
  | ToolResultMessagePayload

export interface MessageEnvelope extends BaseEnvelope {
  type: 'message'
  message: MessagePayload
}

// --- Union of all JSONL line types ---

export type JsonlMessage =
  | SessionMessage
  | ModelChangeMessage
  | ThinkingLevelChangeMessage
  | CustomMessage
  | MessageEnvelope

// --- Sidecar API types ---

export interface SessionInfo {
  id: string
  started_at: string
  message_count: number
  is_current: boolean
  filename: string
}

export interface FileTreeNode {
  name: string
  type: 'file' | 'dir'
  modified: boolean
  children?: FileTreeNode[]
}

export interface FileContentResponse {
  path: string
  content: string
  language: string
}

export interface DiffResultResponse {
  diff: string
  files_changed: number
  insertions: number
  deletions: number
}

// --- WebSocket events from sidecar ---

export interface JsonlLiveEvent {
  type: 'jsonl'
  data: JsonlMessage
}

export interface FileChangeEvent {
  type: 'file_change'
  path: string
  action: 'modified' | 'created' | 'deleted'
}

export type SidecarWsEvent = JsonlLiveEvent | FileChangeEvent
