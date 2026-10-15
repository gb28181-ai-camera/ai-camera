import { defineConfig } from 'vite'

// https://www.electronforge.io/config/plugins/vite
export default defineConfig({
  build: {
    rollupOptions: {
      // electron 由运行时提供，不参与打包
      external: ['electron'],
    },
  },
})
