import { lazy, Suspense } from 'react'
import type { ComponentType, LazyExoticComponent, ReactNode } from 'react'
import { BrowserRouter, HashRouter, Navigate, Route, Routes } from 'react-router-dom'
import { App as AntdApp, ConfigProvider, Spin } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import MainLayout from './layouts/MainLayout'
import { useAuthStore } from './store/authStore'

dayjs.locale('zh-cn')

declare const __IS_ELECTRON__: boolean

/** 桌面端（Electron 渲染进程）标识，由各 vite 配置的 define 注入 */
const IS_ELECTRON = typeof __IS_ELECTRON__ !== 'undefined' && __IS_ELECTRON__

type LazyPage = LazyExoticComponent<ComponentType>

function PageLoading() {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '60vh',
        width: '100%',
      }}
    >
      <Spin size="large" tip="加载中..." />
    </div>
  )
}

/** 未登录拦截：无 token 时重定向到登录页 */
function ProtectedRoute({ children }: { children: ReactNode }) {
  const token = useAuthStore((state) => state.token)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

const Login = lazy(() => import('./pages/Login'))

/**
 * 业务路由表（全部懒加载）。
 * path 为相对于 `/`（MainLayout）的子路由，与 `pages/` 目录结构一一对应。
 */
const ROUTES: { path: string; page: LazyPage }[] = [
  { path: 'dashboard', page: lazy(() => import('./pages/Dashboard')) },

  // 个人中心与系统管理
  { path: 'profile', page: lazy(() => import('./pages/system/Profile')) },
  { path: 'system/user', page: lazy(() => import('./pages/system/UserList')) },
  { path: 'system/role', page: lazy(() => import('./pages/system/RoleList')) },
  { path: 'system/menu', page: lazy(() => import('./pages/system/MenuTree')) },
  { path: 'system/dept', page: lazy(() => import('./pages/system/DeptTree')) },
  { path: 'system/post', page: lazy(() => import('./pages/system/PostList')) },
  { path: 'system/config', page: lazy(() => import('./pages/system/ConfigList')) },
  { path: 'system/operation-log', page: lazy(() => import('./pages/system/OperationLogList')) },
  { path: 'system/notification', page: lazy(() => import('./pages/system/NotificationList')) },

  // 基础信息
  { path: 'basic/region', page: lazy(() => import('./pages/basic/RegionList')) },
  { path: 'basic/risk-type', page: lazy(() => import('./pages/basic/RiskTypeList')) },
  { path: 'basic/alarm-type', page: lazy(() => import('./pages/basic/AlarmTypeList')) },

  // 资源管理（设备 / 语音 / 物联网卡 / 设备模式）
  { path: 'settings/device', page: lazy(() => import('./pages/setting/DeviceList')) },
  { path: 'settings/voice', page: lazy(() => import('./pages/setting/VoiceConfigList')) },
  { path: 'settings/iot-card', page: lazy(() => import('./pages/setting/IotCardList')) },
  { path: 'settings/iot-card-replace', page: lazy(() => import('./pages/setting/IotCardReplaceList')) },
  { path: 'settings/device-mode', page: lazy(() => import('./pages/setting/DeviceModeList')) },
  { path: 'settings/alarm', page: lazy(() => import('./pages/setting/AlarmSettingList')) },
  { path: 'settings/risk-notify', page: lazy(() => import('./pages/setting/RiskNotifySettingList')) },
  { path: 'settings/webhooks', page: lazy(() => import('./pages/setting/WebhookList')) },

  // 升级管理（固件 / AI 模型 / 固件类型 / 设备升级）
  { path: 'settings/upgrade', page: lazy(() => import('./pages/upgrade/DeviceUpgradeList')) },
  { path: 'settings/firmware', page: lazy(() => import('./pages/upgrade/FirmwareList')) },
  { path: 'settings/model', page: lazy(() => import('./pages/upgrade/AiModelList')) },
  { path: 'settings/firmware-type', page: lazy(() => import('./pages/upgrade/FirmwareTypeList')) },

  // 第三方平台配置（个推 / 短信）
  { path: 'settings/getui', page: lazy(() => import('./pages/setting/GetuiConfig')) },
  { path: 'settings/sms', page: lazy(() => import('./pages/setting/SmsConfig')) },

  // 风险监控
  { path: 'risk/ai-risk', page: lazy(() => import('./pages/risk/AiRiskList')) },
  { path: 'risk/ai-monitor', page: lazy(() => import('./pages/risk/AiMonitor')) },
  { path: 'risk/monitor', page: lazy(() => import('./pages/risk/RiskMonitorList')) },

  // 设备监控
  { path: 'monitor/realtime', page: lazy(() => import('./pages/monitor/RealtimeMonitor')) },
  { path: 'device/monitor', page: lazy(() => import('./pages/device/DeviceMonitorList')) },
  { path: 'device/history', page: lazy(() => import('./pages/device/DeviceHistoryList')) },
  { path: 'device/iot-card-monitor', page: lazy(() => import('./pages/device/IotCardMonitorList')) },
  { path: 'device/gis', page: lazy(() => import('./pages/device/GisMap')) },

  // 设备告警
  { path: 'alarm/list', page: lazy(() => import('./pages/alarm/AlarmList')) },

  // 巡检中心
  { path: 'inspect/task', page: lazy(() => import('./pages/inspect/InspectTaskList')) },
  { path: 'inspect/record', page: lazy(() => import('./pages/inspect/InspectRecordList')) },

  // 统计分析
  { path: 'stats/risk', page: lazy(() => import('./pages/stats/RiskStats')) },
  { path: 'stats/alarm', page: lazy(() => import('./pages/stats/AlarmStats')) },
  { path: 'screen', page: lazy(() => import('./pages/stats/ScreenPage')) },
]

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <MainLayout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        {ROUTES.map(({ path, page: Page }) => (
          <Route key={path} path={path} element={<Page />} />
        ))}
      </Route>
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}

/**
 * 应用根组件。
 * - 桌面端使用 HashRouter（`file://` 下 BrowserRouter 不可用），Web 端使用 BrowserRouter + basename `/admin`
 * - 全局 antd 主题：中文语言包、紧凑组件尺寸、12px 字号
 */
export default function App() {
  return (
    <ConfigProvider locale={zhCN} componentSize="small" theme={{ token: { fontSize: 12 } }}>
      <AntdApp>
        {IS_ELECTRON ? (
          <HashRouter>
            <Suspense fallback={<PageLoading />}>
              <AppRoutes />
            </Suspense>
          </HashRouter>
        ) : (
          <BrowserRouter basename="/admin">
            <Suspense fallback={<PageLoading />}>
              <AppRoutes />
            </Suspense>
          </BrowserRouter>
        )}
      </AntdApp>
    </ConfigProvider>
  )
}
