# AI Camera 移动端（Flutter）

AI Camera 智能监控平台的移动端 App，供**巡检人、区域负责人、管理者**在手机上完成
登录、首页看板、待办、AI 风险处置、设备告警处置、巡检任务处理、设备查看、通知与个人中心。

后端接口统一挂在 `/app` 前缀下，响应格式 `{code, message, data}`，
鉴权使用 `Authorization: Bearer <token>`。

> 当前版本: **v0.3.0** | 最后更新: 2026-10-24

---

## 技术栈

| 领域 | 选型 | 说明 |
|---|---|---|
| 框架 | Flutter 3.44+ / Dart `sdk: ^3.9.0` | 仅使用稳定约束 |
| 路由 | `go_router` | `refreshListenable` + `redirect` 驱动登录态重定向 |
| 网络 | `dio` | 超时参数真实传入 `BaseOptions` |
| 状态 | `flutter_dva` | DVA 风格 Model / reducers / effects / dispatch |
| 本地存储 | `shared_preferences` | 持久化 token 与用户信息 |
| 时间格式化 | `intl` | 统一时间展示 |
| 轻提示 | `ScaffoldMessenger`（Flutter 内置） | 统一走 `SnackBar`（`Extensions.showToast` / `AppNavigator.showToast`） |
| 测试 | `flutter_test` + `mockito` | 自定义 Dio Adapter，无需后端 |
| 图表 | 不引入图表库 | 首页用纯 Widget（`LinearProgressIndicator`）画进度条 |
| MQTT | `mqtt_client` | WebRTC 信令通道（SDP offer/answer） |
| BLE | `flutter_blue_plus` **1.x** | 蓝牙配网（Nordic UART Service）。2.x 起需商业许可，故固定 1.36.8 |
| 扫码 | `mobile_scanner` | 分享码 / 设备二维码 |
| 权限 | `permission_handler` **12.x** | 蓝牙、定位（Android ≤11）、相机。13.x 要求 compileSdk 37，当前工程为 36 |
| E2E | 未引入 `patrol` | 后续版本补充 |

> **注意**：`fluttertoast` 已从依赖中移除；全仓提示统一使用 `ScaffoldMessenger` + `SnackBar`
> （见 `lib/utils/Extensions.dart` 的 `showToast` 扩展与 `lib/utils/AppNavigator.dart`
> 的 `AppNavigator.showToast`）。

`ApiService.getHeaders()` 的返回类型为 `Map<String, dynamic>`（非 `Map<String, String>`），
以便同时容纳 `Authorization` 字符串头与调用方传入的自定义头。

**状态管理的使用范围**：只有跨页共享的状态才用 DVA —— 当前为 `auth`（登录态）
与 `menu`（菜单树与按钮权限）。其余页面一律 `StatefulWidget` + `setState`。

---

## 目录结构

```
app/
├── pubspec.yaml
├── analysis_options.yaml
├── README.md
├── lib/
│   ├── main.dart                      # 入口：预热本地存储 → 建 store → 恢复登录态 → runApp
│   ├── config/api_config.dart         # baseUrl（支持 --dart-define 覆盖）与超时
│   ├── constants/
│   │   ├── DColor.dart                # 颜色常量门面（转发 AppColors）
│   │   └── DefaultTheme.dart          # 组装 ThemeData
│   ├── theme/
│   │   ├── app_colors.dart            # 颜色令牌
│   │   ├── app_text.dart              # 文字样式令牌
│   │   └── app_spacing.dart           # 间距 / 圆角 / 组件尺寸令牌
│   ├── layout/Gaps.dart               # Gaps.vGap8 等间距组件与 .dp 扩展
│   ├── models/
│   │   ├── index.dart                 # 导出 + AppStore 全局状态门面
│   │   ├── auth_model.dart            # AuthState / AuthModel（namespace=auth）
│   │   └── menu_model.dart            # MenuState / MenuModel（namespace=menu）
│   ├── router/app_router.dart         # 路由表 + 底部 Tab 外壳 + 登录态 redirect
│   ├── services/
│   │   ├── api_service.dart           # HTTP 门面：注入 Bearer、统一错误处理
│   │   ├── IotAppController.dart      # /app/iot/* 与 /app/rtc/* 接口 + 扫码内容模型
│   │   ├── MqttSignalingClient.dart   # MQTT 信令（offer/answer 匹配）+ MqttWebRtcClient
│   │   ├── BleProvisioner.dart        # 蓝牙配网（NUS 帧编解码、扫描、会话）
│   │   └── generated/
│   │       ├── AuthController.dart    # POST /app/auth/login
│   │       ├── AppController.dart     # 其余 /app/* 端点
│   │       ├── GbController.dart      # /app/gb/* 国标视频端点（13 个）
│   │       └── definitions/
│   │           ├── Definition.dart    # 数据模型（与后端 JSON tag 一一对应）
│   │           ├── AppGb.dart         # 国标设备/通道/播放/回放/报警模型
│   │           ├── Response.dart      # {code,message,data} 包装与强类型响应
│   │           └── LoginReq.dart      # 各请求体
│   ├── utils/
│   │   ├── AppNavigator.dart          # 无 context 场景的导航（401 跳登录页）
│   │   ├── SharedPreferenceUtil.dart  # 本地存储读写
│   │   ├── Extensions.dart            # 时间解析/格式化、字符串、Context 扩展
│   │   └── menu_icon.dart             # 后端图标名 → IconData 映射
│   ├── widgets/
│   │   ├── loading_indicator.dart     # 加载中 / 按钮内嵌 spinner / 列表底部状态
│   │   ├── empty_state.dart           # 空态与错误态
│   │   ├── stat_card.dart             # 统计卡 / 进度统计卡 / 状态标签
│   │   └── entity_list_page.dart      # 通用分页列表 + 卡片 + 信息行 + 远程图片
│   └── pages/                         # 按领域分目录（gb/ 国标视频 4 页、iot/ 设备接入 4 页）
└── test/                              # 单元/组件测试（含 iot/ 扫码、BLE 帧、MQTT 信令、接口）
```

---

## 启动

```bash
cd app
flutter pub get
flutter run
```

### 后端地址

默认按平台选择：

| 平台 | 默认 baseUrl |
|---|---|
| Android 模拟器 | `http://10.0.2.2:8080` |
| iOS 模拟器 / 桌面 | `http://localhost:8080` |

**真机调试必须指定宿主机的局域网 IP**：

```bash
flutter run --dart-define=API_BASE_URL=http://192.168.1.100:8080
```

### 构建 APK

```bash
cd app
flutter build apk --debug     # 产物 build/app/outputs/flutter-apk/app-debug.apk
flutter build apk --release   # 正式包
```

> 环境前提：Android SDK 需装 **cmdline-tools** 并接受许可（`flutter doctor --android-licenses`）。

### 构建排错（本机已踩过的坑）

以下四项缺一不可，均已在仓库中配置好，换机器时若构建失败优先按此排查：

| 现象 | 原因 | 处置 |
|:---|:---|:---|
| `flutter run` 报 “not configured for …” / 无法安装到设备 | **缺少平台目录**（`android/`、`ios/` 等） | 已补 `android/` `ios/`。若被误删，在 `app/` 下执行 `flutter create --org com.boticz --project-name ai_camera_app --platforms android,ios .`（不会覆盖已有 `lib/`） |
| Gradle 卡在下载发行包、报 `Timeout ... waiting for exclusive access to gradle-*-all.zip` | `services.gradle.org` 在国内拉不动 | `android/gradle/wrapper/gradle-wrapper.properties` 已改用**腾讯云镜像**（与 boticz-mes 一致） |
| `Could not close incremental caches … kotlin` | Windows 上 Kotlin 增量编译的文件占用/长路径问题 | `android/gradle.properties` 已设 **`kotlin.incremental=false`**（与 boticz-mes 一致） |
| `checkDebugAarMetadata` 报某插件 `compiled against android-33` 但依赖要求 34+ | 插件自身 compileSdk 过旧 | 已**移除 `fluttertoast`**（该依赖全仓零调用，提示统一走 `ScaffoldMessenger`）。新增插件若踩到同类问题，优先考虑移除或换维护中的替代品，而非强行抬高 compileSdk |
| `checkDebugAarMetadata` 报 `permission_handler_android` 要求 compileSdk **37**，而工程为 36 | 13.x 起插件抬高了 compileSdk | 已固定 `permission_handler: ^12.0.3`；不要为此把 compileSdk 提到 37（AGP 9 最高推荐 36） |

**明文流量**：开发/测试环境后端是 `http`，`android/app/src/main/AndroidManifest.xml` 已设
`android:usesCleartextTraffic="true"`（Android 9+ 默认禁止）。**正式环境切 https 后应移除该属性。**

**允许的构建后验证**（不依赖 Android SDK，CI 上也能跑）：

```bash
flutter analyze   # 要求 0 error / 0 warning / 0 info
flutter test
```

---

## 测试

```bash
cd app
flutter test                                  # 全部用例
flutter test test/auth_model_test.dart        # 单个文件
flutter analyze                               # 静态检查（要求 0 error 0 warning）
```

测试完全不依赖后端：`test/test_dio_adapter.dart` 提供三个 `HttpClientAdapter` 实现
（`PathRoutingAdapter` / `FixedAdapter` / `ErrorAdapter`），按路径返回预设 JSON
并记录请求，用于断言请求头、请求体与错误处理分支。

---

## 与后端的接口约定

### 响应格式

```json
{ "code": 200, "message": "ok", "data": { } }
```

- `code == 200` 视为成功；
- 分页载荷为 `{total, page, page_size, items}`，前端统一用 `RPageResponse<T>.items` 读取；
- 简单列表响应 `data` 直接是数组，用 `RListResponse<T>.items` 读取；
- 单条详情用 `RSingleResponse<T>.value` 读取。

### 端点清单（`lib/services/generated/AppController.dart`）

| 方法 | 路径 | Controller 方法 | 响应类型 |
|---|---|---|---|
| POST | `/app/auth/login` | `AuthController.loginUsingPost` | `RLogin` |
| GET | `/app/menus` | `menusUsingGet` | `RAppMenuList` |
| GET | `/app/home` | `homeUsingGet` | `RAppHome` |
| GET | `/app/todos` | `todosUsingGet` | `RAppTodoList` |
| GET | `/app/risks` | `risksUsingGet` | `RPageRiskRecord` |
| GET | `/app/risks/:id` | `riskDetailUsingGet` | `RRiskRecord` |
| PUT | `/app/risks/:id/handle` | `handleRiskUsingPut` | `REmptyResponse` |
| PUT | `/app/risks/:id/accurate` | `updateRiskAccurateUsingPut` | `REmptyResponse` |
| GET | `/app/alarms` | `alarmsUsingGet` | `RPageAlarmRecord` |
| PUT | `/app/alarms/:id/handle` | `handleAlarmUsingPut` | `REmptyResponse` |
| GET | `/app/inspect-tasks` | `inspectTasksUsingGet` | `RPageInspectTask` |
| GET | `/app/inspect-tasks/:id` | `inspectTaskDetailUsingGet` | `RInspectTask` |
| PUT | `/app/inspect-tasks/:id/finish` | `finishInspectTaskUsingPut` | `REmptyResponse` |
| GET | `/app/devices` | `devicesUsingGet` | `RPageDevice` |
| GET | `/app/devices/:id` | `deviceDetailUsingGet` | `RDevice` |
| GET | `/app/notifications` | `notificationsUsingGet` | `RPageNotification` |
| PUT | `/app/notifications/:id/read` | `markNotificationReadUsingPut` | `REmptyResponse` |
| GET | `/app/profile` | `profileUsingGet` | `RUser` |
| PUT | `/app/profile` | `updateProfileUsingPut` | `REmptyResponse` |
| PUT | `/app/profile/password` | `changePasswordUsingPut` | `REmptyResponse` |

**国标视频（`GbController.dart`）**

| 方法 | 路径 | Controller 方法 | 响应类型 |
|---|---|---|---|
| GET | `/app/gb/devices` | `devicesUsingGet` | `RAppGbDevicePage` |
| GET | `/app/gb/channels` | `channelsUsingGet` | `RAppGbChannelPage` |
| POST | `/app/gb/play/:channelId` | `playUsingPost` | `RAppGbPlay` |
| POST | `/app/gb/play/:channelId/stop` | `stopPlayUsingPost` | `REmptyResponse` |
| POST | `/app/gb/play/:channelId/snapshot` | `snapshotUsingPost` | `RAppGbSnapshot` |
| POST | `/app/gb/ptz/:channelId` | `ptzUsingPost` | `REmptyResponse` |
| GET | `/app/gb/playback/records` | `recordsUsingGet` | `RAppGbRecordQuery` |
| POST | `/app/gb/playback` | `startPlaybackUsingPost` | `RAppGbPlayback` |
| POST | `/app/gb/playback/control` | `controlPlaybackUsingPost` | `REmptyResponse` |
| POST | `/app/gb/playback/stop` | `stopPlaybackUsingPost` | `REmptyResponse` |
| POST | `/app/gb/talk/:channelId/start` | `startTalkUsingPost` | `RAppGbVoiceSession` |
| POST | `/app/gb/talk/:channelId/stop` | `stopTalkUsingPost` | `REmptyResponse` |
| GET | `/app/gb/alarms` | `alarmsUsingGet` | `RAppGbAlarmPage` |

> 移动端返回的是**裁剪后**的结构（`AppGb.dart`），不含 `sip_password`、国标内部字段等。
> 若需增加字段，改后端 `internal/dto/response/app_gb.go` 与本文件对应模型，两处保持一致。

**IOT 设备接入（`IotAppController.dart`，手写，无 Dart 生成器）**

| 方法 | 路径 | Controller 方法 | 响应类型 |
|---|---|---|---|
| POST | `/app/iot/devices/claim` | `claim` | `RIotDevice` |
| POST | `/app/iot/devices/bind` | `bind` | `RIotDevice` |
| POST | `/app/iot/devices/bind-qr` | `bindByQr` | `RIotDevice` |
| GET | `/app/iot/devices` | `devices` | `RIotDeviceList`（兼容数组 / 分页） |
| GET | `/app/iot/devices/:id` | `deviceDetail` | `RIotDevice` |
| GET | `/app/iot/devices/:id/services` | `services` | `RIotServices`（`{product, groups, mqtt, turn}`） |
| GET | `/app/rtc/ice-servers` | `iceServers` | `RIceServers`（`data` 为数组） |

- 字段名与后端 JSON tag 一一对应（`device_code`、`product_name`、`device_secret` 等）。
- `devices` 同时接受 `data: []` 与 `data: {total, items}` 两种形态，
  避免后端「分页/列表」二选一实现时列表页空白；认领/绑定/详情响应里的
  `{device, account, ...}` 包装由 `IotDevice.fromJson` 统一拆解。
- 响应结构是 `{product, groups:[{...,services:[...]}], mqtt, turn}`；凭据在
  **每个服务**上（产品×服务级），MQTT 使用该服务自己的 `username`/`password`
  （App 已通过 JWT 鉴权，明文是登录所必需）。
- MQTT 连接优先使用后端下发的 `mqtt.url` 与 `mqtt.topic_prefix`
  （ACL 隔离边界，客户端不自行拼接）；缺失时才回退到服务里的 MQTT 地址。
- `ice-servers` 未配置 TURN 时返回空数组 → App 按直连处理，不视为失败。

### 调用规范

- **必须走生成的 Controller**：禁止在页面中手写 URL 或直接 `import` Dio 发请求；
- **禁止绕过类型**：不得使用 `as any` 式的动态访问，一律通过响应类型取值
  （如 `resp.page.items`、`resp.value`、`resp.items`）；
- 后端结构变更时，同步修改 `definitions/Definition.dart` 与对应响应类型。

### 鉴权与 401

`ApiService.getHeaders()` 为每个请求注入 `Authorization: Bearer <token>`，
token 取自内存缓存（优先）→ `SharedPreferences`（兜底）。

收到 **HTTP 401 或业务 `code == 401`** 时统一：清空内存与本地 token → 提示 →
跳转登录页（`AppNavigator.toLoginPage()`）。并发请求只处理一次。
所有请求方法**不抛异常**，一律返回 `dio.Response`；网络异常降级为合成响应（`code = -1`）。

---

## 设计稿等比缩放（重要）

UI 稿按 **414 逻辑像素宽**出图，真机宽度差异很大（360 ~ 480+）。若各处直接写像素值，
小屏会拥挤、大屏会松散 —— 即所谓「设计偏差」。本项目用一层缩放把设计稿尺寸换算成设备尺寸：

```
Screen.sizeByDesign(设计值)          ← lib/constants/Screen.dart
        ↑
AppSpacing / AppText / Dimens         ← 设计令牌（已缩放），页面只应使用这些
```

**页面写法约定**：

```dart
// ✅ 间距与圆角走令牌
Padding(padding: EdgeInsets.all(AppSpacing.lg), child: ...)
SizedBox(height: AppSpacing.md)
decoration: BoxDecoration(borderRadius: AppRadius.card)

// ✅ 非标准尺寸用 .dp 语法糖（返回缩放后的 double）
SizedBox(height: 18.dp)
EdgeInsets.all(12.dp)

// ❌ 禁止裸数字（会在不同屏宽下走样）
SizedBox(height: 16)
```

| 令牌 | 用途 |
|:---|:---|
| `AppSpacing.xs/sm/md/lg/xl/xxl/xxxl` | 间距，设计稿 4/8/12/16/20/24/32 |
| `AppSpacing.pagePadding` | 页面左右统一留白（设计稿 16） |
| `AppSpacing.buttonHeight` / `inputHeight` / `thumbnailSize` / `navIconSize` | 组件尺寸 |
| `AppRadius.card/control/tag/round` | 圆角 |
| `AppText.title/section/body/bodyStrong/caption/hint/statNumber/tag/button/empty` | 文字样式（**字号同样缩放**） |
| `Dimens.gap_dpN` / `Gaps.vGapN` / `hGapN` | 更细粒度的间距（语义令牌不够用时） |

**三条硬规则**：

1. **只缩不放**：屏幕短边超过设计稿宽度时直接采用设计稿原值，**不放大**。
   （若等比放大会让 800 宽的桌面窗口把界面撑到 1.93 倍，观感「虚胖」。）
   大屏应通过 `Screen.mode` 切换布局，而不是放大尺寸。
2. **`1` 不参与缩放**：分割线/描边粗细用 `AppSpacing.dividerThickness` / `borderThickness`，
   写 `1.dp` 会让细线在缩小后消失。
3. **令牌不是编译期常量**：因为依赖运行时屏幕尺寸，`const SizedBox(height: AppSpacing.md)`
   **无法编译**，需去掉 `const`。这是使用缩放令牌的固有代价。

**换算规则**（`lib/constants/Screen.dart`）：

```
短边 = min(屏宽, 屏高)
radio = max(414 / 短边, 1)        ← 钳制为 1，即只缩不放
实际值 = 设计值 / radio
```

屏幕尺寸变化（旋转 / 桌面端缩放窗口）会通过 `WidgetsBindingObserver` 自动失效缓存。
`main.dart` 中已 `WidgetsFlutterBinding.ensureInitialized()`，令牌在首帧前不可访问。

---

## 路由表

| 路径 | 页面 | 说明 |
|---|---|---|
| `/login` | 登录 | 独立于底部导航 |
| `/` | 首页看板 | 底部 Tab |
| `/todos` | 待办 | 底部 Tab |
| `/risks` | 风险列表 | 底部 Tab，支持 `?status=` |
| `/devices` | 设备列表 | 底部 Tab，支持 `?status=` |
| `/profile` | 我的 | 底部 Tab |
| `/risks/detail?id=` | 风险详情 | 独立路由 |
| `/risks/handle?id=` | 风险处理 | 独立路由 |
| `/alarms` | 告警列表 | 独立路由，支持 `?status=` |
| `/alarms/handle?id=` | 告警处理 | 独立路由 |
| `/inspect-tasks` | 巡检任务列表 | 独立路由，支持 `?status=` |
| `/inspect-tasks/detail?id=` | 巡检任务详情 | 独立路由 |
| `/devices/detail?id=` | 设备详情 | 独立路由 |
| `/notifications` | 通知列表 | 独立路由 |
| `/gb/devices` | 国标设备/通道 | 按设备懒加载展开通道 |
| `/gb/live?channelId=&name=` | 国标实时播放 | WebRTC(WHEP) 优先 + HTTP-FLV 回退；含云台与对讲入口 |
| `/gb/playback?channelId=&name=` | 国标录像回放 | 未带 channelId 时先选通道 |
| `/gb/alarms` | AI / 国标报警 | 只读列表 + 详情弹层 |
| `/iot/devices` | 我的设备 | 已绑定设备列表，点击选择通道进入播放 |
| `/iot/add` | 添加设备 | 「蓝牙配网」「扫一扫添加」两个入口 |
| `/iot/ble?token=` | 蓝牙配网 | 扫描 → 连接 → Wi-Fi → 分步配网 → 成功 |
| `/iot/scan` | 扫一扫 | 分享码 `device_share` / 设备码 `device` 分流 |

国标实时页新增两个可选查询参数（不影响原有调用）：

- `deviceId=<ac_device.id>`：MQTT 信令需要它查询设备服务组；
- `mode=mqtt`：优先使用 MQTT 信令（仅当同时带 `deviceId` 时生效）。

详情页参数通过 `?id=` 查询串传递，页面内用 `int.tryParse` 解析，解析失败展示错误态。

### 国标播放的两次「失败要能看见」

实现上刻意做了两件事，因为它们对应的是**静默失败**：

1. **WebRTC 的成功判据不是「answer 返回 200」，而是 10 秒内 `onTrack` 收到视频轨。**
   H.265 在部分 Android 机型上会协商成功、ICE 也连上，但永远没有视频帧 ——
   只看 answer 会得到一个「播放中」的黑屏。
2. **无论从哪条路径失败，都要显示具体原因并回退 FLV**；两条都失败时显示两段原因与重试按钮，
   不允许黑屏无提示。

---

## IOT 设备接入（v0.3.0）

### 添加设备的两种方式

| 方式 | 适用条件 | 关键接口 |
|---|---|---|
| 蓝牙配网 | 设备首次使用、尚未联网 | BLE 协议（见下）+ `POST /app/iot/devices/bind` |
| 扫一扫 | 设备已联网 / 机身二维码 | `POST /app/iot/devices/claim` 或 `POST /app/iot/devices/bind-qr` |

扫码解析兼容**链接格式**（新格式优先，与 `neme-ai-app` 一致）与 **JSON 格式**（本项目早期约定）：

```text
# 链接：http:// / https:// 前缀可省略
boticz.cn/qrDevice/{product_code}/{device_code}   → POST /app/iot/devices/bind-qr
boticz.cn/deviceShareToken/{token}                → POST /app/iot/devices/claim
```

```jsonc
// JSON：分享码（主账号生成，一次性）
{"type":"device_share","token":"..."}
// JSON：设备二维码（机身/包装上的固定资产标识）
{"type":"device","product_code":"P-OD-01","device_code":"CAM-0005"}
```

顺序：`trim` → 先按 URL 解析（仅接受 `boticz.cn` 及其子域）→ 失败再按 JSON 解析。
其余内容一律提示「不是 AI Camera 设备二维码」。

### 蓝牙配网协议（与固件约定）

- Service：`6e400001-b5a3-f393-e0a9-e50e24dcca9e`（Nordic UART Service）
- RX（App 写入）：`6e400002-b5a3-f393-e0a9-e50e24dcca9e`
- TX（设备通知）：`6e400003-b5a3-f393-e0a9-e50e24dcca9e`
- 帧：UTF-8 JSON + `\n`，**一问一答**

| 步骤 | App → 设备 | 设备 → App |
|---|---|---|
| 握手 | `{"cmd":"hello","product_code":"..."}` | `{"status":"ok","device_code":"..."}` |
| Wi-Fi | `{"cmd":"wifi","ssid":"...","password":"..."}` | `{"status":"ok"}` / `{"status":"error","message":"..."}` |
| 绑定 | `{"cmd":"bind","token":"<share_token 或空>"}` | `{"status":"ok","device_code":"...","device_secret":"..."}` |

设备返回 `device_code` + `device_secret` 后，App 再调
`POST /app/iot/devices/bind` 建立「当前用户 ↔ 设备」关系。两步都成功才算配网完成。

**为什么 `device_secret` 由设备返回**：服务端据此确认 App 与设备在同一物理现场
（BLE 的短距离特性就是信任来源），避免仅凭设备编号就能把别人的摄像头绑到自己账号下。

**权限**：

- Android 12+：`BLUETOOTH_SCAN` / `BLUETOOTH_CONNECT`（运行时申请）；
- Android ≤11：BLE 扫描被系统强制要求 `ACCESS_FINE_LOCATION`；
- 权限被永久拒绝时给出「去系统设置开启」的可操作提示（`openAppSettings()`）。

### MQTT 作为 WebRTC 信令

```
订阅  iot/<group_code>/app/<client_id>/signal
发布  iot/<group_code>/signal/backend
```

```jsonc
{"type":"offer","request_id":"<uuid>","client_id":"...","channel_id":123,"stream":"0BEBC201","sdp":"v=0..."}
{"type":"answer","request_id":"<uuid>","sdp":"v=0..."}
{"type":"error","request_id":"<uuid>","code":"...","message":"..."}
```

流程：

1. 调 `POST /app/gb/play/:channelId` 拿 `stream`；
2. 用设备服务组里的 MQTT 服务（host/port）与产品×服务凭据连接 Broker；
3. `GET /app/rtc/ice-servers` 取 ICE 配置（为空则直连）；
4. `createOffer` → `setLocalDescription` → 等 ICE gathering；
5. 发布 offer，等待**同一 `request_id`** 的 answer（10 秒超时）；
6. `setRemoteDescription(answer)`，等 `onTrack` 收到视频轨才判定成功。

**为什么 MQTT 失败要回退 HTTP WHEP**：MQTT 只承载信令，媒体面仍是 WebRTC。
失败通常与媒体无关（broker 不可达、产品服务凭据失效、后端未部署 MQTT 转发），
而设备/媒体节点往往仍提供 WHEP 地址。回退可以避免一条信令链路故障导致完全看不到画面。
播放页的尝试顺序为 **MQTT → WHEP → HTTP-FLV**，每层失败都记录具体原因并展示。

**为什么分享令牌一次性**：二维码可能被拍照/转发，长期有效等于把设备永久分享给
看到图片的任何人；一次性令牌由后端认领后立即作废，泄露窗口只有一次。

### IOT 设备接入的已知边界（需真机验证）

1. **BLE 协议尚未与真实固件联调**：UUID、帧格式按约定实现，
   若固件使用不同 MTU 分包/自定义特征，需要同步调整 `BleProvisioner.dart`。
2. **Wi-Fi 名称需手动输入**：未读取系统当前 SSID（Android 需定位权限且限制较多），
   避免「看起来自动填了实际是错的」。
3. **MQTT 媒体面依赖后端实现**：App 只实现信令与 WebRTC 协商；
   `iot/<group>/signal/backend` 的转发、answer 生成需后端配合。
4. **ICE 配置**：MQTT 与 WHEP 两条 WebRTC 路径都会请求 `/app/rtc/ice-servers`；
   未配置 TURN 时后端返回空数组，App 按直连处理（跨公网需运维配置 TURN）。
5. **播放页 MQTT 模式需要 `deviceId`**：从「我的设备」进入时会自动带上；
   从国标监控入口进入仍走 WHEP/FLV 路径（WHEP 同样使用下发的 TURN 凭据）。
6. **iOS 权限**：`Info.plist` 已包含相机、麦克风、蓝牙配网
   （`NSBluetoothAlwaysUsageDescription`）与局域网（`NSLocalNetworkUsageDescription`）用途说明。
7. **分享码展示形态**：分享页展示 `qr_content` 链接 + 复制按钮，未引入二维码图形库
   （沿用“不新增依赖”的取舍）；家人可通过复制的链接在 App 内扫码/识别。

## 实现说明与已知取舍

1. **本版本不做相机上传**：风险处理、告警处理、完成巡检的「图片」字段以**文本 URL 输入**
   代替拍照上传（后端字段本身即为字符串路径）。

2. **flutter_dva 的两处必要适配**（均已在实际运行中验证，非绕路方案）：

   - **reducer 必须就地写入状态**：flutter_dva 允许 effect 内嵌套 `dispatch`，
     但外层 `RootReducer.runReducer` 在 effect 返回后会把「进入 effect 前捕获的
     `StoreOfState` 快照」写回状态表。若 reducer 返回**新建**的 `StoreOfState`，
     嵌套 dispatch 产生的状态会被这个陈旧快照覆盖（表现为 effect 里的更新全部丢失）。
     因此 `AuthModel` / `MenuModel` 的 reducer 统一 `state.setState(...)` 后
     `return state`（返回同一个对象），写回即等价于空操作。

   - **`logout` 只作为 effect 名，reducer 改名为 `logoutSuccess`**：
     `DvaReducer.runReducer` 先匹配 reducers 再匹配 effects，**同名 reducer 会屏蔽同名 effect**。
     若 reducer 占用 `logout`，`auth/logout` 只会重置内存状态而不会清理本地 token。
     故对外唯一入口（effect）保留 `logout`，状态清理 reducer 命名为 `logoutSuccess`。

3. **路由刷新同时挂 `onAction` 与 `onStateChange`**：reducer 就地写入时，
   若两次 dispatch 落在同一毫秒，flutter_dva 的时间戳比较会判定「未变化」而不触发
   `onStateChange`，导致路由错过重定向时机；`onAction` 对每个 action 必触发，作为兜底。

4. **flutter_dva 默认不注册 `onError`**，reducer/effect 中的异常会被**静默吞掉**。
   `main.dart` 与测试引导函数均显式传入 `onError` 打印日志，便于排查。

5. **菜单图标为静态映射表**：后端 `menu.icon` 是字符串，移动端不引入动态图标字体，
   未登记的图标名回退为 `Icons.circle_outlined`（见 `utils/menu_icon.dart`）。

6. **列表状态在切换底部 Tab 后重置**：`ShellRoute` 不保留各 Tab 的页面状态，
   切换即重新拉取。如需保留，可后续改为 `StatefulShellRoute.indexedStack`。

### 国标视频的已知边界（v0.2.0）

1. **对讲只有会话控制，没有音频采集回推**：`startTalk/stopTalk` 会建立会话，
   但 App 侧的麦克风采集与 RTP 回推尚未接入（需要原生音频推流 + `RECORD_AUDIO` 权限），
   界面上也标注了「需真机验证」。真正能听见声音要等二期语音模块。
2. **回放进度为本地估算**：后端没有「当前播放位置」查询接口，进度按墙钟 × 倍速推算；
   seek 的精度取决于设备的 MANSRTSP 支持。
3. **报警页只读**：不提供抓图、处理、删除 —— 这些操作在 Web 后台完成。
4. **回放页的播放器代码是实时页的最小副本**：实时页的播放内核与云台/对讲/抓图强耦合，
   在未做统一抽取前先复制一份并保持同步；后续重构时应抽成公共组件并删除副本。
5. **iOS 权限已声明**：`NSCameraUsageDescription` / `NSMicrophoneUsageDescription` /
   `NSBluetoothAlwaysUsageDescription` / `NSLocalNetworkUsageDescription` 均在
   `ios/Runner/Info.plist` 中；真机首次调用仍会弹出系统授权。

---

## 版本发布

版本更新时需同步补充：
[`docs/敏捷开发/CHANGE_LOG.md`](../docs/敏捷开发/CHANGE_LOG.md) 与
[`app/README.md`](README.md)。
