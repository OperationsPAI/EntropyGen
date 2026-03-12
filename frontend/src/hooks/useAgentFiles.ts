import { useState, useCallback, useMemo } from 'react'
import type { Agent, UpdateAgentDto } from '../types/agent'
import type { AgentFile } from '../components/agent/FileEditor'

interface UseAgentFilesReturn {
  files: AgentFile[]
  activeFile: string
  fileContents: Record<string, string>
  dirty: Record<string, boolean>
  setActiveFile: (name: string) => void
  updateContent: (name: string, content: string) => void
  getSavePayload: (name: string) => UpdateAgentDto | null
  markSaved: (name: string) => void
}

export function useAgentFiles(_agent: Agent): UseAgentFilesReturn {
  const [activeFile, setActiveFile] = useState('')
  const [fileContents, setFileContents] = useState<Record<string, string>>({})
  const [originals] = useState<Record<string, string>>({})

  const dirty = useMemo(() => {
    const d: Record<string, boolean> = {}
    for (const key of Object.keys(fileContents)) {
      d[key] = fileContents[key] !== originals[key]
    }
    return d
  }, [fileContents, originals])

  const files: AgentFile[] = useMemo(() => [], [])

  const updateContent = useCallback((name: string, content: string) => {
    setFileContents((prev) => ({ ...prev, [name]: content }))
  }, [])

  const getSavePayload = useCallback(
    (_name: string): UpdateAgentDto | null => {
      return null
    },
    [],
  )

  const markSaved = useCallback((name: string) => {
    setFileContents((prev) => {
      const val = prev[name]
      originals[name] = val
      return { ...prev }
    })
  }, [originals])

  return { files, activeFile, fileContents, dirty, setActiveFile, updateContent, getSavePayload, markSaved }
}
