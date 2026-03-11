import type { AgentPhase } from '../../types/agent'

const PHASE_STYLES: Record<AgentPhase, { bg: string; color: string; label: string }> = {
  Running:      { bg: '#dce5dc', color: '#2a402a', label: 'Running' },
  Error:        { bg: '#e5dcdc', color: '#402a2a', label: 'Error' },
  Paused:       { bg: '#e6e1dc', color: '#5c5752', label: 'Paused' },
  Initializing: { bg: '#d8e8f5', color: '#1a2e4a', label: 'Init' },
  Pending:      { bg: '#f5edcc', color: '#4a3a0a', label: 'Pending' },
}

export default function AgentPhaseTag({ phase }: { phase: AgentPhase }) {
  const style = PHASE_STYLES[phase] ?? PHASE_STYLES.Pending
  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', gap: '5px',
      padding: '3px 10px', borderRadius: '999px',
      backgroundColor: style.bg, color: style.color,
      fontSize: '0.75rem', fontWeight: 600,
    }}>
      <span style={{ width: '5px', height: '5px', borderRadius: '50%', backgroundColor: style.color }} />
      {style.label}
    </span>
  )
}
