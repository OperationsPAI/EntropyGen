import { useToastStore } from '../stores/toastStore'

export function useToast() {
  const addToast = useToastStore((state) => state.addToast)

  return {
    success: (title: string, description?: string) =>
      addToast({ type: 'success', title, description }),
    error: (title: string, description?: string) =>
      addToast({ type: 'error', title, description }),
    info: (title: string, description?: string) =>
      addToast({ type: 'info', title, description }),
  }
}
