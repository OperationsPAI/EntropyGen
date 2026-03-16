export function giteaRepoUrl(base: string, repo: string): string {
  return `${base}/${repo}`
}

export function giteaIssuesUrl(base: string, repo: string, username?: string): string {
  const url = `${base}/${repo}/issues`
  return username ? `${url}?assignee=${encodeURIComponent(username)}` : url
}

export function giteaPRsUrl(base: string, repo: string, username?: string): string {
  const url = `${base}/${repo}/pulls`
  return username ? `${url}?assignee=${encodeURIComponent(username)}` : url
}

export function giteaCurrentTaskUrl(base: string, repo: string, type: 'issue' | 'pr', number: number): string {
  const path = type === 'pr' ? 'pulls' : 'issues'
  return `${base}/${repo}/${path}/${number}`
}
