import DeviceList from '../iot/DeviceList'

/**
 * 实时音视频 → 设备管理。
 *
 * 复用 IOT 设备管理页，只展示产品绑定了 WebRTC 服务组的设备。
 */
export default function WebRtcDeviceList() {
  return (
    <DeviceList
      serviceGroupType="webrtc"
      title="实时音视频设备"
      subTitle="仅展示产品已绑定 WebRTC 服务组的设备；可维护产品、主账号与接入密钥"
    />
  )
}
