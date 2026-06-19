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
    rollupOptions: {
      output: {
        // 拆分大依赖为独立 chunk，避免单 689KB bundle：react 运行时、recharts、
        // tanstack-query、字体各自独立，利于浏览器并行加载与长期缓存。
        manualChunks: {
          react: ['react', 'react-dom'],
          recharts: ['recharts'],
          query: ['@tanstack/react-query'],
          fonts: ['@fontsource-variable/geist', '@fontsource-variable/newsreader'],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
