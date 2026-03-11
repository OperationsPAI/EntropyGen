import { create } from 'zustand'
import type { RealtimeEvent } from '../types/event'

const MAX_EVENTS = 50

interface EventStore {
  events: RealtimeEvent[]
  addEvent: (event: RealtimeEvent) => void
  clearEvents: () => void
}

export const useEventStore = create<EventStore>((set) => ({
  events: [],

  addEvent: (event) =>
    set((state) => ({
      events: [event, ...state.events].slice(0, MAX_EVENTS),
    })),

  clearEvents: () => set({ events: [] }),
}))
