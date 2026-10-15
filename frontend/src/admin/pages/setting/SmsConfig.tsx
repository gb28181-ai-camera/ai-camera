import React, { useCallback, useEffect, useState } from 'react'
import { Form, Input, Button, Alert, Space, message } from 'antd'
import { SaveOutlined, ReloadOutlined } from '@ant-design/icons'
import PageContainer from '../../components/PageContainer'
import { SettingController } from '../../api/generated/SettingController'
import type { UpdateSmsReq } from '../../api/generated/definitions/UpdateSmsReq'
import { useAuthStore } from '../../store/authStore'

/** 腾讯云短信配置表单值 */
interface SmsFormValues {
  secret_id: string
  secret_key: string
  sdk_app_id: string
  sign_name: string
}

/**
 * 腾讯云短信配置。
 * SecretKey 由后端脱敏返回，提交脱敏串时后端会跳过不覆盖。
 */
const SmsConfig: React.FC = () => {
  const permCodes = useAuthStore((s) => s.permCodes)
  const canUpdate = permCodes.includes('setting:thirdparty:sms:update')

  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<SmsFormValues>()

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const res = await SettingController.getSms()
      form.setFieldsValue({
        secret_id: res.data?.secret_id ?? '',
        secret_key: res.data?.secret_key ?? '',
        sdk_app_id: res.data?.sdk_app_id ?? '',
        sign_name: res.data?.sign_name ?? '',
      })
    } catch {
      message.error('获取短信配置失败')
    } finally {
      setLoading(false)
    }
  }, [form])

  useEffect(() => { fetchData() }, [fetchData])

  const handleSave = async () => {
    const values = await form.validateFields()
    const payload: UpdateSmsReq = {
      secret_id: values.secret_id,
      secret_key: values.secret_key,
      sdk_app_id: values.sdk_app_id,
      sign_name: values.sign_name,
    }
    setSaving(true)
    try {
      await SettingController.updateSms(payload)
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
      title="腾讯云短信配置"
      subTitle="配置腾讯云短信通道，用于短信告警通知"
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
        description="SecretKey 由服务端脱敏返回，未修改时请保留脱敏串，服务端将跳过不覆盖原值。"
      />

      <Form form={form} layout="vertical" style={{ maxWidth: 560 }} disabled={loading}>
        <Form.Item name="secret_id" label="SecretId" rules={[{ required: true, message: '请输入 SecretId' }]}>
          <Input placeholder="请输入腾讯云 SecretId，如 AKIDxxxx" maxLength={100} autoComplete="off" />
        </Form.Item>
        <Form.Item name="secret_key" label="SecretKey" rules={[{ required: true, message: '请输入 SecretKey' }]}>
          <Input.Password placeholder="请输入腾讯云 SecretKey" maxLength={200} autoComplete="new-password" />
        </Form.Item>
        <Form.Item name="sdk_app_id" label="SdkAppId" rules={[{ required: true, message: '请输入短信应用 SdkAppId' }]}>
          <Input placeholder="请输入短信应用 SdkAppId，如 1400000000" maxLength={50} autoComplete="off" />
        </Form.Item>
        <Form.Item name="sign_name" label="短信签名" rules={[{ required: true, message: '请输入短信签名' }]}>
          <Input placeholder="请输入短信签名，如：AI摄像头" maxLength={100} autoComplete="off" />
        </Form.Item>
      </Form>
    </PageContainer>
  )
}

export default SmsConfig
