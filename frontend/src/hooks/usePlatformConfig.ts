import { useState, useEffect } from 'react'
import { getPlatformConfig } from '../api/config'

interface PlatformConfig {
  gitea_base_url: string
}

export function usePlatformConfig(): PlatformConfig | null {
  const [config, setConfig] = useState<PlatformConfig | null>(null)
  useEffect(() => {
    getPlatformConfig().then(setConfig).catch(() => {})
  }, [])
  return config
}
