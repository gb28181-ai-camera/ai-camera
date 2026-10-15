# AI Camera 移动端（Flutter）

AI Camera 智能监控平台的移动端 App，供**巡检人、区域负责人、管理者**在手机上完成
登录、首页看板、待办、AI 风险处置、设备告警处置、巡检任务处理、设备查看、通知与个人中心。

后端接口统一挂在 `/app` 前缀下，响应格式 `{code, message, data}`，
鉴权使用 `Authorization: Bearer <token>`。

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
| E2E | 未引入 `patrol` | 后续版本补充 |

> **注意**：`pubspec.yaml` 中虽声明了 `fluttertoast ^8.2.8`，但 `lib/` 下**无任何调用**，
> 全仓提示统一使用 `ScaffoldMessenger` + `SnackBar`（见 `lib/utils/Extensions.dart` 的
> `showToast` 扩展与 `lib/utils/AppNavigator.dart` 的 `AppNavigator.showToast`）。

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
│   │   └── generated/
│   │       ├── AuthController.dart    # POST /app/auth/login
│   │       ├── AppController.dart     # 其余全部 /app/* 端点
│   │       └── definitions/
│   │           ├── Definition.dart    # 数据模型（与后端 JSON tag 一一对应）
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
│   └── pages/                         # 14 个页面，按领域分目录
└── test/                              # 27 个用例
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

详情页参数通过 `?id=` 查询串传递，页面内用 `int.tryParse` 解析，解析失败展示错误态。

---

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

---

## 版本发布

版本更新时需同步补充：
[`docs/敏捷开发/CHANGE_LOG.md`](../docs/敏捷开发/CHANGE_LOG.md) 与
[`app/README.md`](README.md)。
