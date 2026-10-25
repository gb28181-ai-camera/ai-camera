import ServiceGroupList from '../iot/ServiceGroupList'

/**
 * 实时音视频 → 服务设置。
 *
 * 固定展示 WebRTC 服务组，维护组内服务地址与端口。
 */
export default function WebRtcServiceSettings() {
  return (
    <ServiceGroupList
      groupType="webrtc"
      title="实时音视频服务设置"
      subTitle="维护 WebRTC 服务组下的服务地址与端口；被产品引用后只读，需先解绑"
    />
  )
}
