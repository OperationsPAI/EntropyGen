import { create } from 'zustand'
import type { Agent, AgentPhase } from '../types/agent'

interface AgentStore {
  agents: Agent[]
  loading: boolean
  error: string | null
  setAgents: (agents: Agent[]) => void
  setLoading: (loading: boolean) => void
  setError: (error: string | null) => void
  updateAgentTokens: (agentId: string, todayTokens: number) => void
  updateAgentPhase: (agentId: string, phase: AgentPhase) => void
}

export const useAgentStore = create<AgentStore>((set) => ({
  agents: [],
  loading: false,
  error: null,

  setAgents: (agents) => set({ agents }),
  setLoading: (loading) => set({ loading }),
  setError: (error) => set({ error }),

  updateAgentTokens: (agentId, todayTokens) =>
    set((state) => ({
      agents: state.agents.map((agent) =>
        agent.name === agentId
          ? {
              ...agent,
              status: {
                ...agent.status,
                tokenUsage: { ...agent.status.tokenUsage, today: todayTokens },
              },
            }
          : agent
      ),
    })),

  updateAgentPhase: (agentId, phase) =>
    set((state) => ({
      agents: state.agents.map((agent) =>
        agent.name === agentId
          ? { ...agent, status: { ...agent.status, phase } }
          : agent
      ),
    })),
}))
