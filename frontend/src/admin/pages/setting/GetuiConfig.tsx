import React, { useCallback, useEffect, useState } from 'react'
import { Form, Input, Button, Alert, Space, message } from 'antd'
import { SaveOutlined, ReloadOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import { SettingController } from '../../api/generated/SettingController'
import type { UpdateGetuiReq } from '../../api/generated/definitions/UpdateGetuiReq'
import { useAuthStore } from '../../store/authStore'

/** 个推配置表单值 */
interface GetuiFormValues {
  app_id: string
  app_key: string
  app_secret: string
  master_secret: string
}

/**
 * 个推账号绑定。
 * 敏感字段（AppSecret / MasterSecret）由后端脱敏返回，提交脱敏串时后端会跳过不覆盖。
 */
const GetuiConfig: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canUpdate = permCodes.includes('setting:thirdparty:getui:update')

  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<GetuiFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await SettingController.getGetui()
      form.setFieldsValue({
        app_id: res.data?.app_id ?? '',
        app_key: res.data?.app_key ?? '',
        app_secret: res.data?.app_secret ?? '',
        master_secret: res.data?.master_secret ?? '',
      })
    } catch {
      message.error('获取个推配置失败')
    } finally {
      setLoading(false)
    }
  }, [form])

  useEffect(() => { fetchData() }, [fetchData])

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: UpdateGetuiReq = {
      app_id: values.app_id,
      app_key: values.app_key,
      app_secret: values.app_secret,
      master_secret: values.master_secret,
    }
    setSaving(true)
    try {
      await SettingController.updateGetui(payload)
      message.success('保存成功')
      fetchData()
    } catch {
      message.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <PageContainer
      title="个推账号绑定"
      subTitle="配置个推推送账号，用于移动端 APP 推送通知"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>
            刷新
          </Button>
          {canUpdate && (
            <Button type="primary" icon={<SaveOutlined />} onClick={handleSave} loading={saving}>
              保存
            </Button>
          )}
        </Space>
      }
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 24 }}
        message="敏感配置以脱敏形式展示"
        description="AppSecret 与 MasterSecret 由服务端脱敏返回，未修改时请保留脱敏串，服务端将跳过不覆盖原值。"
      />

      <Form form={form} layout="vertical" style={{ maxWidth: 560 }} disabled={loading}>
        <Form.Item name="app_id" label="AppID" rules={[{ required: true, message: '请输入 AppID' }]}>
          <Input placeholder="请输入个推 AppID" maxLength={100} autoComplete="off" />
        </Form.Item>
        <Form.Item name="app_key" label="AppKey" rules={[{ required: true, message: '请输入 AppKey' }]}>
          <Input placeholder="请输入个推 AppKey" maxLength={100} autoComplete="off" />
        </Form.Item>
        <Form.Item name="app_secret" label="AppSecret" rules={[{ required: true, message: '请输入 AppSecret' }]}>
          <Input.Password placeholder="请输入个推 AppSecret" maxLength={200} autoComplete="new-password" />
        </Form.Item>
        <Form.Item name="master_secret" label="MasterSecret" rules={[{ required: true, message: '请输入 MasterSecret' }]}>
          <Input.Password placeholder="请输入个推 MasterSecret" maxLength={200} autoComplete="new-password" />
        </Form.Item>
      </Form>
    </PageContainer>
  )
}

export default GetuiConfig
