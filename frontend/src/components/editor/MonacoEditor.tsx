import Editor, { type EditorProps } from '@monaco-editor/react'

interface Props extends Omit<Partial<EditorProps>, 'onChange'> {
  value: string
  onChange?: (value: string) => void
  language?: string
  readOnly?: boolean
  height?: string
}

export default function MonacoEditor({
  value,
  onChange,
  language = 'markdown',
  readOnly = false,
  height = '400px',
  ...rest
}: Props) {
  return (
    <Editor
      height={height}
      language={language}
      value={value}
      onChange={(v) => onChange?.(v ?? '')}
      options={{
        readOnly,
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        fontSize: 13,
        lineNumbers: 'off',
        wordWrap: 'on',
        padding: { top: 12, bottom: 12 },
        renderLineHighlight: 'none',
        overviewRulerLanes: 0,
      }}
      theme="light"
      {...rest}
    />
  )
}
