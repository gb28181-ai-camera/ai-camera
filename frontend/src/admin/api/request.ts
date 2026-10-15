import axios from 'axios'
import { message } from 'antd'

const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

// 请求拦截器 - 注入 Token（从 localStorage 读取，保证 React 挂载前也可用）
request.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

/** 判定请求是否来自登录接口（登录失败需要留在原地提示，而不是整页跳转） */
function isLoginRequest(url?: string): boolean {
  return String(url ?? '').includes('/auth/login')
}

// 响应拦截器 - 统一错误处理
request.interceptors.response.use(
  (res) => res.data, // 拆包：调用方拿到 { code, message, data }
  (err) => {
    // 登录接口的失败由登录页 catch 后提示：
    // 既不做整页跳转（会让页面「闪一下」且用户看不到失败原因），也不在此弹窗（避免重复提示）。
    if (isLoginRequest(err.config?.url)) {
      return Promise.reject(err)
    }
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      // 保留 vite base 前缀（如 /admin/），避免跳转到不存在的 /login
      window.location.href = `${import.meta.env.BASE_URL || '/'}login`
    } else {
      message.error(err.response?.data?.message || '请求失败')
    }
    return Promise.reject(err)
  }
)

export default request
