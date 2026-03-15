import { useEffect, useState } from 'react'
import ReactECharts from '../../components/ReactECharts'
import { PageHeader, Card, EmptyState } from '../../components/ui'
import { monitorApi } from '../../api/monitor'
import type { TokenUsageSummary, AgentActivityPoint, ModelDistributionItem, LatencyPoint, AgentRankingItem } from '../../api/monitor'

const HOURS = Array.from({ length: 24 }, (_, i) => `${i}:00`)

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

  const tokenTrendOption = buildTokenTrendOption(tokenData)
  const heatmapOption = buildHeatmapOption(activityData)
  const pieOption = buildPieOption(modelData)
  const latencyOption = buildLatencyOption(latencyData)
  const rankOption = buildRankOption(rankingData)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <PageHeader title="Monitoring" />
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
        <Card title="Token Usage (Last 30 Days)">
          <div style={{ height: '200px' }}>
            {loading ? <LoadingPlaceholder /> : tokenData.length === 0 ? (
              <EmptyState title="No data" description="No token usage data available yet" />
            ) : (
              <ReactECharts option={tokenTrendOption} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
        <Card title="Activity Heatmap (by Hour)">
          <div style={{ height: '200px' }}>
            {loading ? <LoadingPlaceholder /> : activityData.length === 0 ? (
              <EmptyState title="No data" description="No activity data available yet" />
            ) : (
              <ReactECharts option={heatmapOption} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '12px' }}>
        <Card title="Model Distribution">
          <div style={{ height: '200px' }}>
            {loading ? <LoadingPlaceholder /> : modelData.length === 0 ? (
              <EmptyState title="No data" description="No model usage data yet" />
            ) : (
              <ReactECharts option={pieOption} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
        <Card title="Avg Latency Trend">
          <div style={{ height: '200px' }}>
            {loading ? <LoadingPlaceholder /> : latencyData.length === 0 ? (
              <EmptyState title="No data" description="No latency data available yet" />
            ) : (
              <ReactECharts option={latencyOption} style={{ height: '100%' }} />
            )}
          </div>
        </Card>
        <Card title="Agent Activity Ranking (Today)">
          <div style={{ height: '200px' }}>
            {loading ? <LoadingPlaceholder /> : rankingData.length === 0 ? (
              <EmptyState title="No data" description="No agent activity yet today" />
            ) : (
              <ReactECharts option={rankOption} style={{ height: '100%' }} />
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
  // Group by agent, aggregate daily tokens
  const agentDays = new Map<string, Map<string, number>>()
  for (const d of data) {
    if (!agentDays.has(d.agent_id)) agentDays.set(d.agent_id, new Map())
    const existing = agentDays.get(d.agent_id)!.get(d.date) ?? 0
    agentDays.get(d.agent_id)!.set(d.date, existing + d.tokens_in + d.tokens_out)
  }

  const allDates = [...new Set(data.map((d) => d.date))].sort()
  const agents = [...agentDays.keys()].sort()

  return {
    tooltip: { trigger: 'axis' as const },
    legend: { data: agents, bottom: 0 },
    grid: { top: 16, bottom: 40, left: 0, right: 0, containLabel: true },
    xAxis: { type: 'category' as const, data: allDates, axisLabel: { fontSize: 10, color: '#665f58' }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'value' as const, axisLine: { show: false }, splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } }, axisLabel: { fontSize: 10, color: '#665f58' } },
    series: agents.map((a) => ({
      name: a,
      type: 'line' as const,
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2 },
      data: allDates.map((date) => agentDays.get(a)?.get(date) ?? 0),
    })),
  }
}

function buildHeatmapOption(data: AgentActivityPoint[]) {
  const agents = [...new Set(data.map((d) => d.agent_id))].sort()
  const maxCount = Math.max(1, ...data.map((d) => d.count))

  const heatData = data.map((d) => [d.hour, agents.indexOf(d.agent_id), d.count])

  return {
    tooltip: {},
    grid: { top: 16, bottom: 40, left: 80, right: 16 },
    xAxis: { type: 'category' as const, data: HOURS, axisLabel: { fontSize: 9, color: '#665f58' }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'category' as const, data: agents, axisLabel: { fontSize: 10, color: '#665f58' }, axisLine: { show: false } },
    visualMap: { min: 0, max: maxCount, calculable: true, orient: 'horizontal' as const, bottom: 0, left: 'center', inRange: { color: ['#dce5dc', '#2a402a'] } },
    series: [{ type: 'heatmap' as const, data: heatData, label: { show: false } }],
  }
}

function buildPieOption(data: ModelDistributionItem[]) {
  return {
    tooltip: { trigger: 'item' as const },
    legend: { bottom: 0 },
    series: [{
      type: 'pie' as const,
      radius: ['40%', '70%'],
      data: data.map((d) => ({ name: d.model, value: d.count })),
      label: { formatter: '{b}: {d}%' },
    }],
  }
}

function buildLatencyOption(data: LatencyPoint[]) {
  return {
    tooltip: { trigger: 'axis' as const },
    legend: { data: ['avg', 'p95'], bottom: 0 },
    grid: { top: 16, bottom: 40, left: 0, right: 16, containLabel: true },
    xAxis: { type: 'category' as const, data: data.map((d) => d.date), axisLabel: { fontSize: 9, color: '#665f58' }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'value' as const, axisLine: { show: false }, splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } }, axisLabel: { fontSize: 9, color: '#665f58', formatter: '{value}ms' } },
    series: [
      { name: 'avg', type: 'line' as const, smooth: true, symbol: 'none', lineStyle: { width: 2 }, data: data.map((d) => Math.round(d.avg_ms)) },
      { name: 'p95', type: 'line' as const, smooth: true, symbol: 'none', lineStyle: { width: 2, color: '#e5502b' }, data: data.map((d) => Math.round(d.p95_ms)) },
    ],
  }
}

function buildRankOption(data: AgentRankingItem[]) {
  const sorted = [...data].sort((a, b) => a.total_usage - b.total_usage)
  return {
    tooltip: {},
    grid: { top: 16, bottom: 20, left: 0, right: 16, containLabel: true },
    xAxis: { type: 'value' as const, axisLine: { show: false }, splitLine: { lineStyle: { color: 'rgba(17,17,17,0.07)' } }, axisLabel: { fontSize: 9, color: '#665f58' } },
    yAxis: { type: 'category' as const, data: sorted.map((d) => d.agent_id), axisLabel: { fontSize: 10, color: '#665f58' }, axisLine: { show: false } },
    series: [{ type: 'bar' as const, data: sorted.map((d) => d.total_usage), itemStyle: { borderRadius: [0, 4, 4, 0], color: '#111' }, barMaxWidth: 20 }],
  }
}
