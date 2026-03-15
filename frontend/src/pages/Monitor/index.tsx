import { useEffect, useState } from 'react'
import ReactECharts from '../../components/ReactECharts'
import { PageHeader, Card, EmptyState } from '../../components/ui'
import { monitorApi } from '../../api/monitor'
import type { TokenUsageSummary, AgentActivityPoint, ModelDistributionItem, LatencyPoint, AgentRankingItem } from '../../api/monitor'
import styles from './Monitor.module.css'

const HOURS = Array.from({ length: 24 }, (_, i) => `${i}:00`)

/** Strip "agent-" prefix for display */
function shortName(agentId: string): string {
  return agentId.startsWith('agent-') ? agentId.slice(6) : agentId
}

/** Format date string (2026-03-15T00:00:00Z -> 3/15) */
function shortDate(dateStr: string): string {
  const d = new Date(dateStr)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

export default function Monitor() {
  const [tokenData, setTokenData] = useState<TokenUsageSummary[]>([])
  const [activityData, setActivityData] = useState<AgentActivityPoint[]>([])
  const [modelData, setModelData] = useState<ModelDistributionItem[]>([])
  const [latencyData, setLatencyData] = useState<LatencyPoint[]>([])
  const [rankingData, setRankingData] = useState<AgentRankingItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([
      monitorApi.getTokenTrend(30).catch(() => []),
      monitorApi.getActivityHeatmap(7).catch(() => []),
      monitorApi.getModelDistribution(30).catch(() => []),
      monitorApi.getLatencyTrend(30).catch(() => []),
      monitorApi.getAgentRanking().catch(() => []),
    ]).then(([tokens, activity, models, latency, ranking]) => {
      setTokenData(tokens)
      setActivityData(activity)
      setModelData(models)
      setLatencyData(latency)
      setRankingData(ranking)
    }).finally(() => setLoading(false))
  }, [])

  return (
    <div className={styles.page}>
      <PageHeader title="Monitoring" />

      {/* Row 1: Token Usage + Activity Heatmap */}
      <div className={styles.row2}>
        <Card title="Request Volume (Last 30 Days)">
          <div className={styles.chartTall}>
            {loading ? <LoadingPlaceholder /> : tokenData.length === 0 ? (
              <EmptyState title="No data" description="No agent activity data available yet" />
            ) : (
              <ReactECharts option={buildTokenTrendOption(tokenData)} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
        <Card title="Activity Heatmap (by Hour)">
          <div className={styles.chartTall}>
            {loading ? <LoadingPlaceholder /> : activityData.length === 0 ? (
              <EmptyState title="No data" description="No activity data available yet" />
            ) : (
              <ReactECharts option={buildHeatmapOption(activityData)} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
      </div>

      {/* Row 2: Model Distribution + Latency + Ranking */}
      <div className={styles.row3}>
        <Card title="Model Distribution">
          <div className={styles.chartShort}>
            {loading ? <LoadingPlaceholder /> : modelData.length === 0 ? (
              <EmptyState title="No data" description="No model usage data yet" />
            ) : (
              <ReactECharts option={buildPieOption(modelData)} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
        <Card title="Latency Trend">
          <div className={styles.chartShort}>
            {loading ? <LoadingPlaceholder /> : latencyData.length === 0 ? (
              <EmptyState title="No data" description="No latency data available yet" />
            ) : (
              <ReactECharts option={buildLatencyOption(latencyData)} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
        <Card title="Agent Ranking (Today)">
          <div className={styles.chartShort}>
            {loading ? <LoadingPlaceholder /> : rankingData.length === 0 ? (
              <EmptyState title="No data" description="No agent activity yet today" />
            ) : (
              <ReactECharts option={buildRankOption(rankingData)} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
      </div>
    </div>
  )
}

function LoadingPlaceholder() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: 'var(--text-muted)', fontSize: '0.875rem' }}>
      Loading...
    </div>
  )
}

function buildTokenTrendOption(data: TokenUsageSummary[]) {
  // Group by agent, aggregate daily request counts (more meaningful than 0-token sums)
  const agentDays = new Map<string, Map<string, number>>()
  for (const d of data) {
    const name = shortName(d.agent_id)
    if (!agentDays.has(name)) agentDays.set(name, new Map())
    const dateKey = shortDate(d.date)
    const existing = agentDays.get(name)!.get(dateKey) ?? 0
    agentDays.get(name)!.set(dateKey, existing + d.request_count)
  }

  const allDates = [...new Set(data.map((d) => shortDate(d.date)))].sort((a, b) => {
    const [am, ad] = a.split('/').map(Number)
    const [bm, bd] = b.split('/').map(Number)
    return am !== bm ? am - bm : ad - bd
  })
  const agents = [...agentDays.keys()].sort()

  return {
    tooltip: { trigger: 'axis' as const },
    legend: {
      data: agents,
      bottom: 0,
      type: 'scroll' as const,
      textStyle: { fontSize: 11 },
    },
    grid: { top: 16, bottom: 36, left: 8, right: 8, containLabel: true },
    xAxis: {
      type: 'category' as const,
      data: allDates,
      axisLabel: { fontSize: 10, color: '#665f58', interval: 'auto' as const },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value' as const,
      name: 'requests',
      nameTextStyle: { fontSize: 10, color: '#999' },
      axisLine: { show: false },
      splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } },
      axisLabel: { fontSize: 10, color: '#665f58' },
    },
    series: agents.map((a) => ({
      name: a,
      type: 'bar' as const,
      stack: 'total',
      data: allDates.map((date) => agentDays.get(a)?.get(date) ?? 0),
      emphasis: { focus: 'series' as const },
    })),
  }
}

function buildHeatmapOption(data: AgentActivityPoint[]) {
  const agents = [...new Set(data.map((d) => shortName(d.agent_id)))].sort()
  const maxCount = Math.max(1, ...data.map((d) => d.count))

  const heatData = data.map((d) => [d.hour, agents.indexOf(shortName(d.agent_id)), d.count])

  // Dynamic left margin based on longest agent name
  const maxLabelLen = Math.max(40, ...agents.map((a) => a.length * 7))

  return {
    tooltip: {
      formatter: (p: { data: number[] }) => {
        const [hour, agentIdx, count] = p.data
        return `${agents[agentIdx]}<br/>${hour}:00 — ${count} requests`
      },
    },
    grid: { top: 8, bottom: 40, left: Math.min(maxLabelLen, 120), right: 16 },
    xAxis: {
      type: 'category' as const,
      data: HOURS,
      axisLabel: { fontSize: 9, color: '#665f58', interval: 1 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitArea: { show: true },
    },
    yAxis: {
      type: 'category' as const,
      data: agents,
      axisLabel: {
        fontSize: 10,
        color: '#665f58',
        width: 100,
        overflow: 'truncate' as const,
      },
      axisLine: { show: false },
    },
    visualMap: {
      min: 0,
      max: maxCount,
      calculable: true,
      orient: 'horizontal' as const,
      bottom: 0,
      left: 'center',
      itemWidth: 12,
      itemHeight: 80,
      textStyle: { fontSize: 10 },
      inRange: { color: ['#f5f0eb', '#c4d7c4', '#6b9b6b', '#2a402a'] },
    },
    series: [{ type: 'heatmap' as const, data: heatData, label: { show: false } }],
  }
}

function buildPieOption(data: ModelDistributionItem[]) {
  return {
    tooltip: { trigger: 'item' as const },
    legend: { bottom: 0, textStyle: { fontSize: 11 } },
    series: [{
      type: 'pie' as const,
      radius: ['40%', '70%'],
      center: ['50%', '45%'],
      data: data.map((d) => ({ name: d.model, value: d.count })),
      label: { formatter: '{b}\n{d}%', fontSize: 11 },
    }],
  }
}

function buildLatencyOption(data: LatencyPoint[]) {
  return {
    tooltip: { trigger: 'axis' as const },
    legend: { data: ['avg', 'p95'], bottom: 0, textStyle: { fontSize: 11 } },
    grid: { top: 16, bottom: 36, left: 8, right: 16, containLabel: true },
    xAxis: {
      type: 'category' as const,
      data: data.map((d) => shortDate(d.date)),
      axisLabel: { fontSize: 9, color: '#665f58' },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    yAxis: {
      type: 'value' as const,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } },
      axisLabel: { fontSize: 9, color: '#665f58', formatter: '{value}ms' },
    },
    series: [
      { name: 'avg', type: 'line' as const, smooth: true, symbol: 'none', lineStyle: { width: 2 }, areaStyle: { opacity: 0.1 }, data: data.map((d) => Math.round(d.avg_ms)) },
      { name: 'p95', type: 'line' as const, smooth: true, symbol: 'none', lineStyle: { width: 2, color: '#e5502b', type: 'dashed' as const }, data: data.map((d) => Math.round(d.p95_ms)) },
    ],
  }
}

function buildRankOption(data: AgentRankingItem[]) {
  // Take top 10, sort ascending for horizontal bar chart
  const top = [...data].slice(0, 10).sort((a, b) => a.total_usage - b.total_usage)
  const names = top.map((d) => shortName(d.agent_id))

  // Dynamic left margin
  const maxLabelLen = Math.max(40, ...names.map((n) => n.length * 7))

  return {
    tooltip: {
      formatter: (p: { name: string; value: number }) => `${p.name}: ${p.value.toLocaleString()} requests`,
    },
    grid: { top: 8, bottom: 8, left: Math.min(maxLabelLen, 100), right: 16 },
    xAxis: {
      type: 'value' as const,
      axisLine: { show: false },
      splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } },
      axisLabel: { fontSize: 9, color: '#665f58' },
    },
    yAxis: {
      type: 'category' as const,
      data: names,
      axisLabel: { fontSize: 11, color: '#665f58' },
      axisLine: { show: false },
    },
    series: [{
      type: 'bar' as const,
      data: top.map((d) => d.total_usage),
      itemStyle: { borderRadius: [0, 4, 4, 0], color: '#2a402a' },
      barMaxWidth: 20,
      label: {
        show: true,
        position: 'right' as const,
        fontSize: 10,
        color: '#665f58',
        formatter: (p: { value: number }) => p.value.toLocaleString(),
      },
    }],
  }
}
