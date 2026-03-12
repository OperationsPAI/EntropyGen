import MonacoEditor from '../editor/MonacoEditor'
import { Button } from '../ui'
import styles from './FileEditor.module.css'

export interface AgentFile {
  name: string
  language: string
  content: string
  readOnly: boolean
  description: string
  warning?: string
}

interface FileEditorProps {
  files: AgentFile[]
  activeFile: string
  onSelectFile: (name: string) => void
  onChangeContent: (name: string, content: string) => void
  onSave: (name: string) => void
  saving: boolean
  dirty: Record<string, boolean>
}

export default function FileEditor({
  files,
  activeFile,
  onSelectFile,
  onChangeContent,
  onSave,
  saving,
  dirty,
}: FileEditorProps) {
  const current = files.find((f) => f.name === activeFile)

  return (
    <div className={styles.container}>
      <div className={styles.fileList}>
        <div className={styles.fileListHeader}>Files</div>
        {files.map((f) => (
          <button
            key={f.name}
            className={`${styles.fileItem} ${f.name === activeFile ? styles.fileItemActive : ''}`}
            onClick={() => onSelectFile(f.name)}
          >
            {dirty[f.name] && <span className={styles.dirtyDot} />}
            <span>{f.name}</span>
            {f.readOnly && <span className={styles.readOnlyBadge}>RO</span>}
          </button>
        ))}
      </div>

      {current && (
        <div className={styles.editorPane}>
          <div className={styles.editorHeader}>
            <span className={styles.editorFileName}>{current.name}</span>
            <span className={styles.editorDesc}>{current.description}</span>
            {current.warning && <span className={styles.warningBadge}>{current.warning}</span>}
            {!current.readOnly && (
              <div className={styles.saveBtn}>
                <Button
                  size="sm"
                  onClick={() => onSave(current.name)}
                  loading={saving}
                  disabled={!dirty[current.name]}
                >
                  Save
                </Button>
              </div>
            )}
          </div>
          <div className={styles.editorBody}>
            <MonacoEditor
              value={current.content}
              onChange={current.readOnly ? undefined : (v) => onChangeContent(current.name, v)}
              language={current.language}
              readOnly={current.readOnly}
              height="100%"
            />
          </div>
        </div>
      )}
    </div>
  )
}
