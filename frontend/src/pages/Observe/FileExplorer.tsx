import { useState, useEffect, useCallback, useMemo } from 'react'
import { IconFolder, IconFolderOpen, IconFile, IconTreeTriangleRight, IconTreeTriangleDown } from '@douyinfe/semi-icons'
import { observeApi } from '../../api/observe'
import type { FileTreeNode } from '../../types/observe'
import styles from './ObserveDetail.module.css'

interface FileExplorerProps {
  agentName: string
  activePath: string
  onFileSelected: (path: string) => void
  followMode: boolean
  onToggleFollow: () => void
  treeRefreshKey: number
}

export default function FileExplorer({
  agentName,
  activePath,
  onFileSelected,
  followMode,
  onToggleFollow,
  treeRefreshKey,
}: FileExplorerProps) {
  const [tree, setTree] = useState<FileTreeNode[]>([])
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

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

  // Auto-expand directories that contain modified files or the active file.
  useEffect(() => {
    const paths = new Set<string>()
    collectExpandedPaths(tree, '', paths, activePath)
    if (paths.size > 0) {
      setExpanded((prev) => {
        const next = new Set(prev)
        for (const p of paths) next.add(p)
        return next
      })
    }
  }, [tree, activePath])

  const toggleDir = useCallback((dirPath: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(dirPath)) {
        next.delete(dirPath)
      } else {
        next.add(dirPath)
      }
      return next
    })
  }, [])

  const handleFileClick = useCallback((path: string) => {
    onFileSelected(path)
  }, [onFileSelected])

  const visibleItems = useMemo(
    () => buildVisibleItems(tree, expanded),
    [tree, expanded],
  )

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
        {visibleItems.length === 0 ? (
          <div className={styles.explorerEmpty}>No files available</div>
        ) : (
          visibleItems.map((f) => (
            <div
              key={f.path}
              className={`${styles.fileTreeItem} ${f.path === activePath ? styles.fileTreeItemActive : ''}`}
              onClick={() => f.isDir ? toggleDir(f.path) : handleFileClick(f.path)}
              style={{ paddingLeft: `${8 + f.depth * 14}px` }}
            >
              {f.isDir ? (
                <>
                  <span className={styles.fileTreeToggle}>
                    {expanded.has(f.path) ? <IconTreeTriangleDown size="extra-small" /> : <IconTreeTriangleRight size="extra-small" />}
                  </span>
                  <span className={styles.fileIcon}>
                    {expanded.has(f.path) ? <IconFolderOpen size="small" /> : <IconFolder size="small" />}
                  </span>
                </>
              ) : (
                <span className={styles.fileIcon}><IconFile size="small" /></span>
              )}
              <span className={styles.fileName}>{f.name}</span>
              {f.status && (
                <span
                  className={`${styles.fileStatus} ${
                    f.status === 'modified' ? styles.fileStatusModified :
                    f.status === 'added' ? styles.fileStatusAdded : ''
                  }`}
                >
                  {f.status === 'modified' ? '\u25CF' : f.status === 'added' ? '+' : ''}
                </span>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  )
}

interface VisibleItem {
  name: string
  path: string
  isDir: boolean
  status?: string
  depth: number
}

/**
 * Build the list of items that should be rendered based on which
 * directories are currently expanded. Collapsed directories hide
 * all their descendants, dramatically reducing DOM node count.
 */
function buildVisibleItems(
  nodes: FileTreeNode[],
  expanded: Set<string>,
  depth = 0,
  parentPath = '',
): VisibleItem[] {
  const result: VisibleItem[] = []
  for (const node of nodes) {
    const path = parentPath ? `${parentPath}/${node.name}` : node.name
    const isDir = node.type === 'dir'
    result.push({
      name: node.name,
      path,
      isDir,
      status: node.modified ? 'modified' : undefined,
      depth,
    })
    // Only recurse into children when the directory is expanded.
    if (isDir && node.children && expanded.has(path)) {
      result.push(...buildVisibleItems(node.children, expanded, depth + 1, path))
    }
  }
  return result
}

/**
 * Walk the tree and collect directory paths that should be auto-expanded:
 * - directories containing modified files
 * - ancestor directories of the currently active file
 */
function collectExpandedPaths(
  nodes: FileTreeNode[],
  parentPath: string,
  result: Set<string>,
  activePath: string,
): boolean {
  let hasRelevant = false
  for (const node of nodes) {
    const path = parentPath ? `${parentPath}/${node.name}` : node.name
    if (node.type === 'dir' && node.children) {
      const childRelevant = collectExpandedPaths(node.children, path, result, activePath)
      if (childRelevant || node.modified) {
        result.add(path)
        hasRelevant = true
      }
    } else {
      if (node.modified || path === activePath) {
        hasRelevant = true
      }
    }
  }
  return hasRelevant
}
