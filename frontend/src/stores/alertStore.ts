import { create } from 'zustand'
import type { AlertEvent } from '../types/event'

// 60s dedup window
const DEDUP_WINDOW_MS = 60_000

interface AlertStore {
  alerts: AlertEvent[]
  banner: AlertEvent | null
  // Map key: `${agent_id}:${alert_type}`, value: last timestamp ms
  _dedup: Map<string, number>
  addAlert: (alert: AlertEvent) => void
  dismissBanner: () => void
}

export const useAlertStore = create<AlertStore>((set, get) => ({
  alerts: [],
  banner: null,
  _dedup: new Map(),

  addAlert: (alert) => {
    const key = `${alert.agent_id}:${alert.alert_type}`
    const now = Date.now()
    const lastSeen = get()._dedup.get(key) ?? 0

    if (now - lastSeen < DEDUP_WINDOW_MS) {
      // silenced, same type already shown within 60s
      return
    }

    const newDedup = new Map(get()._dedup)
    newDedup.set(key, now)

    set((state) => ({
      alerts: [alert, ...state.alerts],
      banner: alert.alert_type === 'agent.crash_loop' ? alert : state.banner,
      _dedup: newDedup,
    }))
  },

  dismissBanner: () => set({ banner: null }),
}))
