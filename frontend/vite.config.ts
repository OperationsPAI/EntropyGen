import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import semi from '@douyinfe/vite-plugin-semi'

export default defineConfig({
  plugins: [
    react(),
    semi({
      theme: '@douyinfe/semi-theme-default',
    }),
  ],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
})
