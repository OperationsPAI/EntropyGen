import { useMemo } from 'react'
import styles from './ObserveDetail.module.css'

interface SmartContentProps {
  text: string
  maxLen?: number
}

type ContentType = 'json' | 'markdown' | 'plain'

/** Detect whether text is JSON, markdown, or plain text. */
function detectContentType(text: string): ContentType {
  const trimmed = text.trim()

  // Try JSON first — must start with { or [
  if ((trimmed.startsWith('{') || trimmed.startsWith('[')) && trimmed.length > 2) {
    try {
      JSON.parse(trimmed)
      return 'json'
    } catch {
      // not valid JSON, continue
    }
  }

  // Detect markdown patterns: headers, code blocks, lists, links, bold/italic
  const mdPatterns = [
    /^#{1,6}\s/m,              // headers
    /^```/m,                   // code blocks
    /^\s*[-*+]\s/m,            // unordered lists
    /^\s*\d+\.\s/m,            // ordered lists
    /\[.+?\]\(.+?\)/,          // links
    /\*\*.+?\*\*/,             // bold
    /^\s*>/m,                  // blockquotes
  ]

  let matchCount = 0
  for (const pat of mdPatterns) {
    if (pat.test(trimmed)) matchCount++
  }
  // Require at least 2 markdown patterns to classify as markdown
  if (matchCount >= 2) return 'markdown'
  // Or a single strong indicator: code block fences or headers
  if (/^```/m.test(trimmed) || /^#{1,3}\s.+/m.test(trimmed)) return 'markdown'

  return 'plain'
}

/** Render JSON with syntax highlighting. */
function JsonContent({ text }: { text: string }) {
  const formatted = useMemo(() => {
    try {
      return JSON.stringify(JSON.parse(text.trim()), null, 2)
    } catch {
      return text
    }
  }, [text])

  return (
    <pre className={styles.jsonContent}>
      <code>{formatted}</code>
    </pre>
  )
}

/** Simple markdown renderer — handles headers, code blocks, bold, inline code, links, lists, blockquotes. */
function MarkdownContent({ text }: { text: string }) {
  const html = useMemo(() => renderMarkdown(text), [text])
  return (
    <div
      className={styles.markdownContent}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function renderMarkdown(text: string): string {
  const lines = text.split('\n')
  const result: string[] = []
  let inCodeBlock = false
  let codeLines: string[] = []
  let inList = false
  let listType: 'ul' | 'ol' = 'ul'

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    // Code blocks
    if (line.trimStart().startsWith('```')) {
      if (!inCodeBlock) {
        if (inList) { result.push(listType === 'ul' ? '</ul>' : '</ol>'); inList = false }
        inCodeBlock = true
        codeLines = []
      } else {
        result.push(`<pre class="${styles.mdCodeBlock}"><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`)
        inCodeBlock = false
      }
      continue
    }
    if (inCodeBlock) {
      codeLines.push(line)
      continue
    }

    // Close list if line is not a list item
    const isUnorderedItem = /^\s*[-*+]\s/.test(line)
    const isOrderedItem = /^\s*\d+\.\s/.test(line)
    if (inList && !isUnorderedItem && !isOrderedItem && line.trim() !== '') {
      result.push(listType === 'ul' ? '</ul>' : '</ol>')
      inList = false
    }

    // Empty line
    if (line.trim() === '') {
      if (inList) { result.push(listType === 'ul' ? '</ul>' : '</ol>'); inList = false }
      continue
    }

    // Headers
    const headerMatch = line.match(/^(#{1,6})\s+(.+)/)
    if (headerMatch) {
      const level = headerMatch[1].length
      result.push(`<h${level} class="${styles.mdHeader}">${inlineFormat(headerMatch[2])}</h${level}>`)
      continue
    }

    // Blockquote
    if (line.trimStart().startsWith('> ')) {
      result.push(`<blockquote class="${styles.mdBlockquote}">${inlineFormat(escapeHtml(line.replace(/^\s*>\s?/, '')))}</blockquote>`)
      continue
    }

    // Unordered list
    if (isUnorderedItem) {
      if (!inList || listType !== 'ul') {
        if (inList) result.push(listType === 'ul' ? '</ul>' : '</ol>')
        result.push('<ul class="' + styles.mdList + '">')
        inList = true
        listType = 'ul'
      }
      result.push(`<li>${inlineFormat(escapeHtml(line.replace(/^\s*[-*+]\s/, '')))}</li>`)
      continue
    }

    // Ordered list
    if (isOrderedItem) {
      if (!inList || listType !== 'ol') {
        if (inList) result.push(listType === 'ul' ? '</ul>' : '</ol>')
        result.push('<ol class="' + styles.mdList + '">')
        inList = true
        listType = 'ol'
      }
      result.push(`<li>${inlineFormat(escapeHtml(line.replace(/^\s*\d+\.\s/, '')))}</li>`)
      continue
    }

    // Paragraph
    result.push(`<p class="${styles.mdParagraph}">${inlineFormat(escapeHtml(line))}</p>`)
  }

  // Close unclosed blocks
  if (inCodeBlock) {
    result.push(`<pre class="${styles.mdCodeBlock}"><code>${escapeHtml(codeLines.join('\n'))}</code></pre>`)
  }
  if (inList) {
    result.push(listType === 'ul' ? '</ul>' : '</ol>')
  }

  return result.join('\n')
}

/** Apply inline formatting: bold, italic, inline code, links. */
function inlineFormat(text: string): string {
  return text
    // inline code (must be before bold/italic to avoid conflicts)
    .replace(/`([^`]+?)`/g, `<code class="${styles.mdInlineCode}">$1</code>`)
    // bold
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    // italic
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    // links
    .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
}

export default function SmartContent({ text, maxLen }: SmartContentProps) {
  const truncated = maxLen && text.length > maxLen ? text.slice(0, maxLen) + '...' : text
  const contentType = useMemo(() => detectContentType(truncated), [truncated])

  switch (contentType) {
    case 'json':
      return <JsonContent text={truncated} />
    case 'markdown':
      return <MarkdownContent text={truncated} />
    default:
      return <span className={styles.plainContent}>{truncated}</span>
  }
}
