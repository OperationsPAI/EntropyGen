import { client } from './generated/client.gen';

export function setupApiClient() {
  client.instance.interceptors.request.use((config) => {
    const token = localStorage.getItem('jwt_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  });

  client.instance.interceptors.response.use(
    (response) => response,
    (error) => Promise.reject(error),
  );
}

// Re-export the client for direct usage (e.g., WebSocket URLs, manual requests)
export { client };
