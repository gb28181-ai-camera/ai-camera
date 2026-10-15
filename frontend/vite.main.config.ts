import { defineConfig } from 'vite'
import path from 'path'

// https://www.electronforge.io/config/plugins/vite
export default defineConfig({
  build: {
    rollupOptions: {
      // electron 由运行时提供，不参与打包
      external: ['electron'],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
})
