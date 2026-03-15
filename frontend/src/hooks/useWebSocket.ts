import { useEffect, useRef, useCallback } from 'react'
import { useAgentStore } from '../stores/agentStore'
import { useEventStore } from '../stores/eventStore'
import { useAlertStore } from '../stores/alertStore'
import { useWsStore } from '../stores/wsStore'
import type { RealtimeEvent, AlertEvent, LLMInferencePayload } from '../types/event'
import type { AgentPhase } from '../types/agent'

const MAX_RECONNECT = 5
const BACKOFF_DELAYS = [3000, 6000, 12000, 24000, 48000]

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const { updateAgentTokens, updateAgentPhase } = useAgentStore()
  const { addEvent } = useEventStore()
  const { addAlert } = useAlertStore()
  const { setConnected, incrementReconnect, resetReconnect } = useWsStore()

  const connect = useCallback(() => {
    const token = localStorage.getItem('jwt_token')
    if (!token) return

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/ws/events?token=${token}`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
      resetReconnect()
    }

    ws.onmessage = (evt) => {
      try {
        const event: RealtimeEvent = JSON.parse(evt.data as string)

        // Add all events to the live stream
        addEvent(event)

        if (event.event_type === 'gateway.llm_inference') {
          const payload = event.payload as unknown as LLMInferencePayload
          updateAgentTokens(event.agent_id, payload.tokens_in + payload.tokens_out)
        } else if (event.event_type === 'operator.agent_alert') {
          const alertEvent: AlertEvent = {
            id: crypto.randomUUID(),
            alert_type: event.payload.alert_type as AlertEvent['alert_type'],
            agent_id: event.agent_id,
            message: event.payload.message as string,
            timestamp: event.timestamp,
            details: event.payload,
          }
          addAlert(alertEvent)
          updateAgentPhase(event.agent_id, 'Error' as AgentPhase)
        }
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      setConnected(false)
      wsRef.current = null

      const count = useWsStore.getState().reconnectCount
      if (count < MAX_RECONNECT) {
        const delay = BACKOFF_DELAYS[count] ?? BACKOFF_DELAYS[BACKOFF_DELAYS.length - 1]
        incrementReconnect()
        reconnectTimerRef.current = setTimeout(connect, delay)
      }
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [addAlert, addEvent, incrementReconnect, resetReconnect, setConnected, updateAgentPhase, updateAgentTokens])

  useEffect(() => {
    connect()
    return () => {
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      wsRef.current?.close()
    }
  }, [connect])

  return { connected: useWsStore((s) => s.connected) }
}
