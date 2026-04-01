// --- Workspace observation types (framework-agnostic) ---

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

// --- WebSocket events from observer sidecar ---

export interface FileChangeEvent {
  type: 'file_change'
  path: string
  action: 'modified' | 'created' | 'deleted'
}

export type SidecarWsEvent = FileChangeEvent
