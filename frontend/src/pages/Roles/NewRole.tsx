import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { rolesApi } from '../../api/roles'
import { PageHeader, Card, Button, Input, Textarea, Select } from '../../components/ui'
import { useToast } from '../../hooks/useToast'
import styles from './Roles.module.css'

const NAME_PATTERN = /^[a-z][a-z0-9-]*$/

const ROLE_TYPES = [
  { value: '', label: 'Custom (no template)' },
  { value: 'developer', label: 'Developer' },
  { value: 'reviewer', label: 'Reviewer' },
  { value: 'sre', label: 'SRE' },
  { value: 'observer', label: 'Observer' },
] as const

const INITIAL_FILES = [
  { name: 'SOUL.md', defaultChecked: true },
  { name: 'PROMPT.md', defaultChecked: true },
  { name: 'AGENTS.md', defaultChecked: true },
  { name: 'MEMORY.md', defaultChecked: false },
]

const ROLE_SKILLS: Record<string, string[]> = {
  developer: ['gitea-api', 'git-ops'],
  reviewer: ['gitea-api'],
  sre: ['gitea-api', 'git-ops', 'kubectl-ops'],
  observer: ['gitea-api'],
}

export default function NewRole() {
  const navigate = useNavigate()
  const toast = useToast()
  const [form, setForm] = useState({ name: '', description: '', roleType: '' })
  const [selectedFiles, setSelectedFiles] = useState(['SOUL.md', 'PROMPT.md', 'AGENTS.md'])
  const [creating, setCreating] = useState(false)

  const nameValid = form.name === '' || NAME_PATTERN.test(form.name)
  const canCreate = form.name !== '' && NAME_PATTERN.test(form.name)

  const toggleFile = (filename: string, checked: boolean) => {
    setSelectedFiles((prev) =>
      checked ? [...prev, filename] : prev.filter((f) => f !== filename),
    )
  }

  const handleCreate = async () => {
    setCreating(true)
    try {
      await rolesApi.createRole({
        name: form.name,
        description: form.description,
        role: form.roleType || undefined,
        initial_files: selectedFiles,
      })
      toast.success('Role created', form.name)
      navigate(`/roles/${form.name}`)
    } catch {
      toast.error('Create failed', `Could not create role "${form.name}"`)
    } finally {
      setCreating(false)
    }
  }

  const builtinSkills = form.roleType ? (ROLE_SKILLS[form.roleType] ?? []) : []

  return (
    <div className={styles.page}>
      <PageHeader
        title="New Role"
        breadcrumbs={[
          { label: 'Roles', path: '/roles' },
          { label: 'New Role' },
        ]}
      />

      <Card>
        <div className={styles.formBody}>
          <div className={styles.nameInput}>
            <Input
              label="Name"
              value={form.name}
              onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
            />
            {nameValid ? (
              <div className={styles.hint}>lowercase, hyphens only (e.g. my-role)</div>
            ) : (
              <div className={styles.hintError}>Must start with a lowercase letter, then lowercase letters, digits, or hyphens</div>
            )}
          </div>

          <Textarea
            label="Description"
            value={form.description}
            onChange={(e) => setForm((p) => ({ ...p, description: e.target.value }))}
            rows={3}
          />

          <Select
            label="Role Type"
            value={form.roleType}
            onChange={(e) => setForm((p) => ({ ...p, roleType: e.target.value }))}
          >
            {ROLE_TYPES.map((rt) => (
              <option key={rt.value} value={rt.value}>{rt.label}</option>
            ))}
          </Select>
          <div className={styles.hint}>
            Selecting a role type auto-populates files with builtin templates and skills
          </div>

          <div className={styles.filesSection}>
            <div className={styles.filesSectionTitle}>Initial Files</div>
            <div className={styles.checkboxGroup}>
              {INITIAL_FILES.map((file) => (
                <label key={file.name} className={styles.checkboxLabel}>
                  <input
                    type="checkbox"
                    checked={selectedFiles.includes(file.name)}
                    onChange={(e) => toggleFile(file.name, e.target.checked)}
                  />
                  {file.name}
                </label>
              ))}
            </div>
            <div className={styles.filesHint}>Checked files are created with builtin templates</div>
          </div>

          {builtinSkills.length > 0 && (
            <div className={styles.filesSection}>
              <div className={styles.filesSectionTitle}>Builtin Skills (auto-injected)</div>
              <div className={styles.checkboxGroup}>
                {builtinSkills.map((skill) => (
                  <label key={skill} className={styles.checkboxLabel}>
                    <input type="checkbox" checked disabled />
                    {skill}
                  </label>
                ))}
              </div>
            </div>
          )}

          <div className={styles.formActions}>
            <Button variant="secondary" onClick={() => navigate('/roles')}>
              Cancel
            </Button>
            <Button onClick={handleCreate} loading={creating} disabled={!canCreate}>
              Create Role
            </Button>
          </div>
        </div>
      </Card>
    </div>
  )
}
