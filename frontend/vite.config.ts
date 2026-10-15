import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// 管理后台（Web）构建：入口 index.html，base /admin/，产物 dist/
export default defineConfig({
  base: '/admin/',
  plugins: [react()],
  define: {
    __IS_ELECTRON__: 'false',
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '~': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/web': {
        target: `http://localhost:${process.env.VITE_BACKEND_PORT || '8080'}`,
        changeOrigin: true,
      },
      '/app': {
        target: `http://localhost:${process.env.VITE_BACKEND_PORT || '8080'}`,
        changeOrigin: true,
      },
      '/uploads': {
        target: `http://localhost:${process.env.VITE_BACKEND_PORT || '8080'}`,
        changeOrigin: true,
      },
    },
  },
})
