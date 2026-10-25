import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// 官网介绍页（Web）构建：入口 website.html，base /，产物 dist-website/
// 官网不依赖后端，因此不配置 proxy。
export default defineConfig({
  base: '/',
  // 官网不引用 public/ 下的任何资源；若不关闭，Vite 会把管理后台的
  // h265web/jessibuca 播放器静态资源（约 9.8MB）一起复制进 dist-website。
  publicDir: false,
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
