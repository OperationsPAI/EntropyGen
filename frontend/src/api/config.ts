import { getConfig } from './generated/sdk.gen'

/* eslint-disable @typescript-eslint/no-explicit-any */

interface PlatformConfig {
  gitea_base_url: string
}

let cached: PlatformConfig | null = null

export async function getPlatformConfig(): Promise<PlatformConfig> {
  if (cached) return cached
  const r = await getConfig()
  const body = r.data as any
  cached = (body?.data ?? body) as PlatformConfig
  return cached!
}

export function getGiteaBaseUrl(): string {
  return cached?.gitea_base_url ?? ''
}
