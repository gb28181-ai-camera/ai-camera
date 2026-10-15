# AI Camera 移动端开发文档

> 版本: v0.1.0
> 最后更新: 2026-09-21
> 范围: `app/`（Flutter）—— 现场巡检员与安全员处理风险、告警与巡检任务的移动入口

## 1. 技术栈

| 项目 | 选择 | 说明 |
|:-----|:-----|:-----|
| 框架 | Flutter（SDK `^3.9.0`） | 一套代码覆盖 iOS / Android |
| 语言 | Dart | `flutter_lints ^5.0.0` |
| 路由 | `go_router ^14.6.0` | 声明式路由 + `redirect` 实现登录拦截 |
| 状态管理 | `flutter_dva ^1.0.9` | Dva 风格 Model（State / Reducers / Effects），`DvaState` 为状态基类 |
| HTTP | `dio ^5.7.0` | 统一门面 `ApiService`，含 JWT 注入与 401 处理 |
| 本地存储 | `shared_preferences ^2.3.0` | token 与用户信息持久化（`SharedPreferenceUtil`） |
| 格式化 | `intl ^0.19.0` | 日期与数字格式化 |
| 轻提示 | `ScaffoldMessenger`（Flutter 内置） | 统一走 `SnackBar`；**未使用** `fluttertoast`（依赖虽在 `pubspec.yaml` 中声明，但 `lib/` 下无任何调用） |
| 测试 | `flutter_test` + `mockito ^5.4.4` | 单元测试与 Mock |

> **注意**：移动端**不引入动态图标字体**。后端菜单的 `icon` 是字符串（如 `dashboard`、`camera`），
> 由 `lib/utils/menu_icon.dart` 的 42 项静态映射转成 `IconData`，未知图标回退 `Icons.circle_outlined`。

## 2. 目录结构

```
app/
├── pubspec.yaml                      # 依赖清单（name: ai_camera_app）
├── analysis_options.yaml             # flutter_lints + 关闭 file_names 规则
├── README.md                         # 快速开始与页面清单
├── lib/
│   ├── main.dart                     # 入口：Dva.start → dispatch restoreToken → runApp
│   ├── config/api_config.dart        # baseUrl 解析（--dart-define 优先）+ 三项超时 + /app 前缀
│   ├── constants/
│   │   ├── DColor.dart               # 颜色常量门面（别名转发 AppColors）
│   │   └── DefaultTheme.dart         # ThemeData 装配（AppBar/Card/Button/Input/...）
│   ├── theme/
│   │   ├── app_colors.dart           # 配色令牌（与官网同一支主色 #2B7FFF）
│   │   ├── app_spacing.dart          # 间距 / 圆角 / 组件尺寸令牌
│   │   └── app_text.dart             # 10 个文字样式令牌（title/section/body/hint/...）
│   ├── layout/Gaps.dart              # 间隙组件 + num 扩展（12.dp / 12.dpW / 12.box）
│   ├── models/
│   │   ├── index.dart                # 模型导出 + AppStore 全局门面 + authChangeNotifier
│   │   ├── auth_model.dart           # 登录态 Model（namespace 'auth'）
│   │   └── menu_model.dart           # 菜单/权限 Model（namespace 'menu'）
│   ├── pages/                        # 按业务模块分目录（login/home/todos/risk/alarm/inspect/device/notification/profile）
│   ├── router/app_router.dart        # GoRouter：ShellRoute 5 Tab + 独立页 + redirect + MainShell
│   ├── services/
│   │   ├── api_service.dart          # 全局 HTTP 门面（Dio + token 缓存 + getHeaders + 401）
│   │   └── generated/                # ⚠️ 页面中禁止手写 URL
│   │       ├── AuthController.dart   # 登录（/app/auth/login）
│   │       ├── AppController.dart    # /app/* 全部端点（19 个路径常量 + 静态方法）
│   │       └── definitions/
│   │           ├── Definition.dart   # defInt/defStr 解析工具 + PageData + 16 个业务模型
│   │           ├── LoginReq.dart     # 6 个请求体（登录/资料/密码/风险处理/准确性/巡检）
│   │           └── Response.dart     # Response 基类 + RSingle/RList/RPage + 端点响应类型
│   ├── utils/
│   │   ├── AppNavigator.dart         # 无 context 导航门面（401 跳登录、根级 SnackBar）
│   │   ├── SharedPreferenceUtil.dart # SharedPreferences 门面（须在 runApp 前 init）
│   │   ├── Extensions.dart           # 时间解析、字符串、数值、BuildContext 扩展
│   │   └── menu_icon.dart            # 菜单 icon 字符串 → IconData（42 项映射）
│   └── widgets/
│       ├── entity_list_page.dart     # 通用分页列表骨架 + EntityCard/RemoteImage/InfoRow
│       ├── stat_card.dart            # StatCard / ProgressStatCard / StatusTag
│       ├── empty_state.dart          # EmptyState + EmptyState.error 工厂
│       └── loading_indicator.dart    # LoadingIndicator / InlineSpinner / ListFooter
└── test/
    ├── test_dio_adapter.dart         # 测试适配器与 DVA 引导工具（非测试文件本身）
    ├── auth_model_test.dart          # AuthModel：登录/失败/空参/恢复/登出
    ├── menu_model_test.dart          # MenuModel：加载/权限扁平化
    ├── login_page_test.dart          # LoginPage widget 测试
    └── services/api_service_test.dart# ApiService：超时/baseUrl/请求头/错误处理
```

> **注意**：`lib/services/generated/` 是**按 Swagger 生成的 Controller 风格手工对齐契约**的代码
> （Dart 侧无代码生成器），字段名与后端 JSON tag 严格一一对应；后端契约变更后必须同步核对本目录。

## 3. 状态管理（flutter_dva）

### 3.1 Model / Reducers / Effects 模式

```
  View（StatefulWidget）
    AppStore.login(u, p) → AppStore.dispatch('auth/login')
        ▼
  Effect 'auth/login'（异步）
    dispatch('loginStart')            → reducer 置 loading
    AuthController.loginUsingPost(..) → 调后端
    写 ApiService.setToken + SharedPreferences
    dispatch('loginSuccess', data)    → reducer 写状态
    dispatch('menu/load')             → 跨 namespace 派发
        ▼
  Reducer（同步，必须就地 setState 并返回同一个 StoreOfState）
        ▼
  authChangeNotifier.value++  →  GoRouter 重跑 redirect
```

**硬约束一：Reducer 必须就地 `setState` 并返回同一个 `StoreOfState`**

> 不能返回新建的 `StoreOfState`。原因是 flutter_dva 允许 effect 内嵌套 dispatch，而外层
> `RootReducer.runReducer` 在 effect 返回后会把**进入 effect 前捕获的 `StoreOfState` 快照**写回状态表；
> 若 reducer 返回新对象，嵌套 dispatch 产生的新状态会被这个陈旧快照覆盖，表现为
> 「effect 里的状态更新全部丢失」。就地 `setState` 时快照即当前对象，写回等价于空操作，嵌套更新得以保留。

```dart
// app/lib/models/menu_model.dart（片段）
'loadSuccess': (StoreOfState<DvaState> state, DvaAction action) {
  final List<MenuNode> menus = action.payload is List
      ? (action.payload as List).whereType<MenuNode>().toList(growable: false)
      : const <MenuNode>[];
  final List<String> perms = <String>[];
  for (final MenuNode m in menus) {
    perms.addAll(m.permissionCodes);
  }
  state.setState(_of(state).copyWith(menus: menus, permissions: perms, loading: false, clearError: true));
  return state;   // ← 必须返回同一个对象
},
```

**硬约束二：Reducer 与 Effect 不能同名**

> flutter_dva 的 `DvaReducer.runReducer` 先匹配 reducers 再匹配 effects，同名 reducer 会**屏蔽同名 effect**，
> 导致 `auth/logout` 只清内存状态、不清理本地 token。故 effect 占用 `logout` 这个名字（对外唯一入口），
> reducer 改名为 `logoutSuccess`。

**硬约束三：State 必须继承 `DvaState`**——flutter_dva 内部会把 `StoreOfState<AuthState>` 向下转型为
`StoreOfState<DvaState>`，不继承会在运行期抛类型错误。每个 Model 用私有静态 `_of` 做安全取值：

```dart
/// 从 [StoreOfState] 中取出 [AuthState]，类型不符时回退初始态。
static AuthState _of(StoreOfState<DvaState> state) {
  final DvaState? raw = state.getState();
  return raw is AuthState ? raw : AuthState.initial();
}
```

### 3.2 读取状态的约定（`AppStore` 门面）

> **与常见写法不同**：本项目**不使用** `Connect<T>` 包装与 `didMounted` 生命周期钩子，也不在页面里
> 直接调 `Provider().store`。读取统一走 `lib/models/index.dart` 的 `AppStore` 门面，变更通知统一走
> `authChangeNotifier`。

```dart
// app/lib/models/index.dart（片段）
/// 全局状态访问门面。
class AppStore {
  AppStore._();

  /// 底层 Store；`Dva.start()` 之前为 null。
  static Store? get store => Provider().store;

  /// 是否已初始化。
  static bool get isReady => store != null;

  /// 当前登录态；未初始化时返回初始（未登录）状态。
  static AuthState get auth {
    final Store? s = store;
    if (s == null) return AuthState.initial();
    final AuthState? v = s.rootState['auth']?.getState() as AuthState?;
    return v ?? AuthState.initial();
  }

  /// 当前菜单 / 权限状态；未初始化时返回初始状态。
  static MenuState get menu {
    final Store? s = store;
    if (s == null) return MenuState.initial();
    final MenuState? v = s.rootState['menu']?.getState() as MenuState?;
    return v ?? MenuState.initial();
  }

  static bool get isLoggedIn => auth.isLoggedIn;
  static bool hasPerm(String code) => menu.hasPerm(code);

  /// 派发 action，[type] 需带命名空间（如 `auth/login`）。
  static Future<dynamic> dispatch(String type, [dynamic payload]) async {
    final Store? s = store;
    if (s == null) return null;
    return s.dispatch(DvaAction(type, payload));
  }

  static Future<void> login(String userName, String password) => dispatch(
        'auth/login',
        <String, dynamic>{'user_name': userName, 'password': password},
      );

  static Future<void> logout() => dispatch('auth/logout');
  static Future<void> reloadMenu() => dispatch('menu/load');
}
```

| 方式 | 场景 | 示例 |
|:-----|:-----|:-----|
| `AppStore.auth` / `AppStore.menu` | `build` 或事件回调中按需读一次 | `AppStore.auth.displayName`、`AppStore.hasPerm('risk:handle')` |
| `AppStore.dispatch(type, payload)` | 派发任意 action（带命名空间） | `AppStore.dispatch('auth/restoreToken')` |
| 语义化方法 `login` / `logout` / `reloadMenu` | 常用动作的语法糖，优先用 | `await AppStore.login(user, pwd)` |

> **注意**：`store.rootState[namespace]?.getState()` 就是 `dvaGetStateByKey` 的等价写法——flutter_dva
> 未导出独立的 `dvaGetStateByKey` 帮助函数，本项目把「按 namespace 取强类型状态」收敛在 `AppStore`
> 的两个 getter 里，页面不再重复写 `rootState` 取值与类型断言。

**变更驱动的刷新方式**：页面在 `initState` 监听 `authChangeNotifier`，回调中 `setState`：

```dart
// app/lib/pages/login/login_page.dart（片段）
@override
void initState() {
  super.initState();
  // 登录态（loading / error）变化时刷新页面。
  authChangeNotifier.addListener(_onAuthChanged);
}

@override
void dispose() {
  authChangeNotifier.removeListener(_onAuthChanged);
  _userCtrl.dispose();
  _pwdCtrl.dispose();
  _pwdFocus.dispose();
  super.dispose();
}

void _onAuthChanged() {
  if (!mounted) return;   // ← 无 didMounted 钩子，卸载保护必须手写
  setState(() {});
}

/// 当前登录态。
AuthState get _auth => AppStore.auth;
```

### 3.3 注册表 `models/index.dart`

Model 的注册点是 `lib/main.dart` 的 `Dva(DvaOpts(models: [...]))`，`models/index.dart` 负责导出：

```dart
// app/lib/models/index.dart（片段）
library;

import 'package:flutter_dva/flutter_dva.dart';

import 'auth_model.dart';
import 'menu_model.dart';

export 'auth_model.dart';
export 'menu_model.dart';
```

```dart
// app/lib/main.dart（片段）—— 当前注册 2 个 Model
final Dva dva = Dva(
  DvaOpts(
    models: <Model<dynamic>>[AuthModel(), MenuModel()],
    onAction: (DvaAction action) => _notifyAuthChanged(),
    onStateChange: (StoreOfState prev, DvaAction action) => _notifyAuthChanged(),
    // 默认未注册 onError，异常会被静默吞掉；此处至少打印出来便于排查。
    onError: (dynamic error, Map<String, dynamic> context) {
      debugPrint('[dva] action=${context['action']} error=$error');
    },
  ),
);
```

**新增 Model 步骤**：① `lib/models/` 新建 `xxx_model.dart`（`XxxState extends DvaState` +
`XxxModel implements Model<XxxState>`）；② `models/index.dart` 补 `export`；③ `main.dart` 的 `models`
数组追加实例；④ 如需驱动重定向，在 effect/reducer 中 bump `authChangeNotifier`。

> **注意**：`onAction` 与 `onStateChange` **必须双挂**。reducer 就地写入状态时，若两次 dispatch 落在
> 同一毫秒，flutter_dva 的时间戳比较会判定「未变化」而不触发 `onStateChange`，导致路由错过重定向时机；
> `onAction` 对每个 action 必触发，可作为兜底。`onError` 也必须注册，否则异常会被静默吞掉。

## 4. 路由与登录态

### 4.1 路由结构

```
GoRouter
├── /login                        # 独立页，无底部导航
├── ShellRoute（MainShell：BottomNavigationBar 5 Tab）
│   ├── / (首页)  /todos (待办)  /risks (风险)  /devices (设备)  /profile (我的)
└── 独立页（无底部导航）
    ├── /risks/detail?id=         /risks/handle?id=       # 风险详情 / 处理
    ├── /alarms                   /alarms/handle?id=      # 告警列表 / 处理
    ├── /inspect-tasks            /inspect-tasks/detail?id=
    ├── /devices/detail?id=       /notifications
```

**详情页参数一律走 `?id=` 查询参数**，用 `int.tryParse` 解析：

```dart
/// 解析 `?id=` 查询参数为 int，非法返回 null。
static int? parseId(GoRouterState state) =>
    int.tryParse(state.uri.queryParameters['id'] ?? '');
```

### 4.2 redirect 规则与完整代码

`refreshListenable: authChangeNotifier` 使登录态变化时 GoRouter 自动重跑 `redirect`：

```dart
// app/lib/router/app_router.dart（核心片段）
class AppRouter {
  AppRouter._();

  /// 底部 Tab 的路由路径（顺序即导航栏顺序）。
  static const List<String> tabPaths = <String>['/', '/todos', '/risks', '/devices', '/profile'];
  static const List<String> tabLabels = <String>['首页', '待办', '风险', '设备', '我的'];

  static GoRouter? _router;
  static GoRouter get router => _router ??= _create();

  /// 供测试重置单例。
  static void reset() => _router = null;

  static GoRouter _create() {
    final GoRouter r = GoRouter(
      initialLocation: '/',
      navigatorKey: AppNavigator.navigatorKey,
      refreshListenable: authChangeNotifier,     // ← 登录态变化即重跑 redirect
      debugLogDiagnostics: false,
      redirect: _redirect,
      routes: <RouteBase>[
        GoRoute(path: '/login', name: 'login',
          builder: (BuildContext context, GoRouterState state) => const LoginPage()),

        ShellRoute(
          builder: (BuildContext context, GoRouterState state, Widget child) =>
              MainShell(state: state, child: child),
          routes: <RouteBase>[
            GoRoute(path: '/', name: 'home',
              builder: (BuildContext context, GoRouterState state) => const HomePage()),
            GoRoute(path: '/todos', name: 'todos',
              builder: (BuildContext context, GoRouterState state) => const TodoListPage()),
            // /risks、/devices、/profile 同构 ...
          ],
        ),

        GoRoute(path: '/risks/detail', name: 'riskDetail',
          builder: (BuildContext context, GoRouterState state) => const RiskDetailPage()),
        // /risks/handle、/alarms、/alarms/handle、/inspect-tasks、
        // /inspect-tasks/detail、/devices/detail、/notifications 同构 ...
      ],
      errorBuilder: (BuildContext context, GoRouterState state) => Scaffold(
        appBar: AppBar(title: const Text('页面不存在')),
        body: Center(child: Text('未找到路由：${state.uri}')),
      ),
    );

    // 供非 Widget 上下文（401 处理）使用。
    AppNavigator.router = r;
    return r;
  }

  /// 登录态重定向。
  static String? _redirect(BuildContext context, GoRouterState state) {
    final bool loggedIn = AppStore.isLoggedIn;
    final bool atLogin = state.matchedLocation == '/login';

    // 未登录且不在登录页 → 强制登录。
    if (!loggedIn && !atLogin) return '/login';
    // 已登录仍访问登录页 → 回首页。
    if (loggedIn && atLogin) return '/';
    return null;
  }
}
```

| 登录态 | 目标路由 | 结果 |
|:------:|:---------|:-----|
| 未登录 | 非 `/login` | 重定向到 `/login` |
| 未登录 | `/login` | 放行（返回 `null`） |
| 已登录 | `/login` | 重定向到 `/` |
| 已登录 | 非 `/login` | 放行 |

**Tab 选中态**：`MainShell._index` 从后往前匹配 `tabPaths`，保证 `/devices/detail` 之类前缀不被误判：

```dart
// app/lib/router/app_router.dart（片段）
int get _index {
  final String location = state.matchedLocation;
  // 从后往前匹配，保证 `/devices/detail` 之类前缀不误判。
  for (int i = AppRouter.tabPaths.length - 1; i >= 0; i--) {
    final String path = AppRouter.tabPaths[i];
    if (path == '/') {
      if (location == '/') return i;
      continue;
    }
    if (location == path) return i;
  }
  return 0;
}
```

### 4.3 无 context 场景的导航

`ApiService`（401）与 `AuthModel`（登出）运行在 Widget 树之外，统一走 `AppNavigator`：

```dart
// app/lib/utils/AppNavigator.dart（片段）
/// 登录态失效处理：清 token → 跳转登录页。
static Future<void> toLoginPage() async {
  if (SharedPreferenceUtil.isReady) {
    await SharedPreferenceUtil.clearAuth();
  }
  final GoRouter? r = router;
  if (r == null) return;
  // 已在登录页则不重复跳转，避免 redirect 循环。
  final String current = r.state.matchedLocation;
  if (current == '/login') return;
  r.go('/login');
}
```

> **注意**：**Widget 内部优先用 `context.go` / `context.push`**，`AppNavigator` 只服务于无 context 场景。

## 5. 网络层

### 5.1 `api_config.dart` 的 baseUrl 选择规则

```dart
// app/lib/config/api_config.dart（片段）
/// 后端服务基地址。
///
/// - 安卓模拟器内 `10.0.2.2` 指向宿主机的 `localhost`；
/// - iOS 模拟器 / 桌面 / Web 直接用 `localhost`；
/// - 真机调试必须改成宿主机的局域网 IP，例如：
///   `flutter run --dart-define=API_BASE_URL=http://192.168.1.100:8080`
static String get baseUrl {
  const String override = String.fromEnvironment('API_BASE_URL');
  if (override.isNotEmpty) {
    return override;
  }
  if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
    return 'http://10.0.2.2:8080';
  }
  return 'http://localhost:8080';
}
```

| 优先级 | 来源 | 取值 |
|:------:|:-----|:-----|
| 1 | `--dart-define=API_BASE_URL=...` | 显式指定（**真机调试与发布包必须使用**） |
| 2 | Android 平台默认 | `http://10.0.2.2:8080`（模拟器内指向宿主机 localhost） |
| 3 | 其他平台默认 | `http://localhost:8080` |

**超时配置**（在 `BaseOptions` 中真正生效）：

```dart
/// 建立连接超时时间。
static const Duration connectTimeout = Duration(seconds: 10);
/// 接收响应超时时间。
static const Duration receiveTimeout = Duration(seconds: 15);
/// 发送请求超时时间。
static const Duration sendTimeout = Duration(seconds: 15);
```

> **注意**：`ApiService` 的单元测试专门断言 `BaseOptions.connectTimeout == const Duration(seconds: 10)`。
> 这是同类项目踩过的坑——声明了超时常量却没传给 `BaseOptions`，超时实际不生效。

### 5.2 `api_service.dart` 的 getHeaders 注入 token

```dart
// app/lib/services/api_service.dart（片段）
/// 构造请求头，注入 Bearer token。
///
/// [headers] 中的同名字段优先（便于个别接口覆盖 Content-Type）。
static Map<String, dynamic> getHeaders([Map<String, dynamic>? headers]) {
  final Map<String, dynamic> result = <String, dynamic>{
    'Accept': 'application/json',
    'Content-Type': 'application/json',
  };
  final String? t = token;
  if (t != null && t.isNotEmpty) {
    result['Authorization'] = 'Bearer $t';
  }
  if (headers != null) {
    result.addAll(headers);
  }
  return result;
}

/// 构造 GET 查询参数（剔除 null / 空串，避免后端把空串当成筛选条件）。
static Map<String, dynamic> cleanQuery(Map<String, dynamic>? query) {
  final Map<String, dynamic> result = <String, dynamic>{};
  query?.forEach((String k, dynamic v) {
    if (v == null) return;
    if (v is String && v.isEmpty) return;
    result[k] = v;
  });
  return result;
}

/// 内存中的 token 缓存。热路径（每次请求）先读它，避免反复访问磁盘。
static String? _tokenCache;

/// 当前 token：内存缓存优先，未命中时回落到 SharedPreferences 并回填缓存。
static String? get token {
  final String? cached = _tokenCache;
  if (cached != null && cached.isNotEmpty) return cached;
  if (!SharedPreferenceUtil.isReady) return null;
  final String? stored = SharedPreferenceUtil.getToken();
  _tokenCache = stored;
  return stored;
}
```

`BaseOptions` 关键项：`validateStatus: (status) => status != null && status < 500` —— **4xx 不抛异常**，
交由 `_handleResponse` 读后端 `{code, message, data}` 判断业务成败；超时三项直接取自 `ApiConfig`。

### 5.3 401 处理

```dart
// app/lib/services/api_service.dart（片段）
/// 统一业务错误处理：401 跳登录，其余非 200 弹提示。
static Future<void> _handleResponse(Response<dynamic> response) async {
  final int httpStatus = response.statusCode ?? 0;
  final dynamic body = response.data;
  final int code = _bodyCode(body);

  // HTTP 401，或业务 code=401，均视为登录态失效。
  if (httpStatus == 401 || code == 401) {
    await _handleUnauthorized();
    return;
  }

  if (httpStatus >= 400) {
    _toast(_bodyMessage(body) ?? '请求失败（HTTP $httpStatus）');
    return;
  }

  // 后端统一响应体中的业务错误码。
  if (code != 0 && code != 200) {
    _toast(_bodyMessage(body) ?? '请求失败（code=$code）');
  }
}

/// 正在执行 401 跳转，避免并发请求触发多次跳转 / 多次提示。
static bool _redirecting = false;

/// 登录态失效：清 token → 跳登录页。并发请求只处理一次。
static Future<void> _handleUnauthorized() async {
  if (_redirecting) return;
  _redirecting = true;
  try {
    await clearToken();
    _toast('登录已过期，请重新登录');
    await AppNavigator.toLoginPage();
  } finally {
    // 仅抑制同一次失效期间的并发请求重复跳转；
    // 不使用延时复位，避免在 widget 测试中残留未完成的 Timer。
    _redirecting = false;
  }
}
```

| 情形 | 处理 |
|:-----|:-----|
| HTTP 401 或 业务 `code == 401` | `clearToken()`（内存 + 磁盘）→ SnackBar → `AppNavigator.toLoginPage()`；`_redirecting` 抑制并发重复跳转 |
| HTTP 400~499（非 401） | 弹 `message`，返回原响应，**不抛异常** |
| 业务 `code != 0 && != 200` | 弹 `message` |
| 网络异常（断网 / 超时 / 证书） | `_handleDioException` 按 `DioExceptionType` 映射中文文案，弹提示并返回**合成响应**（`code = -1`） |

## 6. 生成的API客户端

### 6.1 静态方法风格

`lib/services/generated/` 下的 Controller 为**无实例的静态方法集合**（构造函数私有 `Xxx._()`），
每个方法：① 用 `ApiService.get/post/put/delete` 发请求；② 把 `response.data` 交给强类型响应类的
`fromJson`；③ 返回强类型响应。

```dart
// app/lib/services/generated/AuthController.dart（全文）
/// 移动端认证接口（Swagger Tag: AppAuth）。
///
/// 本文件由「后端接口契约」生成风格手工维护，**禁止在页面中手写 URL 调用**，
/// 所有请求必须走本类的静态方法。
library;

import 'package:dio/dio.dart' as dio;

import '../api_service.dart';
import 'definitions/LoginReq.dart';
import 'definitions/Response.dart';

/// 认证控制器。
class AuthController {
  AuthController._();

  /// 移动端登录路径。
  static String loginUsingPostPath = '/app/auth/login';

  /// 用户登录。
  ///
  /// 与 `POST /web/auth/login` 共用登录逻辑，返回相同的 JWT 载荷
  /// （token / user_id / name / dept_id / data_perm / cross_dept_ids / permissions）。
  static Future<RLogin> loginUsingPost(
    LoginReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      loginUsingPostPath,
      <String, dynamic>{},
      data.toJson(),
      headers,
    );
    return RLogin()..fromJson(response.data);
  }
}
```

**带路径参数**的端点：在路径常量后拼 `/$id`。

```dart
// app/lib/services/generated/AppController.dart（片段）
/// 风险详情（拼接 ID）。
static String riskDetailUsingGetPath = '/app/risks';

static Future<RRiskRecord> riskDetailUsingGet(
  int id, [
  Map<String, dynamic>? headers,
]) async {
  final dio.Response<dynamic> response = await ApiService.get(
    '$riskDetailUsingGetPath/$id',
    headers: headers,
  );
  return RRiskRecord()..fromJson(response.data);
}
```

**带查询参数**的端点：页面筛选新增条件时在此传 `query`（路径注释会列出全部支持的参数名）。

```dart
// app/lib/services/generated/AppController.dart（片段）
/// 分页查询风险监控。
///
/// 支持参数：`page`、`page_size`、`keyword`、`risk_type_id`、`region_id`、
/// `device_id`、`device_code`、`status`、`detect_accurate`、`read`、
/// `start_dt`、`end_dt`。
static Future<RPageRiskRecord> risksUsingGet({
  Map<String, dynamic>? query,
  Map<String, dynamic>? headers,
}) async {
  final dio.Response<dynamic> response = await ApiService.get(
    risksUsingGetPath,
    query: query,
    headers: headers,
  );
  return RPageRiskRecord()..fromJson(response.data);
}
```

### 6.2 响应类型的层次

```
Response（code / message / data + isSuccess / isUnauthorized / errorMessage）
├── RSingleResponse<T>  → value                 （data 为对象）
├── RListResponse<T>    → items                 （data 为数组，对应 RXxxList）
├── RPageResponse<T>    → items / total / page  （data 为 {total,page,page_size,items}）
└── REmptyResponse      → ok                    （写操作，对应 BaseResponse）

端点响应类型（绑定解析函数与语义化 getter）：
RLogin / RAppMenuList / RAppHome / RAppTodoList / RPageRiskRecord / RRiskRecord /
RPageAlarmRecord / RPageInspectTask / RInspectTask / RPageDevice / RDevice /
RPageNotification / RUser
```

### 6.3 完整调用示例

**分页查询**（页面 → Controller → ApiService → 强类型响应）：

```dart
// app/lib/pages/risk/risk_list_page.dart（片段）
import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/Definition.dart';
import '../../services/generated/definitions/Response.dart';
import '../../widgets/entity_list_page.dart';

/// 分页取数。
Future<PageDataLike<RiskRecord>> _fetch(int page, int pageSize) async {
  final RPageRiskRecord resp = await AppController.risksUsingGet(
    query: <String, dynamic>{
      'page': page,
      'page_size': pageSize,
      if (_status.isNotEmpty) 'status': _status,
    },
  );
  return PageDataAdapter<RiskRecord>(
    total: resp.total,
    items: resp.items,
    hasMore: resp.page.hasMore,
  );
}
```

**写操作**（风险处理）：

```dart
// app/lib/pages/risk/risk_handle_page.dart（片段）
final REmptyResponse resp = await AppController.handleRiskUsingPut(
  _id!,
  HandleReq(handleResult: result, handleImage: _imageCtrl.text.trim()),
);
if (!resp.ok) {
  setState(() { _submitting = false; _error = resp.errorMessage; });
  return;
}
context.showToast('处理成功');
// 返回 true 告知列表 / 详情页需要刷新。
context.pop(true);
```

> **注意**：**页面中禁止手写接口 URL**。全部调用必须通过 `lib/services/generated/` 的 Controller 静态方法；
> `ApiService` 是唯一 HTTP 门面，生成的 Controller 只调用它，页面不直接持有 Dio。
> 后端契约变更后（见 [后端开发/00-接口契约.md](../后端开发/00-接口契约.md) §3.14 与 §5.10），
> 必须同步核对 `services/generated/` 全部文件。

## 7. 主题与设计令牌

### 7.1 设计令牌三件套

| 文件 | 内容 | 关键约束 |
|:-----|:-----|:---------|
| `theme/app_colors.dart` | 品牌色 / 背景 / 文字 / 语义色 / 等级色 | **禁止在页面中硬编码色值** |
| `theme/app_text.dart` | 10 个文字样式（`title`/`section`/`body`/`bodyStrong`/`caption`/`hint`/`statNumber`/`tag`/`button`/`empty`） | **禁止直接 `new TextStyle`** |
| `theme/app_spacing.dart` | 间距（4 的倍数）/ 圆角 / 组件尺寸 | **禁止魔法数字** |

```dart
// app/lib/theme/app_colors.dart（片段）
class AppColors {
  AppColors._();

  /// 主色（品牌蓝），用于主按钮、选中态、强调文字。
  static const Color primary = Color(0xFF2B7FFF);
  static const Color primaryLight = Color(0xFFE8F1FF);
  static const Color background = Color(0xFFF5F7FA);
  static const Color card = Color(0xFFFFFFFF);
  static const Color textPrimary = Color(0xFF1D2129);
  static const Color textTertiary = Color(0xFF86909C);
  static const Color success = Color(0xFF00B42A);
  static const Color warning = Color(0xFFFF7D00);
  static const Color danger = Color(0xFFF53F3F);

  /// 风险等级 high / medium / low 对应的配色。
  static const Map<String, Color> levelColors = <String, Color>{
    'high': danger, 'medium': warning, 'low': primary,
  };

  /// 处理状态 pending / handled 对应的配色。
  static Color statusColor(String? status) => status == 'handled' ? success : warning;

  /// 设备运行状态 online / offline 对应的配色。
  static Color deviceStatusColor(String? status) => status == 'online' ? success : offline;
}
```

```dart
// app/lib/theme/app_spacing.dart（片段）
static const double xs = 4.0;
static const double sm = 8.0;
static const double md = 12.0;
static const double lg = 16.0;
static const double xl = 20.0;
static const double xxl = 24.0;
static const double xxxl = 32.0;

/// 页面左右统一留白。
static const double pagePadding = 16.0;

static const double radiusCard = 12.0;
static const double radiusControl = 8.0;
static const double radiusTag = 4.0;

/// 主按钮高度。
static const double buttonHeight = 48.0;
/// 列表缩略图边长。
static const double thumbnailSize = 72.0;
/// 详情页大图高度。
static const double detailImageHeight = 240.0;
```

`DColor` 是历史别名，取值全部转发 `AppColors`，新旧写法取到同一个值：

```dart
// app/lib/constants/DColor.dart（片段）
/// 颜色常量门面（沿用 boticz-mes 的 DColor 命名习惯）。
class DColor {
  DColor._();
  static const Color primary = AppColors.primary;
  static const Color textPrimary = AppColors.textPrimary;
  // ...
}
```

### 7.2 `.dp` 扩展与 `Gaps` 用法

```dart
// app/lib/layout/Gaps.dart（全文要点）
class Gaps {
  Gaps._();

  static const Widget hGap4 = SizedBox(width: 4.0);
  static const Widget hGap8 = SizedBox(width: 8.0);
  static const Widget hGap12 = SizedBox(width: 12.0);
  static const Widget hGap16 = SizedBox(width: 16.0);

  static const Widget vGap4 = SizedBox(height: 4.0);
  static const Widget vGap8 = SizedBox(height: 8.0);
  static const Widget vGap12 = SizedBox(height: 12.0);
  static const Widget vGap16 = SizedBox(height: 16.0);
  static const Widget vGap20 = SizedBox(height: 20.0);
  static const Widget vGap24 = SizedBox(height: 24.0);

  /// 撑满剩余空间。
  static const Widget expand = Expanded(child: SizedBox.shrink());
}

/// 数值转 SizedBox 的语法糖：`12.dp` 等价于 `SizedBox(height: 12)`。
extension GapExtension on num {
  /// 纵向留白。
  SizedBox get dp => SizedBox(height: toDouble());
  /// 横向留白。
  SizedBox get dpW => SizedBox(width: toDouble());
  /// 正方形占位。
  SizedBox get box => SizedBox(width: toDouble(), height: toDouble());
}
```

| 场景 | 用法 |
|:-----|:-----|
| `Column` / `Row` 内固定间距 | `Gaps.vGap8` / `Gaps.hGap8`（const，零构建成本） |
| 一次性数值间距 | `12.dp` / `12.dpW` / `72.box` |
| 取令牌值 | `SizedBox(height: AppSpacing.md)` |
| 撑满剩余 | `Expanded(child: ...)` 或 `Gaps.expand` |

### 7.3 主题装配

`ThemeData` 统一在 `lib/constants/DefaultTheme.dart` 装配（Material3 + 浅色 + `scaffoldBackgroundColor:
AppColors.background`），页面里**不要**重复覆盖 `Theme`：

```dart
// app/lib/main.dart（片段）
return MaterialApp.router(
  title: 'AI Camera',
  debugShowCheckedModeBanner: false,
  theme: DefaultTheme.theme,
  routerConfig: AppRouter.router,
  builder: (BuildContext context, Widget? child) {
    // 锁定文字缩放上限，避免系统超大字体撑破布局。
    final MediaQueryData mq = MediaQuery.of(context);
    return MediaQuery(
      data: mq.copyWith(
        textScaler: mq.textScaler.clamp(minScaleFactor: 0.9, maxScaleFactor: 1.2),
      ),
      child: child ?? const SizedBox.shrink(),
    );
  },
);
```

## 8. 页面开发模式

### 8.1 列表页（`EntityListPage` 骨架）

列表页只需提供 `fetcher` 与 `itemBuilder`，分页 / 下拉刷新 / 空态 / 失败重试 / 上滑加载由骨架统一处理：

```dart
// app/lib/pages/risk/risk_list_page.dart（精简）
class _RiskListPageState extends State<RiskListPage> {
  /// 当前筛选状态，空串表示全部。
  String _status = '';
  /// 刷新令牌：详情/处理页返回后自增，驱动列表重新加载。
  int _tick = 0;

  /// 分页取数。
  Future<PageDataLike<RiskRecord>> _fetch(int page, int pageSize) async {
    final RPageRiskRecord resp = await AppController.risksUsingGet(
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        if (_status.isNotEmpty) 'status': _status,
      },
    );
    return PageDataAdapter<RiskRecord>(
      total: resp.total, items: resp.items, hasMore: resp.page.hasMore,
    );
  }

  /// 切换筛选条件。
  void _onFilterChanged(String value) {
    if (_status == value) return;
    setState(() { _status = value; _tick++; });
  }

  /// 进入详情页，返回后刷新。
  Future<void> _openDetail(RiskRecord risk) async {
    await context.push<bool>('/risks/detail?id=${risk.id}');
    if (!mounted) return;
    setState(() => _tick++);
  }

  @override
  Widget build(BuildContext context) {
    return EntityListPage<RiskRecord>(
      title: 'AI 风险',
      reloadToken: '$_status#$_tick',     // ← 值变化即重新加载第 1 页
      fetcher: _fetch,
      emptyText: _status.isEmpty ? '暂无风险记录' : '暂无该状态的风险',
      emptyIcon: Icons.verified_outlined,
      header: _buildFilterBar(),           // ← 顶部固定筛选条
      separator: const SizedBox(height: AppSpacing.xs),
      itemBuilder: (BuildContext context, RiskRecord risk, int index) => _buildItem(risk),
    );
  }
}
```

单条渲染统一用 `EntityCard` + `RemoteImage` + `StatusTag`：

```dart
// app/lib/pages/risk/risk_list_page.dart（片段）
Widget _buildItem(RiskRecord risk) {
  return EntityCard(
    onTap: () => _openDetail(risk),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        RemoteImage(url: risk.imageUrl,
          width: AppSpacing.thumbnailSize, height: AppSpacing.thumbnailSize),
        const SizedBox(width: AppSpacing.md),
        Expanded(child: Column(/* 标题 + StatusTag + 设备 + 位置 + 时间 */)),
      ],
    ),
  );
}
```

> **注意**：`reloadToken` 是本项目的**筛选/刷新驱动机制**——只要它的值变化（拼接筛选条件 + 自增计数），
> `EntityListPage.didUpdateWidget` 就会重置并重新加载第 1 页。

### 8.2 详情页

详情页无骨架，标准写法：`didChangeDependencies` 解析 `?id=` → `_load()` 取详情 → 三态渲染
（loading / error / 内容），内容用「卡片 + `InfoRow` + `RemoteImage`」拼装：

```dart
// 参照 app/lib/pages/risk/risk_detail_page.dart / device_detail_page.dart（精简骨架）
class _XxxDetailPageState extends State<XxxDetailPage> {
  Xxx? _data;
  bool _loading = true;
  String? _error;
  int? _id;
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;         // ← build 可能多次调用，只解析一次
    _initialized = true;
    _id = int.tryParse(GoRouterState.of(context).uri.queryParameters['id'] ?? '');
    if (_id == null) {
      setState(() { _loading = false; _error = '缺少有效的 ID'; });
      return;
    }
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final RRiskRecord resp = await AppController.riskDetailUsingGet(_id!);
      if (!mounted) return;
      setState(() {
        _data = resp.isSuccess ? resp.value : null;
        _error = resp.isSuccess ? null : resp.errorMessage;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() { _loading = false; _error = '加载失败：$e'; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('详情')),
      body: _loading
          ? const LoadingIndicator(text: '加载中...')
          : _error != null
              ? EmptyState.error(message: _error, onRetry: _load)
              : ListView(
                  padding: const EdgeInsets.all(AppSpacing.pagePadding),
                  children: <Widget>[
                    InfoRow(label: '名称', value: _data?.riskTypeName.orPlaceholder() ?? ''),
                    InfoRow(label: '设备', value: _data?.deviceName.orPlaceholder() ?? ''),
                    InfoRow(label: '发生时间', value: _data?.occurredDT.formatMinute ?? '--'),
                    StatusTag(text: _data?.isHandled == true ? '已处理' : '待处理',
                      color: AppColors.statusColor(_data?.status)),
                  ],
                ),
    );
  }
}
```

### 8.3 表单页（处理页）

处理页 = `Form`（`ListView` + 卡片） + 提交按钮，**错误内联展示**（不弹窗），提交成功后
`context.pop(true)` 告知上一页刷新：

```dart
// app/lib/pages/risk/risk_handle_page.dart（精简）
Future<void> _submit() async {
  if (_submitting) return;

  if (_id == null) {
    setState(() => _error = '缺少有效的风险 ID');
    return;
  }

  final String result = _resultCtrl.text.trim();
  if (result.isEmpty) {
    setState(() => _error = '请填写处理结果');
    return;
  }

  setState(() { _error = null; _submitting = true; });

  try {
    final REmptyResponse resp = await AppController.handleRiskUsingPut(
      _id!,
      HandleReq(handleResult: result, handleImage: _imageCtrl.text.trim()),
    );
    if (!mounted) return;

    if (!resp.ok) {
      setState(() { _submitting = false; _error = resp.errorMessage; });
      return;
    }

    context.showToast('处理成功');
    // 返回 true 告知列表 / 详情页需要刷新。
    context.pop(true);
  } catch (e) {
    if (!mounted) return;
    setState(() { _submitting = false; _error = '提交失败：$e'; });
  }
}
```

三种页面的公共约定：

| 约定 | 说明 |
|:-----|:-----|
| 关键控件与按钮带 `Key` | `key: const Key('handle-result')`、`Key('login-submit')`，供 widget 测试定位 |
| 异步回调首行 `if (!mounted) return;` | 防止页面销毁后 `setState` |
| 三元组状态字段 | `bool _loading` / `String? _error` / 数据字段，三个一起 set |
| 失败文案 | `resp.errorMessage`（后端 message）优先；本地校验错误优先于远程错误 |
| 提交成功后 | `context.pop(true)`（详情/列表进入时）或由 redirect 自动跳转（登录页） |

## 9. 测试规范

### 9.1 `test/` 布局

| 文件 | 类型 | 覆盖 |
|:-----|:-----|:-----|
| `test/test_dio_adapter.dart` | 工具（非测试） | `PathRoutingAdapter` / `FixedAdapter` / `ErrorAdapter` / `buildTestDio` / `bootDva` / `readState` / `dispatchAction` |
| `test/services/api_service_test.dart` | 单测 | baseUrl 与超时进 `BaseOptions`、请求头注入、查询参数净化、401/500/断网/超时降级 |
| `test/auth_model_test.dart` | 单测 | 登录成功/失败/空参、`restoreToken`、`logout` |
| `test/menu_model_test.dart` | 单测 | 菜单加载、权限扁平化 |
| `test/login_page_test.dart` | Widget 测试 | 渲染、空输入校验、错误展示、loading 态 |

### 9.2 `PathRoutingAdapter` 用法

**核心原则：单元测试完全不依赖真实后端**。用适配器替换 `ApiService.dio`：

```dart
// 只放行登录与菜单两个端点，其后所有断言基于适配器记录的请求
final PathRoutingAdapter adapter = PathRoutingAdapter(
  <String, Object>{
    '/app/auth/login': loginOkBody(),
    '/app/menus': menusBody(),
  },
);
ApiService.dio = buildTestDio(adapter);

await dispatchAction(dva, 'auth/login', <String, dynamic>{
  'user_name': 'admin', 'password': 'admin123',
});

// 断言请求体字段（后端 JSON 是 user_name，不是 username）
final RequestOptions loginReq =
    adapter.requests.firstWhere((RequestOptions r) => r.path.endsWith('/app/auth/login'));
final Map<String, dynamic> body = loginReq.data as Map<String, dynamic>;
expect(body['user_name'], 'admin');

// 断言端点调用次数与请求头
expect(adapter.countOf('/app/menus'), 1);
expect(adapter.lastRequest!.headers['Authorization'], 'Bearer tk-restored');
```

| 适配器 | 行为 | 适用场景 |
|:-------|:-----|:---------|
| `PathRoutingAdapter(routes)` | 按路径（全等或后缀）匹配返回预设 JSON，未命中返回 404 业务错误 | 多端点联动流程（登录 → 拉菜单） |
| `FixedAdapter(body)` | 所有请求返回同一份 JSON，可指定 HTTP `statusCode` | 单点成功 / 单点 401 / 单点 500 |
| `ErrorAdapter({type})` | 始终抛 `DioException`，可指定类型 | 断网（`connectionError`）/ 超时（`receiveTimeout`） |

辅助函数：`wrapSuccess(data)` / `wrapError(code, message)` 统一构造后端信封；
`adapter.requests` / `lastRequest` / `countOf(suffix)` 用于断言。

### 9.3 `SharedPreferences.setMockInitialValues` 与重置

`SharedPreferenceUtil` 是单例且缓存了实例，测试必须**先重置再重新 init**，否则会串用上一个用例的实例：

```dart
setUp(() async {
  SharedPreferences.setMockInitialValues(<String, Object>{});
  SharedPreferenceUtil.resetForTest();     // ← @visibleForTesting，丢弃缓存实例
  await SharedPreferenceUtil.init();
  ApiService.setToken(null);
  ApiService.resetDio();
});

tearDown(() {
  ApiService.setToken(null);
  ApiService.resetDio();
});
```

预置本地数据验证冷启动恢复：

```dart
SharedPreferences.setMockInitialValues(<String, Object>{
  'auth_token': 'tk-restored',
  'auth_user_id': 3,
  'auth_display_name': '李四',
  'auth_data_perm': 'all',
});
SharedPreferenceUtil.resetForTest();
await SharedPreferenceUtil.init();
```

DVA 引导统一用 `bootDva`（挂钩方式与 `main.dart` 一致，不调用 `runApp`）：

```dart
// 与生产一致：auth 与 menu 同时注册，登录后会顺带拉取菜单。
dva = await bootDva(<Model<dynamic>>[AuthModel(), MenuModel()]);
// 读取强类型状态
final AuthState state = readState<AuthState>(dva, 'auth');
```

### 9.4 运行命令

```bash
cd app
flutter pub get                                      # 安装依赖
flutter test                                         # 运行全部测试
flutter test test/services/api_service_test.dart     # 运行单个文件
flutter analyze                                      # 静态检查（要求 0 error）
```

## 10. 构建与运行

```bash
cd app

# ── 拉取依赖 ──
flutter pub get

# ── 本地运行（默认按平台自动选择地址：Android 模拟器 10.0.2.2，其余 localhost）──
flutter run

# ── 指定后端地址（真机调试必须用局域网 IP）──
flutter run --dart-define=API_BASE_URL=http://192.168.1.100:8080

# ── 指向测试后端（8081）──
flutter run --dart-define=API_BASE_URL=http://192.168.1.227:8081

# ── 测试 / 静态检查 ──
flutter test
flutter analyze

# ── 构建发布包（必须显式指定生产地址）──
flutter build apk --release --dart-define=API_BASE_URL=https://api.ai-camera.com
flutter build ipa --release --dart-define=API_BASE_URL=https://api.ai-camera.com
```

> **注意**：`--dart-define` 的值通过 `String.fromEnvironment('API_BASE_URL')` 在**编译期**读取，
> 因此修改地址后必须**重新编译**（热重载不生效）。

## 11. 开发注意事项

### 11.1 冷启动 token 恢复必须在 runApp 之前

`main.dart` 的启动顺序**不可调换**：

```
1. WidgetsFlutterBinding.ensureInitialized()
2. await SharedPreferenceUtil.init()              # 预热本地存储
3. Dva.start()                                    # 建立 store（auth / menu）
4. await AppStore.dispatch('auth/restoreToken')   # ← 必须在 runApp 之前
5. runApp(app())
```

```dart
// app/lib/main.dart（片段）
Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // 1. 预热本地存储，保证随后可同步读取 token。
  await SharedPreferenceUtil.init();

  // 2. 建立 DVA store；驱动路由的 refreshListenable。
  final Dva dva = Dva(DvaOpts(/* 见 3.3 */));

  // 3. 建立 store 并拿到根组件构造器。
  final WidgetCreatorFunction app = await dva.start(() => const AiCameraApp(), () async {});

  // 4. 恢复登录态（刷新页面热重载时同样生效）。
  await AppStore.dispatch('auth/restoreToken');

  runApp(app());
}
```

**违反后果**：第 4 步若放到 `runApp` 之后（例如在 `initState` 里），首帧的 `AppStore.isLoggedIn` 为
`false`，GoRouter 的 `redirect` 会把已有 token 的用户**重定向到 `/login`**，随后恢复成功再跳回首页——
表现为「冷启动必然闪一下登录页」。

```dart
// app/lib/models/auth_model.dart（片段）—— restoreToken effect
'restoreToken': (DvaAction action, ModelEffects effects) async {
  if (!SharedPreferenceUtil.isReady) {
    return;
  }
  final String? token = SharedPreferenceUtil.getToken();
  if (token == null || token.isEmpty) {
    await effects.dispatch(DvaAction('logoutSuccess', null));
    return;
  }

  ApiService.setToken(token);
  await effects.dispatch(
    DvaAction('restoreSuccess', <String, dynamic>{
      'token': token,
      'user_id': SharedPreferenceUtil.getUserId(),
      'name': SharedPreferenceUtil.getDisplayName(),
      'data_perm': SharedPreferenceUtil.getDataPerm(),
    }),
  );
  // 恢复登录态后同步刷新菜单权限。
  await effects.dispatch(DvaAction('menu/load', null));
},
```

### 11.2 SharedPreferences 懒加载陷阱

`SharedPreferenceUtil._instance` 在未 `init()` 时**主动抛 `StateError`**，用于暴露调用顺序错误：

```dart
/// 获取实例；未 [init] 时抛出 StateError，便于暴露调用顺序错误。
static SharedPreferences get _instance {
  final SharedPreferences? p = _prefs;
  if (p == null) {
    throw StateError('SharedPreferenceUtil.init() 必须在读取前调用（通常在 main() 中 await）。');
  }
  return p;
}
```

| 规则 | 说明 |
|:-----|:-----|
| 读之前先判 `isReady` | `ApiService.token` 与 `AuthModel.restoreToken` 都先 `if (!SharedPreferenceUtil.isReady) return ...`，避免抛 `StateError` |
| `init()` 用 `??=` 幂等 | 重复调用不会重建实例 |
| 测试里必须 `resetForTest()` | `@visibleForTesting static void resetForTest() => _prefs = null;`，配合 `setMockInitialValues` 使用；**生产代码不应调用** |

### 11.3 `flutter analyze` 要求 0 error

`analysis_options.yaml` 继承 `flutter_lints ^5.0.0`，并显式放宽两项、收紧四项：

```yaml
include: package:flutter_lints/flutter.yaml

analyzer:
  errors:
    # 生成的 Controller 中大量使用 dynamic 解析，避免噪音
    avoid_print: ignore

linter:
  rules:
    # 允许省略 new / const 关键字
    prefer_const_constructors: true
    prefer_const_literals_to_create_immutables: true
    prefer_final_locals: true
    unnecessary_this: true
    # 生成代码中常见，关闭以免误报
    non_constant_identifier_names: false
    # 目录结构沿用 boticz-mes 的命名习惯（DColor.dart / AppNavigator.dart 等），
    # 文件名不改为 lower_case_with_underscores
    file_names: false
```

**提交前必须 `flutter analyze` 零 error**。实际写法约束：

- 局部变量能 `final` 一律 `final`（`prefer_final_locals`）；
- Widget 构造尽量 `const`（`const SizedBox(height: AppSpacing.md)`）；
- 集合字面量写全类型参数（`<String, dynamic>{...}`、`<Widget>[...]`），与全仓风格一致；
- 不给生成的 Controller 加 lint 抑制注释——`avoid_print` 已在 analyzer 层放过。

### 11.4 其他易错点

| 项 | 说明 |
|:---|:-----|
| 后端登录字段是 `user_name` | **不是** `username`；`LoginReq.toJson()` 已固定，改动会导致 400 |
| 图片字段是相对路径 | `/uploads/2026/09/x.jpg`，展示前必须经 `absoluteImageUrl()` 补全为绝对地址（`RemoteImage` 已内置） |
| 时间字段双格式 | 读取可能是 RFC3339，提交必须是 `yyyy-MM-dd HH:mm:ss`；用 `parseServerTime()` / `.formatMinute` / `.toFullString` 统一处理，不要自己 `DateTime.parse` |
| 查询参数要净化 | 传 `query` 时用 `ApiService.cleanQuery` 剔除 `null` / 空串，否则后端会把空串当筛选条件 |
| 权限判定 | `AppStore.hasPerm('risk:handle')` 走菜单树扁平化后的 `permissions`；未加载到菜单时返回 `false`，超级管理员由后端直接下发全部菜单，无需特判 |
| 跨页参数回传 | 处理页 `context.pop(true)`，列表页 `await context.push<bool>(...)` 收到 `true` 后 `setState(() => _tick++)` 触发重载 |
