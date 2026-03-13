import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { rolesApi } from '../../api/roles'
import MonacoEditor from '../../components/editor/MonacoEditor'
import { PageHeader, Card, Button, Modal, EmptyState } from '../../components/ui'
import { useToast } from '../../hooks/useToast'
import type { Role } from '../../types/agent'
import styles from './Editor.module.css'

function RoleEditorInner({ initialRole }: { initialRole: Role }) {
  const toast = useToast()
  const [role, setRole] = useState(initialRole)
  const [openTabs, setOpenTabs] = useState<string[]>(() => {
    const first = (initialRole.files ?? [])[0]
    return first ? [first.name] : []
  })
  const [activeTab, setActiveTab] = useState(() => (initialRole.files ?? [])[0]?.name ?? '')
  const [fileContents, setFileContents] = useState<Record<string, string>>(() => {
    const map: Record<string, string> = {}
    for (const f of initialRole.files ?? []) {
      map[f.name] = f.content
    }
    return map
  })
  const [originalContents, setOriginalContents] = useState<Record<string, string>>(() => {
    const map: Record<string, string> = {}
    for (const f of initialRole.files ?? []) {
      map[f.name] = f.content
    }
    return map
  })
  const [saving, setSaving] = useState(false)
  const [newFileName, setNewFileName] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  const isDirty = useCallback(
    (name: string) => fileContents[name] !== originalContents[name],
    [fileContents, originalContents],
  )

  const hasAnyDirty = Object.keys(fileContents).some(isDirty)

  useEffect(() => {
    if (!hasAnyDirty) return
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault()
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [hasAnyDirty])

  const handleSave = useCallback(async () => {
    if (!activeTab || !isDirty(activeTab)) return
    setSaving(true)
    try {
      await rolesApi.updateRoleFile(role.name, activeTab, fileContents[activeTab])
      setOriginalContents((prev) => ({ ...prev, [activeTab]: fileContents[activeTab] }))
      toast.success('File saved', activeTab)
    } catch {
      toast.error('Save failed', `Could not save ${activeTab}`)
    } finally {
      setSaving(false)
    }
  }, [activeTab, fileContents, isDirty, role.name, toast])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        handleSave()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [handleSave])

  const openFile = (name: string) => {
    if (!openTabs.includes(name)) {
      setOpenTabs((prev) => [...prev, name])
    }
    setActiveTab(name)
  }

  const closeTab = (name: string) => {
    const next = openTabs.filter((t) => t !== name)
    setOpenTabs(next)
    if (activeTab === name) {
      setActiveTab(next[next.length - 1] ?? '')
    }
  }

  const handleEditorChange = (value: string) => {
    if (!activeTab) return
    setFileContents((prev) => ({ ...prev, [activeTab]: value }))
  }

  const handleCreateFile = async (name: string) => {
    const trimmed = name.trim()
    if (!trimmed) {
      setNewFileName(null)
      return
    }
    if ((role.files ?? []).some((f) => f.name === trimmed)) {
      toast.error('File exists', `"${trimmed}" already exists in this role.`)
      return
    }
    try {
      await rolesApi.updateRoleFile(role.name, trimmed, '')
      setRole((prev) => ({
        ...prev,
        files: [...prev.files, { name: trimmed, content: '', updated_at: new Date().toISOString() }],
      }))
      setFileContents((prev) => ({ ...prev, [trimmed]: '' }))
      setOriginalContents((prev) => ({ ...prev, [trimmed]: '' }))
      setNewFileName(null)
      openFile(trimmed)
    } catch {
      toast.error('Create failed', `Could not create ${trimmed}`)
    }
  }

  const handleDeleteFile = async () => {
    if (!deleteTarget) return
    try {
      await rolesApi.deleteRoleFile(role.name, deleteTarget)
      setRole((prev) => ({
        ...prev,
        files: prev.files.filter((f) => f.name !== deleteTarget),
      }))
      setOpenTabs((prev) => prev.filter((t) => t !== deleteTarget))
      setFileContents((prev) => {
        const { [deleteTarget]: _, ...rest } = prev
        return rest
      })
      setOriginalContents((prev) => {
        const { [deleteTarget]: _, ...rest } = prev
        return rest
      })
      if (activeTab === deleteTarget) {
        const remaining = openTabs.filter((t) => t !== deleteTarget)
        setActiveTab(remaining[remaining.length - 1] ?? '')
      }
      toast.success('File deleted', deleteTarget)
    } catch {
      toast.error('Delete failed', `Could not delete ${deleteTarget}`)
    } finally {
      setDeleteTarget(null)
    }
  }

  const fileNames = (role.files ?? []).map((f) => f.name)

  return (
    <>
      <Card className={styles.editorCard}>
        <div className={styles.fileTree}>
          <div className={styles.fileTreeHeader}>Files</div>
          <div className={styles.fileTreeList}>
            {fileNames.map((name) => (
              <button
                key={name}
                className={`${styles.fileTreeItem} ${activeTab === name ? styles.fileTreeItemActive : ''}`}
                onClick={() => openFile(name)}
              >
                {isDirty(name) && <span className={styles.dirtyDot} />}
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>{name}</span>
                <span
                  className={styles.fileTreeItemDelete}
                  onClick={(e) => {
                    e.stopPropagation()
                    setDeleteTarget(name)
                  }}
                >
                  &times;
                </span>
              </button>
            ))}
          </div>
          <div className={styles.fileTreeDivider} />
          {newFileName !== null ? (
            <input
              className={styles.newFileInput}
              autoFocus
              value={newFileName}
              onChange={(e) => setNewFileName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleCreateFile(newFileName)
                if (e.key === 'Escape') setNewFileName(null)
              }}
              onBlur={() => {
                if (newFileName.trim()) {
                  handleCreateFile(newFileName)
                } else {
                  setNewFileName(null)
                }
              }}
              placeholder="filename.md"
            />
          ) : (
            <button className={styles.newFileBtn} onClick={() => setNewFileName('')}>
              + New File
            </button>
          )}
        </div>

        <div className={styles.editorPanel}>
          {openTabs.length > 0 && (
            <div className={styles.tabBar}>
              {openTabs.map((name) => (
                <button
                  key={name}
                  className={`${styles.tab} ${activeTab === name ? styles.tabActive : ''}`}
                  onClick={() => setActiveTab(name)}
                >
                  <span>{name}</span>
                  {isDirty(name) ? (
                    <span className={styles.dirtyDot} />
                  ) : (
                    <span
                      className={styles.tabClose}
                      onClick={(e) => {
                        e.stopPropagation()
                        closeTab(name)
                      }}
                    >
                      &times;
                    </span>
                  )}
                </button>
              ))}
            </div>
          )}

          {activeTab ? (
            <div className={styles.editorBody}>
              <MonacoEditor
                value={fileContents[activeTab] ?? ''}
                onChange={handleEditorChange}
                language="markdown"
                height="100%"
              />
            </div>
          ) : (
            <div className={styles.emptyEditor}>
              {fileNames.length > 0
                ? 'Select a file to edit'
                : 'Create a file to get started'}
            </div>
          )}

          <div className={styles.statusBar}>
            <div className={styles.statusBarLeft}>
              {role.agent_count} agent{role.agent_count !== 1 ? 's' : ''} using this role
            </div>
            <div className={styles.statusBarRight}>
              <Button
                size="sm"
                variant="secondary"
                disabled={!activeTab || !isDirty(activeTab)}
                loading={saving}
                onClick={handleSave}
              >
                Save
              </Button>
            </div>
          </div>
        </div>
      </Card>

      <Modal
        open={deleteTarget !== null}
        onClose={() => setDeleteTarget(null)}
        title={`Delete ${deleteTarget}`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button variant="danger" onClick={handleDeleteFile}>Delete</Button>
          </>
        }
      >
        <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem' }}>
          Are you sure you want to delete &quot;{deleteTarget}&quot;? This action cannot be undone.
        </p>
      </Modal>
    </>
  )
}

export default function RoleEditor() {
  const { name } = useParams<{ name: string }>()
  const toast = useToast()
  const [role, setRole] = useState<Role | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!name) return
    rolesApi.getRole(name)
      .then(setRole)
      .catch(() => toast.error('Failed to load role', name))
      .finally(() => setLoading(false))
  }, [name])

  if (loading) {
    return (
      <div className={styles.page}>
        <PageHeader
          breadcrumbs={[{ label: 'Roles', path: '/roles' }, { label: name ?? '' }]}
          title={name ?? ''}
        />
        <Card>
          <div className={`${styles.skeleton} ${styles.skeletonBlock}`} />
        </Card>
      </div>
    )
  }

  if (!role) {
    return (
      <div className={styles.page}>
        <PageHeader
          breadcrumbs={[{ label: 'Roles', path: '/roles' }, { label: name ?? '' }]}
          title={name ?? ''}
        />
        <Card>
          <EmptyState title="Role not found" description={`No role with name "${name}" exists.`} />
        </Card>
      </div>
    )
  }

  return (
    <div className={styles.page}>
      <PageHeader
        breadcrumbs={[{ label: 'Roles', path: '/roles' }, { label: role.name }]}
        title={role.name}
      />
      <RoleEditorInner initialRole={role} />
    </div>
  )
}
