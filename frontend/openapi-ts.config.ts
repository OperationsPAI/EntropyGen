import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../docs/openapi.json',
  output: {
    path: 'src/api/generated',
    lint: false,
  },
  plugins: [
    {
      name: '@hey-api/client-axios',
      runtimeConfigPath: '../client-config',
    },
    '@hey-api/sdk',
    '@hey-api/typescript',
  ],
});
