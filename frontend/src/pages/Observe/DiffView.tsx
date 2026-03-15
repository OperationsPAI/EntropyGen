import styles from './ObserveDetail.module.css'

interface DiffViewProps {
  diff: string
}

export default function DiffView({ diff }: DiffViewProps) {
  if (!diff) {
    return (
      <div className={styles.loadingState}>No diff available</div>
    )
  }

  const lines = diff.split('\n')

  return (
    <div className={styles.diffView}>
      {lines.map((line, i) => {
        let className = styles.diffLineContext
        if (line.startsWith('diff --git')) {
          className = styles.diffFileHeader
        } else if (line.startsWith('@@')) {
          className = styles.diffHunkHeader
        } else if (line.startsWith('+') && !line.startsWith('+++')) {
          className = styles.diffLineAdded
        } else if (line.startsWith('-') && !line.startsWith('---')) {
          className = styles.diffLineRemoved
        } else if (line.startsWith('+++') || line.startsWith('---')) {
          className = styles.diffFileHeaderPath
        }

        return (
          <div key={i} className={className}>
            <span className={styles.diffLineContent}>{line || '\u00A0'}</span>
          </div>
        )
      })}
    </div>
  )
}
