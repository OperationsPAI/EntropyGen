import { useEffect, useRef } from 'react'
import * as echarts from 'echarts'

interface Props {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  option: Record<string, any>
  style?: React.CSSProperties
}

export default function ReactECharts({ option, style }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<echarts.ECharts | null>(null)

  useEffect(() => {
    if (!containerRef.current) return
    chartRef.current = echarts.init(containerRef.current)

    const observer = new ResizeObserver(() => chartRef.current?.resize())
    observer.observe(containerRef.current)

    return () => {
      observer.disconnect()
      chartRef.current?.dispose()
      chartRef.current = null
    }
  }, [])

  useEffect(() => {
    chartRef.current?.setOption(option, true)
  }, [option])

  return <div ref={containerRef} style={style} />
}
