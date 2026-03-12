import ReactECharts from '../../components/ReactECharts'

function genDays(n: number) {
  return Array.from({ length: n }, (_, i) => {
    const d = new Date()
    d.setDate(d.getDate() - n + i + 1)
    return d.toLocaleDateString('en-US', { month: 'numeric', day: 'numeric' })
  })
}

const AGENTS = ['observer-1', 'developer-1', 'reviewer-1', 'sre-1']
const DAYS30 = genDays(30)
const HOURS = Array.from({ length: 24 }, (_, i) => `${i}:00`)

export default function Monitor() {
  const tokenTrendOption = {
    tooltip: { trigger: 'axis' },
    legend: { data: AGENTS, bottom: 0 },
    grid: { top: 16, bottom: 40, left: 0, right: 0, containLabel: true },
    xAxis: { type: 'category', data: DAYS30, axisLabel: { fontSize: 10, color: '#665f58' }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } }, axisLabel: { fontSize: 10, color: '#665f58' } },
    series: AGENTS.map((a) => ({ name: a, type: 'line', smooth: true, symbol: 'none', lineStyle: { width: 2 }, data: Array.from({ length: 30 }, () => Math.floor(Math.random() * 50000 + 5000)) })),
  }

  const heatmapOption = {
    tooltip: {},
    grid: { top: 16, bottom: 40, left: 80, right: 16 },
    xAxis: { type: 'category', data: HOURS, axisLabel: { fontSize: 9, color: '#665f58' }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'category', data: AGENTS, axisLabel: { fontSize: 10, color: '#665f58' }, axisLine: { show: false } },
    visualMap: { min: 0, max: 20, calculable: true, orient: 'horizontal', bottom: 0, left: 'center', inRange: { color: ['#dce5dc', '#2a402a'] } },
    series: [{ type: 'heatmap', data: AGENTS.flatMap((_, ai) => HOURS.map((_, hi) => [hi, ai, Math.floor(Math.random() * 20)])), label: { show: false } }],
  }

  const pieOption = {
    tooltip: { trigger: 'item' },
    legend: { bottom: 0 },
    series: [{ type: 'pie', radius: ['40%', '70%'], data: [{ name: 'gpt-4o', value: 45 }, { name: 'claude-3-5-sonnet', value: 30 }, { name: 'gpt-4o-mini', value: 25 }], label: { formatter: '{b}: {d}%' } }],
  }

  const latencyOption = {
    tooltip: { trigger: 'axis' },
    grid: { top: 16, bottom: 20, left: 0, right: 16, containLabel: true },
    xAxis: { type: 'category', data: DAYS30, axisLabel: { fontSize: 9, color: '#665f58' }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } }, axisLabel: { fontSize: 9, color: '#665f58', formatter: '{value}ms' } },
    series: [{ type: 'line', smooth: true, symbol: 'none', lineStyle: { width: 2, color: '#e5502b' }, data: Array.from({ length: 30 }, () => Math.floor(Math.random() * 1000 + 500)) }],
  }

  const rankOption = {
    tooltip: {},
    grid: { top: 16, bottom: 20, left: 0, right: 16, containLabel: true },
    xAxis: { type: 'value', axisLine: { show: false }, splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } }, axisLabel: { fontSize: 9, color: '#665f58' } },
    yAxis: { type: 'category', data: AGENTS, axisLabel: { fontSize: 10, color: '#665f58' }, axisLine: { show: false } },
    series: [{ type: 'bar', data: [12400, 45200, 8900, 0], itemStyle: { borderRadius: [0, 4, 4, 0], color: '#111' }, barMaxWidth: 20 }],
  }

  const card = (title: string, chart: React.ReactNode) => (
    <div style={{ backgroundColor: 'var(--bg-surface)', borderRadius: '24px', padding: '24px' }}>
      <div style={{ fontWeight: 600, fontSize: '0.9rem', marginBottom: '12px' }}>{title}</div>
      <div style={{ height: '200px' }}>{chart}</div>
    </div>
  )

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <h2 style={{ fontSize: '1.1rem', fontWeight: 600 }}>Monitoring Charts</h2>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
        {card('Token Usage (Last 30 Days)', <ReactECharts option={tokenTrendOption} style={{ height: '100%' }} />)}
        {card('Activity Heatmap (by Hour)', <ReactECharts option={heatmapOption} style={{ height: '100%' }} />)}
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '12px' }}>
        {card('Model Distribution', <ReactECharts option={pieOption} style={{ height: '100%' }} />)}
        {card('Avg Latency Trend', <ReactECharts option={latencyOption} style={{ height: '100%' }} />)}
        {card('Agent Activity Ranking (Today)', <ReactECharts option={rankOption} style={{ height: '100%' }} />)}
      </div>
    </div>
  )
}
