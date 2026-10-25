import ServiceGroupList from '../iot/ServiceGroupList'

/**
 * 智能语音聊天 → 服务设置。
 *
 * 固定展示 AIChat 服务组，维护 WebSocket 接入地址。
 */
export default function AiChatServiceSettings() {
  return (
    <ServiceGroupList
      groupType="aichat"
      title="智能语音聊天服务设置"
      subTitle="维护 AIChat 服务组下的 WebSocket 接入地址；被产品引用后只读，需先解绑"
    />
  )
}
