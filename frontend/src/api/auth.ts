import { apiClient } from './client'

export interface LoginDto {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  username: string
}

export interface UserInfo {
  username: string
  role: string
}

export const authApi = {
  login: (dto: LoginDto) =>
    apiClient.post<LoginResponse>('/auth/login', dto).then((r) => r.data),

  logout: () =>
    apiClient.post('/auth/logout'),

  getMe: () =>
    apiClient.get<UserInfo>('/auth/me').then((r) => r.data),
}
