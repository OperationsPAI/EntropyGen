import { create } from 'zustand'

export interface Toast {
  id: string
  type: 'success' | 'error' | 'info'
  title: string
  description?: string
}

interface ToastStore {
  toasts: Toast[]
  addToast: (toast: Omit<Toast, 'id'>) => void
  removeToast: (id: string) => void
}

const AUTO_DISMISS_MS = 4_000
const MAX_VISIBLE = 5

let counter = 0

export const useToastStore = create<ToastStore>((set, get) => ({
  toasts: [],

  addToast: (toast) => {
    const id = `toast-${++counter}-${Date.now()}`
    const newToast: Toast = { ...toast, id }

    const current = get().toasts
    const updated = current.length >= MAX_VISIBLE
      ? [...current.slice(1), newToast]
      : [...current, newToast]

    set({ toasts: updated })

    setTimeout(() => {
      set((state) => ({
        toasts: state.toasts.filter((t) => t.id !== id),
      }))
    }, AUTO_DISMISS_MS)
  },

  removeToast: (id) => {
    set((state) => ({
      toasts: state.toasts.filter((t) => t.id !== id),
    }))
  },
}))
