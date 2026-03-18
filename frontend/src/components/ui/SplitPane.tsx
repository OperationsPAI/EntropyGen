import { useRef, useCallback, useEffect, type ReactNode } from 'react'
import styles from './SplitPane.module.css'

interface SplitPaneProps {
  left: ReactNode
  right: ReactNode | null
  /** Default left panel width as percentage of container (0-100) */
  defaultLeftWidth?: number
  minLeftWidth?: number
  minRightWidth?: number
  className?: string
}

export default function SplitPane({
  left,
  right,
  defaultLeftWidth = 50,
  minLeftWidth = 100,
  minRightWidth = 100,
  className,
}: SplitPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const leftRef = useRef<HTMLDivElement>(null)
  const dividerRef = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)
  // Store the ratio (0-1) so we can reapply on resize
  const ratioRef = useRef(defaultLeftWidth / 100)
  // Keep min values in refs so ResizeObserver callback stays stable
  const minLeftRef = useRef(minLeftWidth)
  const minRightRef = useRef(minRightWidth)
  minLeftRef.current = minLeftWidth
  minRightRef.current = minRightWidth

  const clampAndApply = useCallback(() => {
    if (!containerRef.current || !leftRef.current) return
    const containerWidth = containerRef.current.getBoundingClientRect().width
    const dividerWidth = 4
    const available = containerWidth - dividerWidth
    if (available <= 0) return
    const minL = Math.min(minLeftRef.current, available * 0.3)
    const minR = Math.min(minRightRef.current, available * 0.3)
    const maxLeft = available - minR
    const clamped = Math.min(maxLeft, Math.max(minL, available * ratioRef.current))
    leftRef.current.style.width = `${clamped}px`
  }, [])

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault()
      dragging.current = true
      dividerRef.current?.classList.add(styles.dividerActive)

      const onMouseMove = (ev: MouseEvent) => {
        if (!dragging.current || !containerRef.current || !leftRef.current) return
        const rect = containerRef.current.getBoundingClientRect()
        const dividerWidth = 4
        const available = rect.width - dividerWidth
        if (available <= 0) return
        const minL = Math.min(minLeftRef.current, available * 0.3)
        const minR = Math.min(minRightRef.current, available * 0.3)
        const maxLeft = available - minR
        const next = Math.min(maxLeft, Math.max(minL, ev.clientX - rect.left))
        leftRef.current.style.width = `${next}px`
        ratioRef.current = next / available
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
    [],
  )

  const collapsed = right === null

  // Initialize + respond to container resize
  useEffect(() => {
    if (!containerRef.current || !leftRef.current) return
    if (collapsed) {
      leftRef.current.style.width = '100%'
      return
    }

    ratioRef.current = defaultLeftWidth / 100
    clampAndApply()

    const container = containerRef.current
    const ro = new ResizeObserver(() => {
      if (!dragging.current) {
        clampAndApply()
      }
    })
    ro.observe(container)
    return () => ro.disconnect()
  }, [collapsed, defaultLeftWidth, clampAndApply])

  return (
    <div ref={containerRef} className={`${styles.container} ${className ?? ''}`}>
      <div
        ref={leftRef}
        className={styles.left}
        style={collapsed ? { width: '100%', flexShrink: 'unset' } : undefined}
      >
        {left}
      </div>
      {!collapsed && (
        <>
          <div
            ref={dividerRef}
            className={styles.divider}
            onMouseDown={handleMouseDown}
          />
          <div className={styles.right}>{right}</div>
        </>
      )}
    </div>
  )
}
