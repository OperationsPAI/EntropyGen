interface PlatformConfig {
  gitea_base_url: string
}

let cached: PlatformConfig | null = null

export async function getPlatformConfig(): Promise<PlatformConfig> {
  if (cached) return cached
  const res = await fetch('/api/config')
  if (!res.ok) throw new Error('Failed to fetch config')
  cached = await res.json()
  return cached!
}

export function getGiteaBaseUrl(): string {
  return cached?.gitea_base_url ?? ''
}
