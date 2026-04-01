import type { CreateClientConfig } from '@hey-api/client-axios';

export const createClientConfig: CreateClientConfig = () => ({
  baseURL: '/api',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});
