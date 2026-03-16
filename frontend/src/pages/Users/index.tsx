import { useState, useEffect, useCallback } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from '../../contexts/AuthContext'
import { apiClient } from '../../api/client'
import PageHeader from '../../components/ui/PageHeader'
import Button from '../../components/ui/Button'
import Modal from '../../components/ui/Modal'
import Input from '../../components/ui/Input'
import Select from '../../components/ui/Select'
import { useToast } from '../../hooks/useToast'
import styles from './Users.module.css'

interface UserRecord {
  id: number
  username: string
  role: 'member' | 'admin'
  createdAt: string
  updatedAt: string
}

interface UserFormData {
  username: string
  password: string
  role: 'member' | 'admin'
}

async function fetchUsers(): Promise<UserRecord[]> {
  const res = await apiClient.get('/users')
  const body = res.data
  return Array.isArray(body?.data) ? body.data : []
}

export default function Users() {
  const { isAdmin } = useAuth()
  const toast = useToast()
  const [users, setUsers] = useState<UserRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<UserRecord | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<UserRecord | null>(null)
  const [form, setForm] = useState<UserFormData>({ username: '', password: '', role: 'member' })
  const [submitting, setSubmitting] = useState(false)

  if (!isAdmin) return <Navigate to="/dashboard" replace />

  const loadUsers = useCallback(async () => {
    setLoading(true)
    try {
      setUsers(await fetchUsers())
    } catch {
      toast.error('Failed to load users')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadUsers() }, [loadUsers])

  const openCreate = () => {
    setForm({ username: '', password: '', role: 'member' })
    setCreateOpen(true)
  }

  const openEdit = (u: UserRecord) => {
    setForm({ username: u.username, password: '', role: u.role })
    setEditTarget(u)
  }

  const handleCreate = async () => {
    if (!form.username || !form.password) {
      toast.error('Username and password are required')
      return
    }
    setSubmitting(true)
    try {
      await apiClient.post('/users', { username: form.username, password: form.password, role: form.role })
      toast.success('User created')
      setCreateOpen(false)
      loadUsers()
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Failed to create user'
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  const handleEdit = async () => {
    if (!editTarget) return
    setSubmitting(true)
    const payload: { role?: string; password?: string } = { role: form.role }
    if (form.password) payload.password = form.password
    try {
      await apiClient.put(`/users/${editTarget.username}`, payload)
      toast.success('User updated')
      setEditTarget(null)
      loadUsers()
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Failed to update user'
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setSubmitting(true)
    try {
      await apiClient.delete(`/users/${deleteTarget.username}`)
      toast.success('User deleted')
      setDeleteTarget(null)
      loadUsers()
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error ?? 'Failed to delete user'
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div style={{ padding: '32px', maxWidth: '900px' }}>
      <PageHeader
        title="User Management"
        description="Manage platform users and their roles."
        actions={<Button onClick={openCreate}>Add User</Button>}
      />

      {loading ? (
        <div style={{ color: 'var(--text-muted)', marginTop: '32px' }}>Loading...</div>
      ) : (
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Username</th>
              <th>Role</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>{u.username}</td>
                <td>
                  <span className={`${styles.badge} ${u.role === 'admin' ? styles.badgeAdmin : styles.badgeMember}`}>
                    {u.role}
                  </span>
                </td>
                <td>{new Date(u.createdAt).toLocaleDateString()}</td>
                <td>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <Button size="sm" variant="secondary" onClick={() => openEdit(u)}>Edit</Button>
                    <Button size="sm" variant="danger" onClick={() => setDeleteTarget(u)}>Delete</Button>
                  </div>
                </td>
              </tr>
            ))}
            {users.length === 0 && (
              <tr><td colSpan={4} style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '32px' }}>No users found</td></tr>
            )}
          </tbody>
        </table>
      )}

      {/* Create user modal */}
      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="Add User"
        footer={
          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button loading={submitting} onClick={handleCreate}>Create</Button>
          </div>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <Input label="Username" value={form.username} onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))} />
          <Input label="Password" type="password" value={form.password} onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))} />
          <Select
            label="Role"
            value={form.role}
            onChange={(e) => setForm((f) => ({ ...f, role: e.target.value as 'member' | 'admin' }))}
          >
            <option value="member">Member</option>
            <option value="admin">Admin</option>
          </Select>
        </div>
      </Modal>

      {/* Edit user modal */}
      <Modal
        open={!!editTarget}
        onClose={() => setEditTarget(null)}
        title={`Edit User: ${editTarget?.username}`}
        footer={
          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setEditTarget(null)}>Cancel</Button>
            <Button loading={submitting} onClick={handleEdit}>Save</Button>
          </div>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <Input label="New Password (leave blank to keep)" type="password" value={form.password} onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))} />
          <Select
            label="Role"
            value={form.role}
            onChange={(e) => setForm((f) => ({ ...f, role: e.target.value as 'member' | 'admin' }))}
          >
            <option value="member">Member</option>
            <option value="admin">Admin</option>
          </Select>
        </div>
      </Modal>

      {/* Delete confirmation */}
      <Modal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title="Delete User"
        footer={
          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>Cancel</Button>
            <Button variant="danger" loading={submitting} onClick={handleDelete}>Delete</Button>
          </div>
        }
      >
        <p>Are you sure you want to delete <strong>{deleteTarget?.username}</strong>? This cannot be undone.</p>
      </Modal>
    </div>
  )
}
