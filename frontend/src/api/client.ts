import axios from 'axios'

export const apiClient = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('jwt_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// No auto-redirect on 401 — guest requests may get 403 for write ops.
// Components are responsible for showing appropriate errors.
apiClient.interceptors.response.use(
  (response) => response,
  (error) => Promise.reject(error)
)
