import { IconFile, IconSourceControl } from '@douyinfe/semi-icons'
import styles from './ObserveDetail.module.css'

interface EditorTabsProps {
  activeTab: 'code' | 'diff'
  onTabChange: (tab: 'code' | 'diff') => void
  filePath?: string
  diffStats?: { filesChanged: number; insertions: number; deletions: number } | null
}

export default function EditorTabs({
  activeTab,
  onTabChange,
  filePath,
  diffStats,
}: EditorTabsProps) {
  const fileName = filePath ? filePath.split('/').pop() : 'No file'

  return (
    <div className={styles.editorTabs}>
      <button
        className={`${styles.editorTab} ${activeTab === 'code' ? styles.editorTabActive : ''}`}
        onClick={() => onTabChange('code')}
      >
        <IconFile size="small" /> {fileName}
      </button>
      <button
        className={`${styles.editorTab} ${activeTab === 'diff' ? styles.editorTabActive : ''}`}
        onClick={() => onTabChange('diff')}
      >
        <IconSourceControl size="small" /> Diff
        {diffStats && (
          <span className={styles.diffTabStats}>
            <span className={styles.diffTabAdd}>+{diffStats.insertions}</span>
            <span className={styles.diffTabDel}>-{diffStats.deletions}</span>
          </span>
        )}
      </button>
    </div>
  )
}
