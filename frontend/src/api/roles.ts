import { apiClient } from './client'
import type { Role, RoleFile, CreateRoleDto } from '../types/agent'

export const rolesApi = {
  getRoles: () =>
    apiClient.get<{ success: boolean; data: Role[] }>('/roles').then((r) => r.data.data ?? []),

  getRole: (name: string) =>
    apiClient.get<{ success: boolean; data: Role }>(`/roles/${name}`).then((r) => r.data.data),

  createRole: (dto: CreateRoleDto) =>
    apiClient.post<{ success: boolean; data: Role }>('/roles', dto).then((r) => r.data.data),

  updateRole: (name: string, data: { description: string }) =>
    apiClient.patch<{ success: boolean; data: Role }>(`/roles/${name}`, data).then((r) => r.data.data),

  deleteRole: (name: string) =>
    apiClient.delete(`/roles/${name}`),

  getRoleFiles: (name: string) =>
    apiClient.get<{ success: boolean; data: RoleFile[] }>(`/roles/${name}/files`).then((r) => r.data.data ?? []),

  getRoleFile: (name: string, filepath: string) =>
    apiClient.get<{ success: boolean; data: RoleFile }>(`/roles/${name}/files/${filepath}`).then((r) => r.data.data),

  updateRoleFile: (name: string, filepath: string, content: string) =>
    apiClient.put<{ success: boolean; data: RoleFile }>(`/roles/${name}/files/${filepath}`, { content }).then((r) => r.data.data),

  deleteRoleFile: (name: string, filepath: string) =>
    apiClient.delete(`/roles/${name}/files/${filepath}`),

  renameRoleFile: (name: string, oldName: string, newName: string) =>
    apiClient.post<{ success: boolean; data: RoleFile }>(`/roles/${name}/rename-file`, { old_name: oldName, new_name: newName }).then((r) => r.data.data),
}
