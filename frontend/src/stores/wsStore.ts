import { create } from 'zustand'

interface WsStore {
  connected: boolean
  reconnectCount: number
  setConnected: (connected: boolean) => void
  incrementReconnect: () => void
  resetReconnect: () => void
}

export const useWsStore = create<WsStore>((set) => ({
  connected: false,
  reconnectCount: 0,

  setConnected: (connected) => set({ connected }),
  incrementReconnect: () => set((state) => ({ reconnectCount: state.reconnectCount + 1 })),
  resetReconnect: () => set({ reconnectCount: 0 }),
}))
