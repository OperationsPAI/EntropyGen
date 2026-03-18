import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { IconDelete, IconDownload } from '@douyinfe/semi-icons'
import { rolesApi } from '../../api/roles'
import { PageHeader, Card, Table, Button, Input, Modal, EmptyState } from '../../components/ui'
import { useToast } from '../../hooks/useToast'
import type { Role } from '../../types/agent'
import styles from './Roles.module.css'

function getRelativeTime(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime()
  const hours = Math.floor(diff / 3600000)
  if (hours < 1) return 'just now'
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

export default function RoleList() {
  const navigate = useNavigate()
  const toast = useToast()
  const [roles, setRoles] = useState<Role[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Role | null>(null)
  const [deleteInput, setDeleteInput] = useState('')
  const importRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    rolesApi.getRoles()
      .then(setRoles)
      .catch(() => setError('Failed to load roles'))
      .finally(() => setLoading(false))
  }, [])

  const handleDelete = async () => {
    if (!deleteTarget || deleteInput !== deleteTarget.name) return
    try {
      await rolesApi.deleteRole(deleteTarget.name)
      setRoles((prev) => prev.filter((r) => r.name !== deleteTarget.name))
      toast.success('Role deleted', deleteTarget.name)
    } catch {
      toast.error('Delete failed', `Could not delete ${deleteTarget.name}`)
    }
    setDeleteTarget(null)
    setDeleteInput('')
  }

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const name = file.name.replace(/\.zip$/i, '')
    try {
      await rolesApi.importRole(name, '', file)
      toast.success('Role imported', name)
      rolesApi.getRoles().then(setRoles)
    } catch {
      toast.error('Import failed', `Could not import ${file.name}`)
    }
    if (importRef.current) importRef.current.value = ''
  }

  const renderTable = () => {
    if (loading) {
      return (
        <div>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className={`${styles.skeleton} ${styles.skeletonRow}`} />
          ))}
        </div>
      )
    }

    if (error) {
      return <EmptyState title="Error loading roles" description={error} />
    }

    if (roles.length === 0) {
      return (
        <EmptyState
          title="No roles yet"
          description="Create your first role to get started."
          action={<Button onClick={() => navigate('/roles/new')}>Create your first role</Button>}
        />
      )
    }

    return (
      <Table>
        <thead>
          <tr>
            {['Name', 'Description', 'Files', 'Agents', 'Updated', 'Actions'].map((h) => (
              <th key={h}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {roles.map((role) => (
            <tr key={role.name}>
              <td className={styles.nameCell}>
                <Link to={`/roles/${role.name}`} className={styles.nameLink}>
                  {role.name}
                </Link>
              </td>
              <td className={styles.mutedCell}>{role.description || '\u2014'}</td>
              <td className={styles.mutedCell}>{role.file_count ?? (role.files ?? []).length}</td>
              <td className={styles.mutedCell} title={`${role.agent_count} agents using this role`}>
                {role.agent_count}
              </td>
              <td className={styles.mutedCell}>{getRelativeTime(role.updated_at)}</td>
              <td>
                <div className={styles.actionsCell}>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => rolesApi.exportRole(role.name)}
                    title="Export as ZIP"
                  >
                    <IconDownload size="small" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setDeleteTarget(role)}
                    disabled={role.agent_count > 0}
                    title={role.agent_count > 0 ? `${role.agent_count} agents are using this role` : undefined}
                  >
                    <IconDelete size="small" style={{ color: role.agent_count > 0 ? undefined : 'var(--accent-orange)' }} />
                  </Button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </Table>
    )
  }

  return (
    <div className={styles.page}>
      <PageHeader
        title="Roles"
        actions={
          <div style={{ display: 'flex', gap: '8px' }}>
            <Button variant="secondary" onClick={() => importRef.current?.click()}>
              Import
            </Button>
            <Button onClick={() => navigate('/roles/new')}>+ New Role</Button>
          </div>
        }
      />

      <input
        ref={importRef}
        type="file"
        accept=".zip"
        style={{ display: 'none' }}
        onChange={handleImport}
      />

      <Card>
        {renderTable()}
      </Card>

      <Modal
        open={!!deleteTarget}
        onClose={() => { setDeleteTarget(null); setDeleteInput('') }}
        title="Delete Role"
        footer={
          <>
            <Button variant="secondary" onClick={() => { setDeleteTarget(null); setDeleteInput('') }}>Cancel</Button>
            <Button variant="danger" onClick={handleDelete} disabled={deleteInput !== deleteTarget?.name}>
              Confirm Delete
            </Button>
          </>
        }
      >
        <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginBottom: 'var(--space-md)' }}>
          This action is irreversible. Type <strong>{deleteTarget?.name}</strong> to confirm.
        </p>
        <Input
          value={deleteInput}
          onChange={(e) => setDeleteInput(e.target.value)}
          placeholder={deleteTarget?.name}
        />
      </Modal>
    </div>
  )
}
