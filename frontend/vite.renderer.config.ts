import { defineConfig, type PluginOption } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// Remove crossorigin attributes for Electron file:// protocol compatibility.
// ES module scripts with crossorigin fail when loaded from file:// URLs.
function removeCrossorigin(): PluginOption {
  return {
    name: 'remove-crossorigin',
    enforce: 'post',
    transformIndexHtml(html) {
      return html.replace(/\s+crossorigin(?=[\s>])/g, '')
    },
  }
}

// Electron 渲染进程构建：入口 index.html，产物由 @electron-forge/plugin-vite 管理。
// 不显式设置 base —— forge 的 vite 插件会注入相对 base（'./'）以适配 file:// 加载。
// https://www.electronforge.io/config/plugins/vite
export default defineConfig({
  plugins: [react(), removeCrossorigin()],
  define: {
    __IS_ELECTRON__: 'true',
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '~': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5199,
    proxy: {
      '/web': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/app': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/uploads': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
