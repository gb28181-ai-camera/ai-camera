import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// 官网介绍页（Web）构建：入口 website.html，base /，产物 dist-website/
// 官网不依赖后端，因此不配置 proxy。
export default defineConfig({
  base: '/',
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
  build: {
    outDir: 'dist-website',
    emptyOutDir: true,
    rollupOptions: {
      input: path.resolve(__dirname, 'website.html'),
    },
  },
  server: {
    port: 5174,
    open: true,
  },
})
