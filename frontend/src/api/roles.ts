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

  getRoleFile: (name: string, filename: string) =>
    apiClient.get<{ success: boolean; data: RoleFile }>(`/roles/${name}/files/${filename}`).then((r) => r.data.data),

  updateRoleFile: (name: string, filename: string, content: string) =>
    apiClient.put<{ success: boolean; data: RoleFile }>(`/roles/${name}/files/${filename}`, { content }).then((r) => r.data.data),

  deleteRoleFile: (name: string, filename: string) =>
    apiClient.delete(`/roles/${name}/files/${filename}`),

  renameRoleFile: (name: string, filename: string, newName: string) =>
    apiClient.post<{ success: boolean; data: RoleFile }>(`/roles/${name}/files/${filename}/rename`, { new_name: newName }).then((r) => r.data.data),
}
