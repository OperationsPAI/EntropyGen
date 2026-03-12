import { useState, useEffect, useCallback } from 'react'
import MonacoEditor from '../../components/editor/MonacoEditor'
import { observeApi } from '../../api/observe'
import type { FileTreeNode } from '../../types/observe'
import styles from './ObserveDetail.module.css'

interface CodePanelProps {
  agentName: string
  selectedFile?: string
  onFileSelected?: (path: string) => void
}

export default function CodePanel({ agentName, selectedFile, onFileSelected }: CodePanelProps) {
  const [tree, setTree] = useState<FileTreeNode[]>([])
  const [fileContent, setFileContent] = useState('')
  const [diff, setDiff] = useState('')
  const [activePath, setActivePath] = useState(selectedFile ?? '')
  const [loading, setLoading] = useState(false)

  // Load file tree
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
  }, [agentName])

  // Load diff summary
  useEffect(() => {
    let cancelled = false
    const loadDiff = async () => {
      try {
        const d = await observeApi.getWorkspaceDiff(agentName)
        if (!cancelled) setDiff(d)
      } catch {
        // ignore
      }
    }
    loadDiff()
    const timer = setInterval(loadDiff, 15_000)
    return () => { cancelled = true; clearInterval(timer) }
  }, [agentName])

  // Respond to external file selection (e.g., from toolCall click)
  useEffect(() => {
    if (selectedFile && selectedFile !== activePath) {
      setActivePath(selectedFile)
    }
  }, [selectedFile, activePath])

  // Load file content when path changes
  useEffect(() => {
    if (!activePath) return
    let cancelled = false
    setLoading(true)
    observeApi.getWorkspaceFile(agentName, activePath)
      .then((content) => {
        if (!cancelled) setFileContent(content)
      })
      .catch(() => {
        if (!cancelled) setFileContent('// Unable to load file')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [agentName, activePath])

  const handleFileClick = useCallback((path: string) => {
    setActivePath(path)
    onFileSelected?.(path)
  }, [onFileSelected])

  const flatFiles = flattenTree(tree)
  const diffStats = parseDiffStats(diff)
  const language = guessLanguage(activePath)

  return (
    <div className={styles.codePanel}>
      <div className={styles.fileTree}>
        <div className={styles.fileTreeTitle}>Workspace</div>
        {flatFiles.length === 0 ? (
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', padding: '4px 8px' }}>
            No files available
          </div>
        ) : (
          flatFiles.map((f) => (
            <div
              key={f.path}
              className={`${styles.fileTreeItem} ${f.path === activePath ? styles.fileTreeItemActive : ''}`}
              onClick={() => handleFileClick(f.path)}
              style={{ paddingLeft: `${8 + f.depth * 12}px` }}
            >
              <span>{f.isDir ? '📁' : '📄'}</span>
              <span>{f.name}</span>
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

      <div className={styles.codeEditor}>
        {loading ? (
          <div className={styles.loadingState}>Loading file...</div>
        ) : activePath ? (
          <MonacoEditor
            value={fileContent}
            language={language}
            readOnly
            height="100%"
          />
        ) : (
          <div className={styles.loadingState}>Select a file to view</div>
        )}
      </div>

      {diffStats && (
        <div className={styles.diffSummary}>
          git diff: {diffStats.filesChanged} file{diffStats.filesChanged !== 1 ? 's' : ''} changed,
          +{diffStats.additions} -{diffStats.deletions} lines
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

function flattenTree(nodes: FileTreeNode[], depth = 0): FlatFile[] {
  const result: FlatFile[] = []
  for (const node of nodes) {
    result.push({
      name: node.name,
      path: node.path,
      isDir: node.isDir,
      status: node.status,
      depth,
    })
    if (node.children) {
      result.push(...flattenTree(node.children, depth + 1))
    }
  }
  return result
}

function parseDiffStats(diff: string): { filesChanged: number; additions: number; deletions: number } | null {
  if (!diff) return null
  const lines = diff.split('\n')
  let additions = 0
  let deletions = 0
  const files = new Set<string>()
  for (const line of lines) {
    if (line.startsWith('diff --git')) {
      const match = line.match(/b\/(.+)$/)
      if (match) files.add(match[1])
    } else if (line.startsWith('+') && !line.startsWith('+++')) {
      additions++
    } else if (line.startsWith('-') && !line.startsWith('---')) {
      deletions++
    }
  }
  if (files.size === 0 && additions === 0 && deletions === 0) return null
  return { filesChanged: files.size, additions, deletions }
}

function guessLanguage(path: string): string {
  if (!path) return 'plaintext'
  const ext = path.split('.').pop()?.toLowerCase()
  const map: Record<string, string> = {
    ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
    go: 'go', py: 'python', rs: 'rust', md: 'markdown', json: 'json',
    yaml: 'yaml', yml: 'yaml', css: 'css', html: 'html', sh: 'shell',
    sql: 'sql', toml: 'toml', dockerfile: 'dockerfile',
  }
  return map[ext ?? ''] ?? 'plaintext'
}
