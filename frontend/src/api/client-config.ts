import type { CreateClientConfig } from './generated/client';

/**
 * Runtime configuration for the generated API client.
 * This function is called by the generated client.gen.ts during initialization.
 */
export const createClientConfig: CreateClientConfig = (override) => ({
  ...override,
  baseURL: '/api',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json', ...override?.headers },
});
