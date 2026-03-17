import { useRef, useCallback, useEffect, type ReactNode } from 'react'
import styles from './SplitPane.module.css'

interface SplitPaneProps {
  left: ReactNode
  right: ReactNode | null
  defaultLeftWidth?: number
  minLeftWidth?: number
  minRightWidth?: number
  /** When set, the right panel gets a fixed default width (percentage) and the left panel fills remaining space */
  defaultRightWidth?: number
  className?: string
}

export default function SplitPane({
  left,
  right,
  defaultLeftWidth = 50,
  minLeftWidth = 300,
  minRightWidth = 320,
  defaultRightWidth,
  className,
}: SplitPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const leftRef = useRef<HTMLDivElement>(null)
  const rightRef = useRef<HTMLDivElement>(null)
  const dividerRef = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)

  // When defaultRightWidth is set, the right panel is sized and left fills remaining space
  const rightAnchored = defaultRightWidth !== undefined

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault()
      dragging.current = true
      dividerRef.current?.classList.add(styles.dividerActive)

      const onMouseMove = (ev: MouseEvent) => {
        if (!dragging.current || !containerRef.current || !leftRef.current) return
        const rect = containerRef.current.getBoundingClientRect()
        const maxLeft = rect.width - minRightWidth - 4
        const next = Math.min(maxLeft, Math.max(minLeftWidth, ev.clientX - rect.left))
        leftRef.current.style.width = `${next}px`
        if (rightAnchored && rightRef.current) {
          rightRef.current.style.width = `${rect.width - next - 4}px`
        }
      }

      const onMouseUp = () => {
        dragging.current = false
        dividerRef.current?.classList.remove(styles.dividerActive)
        document.removeEventListener('mousemove', onMouseMove)
        document.removeEventListener('mouseup', onMouseUp)
      }

      document.addEventListener('mousemove', onMouseMove)
      document.addEventListener('mouseup', onMouseUp)
    },
    [minLeftWidth, minRightWidth, rightAnchored],
  )

  useEffect(() => {
    if (!containerRef.current || !leftRef.current) return
    if (right === null) {
      leftRef.current.style.width = '100%'
    } else if (rightAnchored && rightRef.current) {
      // Right-anchored mode: size right panel first, left gets remainder
      const containerWidth = containerRef.current.getBoundingClientRect().width
      const rightInitial = Math.max(minRightWidth, (containerWidth * defaultRightWidth) / 100)
      const leftInitial = Math.max(minLeftWidth, containerWidth - rightInitial - 4)
      leftRef.current.style.width = `${leftInitial}px`
      rightRef.current.style.width = `${rightInitial}px`
    } else {
      const containerWidth = containerRef.current.getBoundingClientRect().width
      const initial = Math.max(minLeftWidth, (containerWidth * defaultLeftWidth) / 100)
      leftRef.current.style.width = `${initial}px`
    }
  }, [right === null, defaultLeftWidth, defaultRightWidth, minLeftWidth, minRightWidth, rightAnchored])

  const collapsed = right === null

  return (
    <div ref={containerRef} className={`${styles.container} ${className ?? ''}`}>
      <div ref={leftRef} className={rightAnchored ? styles.leftFill : styles.left} style={collapsed ? { width: '100%' } : undefined}>
        {left}
      </div>
      {!collapsed && (
        <>
          <div
            ref={dividerRef}
            className={styles.divider}
            onMouseDown={handleMouseDown}
          />
          <div ref={rightRef} className={rightAnchored ? styles.rightFixed : styles.right}>{right}</div>
        </>
      )}
    </div>
  )
}
