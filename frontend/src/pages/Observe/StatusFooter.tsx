import type { ReactNode } from 'react'
import type { AggregatedTokenUsage } from './tokenUtils'
import type { CurrentTask } from '../../types/agent'
import styles from './ObserveDetail.module.css'

interface StatusFooterProps {
  statusIcon: ReactNode
  statusText: string
  tokenUsage: AggregatedTokenUsage
  repo?: string
  currentTask?: CurrentTask
  giteaBaseUrl?: string
}

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`
  return String(n)
}

export default function StatusFooter({
  statusIcon,
  statusText,
  tokenUsage,
  repo,
  currentTask,
  giteaBaseUrl,
}: StatusFooterProps) {
  return (
    <div className={styles.statusFooter}>
      <div className={styles.statusFooterLeft}>
        <span className={styles.statusFooterIcon}>{statusIcon}</span>
        <span className={styles.statusFooterText}>{statusText}</span>
      </div>
      <div className={styles.statusFooterRight}>
        {repo && (
          <>
            <span className={styles.statusFooterItem}>{repo}</span>
            <span className={styles.statusFooterSep}>|</span>
          </>
        )}
        {currentTask && giteaBaseUrl && (
          <>
            <a
              href={`${giteaBaseUrl}/${currentTask.repo || repo}/${currentTask.type === 'pr' ? 'pulls' : 'issues'}/${currentTask.number}`}
              target="_blank"
              rel="noopener noreferrer"
              className={styles.statusFooterCurrentTask}
            >
              {currentTask.type === 'pr' ? 'PR' : '#'}{currentTask.number}
              {currentTask.title && ` \u00B7 ${currentTask.title}`}
            </a>
            <span className={styles.statusFooterSep}>|</span>
          </>
        )}
        <span className={styles.statusFooterItem}>
          Token: {formatTokenCount(tokenUsage.totalTokens)}
          {tokenUsage.totalTokens > 0 && (
            <span className={styles.statusFooterTokenDetail}>
              {' '}(in:{formatTokenCount(tokenUsage.inputTokens)} / out:{formatTokenCount(tokenUsage.outputTokens)})
            </span>
          )}
        </span>
      </div>
    </div>
  )
}
