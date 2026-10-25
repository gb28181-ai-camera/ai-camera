package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/tests/testutil"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 契约期望表
//
// 硬编码自 docs/开发文档/后端开发/00-接口契约.md §3「路由总表」。
// 这是守门测试：改了契约却忘了加/删路由、或反过来，都会在这里报出来。
// ════════════════════════════════════════════════════════════════

// contractGbRoutes 契约 §10「v0.2.0 路由总表」中**已实现**的端点，格式 "METHOD /path"。
//
// 为什么不直接并进 contractRoutes：§3 是 v0.1.0 的冻结面，§10 是 v0.2.0 的增量面，
// 分批落地。Wave 2（播放/云台/回放/语音/报警/级联）每完成一批就往这里追加，
// 未实现的 §10 端点不列 —— 否则「契约有而路由无」会长期红灯，失去守门意义。
var contractGbRoutes = []string{
	// ── §10.1 国标设备与通道 ──
	"GET /web/gb/devices",
	"GET /web/gb/devices/:id",
	"POST /web/gb/devices/:id/sync-channel",
	"POST /web/gb/devices/:id/diagnose",
	"POST /web/gb/devices/:id/subscribe",
	"DELETE /web/gb/devices/:id/subscribe",
	"POST /web/gb/devices/:id/activation",
	"DELETE /web/gb/devices/:id",
	"GET /web/gb/channels",
	"GET /web/gb/channels/options",
	"GET /web/gb/channels/:id",
	"PUT /web/gb/channels/:id",
	"DELETE /web/gb/channels/:id",
	"PUT /web/gb/channels/:id/region",
	"PUT /web/gb/channels/:id/group",
	"POST /web/gb/channels/:id/reset",

	// ── §10.6 媒体节点（流代理/推流见 Wave 2）──
	"GET /web/gb/media-servers",
	"GET /web/gb/media-servers/:id",
	"POST /web/gb/media-servers",
	"PUT /web/gb/media-servers/:id",
	"DELETE /web/gb/media-servers/:id",
	"GET /web/gb/media-servers/:id/check",
	"GET /web/gb/media-servers/:id/info",

	// ── §10.8 业务分组与订阅 ──
	"GET /web/gb/groups/tree",
	"GET /web/gb/groups",
	"POST /web/gb/groups",
	"PUT /web/gb/groups/:id",
	"DELETE /web/gb/groups/:id",
	"GET /web/gb/subscribes",
	"POST /web/gb/subscribes",
	"DELETE /web/gb/subscribes/:id",

	// ── §10.10 信令日志 ──
	"GET /web/gb/sip-logs",
	"GET /web/gb/sip-logs/:id",
	"GET /web/gb/sip-logs/stats",

	// ── §10.2 播放与云台 ──
	"POST /web/gb/play/:channelId",
	"POST /web/gb/play/:channelId/stop",
	"GET /web/gb/play/:channelId/status",
	"POST /web/gb/play/:channelId/snapshot",
	"POST /web/gb/ptz/:channelId",
	"GET /web/gb/ptz/:channelId/presets",
	"POST /web/gb/ptz/:channelId/presets",
	"DELETE /web/gb/ptz/:channelId/presets/:presetId",
	"POST /web/gb/ptz/:channelId/presets/:presetId/goto",
	"POST /web/gb/ptz/:channelId/cruise",
	"POST /web/gb/ptz/:channelId/scan",
	"POST /web/gb/ptz/:channelId/wiper",
	// GB28181-2022 扩展（同步等待设备业务应答）
	"GET /web/gb/ptz/:channelId/position",
	"POST /web/gb/ptz/:channelId/precise",
	"GET /web/gb/ptz/:channelId/cruise-tracks",
	"GET /web/gb/ptz/:channelId/cruise-tracks/:number",
	"GET /web/gb/ptz/:channelId/home-position",
	"GET /web/gb/ptz/:channelId/sdcard",
	"POST /web/gb/voice/:channelId/broadcast/start",
	"POST /web/gb/voice/:channelId/broadcast/stop",
	"POST /web/gb/voice/:channelId/talk/start",
	"POST /web/gb/voice/:channelId/talk/stop",

	// ── §10.3 回放与下载 ──
	"GET /web/gb/playback/records",
	"POST /web/gb/playback",
	"POST /web/gb/playback/control",
	"POST /web/gb/playback/stop",
	"POST /web/gb/playback/download",
	"GET /web/gb/playback/download",
	"DELETE /web/gb/playback/download/:id",

	// ── §10.4 云端录像与录像计划 ──
	"GET /web/gb/cloud-records/dates",
	"GET /web/gb/cloud-records",
	"DELETE /web/gb/cloud-records",
	"PUT /web/gb/cloud-records/collect",
	"GET /web/gb/cloud-records/:id/play",
	"GET /web/gb/cloud-records/:id/download",
	"GET /web/gb/record-plans",
	"GET /web/gb/record-plans/:id",
	"POST /web/gb/record-plans",
	"PUT /web/gb/record-plans/:id",
	"DELETE /web/gb/record-plans/:id",
	"PUT /web/gb/record-plans/:id/channels",

	// ── §10.5 国标报警与移动位置 ──
	"GET /web/gb/alarms",
	"GET /web/gb/alarms/:id",
	"POST /web/gb/alarms/:id/snapshot",
	"PUT /web/gb/alarms/:id/handle",
	"DELETE /web/gb/alarms",
	"GET /web/gb/positions/latest",
	"GET /web/gb/positions/realtime",
	"GET /web/gb/positions/history",
	"POST /web/gb/positions/subscribe",

	// ── §10.6 拉流代理与推流（媒体节点见上）──
	"GET /web/gb/stream-proxies",
	"POST /web/gb/stream-proxies",
	"PUT /web/gb/stream-proxies/:id",
	"DELETE /web/gb/stream-proxies/:id",
	"POST /web/gb/stream-proxies/:id/start",
	"POST /web/gb/stream-proxies/:id/stop",
	"GET /web/gb/stream-pushes",
	"POST /web/gb/stream-pushes",
	"PUT /web/gb/stream-pushes/:id",
	"DELETE /web/gb/stream-pushes/:id",
	"POST /web/gb/stream-pushes/:id/start",
	"POST /web/gb/stream-pushes/:id/stop",

	// ── v0.4.0 WVP 对齐：设备/通道/云台控制 ──
	"PUT /web/gb/devices/:id",
	"GET /web/gb/devices/:id/sync-status",
	"GET /web/gb/devices/:id/time-statistics",
	"POST /web/gb/devices/:id/reboot",
	"POST /web/gb/devices/:id/guard",
	"POST /web/gb/devices/:id/alarm-reset",
	"POST /web/gb/channels/:id/record-control",
	"GET /web/gb/channels/:id/media-info",
	"PUT /web/gb/ptz/:channelId/presets/:presetId",
	"POST /web/gb/ptz/:channelId/cruise-points",
	"PUT /web/gb/ptz/:channelId/cruise-points",
	"DELETE /web/gb/ptz/:channelId/cruise-points",
	"PUT /web/gb/ptz/:channelId/scan/bounds",

	// ── v0.4.0 WVP 对齐：运维/媒体/日志/分享 ──
	"GET /web/gb/media-servers/:id/metrics",
	"GET /web/gb/media-servers/:id/metrics/history",
	"GET /web/gb/media-servers/:id/streams",
	"GET /web/gb/platform-info",
	"GET /web/gb/logs",
	"GET /web/gb/logs/stream",
	"GET /web/gb/alarms/stream",
	"GET /web/gb/stream-proxies/:id/play",
	"GET /web/gb/stream-pushes/:id/play",
	"POST /web/gb/stream-pushes/import",
	"POST /web/gb/cloud-records/merge-tasks",
	"GET /web/gb/cloud-records/merge-tasks",
	"GET /web/gb/cloud-records/merge-tasks/:id",
	"GET /web/gb/cloud-records/merge-tasks/:id/download",
	"POST /web/gb/channels/:id/share-tokens",
	"GET /web/gb/channels/:id/share-tokens",
	"DELETE /web/gb/channels/:id/share-tokens/:tokenId",
	"GET /public/gb/shares/:token",
	"POST /public/gb/shares/:token/play",

	// ── §10.7 国标级联（配置面）──
	"GET /web/gb/platforms",
	"GET /web/gb/platforms/:id",
	"POST /web/gb/platforms",
	"PUT /web/gb/platforms/:id",
	"DELETE /web/gb/platforms/:id",
	"GET /web/gb/platforms/:id/channels",
	"POST /web/gb/platforms/:id/channels",
	"DELETE /web/gb/platforms/:id/channels",
	"PUT /web/gb/platforms/:id/channels/:channelId",
	"PUT /web/gb/platforms/:id/groups",
	"PUT /web/gb/platforms/:id/regions",
	"GET /web/gb/platforms/cascade/status",
	"POST /web/gb/platforms/cascade/start",
	"POST /web/gb/platforms/cascade/stop",
	"POST /web/gb/platforms/:id/catalog-push",

	// ── §10.9 语音 Agent 配置 ──
	"GET /web/settings/voice-agent",
	"PUT /web/settings/voice-agent",
	"GET /web/settings/voice-agent/providers",
	"POST /web/settings/voice-agent/test",
	"POST /web/settings/voice-agent/apply",

	// ── §10.12 移动端（/app/gb，复用 /web/gb 的 service 层）──
	"GET /app/gb/devices",
	"GET /app/gb/channels",
	"POST /app/gb/play/:channelId",
	"POST /app/gb/play/:channelId/stop",
	"POST /app/gb/play/:channelId/snapshot",
	"POST /app/gb/ptz/:channelId",
	"GET /app/gb/playback/records",
	"POST /app/gb/playback",
	"POST /app/gb/playback/control",
	"POST /app/gb/playback/stop",
	"POST /app/gb/talk/:channelId/start",
	"POST /app/gb/talk/:channelId/stop",
	"GET /app/gb/alarms",

	// ── §10.11 ZLM 回调（独立中间件组，无 JWT）──
	"POST /media/hook/on_server_keepalive",
	"POST /media/hook/on_server_started",
	"POST /media/hook/on_publish",
	"POST /media/hook/on_play",
	"POST /media/hook/on_stream_changed",
	"POST /media/hook/on_stream_none_reader",
	"POST /media/hook/on_stream_not_found",
	"POST /media/hook/on_send_rtp_stopped",
	"POST /media/hook/on_rtp_server_timeout",
	"POST /media/hook/on_record_mp4",
}

// contractIotRoutes 契约 §10.14「IOT 管理」的 Web 端点，格式 "METHOD /path"。
//
// 设备端 §10.15 的 /iot/ota 与 /iot/activate 不在这里：它们不挂 JWT/Casbin，
// 与 /media/hook、/metrics 一样属于「基础设施/设备接入」端点，登记在 infraRoutes。
var contractIotRoutes = []string{
	// ── 产品 ──
	"GET /web/iot/products",
	"GET /web/iot/products/:id",
	"POST /web/iot/products",
	"PUT /web/iot/products/:id",
	"DELETE /web/iot/products/:id",
	"POST /web/iot/products/:id/share-qr",
	"GET /web/iot/products/:id/credentials",
	"GET /web/iot/products/:id/services",
	"POST /web/iot/products/:id/services/:serviceId/reset-credentials",
	"GET /web/iot/products/:id/service-groups",
	"PUT /web/iot/products/:id/service-groups",

	// ── 服务组 ──
	"GET /web/iot/service-groups",
	"GET /web/iot/service-groups/:id",
	"POST /web/iot/service-groups",
	"PUT /web/iot/service-groups/:id",
	"DELETE /web/iot/service-groups/:id",
	"PUT /web/iot/service-groups/:id/services",

	// ── 设备与账号 ──
	"GET /web/iot/devices",
	"GET /web/iot/devices/:id",
	"POST /web/iot/devices/:id/accounts",
	"PUT /web/iot/devices/:id/accounts/:accountId",
	"DELETE /web/iot/devices/:id/accounts/:accountId",
	"POST /web/iot/devices/:id/activate",
	"POST /web/iot/devices/:id/reset-secret",
	"PUT /web/iot/devices/:id/product",
	"POST /web/iot/devices/:id/qr",
	"POST /web/iot/devices/:id/share-token",
	"DELETE /web/iot/devices/:id/share-token",
	"POST /web/iot/devices/:id/unbind",
}

// contractAppIotRoutes 契约 §10.16「移动端 IOT / RTC」的端点。
//
// 这些端点挂在 /app 的 aAuth 组，同样经过 JWT + Casbin，因此必须登记在
// ac_menu_api 中（seed 里挂在移动端「设备」菜单 105 下），否则一律 403。
var contractAppIotRoutes = []string{
	"POST /app/iot/devices/claim",
	"POST /app/iot/devices/bind",
	"POST /app/iot/devices/bind-qr",
	"GET /app/iot/devices",
	"GET /app/iot/devices/:id",
	"GET /app/iot/devices/:id/services",
	"POST /app/iot/devices/:id/share-token",
	"GET /app/rtc/ice-servers",
}

// contractVoiceRoutes v0.6.0「智能语音聊天」端点。
//
// WS 网关（/web|app|device/voice/ws）与 /internal/* 不挂 JWT/Casbin，
// 但同样属于需要守门的对外契约，一并登记；ticket 端点挂在认证组内。
var contractVoiceRoutes = []string{
	"POST /web/voice/ws-ticket",
	"GET /web/voice/ws",
	"POST /app/voice/ws-ticket",
	"GET /app/voice/ws",
	"GET /device/voice/ws",
	"GET /web/voice/memories",
	"GET /web/voice/memories/export",
	"DELETE /web/voice/memories/:id",
	"GET /web/voice/mcp/tools",
	"PUT /web/voice/mcp/tools/:id",
	"GET /web/voice/mcp/logs",
	"POST /internal/mcp",
	"POST /internal/voice/memory/retrieve",
	"POST /internal/voice/memory/write",
	"POST /web/voice/embedding/test",
	"POST /web/voice/memories/backfill",
	// v0.6.0 实时音视频：MQTT 房间信令
	"GET /web/av/devices",
	"POST /web/av/devices/:id/viewer-ticket",
	"GET /web/av/ws",
	// App 通话与离线推送
	"POST /app/av/devices/:id/call-ticket",
	"GET /app/av/ws",
	"POST /app/push-tokens",
	"DELETE /app/push-tokens/:cid",
}

// contractRoutes 契约 §3 列出的全部端点，格式 "METHOD /path"。
var contractRoutes = []string{
	// ── §3.1 公开接口 ──
	"POST /web/auth/login",
	"POST /app/auth/login",
	"GET /health",

	// ── §3.2 API Key 接口 ──
	"POST /web/risks/report",
	"POST /web/alarms/report",
	"POST /web/devices/:id/heartbeat",

	// ── §3.3 个人中心 ──
	"GET /web/profile",
	"PUT /web/profile",
	"PUT /web/profile/password",
	"GET /web/profile/menus",
	"GET /web/profile/api-keys",
	"POST /web/profile/api-keys",
	"DELETE /web/profile/api-keys/:id",

	// ── §3.4 系统管理 / 用户 ──
	"GET /web/system/users",
	"GET /web/system/users/options",
	"GET /web/system/users/:id",
	"POST /web/system/users",
	"PUT /web/system/users/:id",
	"DELETE /web/system/users/:id",
	"PUT /web/system/users/:id/enable",
	"PUT /web/system/users/:id/disable",
	"PUT /web/system/users/:id/reset-password",

	// ── §3.4 系统管理 / 角色 ──
	"GET /web/system/roles",
	"GET /web/system/roles/options",
	"GET /web/system/roles/:id",
	"POST /web/system/roles",
	"PUT /web/system/roles/:id",
	"DELETE /web/system/roles/:id",
	"GET /web/system/roles/:id/menus",
	"PUT /web/system/roles/:id/menus",
	"GET /web/system/roles/:id/depts",
	"PUT /web/system/roles/:id/depts",
	"GET /web/system/roles/:id/posts",
	"PUT /web/system/roles/:id/posts",
	"GET /web/system/roles/:id/users",
	"PUT /web/system/roles/:id/users",

	// ── §3.4 系统管理 / 菜单 ──
	"GET /web/system/menus",
	"GET /web/system/menus/:id",
	"POST /web/system/menus",
	"PUT /web/system/menus/:id",
	"DELETE /web/system/menus/:id",
	"GET /web/system/menus/:id/apis",
	"POST /web/system/menus/:id/apis",
	"DELETE /web/system/menus/apis/:apiId",
	"GET /web/system/menus/:id/roles",
	"PUT /web/system/menus/:id/roles",

	// ── §3.4 系统管理 / 部门 ──
	"GET /web/system/depts",
	"GET /web/system/depts/options",
	"GET /web/system/depts/:id",
	"POST /web/system/depts",
	"PUT /web/system/depts/:id",
	"DELETE /web/system/depts/:id",

	// ── §3.4 系统管理 / 岗位 ──
	"GET /web/system/posts",
	"GET /web/system/posts/options",
	"GET /web/system/posts/:id",
	"POST /web/system/posts",
	"PUT /web/system/posts/:id",
	"DELETE /web/system/posts/:id",

	// ── §3.4 系统管理 / 系统参数 ──
	"GET /web/system/configs",
	"POST /web/system/configs",
	"PUT /web/system/configs/:id",
	"DELETE /web/system/configs/:id",
	"PUT /web/system/configs/:id/enable",
	"PUT /web/system/configs/:id/disable",

	// ── §3.4 系统管理 / 操作日志 ──
	"GET /web/system/logs",
	"GET /web/system/logs/export",
	"DELETE /web/system/logs/clear",
	"GET /web/system/logs/:id",
	"DELETE /web/system/logs/:id",

	// ── §3.4 系统管理 / 系统通知 ──
	"GET /web/system/notifications",
	"GET /web/system/notifications/unread-count",
	"PUT /web/system/notifications/read-all",
	"PUT /web/system/notifications/:id/read",
	"DELETE /web/system/notifications/:id",

	// ── §3.5 基础信息 / 区域 ──
	"GET /web/basic/regions",
	"GET /web/basic/regions/options",
	"GET /web/basic/regions/:id",
	"POST /web/basic/regions",
	"PUT /web/basic/regions/:id",
	"DELETE /web/basic/regions/:id",

	// ── §3.5 基础信息 / 风险类型 ──
	"GET /web/basic/risk-types",
	"GET /web/basic/risk-types/options",
	"GET /web/basic/risk-types/:id",
	"POST /web/basic/risk-types",
	"PUT /web/basic/risk-types/:id",
	"DELETE /web/basic/risk-types/:id",

	// ── §3.5 基础信息 / 告警类型 ──
	"GET /web/basic/alarm-types",
	"GET /web/basic/alarm-types/options",
	"GET /web/basic/alarm-types/:id",
	"POST /web/basic/alarm-types",
	"PUT /web/basic/alarm-types/:id",
	"DELETE /web/basic/alarm-types/:id",

	// ── §3.6 资源管理 / 设备 ──
	"GET /web/devices",
	"GET /web/devices/options",
	"GET /web/devices/history",
	"GET /web/devices/:id",
	"GET /web/devices/:id/history-chart",
	"POST /web/devices",
	"PUT /web/devices/:id",
	"DELETE /web/devices/:id",
	"PUT /web/devices/:id/mode",
	"PUT /web/devices/:id/sync-time",
	"PUT /web/devices/:id/monitor",

	// ── §3.6 资源管理 / 设备模式 ──
	"GET /web/devices/modes",
	"GET /web/devices/modes/options",
	"POST /web/devices/modes",
	"PUT /web/devices/modes/:id",
	"DELETE /web/devices/modes/:id",

	// ── §3.6 资源管理 / 物联网卡 ──
	"GET /web/devices/iot-cards",
	"GET /web/devices/iot-cards/options",
	"POST /web/devices/iot-cards",
	"PUT /web/devices/iot-cards/:id",
	"DELETE /web/devices/iot-cards/:id",
	"GET /web/devices/iot-card-replace-logs",
	"POST /web/devices/iot-card-replace-logs",

	// ── §3.6 资源管理 / 语音配置 ──
	"GET /web/devices/voice-configs",
	"POST /web/devices/voice-configs",
	"PUT /web/devices/voice-configs/:id",
	"DELETE /web/devices/voice-configs/:id",

	// ── §3.7 风险 ──
	"GET /web/risks",
	"GET /web/risks/pending-count",
	"GET /web/risks/summary",
	"GET /web/risks/:id",
	"PUT /web/risks/:id/read",
	"PUT /web/risks/:id/accurate",
	"PUT /web/risks/:id/handle",
	"DELETE /web/risks/:id",

	// ── §3.8 设备告警 ──
	"GET /web/alarms",
	"GET /web/alarms/pending-count",
	"GET /web/alarms/:id",
	"PUT /web/alarms/:id/read",
	"PUT /web/alarms/:id/handle",
	"DELETE /web/alarms/:id",

	// ── §3.9 巡检 ──
	"GET /web/inspects/tasks",
	"GET /web/inspects/tasks/:id",
	"POST /web/inspects/tasks",
	"PUT /web/inspects/tasks/:id",
	"DELETE /web/inspects/tasks/:id",
	"GET /web/inspects/records",
	"GET /web/inspects/records/:id",

	// ── §3.10 升级管理 / 固件类型 ──
	"GET /web/upgrades/firmware-types",
	"GET /web/upgrades/firmware-types/options",
	"POST /web/upgrades/firmware-types",
	"PUT /web/upgrades/firmware-types/:id",
	"DELETE /web/upgrades/firmware-types/:id",

	// ── §3.10 升级管理 / 固件 ──
	"GET /web/upgrades/firmwares",
	"GET /web/upgrades/firmwares/options",
	"GET /web/upgrades/firmwares/:id",
	"POST /web/upgrades/firmwares",
	"PUT /web/upgrades/firmwares/:id",
	"DELETE /web/upgrades/firmwares/:id",

	// ── §3.10 升级管理 / AI 模型 ──
	"GET /web/upgrades/models",
	"GET /web/upgrades/models/options",
	"GET /web/upgrades/models/:id",
	"POST /web/upgrades/models",
	"PUT /web/upgrades/models/:id",
	"DELETE /web/upgrades/models/:id",

	// ── §3.10 升级管理 / 下发、计划、记录 ──
	"POST /web/upgrades/devices",
	"GET /web/upgrades/plans",
	"PUT /web/upgrades/plans/:id/cancel",
	"PUT /web/upgrades/plans/cancel",
	"GET /web/upgrades/records",

	// ── §3.11 告警设置 / 设备告警设置 ──
	"GET /web/settings/alarm-settings",
	"POST /web/settings/alarm-settings",
	"PUT /web/settings/alarm-settings/disable-all",
	"PUT /web/settings/alarm-settings/:id",
	"DELETE /web/settings/alarm-settings/:id",

	// ── §3.11 告警设置 / 风险通知设置 ──
	"GET /web/settings/risk-notify-settings",
	"POST /web/settings/risk-notify-settings",
	"PUT /web/settings/risk-notify-settings/disable-all",
	"PUT /web/settings/risk-notify-settings/:id",
	"DELETE /web/settings/risk-notify-settings/:id",

	// ── §3.11 告警设置 / 第三方平台 ──
	"GET /web/settings/getui",
	"PUT /web/settings/getui",
	"GET /web/settings/sms",
	"PUT /web/settings/sms",

	// ── §3.11 告警设置 / WebHook ──
	"GET /web/settings/webhooks",
	"POST /web/settings/webhooks",
	"GET /web/settings/webhooks/platforms",
	"POST /web/settings/webhooks/test",
	"GET /web/settings/webhooks/:id",
	"PUT /web/settings/webhooks/:id",
	"DELETE /web/settings/webhooks/:id",
	"PUT /web/settings/webhooks/:id/enable",
	"PUT /web/settings/webhooks/:id/disable",
	"POST /web/settings/webhooks/:id/test",

	// ── §3.12 统计分析 ──
	"GET /web/stats/dashboard",
	"GET /web/stats/risk",
	"GET /web/stats/alarm",
	"GET /web/stats/screen",
	"GET /web/stats/traffic",
	"GET /web/stats/device-status",

	// ── §3.13 上传 ──
	"POST /web/upload",
	"POST /web/upload/audio",

	// ── §3.14 移动端 ──
	"GET /app/menus",
	"GET /app/home",
	"GET /app/todos",
	"GET /app/risks",
	"GET /app/risks/:id",
	"PUT /app/risks/:id/handle",
	"PUT /app/risks/:id/accurate",
	"GET /app/alarms",
	"PUT /app/alarms/:id/handle",
	"GET /app/inspect-tasks",
	"GET /app/inspect-tasks/:id",
	"PUT /app/inspect-tasks/:id/finish",
	"GET /app/devices",
	"GET /app/devices/:id",
	"GET /app/notifications",
	"PUT /app/notifications/:id/read",
	"GET /app/profile",
	"PUT /app/profile",
	"PUT /app/profile/password",
}

// infraRoutes 非业务端点：静态文件服务由 gin.Static 自动注册（GET + HEAD 两条）。
// 不属于契约 §3 的范围，反向比对时排除。
var infraRoutes = map[string]bool{
	"GET /uploads/*filepath":  true,
	"HEAD /uploads/*filepath": true,
	// Prometheus 抓取端点：基础设施端点，不是业务 API，不属于契约范围。
	// 预先登记，避免接入指标时被反向比对误判为「实现了契约未列出的端点」。
	"GET /metrics": true,
	// 设备端 IOT 接口（契约 §10.15）：无 JWT / 无 Casbin，用
	// X-Device-Code + X-Device-Secret 鉴权，不属于 §3 的用户接口范围。
	"POST /iot/ota":      true,
	"POST /iot/activate": true,
	// 内部服务间 RPC（MQTT broker 鉴权）：共享 token 鉴权，不属于用户 API 契约。
	"POST /rpc/mqtt/auth": true,
	"POST /rpc/mqtt/acl":  true,
	// TURN 凭据签发：同样属于内部 RPC，不属于用户 API 契约。
	"POST /rpc/turn/credentials": true,
}

// contractGaps 已实现、但契约 §3 尚未列出的端点（实现先行造成的偏差）。
//
// 这些条目在反向比对中豁免，并以 t.Logf 记录；**新增**的未登记路由仍会让用例失败。
// 修复方向是补契约文档，不是删路由——因此这里只登记不修。
var contractGaps = map[string]string{
	"PUT /web/inspects/tasks/:id/read": "契约 §3.9 未列出巡检任务标记已读；" +
		"handler inspectH.MarkTaskRead 已实现，Swagger 注释亦齐备（@Router /web/inspects/tasks/{id}/read [put]）",
}

// routeKey 统一路由键格式。
func routeKey(method, path string) string { return method + " " + path }

// registryOf 把 gin 路由注册表转换为 "METHOD /path" -> handler 名 的映射。
func registryOf(r *gin.Engine) map[string]string {
	out := make(map[string]string, len(r.Routes()))
	for _, rt := range r.Routes() {
		out[routeKey(rt.Method, rt.Path)] = rt.Handler
	}
	return out
}

// ════════════════════════════════════════════════════════════════
// 用例 1：路由总表与契约一致
// ════════════════════════════════════════════════════════════════

// TestRouteTable_MatchesContract 双向比对路由注册表与契约 §3。
//
//	缺项 → 契约承诺了却没有实现；
//	多项 → 实现了契约未冻结的端点（可能是有意新增但忘了更新契约）。
//
// 纯注册表断言，不触碰数据库。
func TestRouteTable_MatchesContract(t *testing.T) {
	r := buildRouter(t)
	got := registryOf(r)

	t.Run("规模", func(t *testing.T) {
		// 契约 §3 共 211 条业务端点，加静态文件 2 条 = 213。
		// 下限设 200，避免小改动就让用例变成脆弱断言。
		if len(got) < 200 {
			t.Errorf("路由总数 %d < 200，疑似路由注册被整体破坏", len(got))
		}
	})

	want := make(map[string]bool, len(contractRoutes)+len(contractGbRoutes)+len(contractIotRoutes)+len(contractAppIotRoutes)+len(contractVoiceRoutes))
	all := append([]string{}, contractRoutes...)
	all = append(all, contractGbRoutes...)
	all = append(all, contractIotRoutes...)
	all = append(all, contractAppIotRoutes...)
	all = append(all, contractVoiceRoutes...)
	for _, key := range all {
		if want[key] {
			t.Errorf("契约期望表内重复条目: %s", key)
			continue
		}
		want[key] = true
	}

	t.Run("契约有而路由无", func(t *testing.T) {
		var missing []string
		for key := range want {
			if _, ok := got[key]; !ok {
				missing = append(missing, key)
			}
		}
		sort.Strings(missing)
		for _, key := range missing {
			t.Errorf("契约 §3 列出的端点未注册到路由: %s", key)
		}
	})

	t.Run("路由有而契约无", func(t *testing.T) {
		var extra []string
		for key := range got {
			if infraRoutes[key] || want[key] {
				continue
			}
			extra = append(extra, key)
		}
		sort.Strings(extra)
		for _, key := range extra {
			if reason, known := contractGaps[key]; known {
				// 已知偏差：不判失败，但必须持续可见。
				t.Logf("已知契约缺口（实现已先行）: %s —— %s", key, reason)
				continue
			}
			t.Errorf("路由注册了契约 §3 未列出的端点: %s（契约与实现需对齐其一）", key)
		}
	})

	t.Run("已知契约缺口不得消失", func(t *testing.T) {
		for key, reason := range contractGaps {
			if _, ok := got[key]; !ok {
				t.Errorf("contractGaps 登记的 %s 已不在路由表中，请同步删除该豁免（原由: %s）", key, reason)
			}
		}
	})
}

// ════════════════════════════════════════════════════════════════
// 用例 2：静态段与 :param 段同级时的注册表形态
// ════════════════════════════════════════════════════════════════

// staticParamPair 描述一组「静态子路径 / :param 兄弟」路由。
type staticParamPair struct {
	Method        string
	Static        string
	StaticHandler string // 期望命中的 handler 名片段
	Param         string
	ParamHandler  string
}

// staticParamPairs 覆盖历史上出过问题的全部同级冲突点。
//
// 回归来源：/web/devices/modes 曾被 /web/devices/:id 吞掉，
// 表现为 id="modes" 走到设备详情、返回「无效的设备ID」。
var staticParamPairs = []staticParamPair{
	{"GET", "/web/devices/modes", "(*DeviceModeHandler).Page", "/web/devices/:id", "(*DeviceHandler).Get"},
	{"GET", "/web/devices/iot-cards", "(*IotCardHandler).Page", "/web/devices/:id", "(*DeviceHandler).Get"},
	{"GET", "/web/devices/voice-configs", "(*VoiceConfigHandler).Page", "/web/devices/:id", "(*DeviceHandler).Get"},
	{"GET", "/web/devices/options", "(*DeviceHandler).Options", "/web/devices/:id", "(*DeviceHandler).Get"},
	{"GET", "/web/devices/history", "(*DeviceHandler).PageHistory", "/web/devices/:id", "(*DeviceHandler).Get"},
	{"DELETE", "/web/system/menus/apis/:apiId", "(*MenuHandler).DeleteAPI", "/web/system/menus/:id", "(*MenuHandler).Delete"},
	{"GET", "/web/settings/webhooks/platforms", "(*WebhookHandler).Platforms", "/web/settings/webhooks/:id", "(*WebhookHandler).Get"},
	{"POST", "/web/settings/webhooks/test", "(*WebhookHandler).TestInline", "/web/settings/webhooks/:id/test", "(*WebhookHandler).Test"},
	{"PUT", "/web/upgrades/plans/cancel", "(*UpgradeHandler).BatchCancelPlans", "/web/upgrades/plans/:id/cancel", "(*UpgradeHandler).CancelPlan"},
	{"GET", "/web/upgrades/firmwares/options", "(*UpgradeHandler).FirmwareOptions", "/web/upgrades/firmwares/:id", "(*UpgradeHandler).GetFirmware"},
	{"GET", "/web/upgrades/models/options", "(*UpgradeHandler).ModelOptions", "/web/upgrades/models/:id", "(*UpgradeHandler).GetModel"},
	{"DELETE", "/web/system/logs/clear", "(*OperationLogHandler).Clear", "/web/system/logs/:id", "(*OperationLogHandler).Delete"},
	{"GET", "/web/system/logs/export", "(*OperationLogHandler).Export", "/web/system/logs/:id", "(*OperationLogHandler).Get"},
	{"GET", "/web/system/users/options", "(*UserHandler).Options", "/web/system/users/:id", "(*UserHandler).Get"},
	{"GET", "/web/system/roles/options", "(*RoleHandler).Options", "/web/system/roles/:id", "(*RoleHandler).Get"},
	{"GET", "/web/system/depts/options", "(*DeptHandler).Options", "/web/system/depts/:id", "(*DeptHandler).Get"},
	{"GET", "/web/system/posts/options", "(*PostHandler).Options", "/web/system/posts/:id", "(*PostHandler).Get"},
	{"GET", "/web/risks/pending-count", "(*RiskHandler).PendingCount", "/web/risks/:id", "(*RiskHandler).Get"},
	{"GET", "/web/alarms/pending-count", "(*AlarmHandler).PendingCount", "/web/alarms/:id", "(*AlarmHandler).Get"},
	{"GET", "/web/basic/regions/options", "(*BasicHandler).RegionOptions", "/web/basic/regions/:id", "(*BasicHandler).GetRegion"},
	{"GET", "/web/basic/risk-types/options", "(*BasicHandler).RiskTypeOptions", "/web/basic/risk-types/:id", "(*BasicHandler).GetRiskType"},
	{"GET", "/web/basic/alarm-types/options", "(*BasicHandler).AlarmTypeOptions", "/web/basic/alarm-types/:id", "(*BasicHandler).GetAlarmType"},
	{"PUT", "/web/settings/alarm-settings/disable-all", "(*SettingHandler).DisableAllAlarm", "/web/settings/alarm-settings/:id", "(*SettingHandler).UpdateAlarmSetting"},
	{"PUT", "/web/settings/risk-notify-settings/disable-all", "(*SettingHandler).DisableAllRiskNotify", "/web/settings/risk-notify-settings/:id", "(*SettingHandler).UpdateRiskNotifySetting"},
}

// TestRouteRegistry_StaticSiblingsPrecedeParam 结构断言。
//
// 断言两件事：
//  1. 静态路径与 :param 兄弟**都**已注册，且各自绑定到预期的 handler
//     （防「静态路径被误绑到 :id 的 handler」或「路由被删除」）；
//  2. 注册表输出中静态路径排在 :param 之前。
//
// 关于第 2 点的真实价值：gin v1.12 的 radix tree 会把静态子节点规范化到
// :param 之前，无论注册顺序如何（见 TestGinNormalizesSiblingOrder）。
// 因此该断言在当今 gin 版本下**不会**因注册顺序写反而失败——它的作用是
// 固定住「静态优先」这一意图，并在 gin 升级改变该行为时立刻报警。
// 语义层面的真实防线是 TestRoutePriority_StaticWinsOverParam。
func TestRouteRegistry_StaticSiblingsPrecedeParam(t *testing.T) {
	r := buildRouter(t)
	routes := r.Routes()

	index := make(map[string]int, len(routes))
	handler := make(map[string]string, len(routes))
	for i, rt := range routes {
		key := routeKey(rt.Method, rt.Path)
		index[key] = i
		handler[key] = rt.Handler
	}

	for _, p := range staticParamPairs {
		p := p
		name := p.Method + " " + p.Static
		t.Run(name, func(t *testing.T) {
			staticKey := routeKey(p.Method, p.Static)
			staticIdx, ok := index[staticKey]
			if !ok {
				t.Fatalf("静态路由未注册: %s", staticKey)
			}
			if p.StaticHandler != "" && !strings.Contains(handler[staticKey], p.StaticHandler) {
				t.Errorf("静态路由 %s 绑定了错误的 handler:\n  实际 %s\n  期望包含 %s",
					staticKey, handler[staticKey], p.StaticHandler)
			}

			paramKey := routeKey(p.Method, p.Param)
			paramIdx, ok := index[paramKey]
			if !ok {
				t.Fatalf(":param 兄弟路由未注册: %s", paramKey)
			}
			if p.ParamHandler != "" && !strings.Contains(handler[paramKey], p.ParamHandler) {
				t.Errorf(":param 路由 %s 绑定了错误的 handler:\n  实际 %s\n  期望包含 %s",
					paramKey, handler[paramKey], p.ParamHandler)
			}

			if staticIdx >= paramIdx {
				t.Errorf("静态路由 %s 在注册表中排在 %s 之后（#%d >= #%d）——静态段必须先于 :param 段",
					staticKey, paramKey, staticIdx, paramIdx)
			}
		})
	}
}

// TestGinNormalizesSiblingOrder 记录 gin v1.12 的路由树行为。
//
// 该行为决定了「注册顺序」本身不是可靠防线：静态与 :param 同级时，
// gin 会把静态子节点排到前面并优先匹配，与注册先后无关。
// 若此用例失败（gin 升级改变了行为），说明注册顺序重新变成了硬约束，
// 需要回到 web.go 逐条核对静态路径的位置。
func TestGinNormalizesSiblingOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := func(c *gin.Context) { c.String(http.StatusOK, "hit") }

	// 故意先注册 :param，再注册静态段。
	e.GET("/probe/:id", h)
	e.GET("/probe/modes", h)

	var staticIdx, paramIdx = -1, -1
	for i, rt := range e.Routes() {
		switch rt.Path {
		case "/probe/modes":
			staticIdx = i
		case "/probe/:id":
			paramIdx = i
		}
	}
	if staticIdx < 0 || paramIdx < 0 {
		t.Fatalf("探测路由未注册: static=%d param=%d", staticIdx, paramIdx)
	}
	if staticIdx > paramIdx {
		t.Errorf("gin 不再规范化同级顺序：静态段 #%d 排在 :param #%d 之后；"+
			"web.go 的注册顺序重新成为硬约束", staticIdx, paramIdx)
	}
}

// ════════════════════════════════════════════════════════════════
// 用例 3：真实请求的 handler 命中（语义防线）
// ════════════════════════════════════════════════════════════════

// TestRoutePriority_StaticWinsOverParam 经真实路由发送请求，按响应特征区分命中的 handler。
//
// 这是「静态路径被 :id 吞掉」的核心回归测试：若被吞掉，
// `GET /web/devices/modes` 会走进设备详情并返回 400「无效的设备ID」。
//
// 需要数据库：多数静态路径的 handler 会执行一次分页/选项查询。
// 用 testutil.WithTx 包一层，所有写入随事务回滚。
func TestRoutePriority_StaticWinsOverParam(t *testing.T) {
	r := buildRouter(t)
	token := validToken(t)

	testutil.WithTx(t, func(_ context.Context, _ *gorm.DB) {
		t.Run("静态段命中各自的列表 handler", func(t *testing.T) {
			for _, target := range []string{
				"/web/devices/modes",
				"/web/devices/iot-cards",
				"/web/devices/voice-configs",
				"/web/devices/options",
				"/web/devices/history",
				"/web/upgrades/firmwares/options",
				"/web/upgrades/models/options",
			} {
				rec := doRequest(r, http.MethodGet, target, token, "")
				resp := decodeResp(t, rec)
				if rec.Code != http.StatusOK || resp.Code != 200 {
					t.Errorf("GET %s: 期望 200，实际 status=%d code=%d message=%q（疑似被 :id 吞掉）",
						target, rec.Code, resp.Code, resp.Message)
				}
			}
		})

		t.Run("WebHook 静态段不被 :id 吞掉", func(t *testing.T) {
			rec := doRequest(r, http.MethodGet, "/web/settings/webhooks/platforms", token, "")
			resp := decodeResp(t, rec)
			if rec.Code != http.StatusOK || resp.Code != 200 {
				t.Errorf("GET /web/settings/webhooks/platforms: 期望 200，实际 status=%d code=%d message=%q"+
					"（message 为「非法的配置ID」即说明落到了 WebhookHandler.Get）",
					rec.Code, resp.Code, resp.Message)
			}

			// POST /web/settings/webhooks/test 命中 TestInline：空对象先撞请求体校验，
			// 错误文案里带请求类型名 TestWebhookInlineReq——只有该 handler 会产出它。
			rec = doRequest(r, http.MethodPost, "/web/settings/webhooks/test", token, "{}")
			resp = decodeResp(t, rec)
			if !strings.Contains(resp.Message, "TestWebhookInlineReq") {
				t.Errorf("POST /web/settings/webhooks/test: 期望命中 WebhookHandler.TestInline"+
					"（错误文案应包含 TestWebhookInlineReq），实际 status=%d code=%d message=%q",
					rec.Code, resp.Code, resp.Message)
			}
		})

		t.Run("升级计划静态段不被 :id 吞掉", func(t *testing.T) {
			// PUT /web/upgrades/plans/cancel 命中 BatchCancelPlans：
			// 空对象撞 ids 的 required 校验；若被 /plans/:id/cancel 吞掉则整条路径不匹配 → gin NoRoute 404。
			rec := doRequest(r, http.MethodPut, "/web/upgrades/plans/cancel", token, "{}")
			resp := decodeResp(t, rec)
			if !strings.Contains(resp.Message, "IDs") {
				t.Errorf("PUT /web/upgrades/plans/cancel: 期望 BatchCancelPlans 的 ids 校验错误，"+
					"实际 status=%d code=%d message=%q", rec.Code, resp.Code, resp.Message)
			}
		})

		t.Run("菜单接口删除不被 :id 吞掉", func(t *testing.T) {
			// DELETE /web/system/menus/apis/:apiId 命中 MenuHandler.DeleteAPI：
			// 不存在的 apiId → 404「接口不存在」。落入 gin NoRoute 时 body 不是统一 JSON。
			rec := doRequest(r, http.MethodDelete, "/web/system/menus/apis/99999999", token, "")
			resp := decodeResp(t, rec)
			if resp.Message != "接口不存在" {
				t.Errorf("DELETE /web/system/menus/apis/99999999: 期望 MenuHandler.DeleteAPI 的「接口不存在」，"+
					"实际 status=%d code=%d message=%q", rec.Code, resp.Code, resp.Message)
			}
		})

		t.Run("反证：:param 分支仍然可用", func(t *testing.T) {
			// 非数字 id 必须落到 DeviceHandler.Get 的 ID 校验，
			// 证明上面对静态段的断言不是因为 :id 整体失效而「蒙对」。
			rec := doRequest(r, http.MethodGet, "/web/devices/not-a-number", token, "")
			resp := decodeResp(t, rec)
			if rec.Code != http.StatusBadRequest || resp.Message != "无效的设备ID" {
				t.Errorf("GET /web/devices/not-a-number: 期望 400「无效的设备ID」，"+
					"实际 status=%d code=%d message=%q", rec.Code, resp.Code, resp.Message)
			}
		})
	})
}

// ════════════════════════════════════════════════════════════════
// 用例 4：公开端点无需认证
// ════════════════════════════════════════════════════════════════

// TestPublicRoutes_NoAuthRequired 公开端点不得被认证中间件拦截。
//
// 断言方式是「不带 Authorization 也拿不到 401/403」：
// /health 直接 200；两个登录端点因空请求体返回 400（参数错误），
// 但绝不应返回 401/403。
func TestPublicRoutes_NoAuthRequired(t *testing.T) {
	r := buildRouter(t)

	t.Run("健康检查", func(t *testing.T) {
		rec := doRequest(r, http.MethodGet, "/health", "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /health: 期望 200，实际 %d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
			t.Errorf("GET /health: 响应体缺少 status=ok: %s", rec.Body.String())
		}
	})

	for _, target := range []string{"/web/auth/login", "/app/auth/login"} {
		target := target
		t.Run("登录免认证 "+target, func(t *testing.T) {
			rec := doRequest(r, http.MethodPost, target, "", "")
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				t.Fatalf("POST %s: 登录端点不应要求认证，实际 status=%d body=%s",
					target, rec.Code, rec.Body.String())
			}
			resp := decodeResp(t, rec)
			if strings.Contains(resp.Message, "认证Token") {
				t.Errorf("POST %s: 落到了认证中间件: %q", target, resp.Message)
			}
		})
	}
}
