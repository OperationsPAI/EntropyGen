import {
  getRoles,
  getRolesByName,
  postRoles,
  patchRolesByName,
  deleteRolesByName,
  getRolesByNameFiles,
  getRolesByNameFilesByFilepath,
  putRolesByNameFilesByFilepath,
  deleteRolesByNameFilesByFilepath,
  postRolesByNameRenameFile,
  getRolesTypes,
  getRolesByNameValidate,
} from './generated/sdk.gen'
import { apiClient } from './client'
import type { Role, RoleFile, CreateRoleDto, RoleType, ValidationIssue } from '../types/agent'

/* eslint-disable @typescript-eslint/no-explicit-any */

export const rolesApi = {
  getRoles: () =>
    getRoles().then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as Role[]
    }),

  getRole: (name: string) =>
    getRolesByName({ path: { name } }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as Role
    }),

  createRole: (dto: CreateRoleDto) =>
    postRoles({ body: dto as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as Role
    }),

  updateRole: (name: string, data: { description: string }) =>
    patchRolesByName({ path: { name }, body: data as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as Role
    }),

  deleteRole: (name: string) =>
    deleteRolesByName({ path: { name } }),

  getRoleFiles: (name: string) =>
    getRolesByNameFiles({ path: { name } }).then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as RoleFile[]
    }),

  getRoleFile: (name: string, filepath: string) =>
    getRolesByNameFilesByFilepath({ path: { name, filepath } }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as RoleFile
    }),

  updateRoleFile: (name: string, filepath: string, content: string) =>
    putRolesByNameFilesByFilepath({ path: { name, filepath }, body: { content } as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as RoleFile
    }),

  deleteRoleFile: (name: string, filepath: string) =>
    deleteRolesByNameFilesByFilepath({ path: { name, filepath } }),

  renameRoleFile: (name: string, oldName: string, newName: string) =>
    postRolesByNameRenameFile({ path: { name }, body: { old_name: oldName, new_name: newName } as any }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as RoleFile
    }),

  getRoleTypes: () =>
    getRolesTypes().then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as RoleType[]
    }),

  validateRole: (name: string) =>
    getRolesByNameValidate({ path: { name } }).then((r) => {
      const body = r.data as any
      return (body?.data ?? []) as ValidationIssue[]
    }),

  exportRole: (name: string) => {
    window.open(`/api/roles/${name}/export`, '_blank')
  },

  importRole: (name: string, description: string, file: File) => {
    const formData = new FormData()
    formData.append('name', name)
    formData.append('description', description)
    formData.append('file', file)
    return apiClient.post<{ success: boolean; data: Role }>('/roles', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then((r) => r.data.data)
  },
}
