import { useState, useEffect } from 'react'
import MonacoEditor from '../../components/editor/MonacoEditor'
import EditorTabs from './EditorTabs'
import DiffView from './DiffView'
import { observeApi } from '../../api/observe'
import type { DiffResultResponse } from '../../types/observe'
import styles from './ObserveDetail.module.css'

interface EditorPanelProps {
  agentName: string
  activePath: string
  activeTab: 'code' | 'diff'
  onTabChange: (tab: 'code' | 'diff') => void
  diffData: DiffResultResponse | null
  followMode: boolean
  contentRefreshKey: number
}

export default function EditorPanel({
  agentName,
  activePath,
  activeTab,
  onTabChange,
  diffData,
  followMode,
  contentRefreshKey,
}: EditorPanelProps) {
  const [fileContent, setFileContent] = useState('')
  const [language, setLanguage] = useState('plaintext')
  const [loading, setLoading] = useState(false)

  // Load file content when path or refresh key changes
  useEffect(() => {
    if (!activePath) return
    let cancelled = false
    setLoading(true)
    observeApi.getWorkspaceFile(agentName, activePath)
      .then((res) => {
        if (!cancelled) {
          setFileContent(res.content)
          setLanguage(res.language || guessLanguage(activePath))
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFileContent('// Unable to load file')
          setLanguage(guessLanguage(activePath))
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [agentName, activePath, contentRefreshKey])

  // In follow mode, auto-reload current file every 5s
  useEffect(() => {
    if (!followMode || !activePath) return
    const timer = setInterval(() => {
      observeApi.getWorkspaceFile(agentName, activePath)
        .then((res) => {
          setFileContent(res.content)
          setLanguage(res.language || guessLanguage(activePath))
        })
        .catch(() => {})
    }, 5_000)
    return () => clearInterval(timer)
  }, [followMode, agentName, activePath])

  const diffStats = diffData
    ? { filesChanged: diffData.files_changed, insertions: diffData.insertions, deletions: diffData.deletions }
    : null

  return (
    <div className={styles.editorPanel}>
      <EditorTabs
        activeTab={activeTab}
        onTabChange={onTabChange}
        filePath={activePath}
        diffStats={diffStats}
      />
      <div className={styles.editorContent}>
        {activeTab === 'code' ? (
          loading ? (
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
          )
        ) : (
          <DiffView diff={diffData?.diff ?? ''} />
        )}
      </div>
    </div>
  )
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
