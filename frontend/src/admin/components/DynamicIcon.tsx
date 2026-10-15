import {
  AppstoreOutlined,
  AlertOutlined,
  ApartmentOutlined,
  BarChartOutlined,
  BellOutlined,
  BookOutlined,
  CameraOutlined,
  CarryOutOutlined,
  CloudUploadOutlined,
  ClusterOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  DesktopOutlined,
  EnvironmentOutlined,
  EyeOutlined,
  FileTextOutlined,
  FundOutlined,
  HistoryOutlined,
  IdcardOutlined,
  LineChartOutlined,
  MobileOutlined,
  MonitorOutlined,
  NotificationOutlined,
  PieChartOutlined,
  PlayCircleOutlined,
  ProfileOutlined,
  ProjectOutlined,
  SafetyCertificateOutlined,
  ScheduleOutlined,
  SettingOutlined,
  ShopOutlined,
  SignalFilled,
  SlidersOutlined,
  SoundOutlined,
  TeamOutlined,
  ThunderboltOutlined,
  ToolOutlined,
  UnorderedListOutlined,
  UserOutlined,
  UsergroupAddOutlined,
  VideoCameraOutlined,
  WarningOutlined,
  WifiOutlined,
} from '@ant-design/icons'
import type { ComponentType } from 'react'

/** 图标名称 → 组件映射。后端菜单的 `icon` 字段存放名称字符串（大小写不敏感）。 */
const ICON_MAP: Record<string, ComponentType> = {
  appstore: AppstoreOutlined,
  alert: AlertOutlined,
  apartment: ApartmentOutlined,
  barchart: BarChartOutlined,
  bell: BellOutlined,
  book: BookOutlined,
  camera: CameraOutlined,
  carryout: CarryOutOutlined,
  cloudupload: CloudUploadOutlined,
  cluster: ClusterOutlined,
  dashboard: DashboardOutlined,
  database: DatabaseOutlined,
  deploymentunit: DeploymentUnitOutlined,
  desktop: DesktopOutlined,
  environment: EnvironmentOutlined,
  eye: EyeOutlined,
  filetext: FileTextOutlined,
  fund: FundOutlined,
  history: HistoryOutlined,
  idcard: IdcardOutlined,
  linechart: LineChartOutlined,
  mobile: MobileOutlined,
  monitor: MonitorOutlined,
  notification: NotificationOutlined,
  piechart: PieChartOutlined,
  playcircle: PlayCircleOutlined,
  profile: ProfileOutlined,
  project: ProjectOutlined,
  safetycertificate: SafetyCertificateOutlined,
  schedule: ScheduleOutlined,
  setting: SettingOutlined,
  shop: ShopOutlined,
  signal: SignalFilled,
  sliders: SlidersOutlined,
  sound: SoundOutlined,
  team: TeamOutlined,
  thunderbolt: ThunderboltOutlined,
  tool: ToolOutlined,
  unorderedlist: UnorderedListOutlined,
  user: UserOutlined,
  usergroupadd: UsergroupAddOutlined,
  videocamera: VideoCameraOutlined,
  warning: WarningOutlined,
  wifi: WifiOutlined,
}

export interface DynamicIconProps {
  /** 图标名称，如 `dashboard` / `DashboardOutlined`；未知名称回退 AppstoreOutlined */
  name?: string
}

/**
 * 按名称字符串渲染 antd 图标。
 * 后端菜单 `icon` 字段为自由文本，这里做归一化（去掉 `Outlined` 后缀、下划线、连字符）。
 */
export default function DynamicIcon({ name }: DynamicIconProps) {
  const key = String(name ?? '')
    .trim()
    .replace(/outlined$/i, '')
    .replace(/[-_\s]/g, '')
    .toLowerCase()

  const Icon = ICON_MAP[key] ?? AppstoreOutlined
  return <Icon />
}
