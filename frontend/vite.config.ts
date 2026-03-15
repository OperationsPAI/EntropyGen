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
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://10.10.10.220:30083',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
  },
})
