import DeviceList from '../iot/DeviceList'

/**
 * 智能语音聊天 → 设备管理。
 *
 * 复用 IOT 设备管理页，只展示产品绑定了 AIChat 服务组的设备。
 */
export default function AiChatDeviceList() {
  return (
    <DeviceList
      serviceGroupType="aichat"
      title="智能语音聊天设备"
      subTitle="仅展示产品已绑定 AIChat 服务组的设备；可维护产品、主账号与接入密钥"
    />
  )
}
