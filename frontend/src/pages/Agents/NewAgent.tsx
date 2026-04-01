import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { agentsApi, type RuntimeType } from '../../api/agents'
import { llmApi, type LLMModel } from '../../api/llm'
import { rolesApi } from '../../api/roles'
import { PageHeader, Card, Button, Input, Select, EmptyState } from '../../components/ui'
import MonacoEditor from '../../components/editor/MonacoEditor'
import { useToast } from '../../hooks/useToast'
import type { Role } from '../../types/agent'
import styles from './NewAgent.module.css'

const STEPS = ['Identity', 'Configuration', 'Infrastructure', 'Review'] as const
const TOTAL_STEPS = STEPS.length

const NAME_PATTERN = /^[a-z][a-z0-9-]*$/

interface FormState {
  name: string
  role: string
  model: string
  temperature: number
  maxTokens: number
  runtimeType: string
  cpuRequest: string
  cpuLimit: string
  memoryRequest: string
  memoryLimit: string
  workspaceSize: string
  repo: string
  permissions: ('read' | 'write' | 'review' | 'merge')[]
}

const INITIAL_FORM: FormState = {
  name: '',
  role: '',
  model: '',
  temperature: 0.7,
  maxTokens: 65536,
  runtimeType: 'openclaw',
  cpuRequest: '500m',
  cpuLimit: '5000m',
  memoryRequest: '1Gi',
  memoryLimit: '2Gi',
  workspaceSize: '5Gi',
  repo: '',
  permissions: ['read'],
}

export default function NewAgent() {
  const navigate = useNavigate()
  const toast = useToast()
  const [step, setStep] = useState(1)
  const [roles, setRoles] = useState<Role[]>([])
  const [rolesLoading, setRolesLoading] = useState(true)
  const [models, setModels] = useState<LLMModel[]>([])
  const [modelsLoading, setModelsLoading] = useState(true)
  const [runtimeTypes, setRuntimeTypes] = useState<RuntimeType[]>([])
  const [runtimeTypesLoading, setRuntimeTypesLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState<FormState>(INITIAL_FORM)
  const [previewOpen, setPreviewOpen] = useState(false)
  const [previewContent, setPreviewContent] = useState('')

  useEffect(() => {
    rolesApi.getRoles()
      .then(setRoles)
      .catch(() => {})
      .finally(() => setRolesLoading(false))
  }, [])

  useEffect(() => {
    llmApi.getModels()
      .then(setModels)
      .catch(() => {})
      .finally(() => setModelsLoading(false))
  }, [])

  useEffect(() => {
    agentsApi.getRuntimeTypes()
      .then((types) => {
        setRuntimeTypes(types)
        const defaultType = types.find((t) => t.default)
        if (defaultType) {
          setForm((prev) => prev.runtimeType === 'openclaw' ? { ...prev, runtimeType: defaultType.type } : prev)
        }
      })
      .catch(() => {})
      .finally(() => setRuntimeTypesLoading(false))
  }, [])

  const selectedRole = roles.find((r) => r.name === form.role)

  const nameValid = form.name === '' || NAME_PATTERN.test(form.name)

  const isStepValid = (s: number): boolean => {
    switch (s) {
      case 1:
        return NAME_PATTERN.test(form.name) && form.role !== ''
      case 2:
        return form.model.trim() !== ''
      case 3:
        return (
          form.cpuRequest.trim() !== '' &&
          form.cpuLimit.trim() !== '' &&
          form.memoryRequest.trim() !== '' &&
          form.memoryLimit.trim() !== '' &&
          form.workspaceSize.trim() !== ''
        )
      default:
        return true
    }
  }

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  const handleCreate = async () => {
    setCreating(true)
    try {
      await agentsApi.createAgent({
        name: form.name,
        spec: {
          role: form.role,
          llm: { model: form.model, temperature: form.temperature, maxTokens: form.maxTokens },
          runtime: { type: form.runtimeType },
          resources: {
            cpuRequest: form.cpuRequest,
            cpuLimit: form.cpuLimit,
            memoryRequest: form.memoryRequest,
            memoryLimit: form.memoryLimit,
            workspaceSize: form.workspaceSize,
          },
          gitea: { repo: form.repo, repos: form.repo ? [form.repo] : [], permissions: form.permissions },
        },
      })
      toast.success('Agent created', form.name)
      navigate(`/agents/${form.name}`)
    } catch {
      toast.error('Create failed', `Could not create agent "${form.name}"`)
    } finally {
      setCreating(false)
    }
  }

  const togglePermission = (perm: 'read' | 'write' | 'review' | 'merge') =>
    setForm((prev) => ({
      ...prev,
      permissions: prev.permissions.includes(perm)
        ? prev.permissions.filter((p) => p !== perm)
        : [...prev.permissions, perm],
    }))

  const handlePreviewToggle = async () => {
    if (previewOpen) {
      setPreviewOpen(false)
      return
    }
    if (!form.role) return
    if (!previewContent) {
      try {
        const file = await rolesApi.getRoleFile(form.role, 'SOUL.md')
        setPreviewContent(file.content)
      } catch {
        setPreviewContent('Failed to load soul.md')
      }
    }
    setPreviewOpen(true)
  }

  const renderStepIndicator = () => (
    <div className={styles.stepIndicator}>
      {STEPS.map((label, i) => {
        const stepNum = i + 1
        const isActive = stepNum === step
        const isCompleted = stepNum < step

        const dotClass = isActive
          ? styles.stepDotActive
          : isCompleted
            ? styles.stepDotCompleted
            : styles.stepDotInactive

        const labelClass = isActive || isCompleted
          ? `${styles.stepLabel} ${styles.stepLabelActive}`
          : styles.stepLabel

        return (
          <div key={label} className={styles.stepItem}>
            {i > 0 && (
              <div
                className={`${styles.stepLine} ${stepNum <= step ? styles.stepLineActive : ''}`}
              />
            )}
            <div className={styles.stepItemColumn}>
              <div
                className={`${styles.stepDot} ${dotClass}`}
                onClick={isCompleted ? () => setStep(stepNum) : undefined}
              >
                {stepNum}
              </div>
              <div className={labelClass}>{label}</div>
            </div>
          </div>
        )
      })}
    </div>
  )

  const renderStepIdentity = () => (
    <div className={styles.formBody}>
      <Input
        label="Agent Name"
        value={form.name}
        onChange={(e) => updateField('name', e.target.value)}
        placeholder="my-agent"
      />
      {nameValid ? (
        <span className={styles.helperText}>lowercase, hyphens only</span>
      ) : (
        <span className={styles.hintError}>Must start with a lowercase letter, then lowercase letters, digits, or hyphens</span>
      )}

      <div className={styles.roleHeading}>
        <span className={styles.sectionLabel}>Select Role</span>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => window.open('/roles/new', '_blank')}
        >
          + New Role
        </Button>
      </div>

      {rolesLoading ? (
        <div style={{ padding: 'var(--space-lg)', color: 'var(--text-muted)', fontSize: '0.875rem' }}>
          Loading roles...
        </div>
      ) : roles.length === 0 ? (
        <EmptyState
          title="No roles available"
          description="Create a role before creating an agent."
          action={
            <Button onClick={() => window.open('/roles/new', '_blank')}>
              Create your first role
            </Button>
          }
        />
      ) : (
        <div className={styles.roleList}>
          {roles.map((role) => (
            <div
              key={role.name}
              className={`${styles.roleItem} ${form.role === role.name ? styles.roleItemSelected : ''}`}
              onClick={() => updateField('role', role.name)}
            >
              <input
                type="radio"
                checked={form.role === role.name}
                onChange={() => updateField('role', role.name)}
              />
              <div className={styles.roleInfo}>
                <div className={styles.roleName}>{role.name}</div>
                <div className={styles.roleMeta}>
                  {role.description} &middot; {(role.files ?? []).length} files &middot; {role.agent_count} agents
                </div>
              </div>
              <a
                className={styles.roleEditLink}
                href={`/roles/${role.name}`}
                target="_blank"
                rel="noopener noreferrer"
                onClick={(e) => e.stopPropagation()}
              >
                Edit &rarr;
              </a>
            </div>
          ))}
        </div>
      )}

      {selectedRole && (selectedRole.files ?? []).length > 0 && (
        <div className={styles.selectedFiles}>
          Selected role files: {(selectedRole.files ?? []).map((f) => f.name).join(', ')}
        </div>
      )}
    </div>
  )

  const renderStepConfiguration = () => (
    <div className={styles.formBody}>
      <span className={styles.sectionLabel}>LLM</span>
      <Select
        label="Model"
        value={form.model}
        onChange={(e) => updateField('model', e.target.value)}
        disabled={modelsLoading}
      >
        <option value="">
          {modelsLoading ? 'Loading models...' : 'Select a model...'}
        </option>
        {models.map((m) => (
          <option key={m.id} value={m.id}>
            {m.name} ({m.provider})
          </option>
        ))}
      </Select>
      {!modelsLoading && models.length === 0 && (
        <span className={styles.helperText}>
          No models available.{' '}
          <a className={styles.infoLink} href="/llm" target="_blank" rel="noopener noreferrer">
            Go to LLM Models to add one &rarr;
          </a>
        </span>
      )}
      <div className={styles.formGrid2}>
        <div>
          <Input
            label="Temperature"
            type="number"
            min={0}
            max={2}
            step={0.1}
            value={form.temperature}
            onChange={(e) => updateField('temperature', parseFloat(e.target.value))}
          />
          <span className={styles.helperText}>0 = deterministic, 2 = creative</span>
        </div>
        <Input
          label="Max Tokens"
          type="number"
          value={form.maxTokens}
          onChange={(e) => updateField('maxTokens', parseInt(e.target.value, 10))}
        />
      </div>

      <div className={styles.infoBox}>
        The prompt content is defined in your role's prompt.md file.
        {form.role && (
          <>
            {' '}
            <a
              className={styles.infoLink}
              href={`/roles/${form.role}`}
              target="_blank"
              rel="noopener noreferrer"
            >
              Edit in Role Editor &rarr;
            </a>
          </>
        )}
      </div>
    </div>
  )

  const renderStepInfrastructure = () => (
    <div className={styles.formBody}>
      <span className={styles.sectionLabel}>Runtime Type</span>
      <Select
        label="Runtime Type"
        value={form.runtimeType}
        onChange={(e) => updateField('runtimeType', e.target.value)}
        disabled={runtimeTypesLoading}
      >
        {runtimeTypesLoading ? (
          <option value={form.runtimeType}>Loading runtime types...</option>
        ) : runtimeTypes.length === 0 ? (
          <option value="openclaw">openclaw (default)</option>
        ) : (
          runtimeTypes.map((rt) => (
            <option key={rt.type} value={rt.type}>
              {rt.type}{rt.default ? ' (default)' : ''}
            </option>
          ))
        )}
      </Select>

      <div className={styles.sectionDivider}>
        <span className={styles.sectionLabel}>Resources</span>
      </div>
      <span className={styles.sectionLabel}>CPU</span>
      <div className={styles.formGrid2}>
        <Input
          label="Request"
          value={form.cpuRequest}
          onChange={(e) => updateField('cpuRequest', e.target.value)}
        />
        <Input
          label="Limit"
          value={form.cpuLimit}
          onChange={(e) => updateField('cpuLimit', e.target.value)}
        />
      </div>
      <span className={styles.sectionLabel}>Memory</span>
      <div className={styles.formGrid2}>
        <Input
          label="Request"
          value={form.memoryRequest}
          onChange={(e) => updateField('memoryRequest', e.target.value)}
        />
        <Input
          label="Limit"
          value={form.memoryLimit}
          onChange={(e) => updateField('memoryLimit', e.target.value)}
        />
      </div>
      <Input
        label="Workspace Size"
        value={form.workspaceSize}
        onChange={(e) => updateField('workspaceSize', e.target.value)}
      />

      <div className={styles.sectionDivider}>
        <span className={styles.sectionLabel}>Gitea</span>
      </div>
      <Input
        label="Repository"
        value={form.repo}
        onChange={(e) => updateField('repo', e.target.value)}
        placeholder="org/repo"
      />
      <div>
        <span className={styles.sectionLabel}>Permissions</span>
        <div className={styles.checkboxGroup}>
          {(['read', 'write', 'review', 'merge'] as const).map((perm) => (
            <label key={perm} className={styles.checkboxLabel}>
              <input
                type="checkbox"
                checked={form.permissions.includes(perm)}
                onChange={() => togglePermission(perm)}
              />
              {perm}
            </label>
          ))}
        </div>
      </div>
    </div>
  )

  const renderStepReview = () => (
    <div className={styles.formBody}>
      <div className={styles.reviewGrid}>
        <div className={styles.reviewSection} onClick={() => setStep(1)}>
          <div className={styles.reviewSectionTitle}>Identity</div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Name</span>
            <span className={styles.reviewValue}>{form.name || '\u2014'}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Role</span>
            <span className={styles.reviewValue}>{form.role || '\u2014'}</span>
          </div>
          {form.role && (
            <Button
              variant="secondary"
              size="sm"
              onClick={(e) => {
                e.stopPropagation()
                handlePreviewToggle()
              }}
            >
              {previewOpen ? 'Hide files' : 'Preview files'}
            </Button>
          )}
          {previewOpen && (
            <div className={styles.previewEditor} onClick={(e) => e.stopPropagation()}>
              <MonacoEditor
                value={previewContent}
                readOnly
                height="200px"
                language="markdown"
              />
            </div>
          )}
        </div>

        <div className={styles.reviewSection} onClick={() => setStep(2)}>
          <div className={styles.reviewSectionTitle}>Configuration</div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Model</span>
            <span className={styles.reviewValue}>{form.model || '\u2014'}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Temperature</span>
            <span className={styles.reviewValue}>{form.temperature}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Max Tokens</span>
            <span className={styles.reviewValue}>{form.maxTokens.toLocaleString()}</span>
          </div>
        </div>

        <div className={styles.reviewSection} onClick={() => setStep(3)}>
          <div className={styles.reviewSectionTitle}>Infrastructure</div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Runtime Type</span>
            <span className={styles.reviewValue}>{form.runtimeType}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>CPU</span>
            <span className={styles.reviewValue}>{form.cpuRequest} / {form.cpuLimit}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Memory</span>
            <span className={styles.reviewValue}>{form.memoryRequest} / {form.memoryLimit}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Disk</span>
            <span className={styles.reviewValue}>{form.workspaceSize}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Repo</span>
            <span className={styles.reviewValue}>{form.repo || '\u2014'}</span>
          </div>
          <div className={styles.reviewRow}>
            <span className={styles.reviewLabel}>Permissions</span>
            <span className={styles.reviewValue}>{form.permissions.join(', ')}</span>
          </div>
        </div>
      </div>

      {selectedRole && (selectedRole.files ?? []).length > 0 && (
        <div className={styles.selectedFiles}>
          Role files: {(selectedRole.files ?? []).map((f) => f.name).join(', ')}
        </div>
      )}

      <div className={styles.warningBox}>
        This will start a new pod consuming resources.
      </div>
    </div>
  )

  const stepRenderers = [renderStepIdentity, renderStepConfiguration, renderStepInfrastructure, renderStepReview]

  return (
    <div className={styles.page}>
      <PageHeader
        breadcrumbs={[{ label: 'Agents', path: '/agents' }, { label: 'New Agent' }]}
        title="New Agent"
      />

      <Card>
        {renderStepIndicator()}
        <div className={styles.stepContent}>
          {stepRenderers[step - 1]()}
        </div>

        <div className={styles.actionBar}>
          <Button variant="secondary" onClick={() => navigate('/agents')}>
            Cancel
          </Button>
          <div className={styles.actionBarRight}>
            {step > 1 && (
              <Button variant="secondary" onClick={() => setStep((s) => s - 1)}>
                Back
              </Button>
            )}
            {step < TOTAL_STEPS ? (
              <Button onClick={() => setStep((s) => s + 1)} disabled={!isStepValid(step)}>
                Next
              </Button>
            ) : (
              <Button onClick={handleCreate} loading={creating} disabled={!isStepValid(1)}>
                Create Agent
              </Button>
            )}
          </div>
        </div>
      </Card>
    </div>
  )
}
