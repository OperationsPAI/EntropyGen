import { postAuthLogin, postAuthLogout, getAuthMe } from './generated/sdk.gen'

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
    postAuthLogin({ body: dto }).then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as LoginResponse
    }),

  logout: () =>
    postAuthLogout(),

  getMe: () =>
    getAuthMe().then((r) => {
      const body = r.data as any
      return (body?.data ?? body) as UserInfo
    }),
}
