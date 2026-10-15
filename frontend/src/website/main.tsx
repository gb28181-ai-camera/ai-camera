import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import './index.css'

const container = document.getElementById('root')

if (!container) {
  throw new Error('官网入口挂载失败：未找到 id 为 "root" 的 DOM 节点')
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
