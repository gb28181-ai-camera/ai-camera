import { useState } from 'react'
import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { App as AntdApp, Button, Card, Form, Input } from 'antd'
import { useNavigate } from 'react-router-dom'
import { AuthWebController } from '../api/generated/AuthWebController'
import { useAuthStore } from '../store/authStore'

interface LoginForm {
  user_name: string
  password: string
}

/**
 * 登录页：`POST /web/auth/login`。
 * 成功后写入 token / user / permissions，并跳转到首页。
 */
export default function Login() {
  const navigate = useNavigate()
  const { message } = AntdApp.useApp()

  const setToken = useAuthStore((state) => state.setToken)
  const setUser = useAuthStore((state) => state.setUser)
  const setPermissions = useAuthStore((state) => state.setPermissions)

  const [loading, setLoading] = useState(false)

  const onFinish = async (values: LoginForm) => {
    setLoading(true)
    try {
      const res = await AuthWebController.login(values)
      const data = res?.data

      if (res?.code !== 200 || !data?.token) {
        message.error(res?.message || '用户名或密码错误')
        return
      }

      setToken(data.token)
      setUser({
        user_id: data.user_id,
        name: data.name,
        dept_id: data.dept_id,
        data_perm: data.data_perm,
        cross_dept_ids: data.cross_dept_ids,
      })
      setPermissions(data.permissions ?? [])

      message.success('登录成功')
      navigate('/', { replace: true })
    } catch (err) {
      // /auth/login 的失败由本页提示（响应拦截器对登录请求不跳转、不弹窗）
      const resp = (err as { response?: { data?: { message?: string } } })?.response
      message.error(resp?.data?.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <Card className="login-card" variant="borderless">
        <div className="login-title">AI Camera</div>
        <div className="login-subtitle">AI 智能监控平台 · 管理后台</div>

        <Form<LoginForm> name="login" size="large" onFinish={onFinish} autoComplete="off">
          <Form.Item name="user_name" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" allowClear />
          </Form.Item>

          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" block loading={loading}>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
