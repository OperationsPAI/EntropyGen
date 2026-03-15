import { useState, useEffect, useCallback } from 'react'
import { IconFolderOpen, IconFile, IconTreeTriangleRight, IconTreeTriangleDown } from '@douyinfe/semi-icons'
import { observeApi } from '../../api/observe'
import type { FileTreeNode, SessionInfo } from '../../types/observe'
import styles from './ObserveDetail.module.css'

interface FileExplorerProps {
  agentName: string
  activePath: string
  onFileSelected: (path: string) => void
  followMode: boolean
  onToggleFollow: () => void
  sessions: SessionInfo[]
  activeSessionId?: string
  onSessionSelect: (id: string | null) => void
  treeRefreshKey: number
}

export default function FileExplorer({
  agentName,
  activePath,
  onFileSelected,
  followMode,
  onToggleFollow,
  sessions,
  activeSessionId,
  onSessionSelect,
  treeRefreshKey,
}: FileExplorerProps) {
  const [tree, setTree] = useState<FileTreeNode[]>([])
  const [sessionsCollapsed, setSessionsCollapsed] = useState(false)

  useEffect(() => {
    let cancelled = false
    const loadTree = async () => {
      try {
        const data = await observeApi.getWorkspaceTree(agentName)
        if (!cancelled) setTree(data ?? [])
      } catch {
        // sidecar may not be reachable
      }
    }
    loadTree()
    const timer = setInterval(loadTree, 15_000)
    return () => { cancelled = true; clearInterval(timer) }
  }, [agentName, treeRefreshKey])

  const handleFileClick = useCallback((path: string) => {
    onFileSelected(path)
  }, [onFileSelected])

  const flatFiles = flattenTree(tree)

  return (
    <div className={styles.fileExplorer}>
      {/* Toolbar */}
      <div className={styles.explorerToolbar}>
        <span className={styles.explorerTitle}>Explorer</span>
        <button
          className={`${styles.followToggle} ${followMode ? styles.followToggleActive : ''}`}
          onClick={onToggleFollow}
          title={followMode ? 'Following agent edits — click to browse freely' : 'Browse mode — click to follow agent'}
        >
          <span className={followMode ? styles.followDot : styles.browseDot} />
          {followMode ? 'Following' : 'Browse'}
        </button>
      </div>

      {/* File tree */}
      <div className={styles.explorerTree}>
        {flatFiles.length === 0 ? (
          <div className={styles.explorerEmpty}>No files available</div>
        ) : (
          flatFiles.map((f) => (
            <div
              key={f.path}
              className={`${styles.fileTreeItem} ${f.path === activePath ? styles.fileTreeItemActive : ''}`}
              onClick={() => !f.isDir && handleFileClick(f.path)}
              style={{ paddingLeft: `${8 + f.depth * 12}px` }}
            >
              <span className={styles.fileIcon}>{f.isDir ? <IconFolderOpen size="small" /> : <IconFile size="small" />}</span>
              <span className={styles.fileName}>{f.name}</span>
              {f.status && (
                <span
                  className={`${styles.fileStatus} ${
                    f.status === 'modified' ? styles.fileStatusModified :
                    f.status === 'added' ? styles.fileStatusAdded : ''
                  }`}
                >
                  {f.status === 'modified' ? '●' : f.status === 'added' ? '+' : ''}
                </span>
              )}
            </div>
          ))
        )}
      </div>

      {/* Sessions divider */}
      <div
        className={styles.explorerSectionHeader}
        onClick={() => setSessionsCollapsed((p) => !p)}
      >
        <span>{sessionsCollapsed ? <IconTreeTriangleRight size="small" /> : <IconTreeTriangleDown size="small" />} Sessions</span>
        <span className={styles.explorerSectionCount}>{sessions.length}</span>
      </div>

      {/* Session list */}
      {!sessionsCollapsed && (
        <div className={styles.explorerSessions}>
          {sessions.length === 0 ? (
            <div className={styles.explorerEmpty}>No sessions</div>
          ) : (
            sessions.map((session) => {
              const id = session.id || session.filename.replace(/\.jsonl$/, '')
              const isActive = activeSessionId === id
              return (
                <div
                  key={id}
                  className={`${styles.explorerSessionItem} ${isActive ? styles.explorerSessionItemActive : ''}`}
                  onClick={() => onSessionSelect(session.is_current ? null : id)}
                >
                  <span className={styles.explorerSessionId}>{id.slice(0, 8)}</span>
                  <span className={`${styles.explorerSessionStatus} ${
                    session.is_current ? styles.sessionStatusActive : styles.sessionStatusDone
                  }`}>
                    {session.is_current ? 'run' : 'done'}
                  </span>
                </div>
              )
            })
          )}
        </div>
      )}
    </div>
  )
}

interface FlatFile {
  name: string
  path: string
  isDir: boolean
  status?: string
  depth: number
}

function flattenTree(nodes: FileTreeNode[], depth = 0, parentPath = ''): FlatFile[] {
  const result: FlatFile[] = []
  for (const node of nodes) {
    const path = parentPath ? `${parentPath}/${node.name}` : node.name
    result.push({
      name: node.name,
      path,
      isDir: node.type === 'dir',
      status: node.modified ? 'modified' : undefined,
      depth,
    })
    if (node.children) {
      result.push(...flattenTree(node.children, depth + 1, path))
    }
  }
  return result
}
