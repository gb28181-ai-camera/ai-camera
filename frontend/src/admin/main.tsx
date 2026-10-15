// 必须最先导入：React 19 下 antd v5 的兼容补丁（wave / message / notification / Modal 等）
import '@ant-design/v5-patch-for-react-19'

import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

const container = document.getElementById('root')
if (!container) {
  throw new Error('未找到 #root 挂载节点，请检查 index.html')
}

ReactDOM.createRoot(container).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
