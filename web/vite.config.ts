import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'

// 构建产物落到 Go embed 目标 cmd/insights/static/dist；
// base=/static/ 与 Go 的 /static/ FileServer 对齐，资源走 /static/assets/*。
export default defineConfig({
  plugins: [react()],
  base: '/static/',
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  build: {
    outDir: '../cmd/insights/static/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
