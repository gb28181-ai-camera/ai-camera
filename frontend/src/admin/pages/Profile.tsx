import { useCallback, useEffect, useState } from 'react'
import {
  Alert, Button, Descriptions, Form, Input, Modal, Popconfirm, Space, Table, Tabs, Tag, message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  CopyOutlined, DeleteOutlined, KeyOutlined, LockOutlined, PlusOutlined, SaveOutlined, UserOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import PageContainer from '../components/PageContainer'
import ActionButtons from '../components/ActionButtons'
import { ProfileController } from '../api/generated/ProfileController'
import type { User } from '../api/generated/definitions/User'
import type { ApiKey } from '../api/generated/definitions/ApiKey'

/** 基本信息表单值 */
interface ProfileFormValues {
  name: string
  email?: string
  phone?: string
  remark?: string
}

/** 修改密码表单值（confirm 仅前端校验使用，不提交后端） */
interface PasswordFormValues {
  old_password: string
  new_password: string
  confirm_password: string
}

/**
 * 个人中心：基本信息 / 修改密码 / API 密钥 三个页签。
 * 登录名与所属部门不可自行修改，需由管理员在「用户管理」中调整。
 */
export default function Profile() {
  const [profile, setProfile] = useState<User | null>(null)
  const [loading, setLoading] = useState(false)
  const [savingProfile, setSavingProfile] = useState(false)
  const [savingPassword, setSavingPassword] = useState(false)

  const [keys, setKeys] = useState<ApiKey[]>([])
  const [keysLoading, setKeysLoading] = useState(false)
  const [keyModalOpen, setKeyModalOpen] = useState(false)
  const [generating, setGenerating] = useState(false)
  // 明文密钥仅生成时返回一次，关闭弹窗即丢弃
  const [plainKey, setPlainKey] = useState('')

  const [form] = Form.useForm<ProfileFormValues>()
  const [pwdForm] = Form.useForm<PasswordFormValues>()
  const [keyForm] = Form.useForm<{ name: string }>()

  const fetchProfile = useCallback(async () => {
    setLoading(true)
    try {
      const res = await ProfileController.getProfile()
      if (res.code === 200) {
        const data = res.data ?? null
        setProfile(data)
        // 只回填可修改字段，避免把 dept / post / roles 等关联对象带入表单
        form.setFieldsValue({
          name: data?.name,
          email: data?.email,
          phone: data?.phone,
          remark: data?.remark,
        })
      } else {
        message.error(res.message || '获取个人资料失败')
      }
    } finally {
      setLoading(false)
    }
  }, [form])

  const fetchKeys = useCallback(async () => {
    setKeysLoading(true)
    try {
      const res = await ProfileController.listApiKeys()
      if (res.code === 200) {
        setKeys(res.data ?? [])
      }
    } finally {
      setKeysLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchProfile()
    fetchKeys()
  }, [fetchProfile, fetchKeys])

  /** 保存基本信息 */
  const handleSaveProfile = async () => {
    const values = await form.validateFields()
    setSavingProfile(true)
    try {
      const res = await ProfileController.updateProfile({
        name: values.name,
        email: values.email,
        phone: values.phone,
        remark: values.remark,
      })
      if (res.code === 200) {
        message.success('保存成功')
        fetchProfile()
      } else {
        message.error(res.message || '保存失败')
      }
    } catch {
      message.error('保存失败，请检查网络后重试')
    } finally {
      setSavingProfile(false)
    }
  }

  /** 修改密码（成功后需重新登录） */
  const handleChangePassword = async () => {
    const values = await pwdForm.validateFields()
    setSavingPassword(true)
    try {
      const res = await ProfileController.changePassword({
        old_password: values.old_password,
        new_password: values.new_password,
      })
      if (res.code === 200) {
        message.success('密码修改成功，请重新登录')
        pwdForm.resetFields()
      } else {
        message.error(res.message || '密码修改失败')
      }
    } catch {
      message.error('密码修改失败，请检查网络后重试')
    } finally {
      setSavingPassword(false)
    }
  }

  /** 生成 API 密钥，明文仅在本次响应中返回 */
  const handleGenerateKey = async () => {
    const values = await keyForm.validateFields()
    setGenerating(true)
    try {
      const res = await ProfileController.generateApiKey({ name: values.name })
      if (res.code === 200) {
        setPlainKey(res.data?.api_key ?? '')
        message.success('生成成功')
        fetchKeys()
      } else {
        message.error(res.message || '生成失败')
      }
    } catch {
      message.error('生成失败，请检查网络后重试')
    } finally {
      setGenerating(false)
    }
  }

  const handleCloseKeyModal = () => {
    setKeyModalOpen(false)
    setPlainKey('')
    keyForm.resetFields()
  }

  const handleCopyKey = async () => {
    try {
      await navigator.clipboard.writeText(plainKey)
      message.success('已复制到剪贴板')
    } catch {
      message.warning('当前环境不支持自动复制，请手动选中复制')
    }
  }

  const handleDeleteKey = async (id?: number) => {
    if (!id) return
    try {
      const res = await ProfileController.deleteApiKey(String(id))
      if (res.code === 200) {
        message.success('已删除，该密钥立即失效')
        fetchKeys()
      } else {
        message.error(res.message || '删除失败')
      }
    } catch {
      message.error('删除失败')
    }
  }

  const keyColumns: ColumnsType<ApiKey> = [
    { title: '密钥名称', dataIndex: 'name', key: 'name', width: 180, ellipsis: true },
    { title: '密钥前缀', dataIndex: 'key_prefix', key: 'key_prefix', width: 120 },
    {
      title: '状态', key: 'active', width: 90,
      render: (_, record) => (record.active === false
        ? <Tag color="default">已停用</Tag>
        : <Tag color="green">启用</Tag>),
    },
    {
      title: '最后使用时间', dataIndex: 'last_used_dt', key: 'last_used_dt', width: 180,
      render: (value: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '从未使用'),
    },
    {
      title: '创建时间', dataIndex: 'created_dt', key: 'created_dt', width: 180,
      render: (value: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '操作', key: 'action', width: 100, fixed: 'right' as const,
      render: (_, record) => (
        <ActionButtons>
          <Popconfirm title="确定删除?删除后该密钥立即失效。" onConfirm={() => handleDeleteKey(record.id)}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </ActionButtons>
      ),
    },
  ]

  const tabItems = [
    {
      key: 'basic',
      label: '基本信息',
      children: (
        <div>
          <Descriptions column={2} size="small" style={{ marginBottom: 24 }} bordered>
            <Descriptions.Item label="登录名">{profile?.user_name ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="姓名">{profile?.name ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="所属部门">{profile?.dept?.name ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="岗位">{profile?.post?.name ?? '-'}</Descriptions.Item>
            <Descriptions.Item label="拥有角色">
              {(profile?.roles ?? []).length > 0
                ? (profile?.roles ?? []).map((role) => <Tag key={role.id} color="blue">{role.name}</Tag>)
                : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="最后登录">
              {profile?.last_login ? dayjs(profile.last_login).format('YYYY-MM-DD HH:mm:ss') : '-'}
            </Descriptions.Item>
          </Descriptions>
          <Form form={form} layout="vertical" disabled={loading}>
            <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
              <Input placeholder="请输入姓名" />
            </Form.Item>
            <Form.Item name="email" label="邮箱" rules={[{ type: 'email', message: '邮箱格式不正确' }]}>
              <Input placeholder="请输入邮箱" />
            </Form.Item>
            <Form.Item name="phone" label="手机号">
              <Input placeholder="请输入手机号" />
            </Form.Item>
            <Form.Item name="remark" label="备注">
              <Input.TextArea rows={3} placeholder="请输入备注" />
            </Form.Item>
            <Form.Item>
              <Button type="primary" icon={<SaveOutlined />} loading={savingProfile} onClick={handleSaveProfile}>
                保存
              </Button>
            </Form.Item>
          </Form>
        </div>
      ),
    },
    {
      key: 'password',
      label: '修改密码',
      children: (
        <Form form={pwdForm} layout="vertical" style={{ maxWidth: 480 }}>
          <Form.Item name="old_password" label="原密码" rules={[{ required: true, message: '请输入原密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="请输入原密码" />
          </Form.Item>
          <Form.Item
            name="new_password"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 6, message: '密码长度不少于 6 位' },
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="请输入新密码" />
          </Form.Item>
          <Form.Item
            name="confirm_password"
            label="确认新密码"
            dependencies={['new_password']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              // 两次输入一致性校验
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('new_password') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次输入的密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="请再次输入新密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" loading={savingPassword} onClick={handleChangePassword}>确认修改</Button>
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'api-keys',
      label: 'API 密钥',
      children: (
        <div>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="API 密钥用于设备上报（风险 / 告警 / 心跳），请求时以 X-Api-Key 头传递。明文密钥只在生成时返回一次。"
          />
          <div style={{ marginBottom: 12 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setKeyModalOpen(true)}>
              生成密钥
            </Button>
          </div>
          <Table
            rowKey="id"
            size="small"
            scroll={{ x: 'max-content' }}
            columns={keyColumns}
            dataSource={keys}
            loading={keysLoading}
            pagination={false}
          />
        </div>
      ),
    },
  ]

  return (
    <PageContainer title="个人中心" subTitle="维护个人资料、登录密码与开放接口密钥">
      <Tabs defaultActiveKey="basic" items={tabItems} />

      <Modal
        className="e2e-profile-apikey-modal"
        title="生成 API 密钥"
        open={keyModalOpen}
        onOk={handleGenerateKey}
        onCancel={handleCloseKeyModal}
        confirmLoading={generating}
        okText={plainKey ? '完成' : '生成'}
        width={560}
      >
        <Form form={keyForm} layout="vertical">
          <Form.Item name="name" label="密钥名称" rules={[{ required: true, message: '请输入密钥名称' }]}>
            <Input prefix={<KeyOutlined />} placeholder="如：北门摄像头网关" disabled={Boolean(plainKey)} />
          </Form.Item>
        </Form>
        {plainKey ? (
          <div>
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message="该明文密钥仅本次展示，请立即复制保存；关闭后无法再次查看。"
            />
            <Space.Compact style={{ width: '100%' }}>
              <Input readOnly value={plainKey} />
              <Button icon={<CopyOutlined />} onClick={handleCopyKey}>复制</Button>
            </Space.Compact>
          </div>
        ) : (
          <Alert type="info" showIcon icon={<UserOutlined />} message="生成后请立即保存明文密钥。" />
        )}
      </Modal>
    </PageContainer>
  )
}
