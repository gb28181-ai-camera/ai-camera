import 'package:flutter/foundation.dart';
import 'package:flutter_dva/flutter_dva.dart';

import '../services/api_service.dart';
import '../services/generated/AuthController.dart';
import '../services/generated/definitions/Definition.dart';
import '../services/generated/definitions/LoginReq.dart';
import '../services/generated/definitions/Response.dart';
import '../utils/AppNavigator.dart';
import '../utils/SharedPreferenceUtil.dart';

/// 登录态变化通知器。
///
/// `GoRouter` 的 `refreshListenable` 挂在这里：AuthModel 每次产出新状态都会
/// bump 一次计数，路由随即重新执行 `redirect`，实现「登录后进首页、登出后回登录页」。
final ValueNotifier<int> authChangeNotifier = ValueNotifier<int>(0);

/// 登录态。
///
/// 必须继承 [DvaState]：flutter_dva 内部会把 `StoreOfState<AuthState>`
/// 向下转型为 `StoreOfState<DvaState>`，不继承会在运行期抛类型错误。
class AuthState extends DvaState {
  /// 构造登录态。
  AuthState({
    this.token,
    this.user,
    this.userId,
    this.name,
    this.dataPerm,
    this.loading = false,
    this.error,
  });

  /// 初始（未登录）状态。
  AuthState.initial()
      : token = null,
        user = null,
        userId = null,
        name = null,
        dataPerm = null,
        loading = false,
        error = null;

  /// JWT。
  final String? token;

  /// 登录返回的载荷（含 permissions、dept_id 等）。
  final LoginData? user;

  /// 用户 ID。
  final int? userId;

  /// 姓名（登录返回的 `name`）。
  final String? name;

  /// 数据权限标识 all/cross_dept/dept/personal。
  final String? dataPerm;

  /// 是否正在登录。
  final bool loading;

  /// 错误信息（登录失败时展示在表单内联位置）。
  final String? error;

  /// 是否已登录。
  bool get isLoggedIn => token != null && token!.isNotEmpty;

  /// 展示用姓名。
  String get displayName => (name == null || name!.isEmpty) ? '用户' : name!;

  /// 是否拥有某权限标识。
  bool hasPermission(String code) =>
      user?.permissions.contains(code) ?? false;

  /// 拷贝并覆盖部分字段。
  ///
  /// [clearError] / [clearToken] 用于显式置空（copyWith 无法用 null 表达清空）。
  AuthState copyWith({
    String? token,
    LoginData? user,
    int? userId,
    String? name,
    String? dataPerm,
    bool? loading,
    String? error,
    bool clearError = false,
    bool clearToken = false,
  }) {
    return AuthState(
      token: clearToken ? null : (token ?? this.token),
      user: clearToken ? null : (user ?? this.user),
      userId: clearToken ? null : (userId ?? this.userId),
      name: clearToken ? null : (name ?? this.name),
      dataPerm: clearToken ? null : (dataPerm ?? this.dataPerm),
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
    );
  }

  @override
  Map<String, dynamic> toJson() => <String, dynamic>{
        'token': token,
        'user_id': userId,
        'name': name,
        'data_perm': dataPerm,
      };

  @override
  void fromJson(Map<String, dynamic> json) {
    // AuthState 为不可变对象，这里仅用于持久化插件的兼容，不做原地修改。
  }
}

/// 登录态模型（DVA：reducers + effects）。
///
/// 命名空间 `auth`，dispatch 形如 `dispatch('auth/login', payload)`。
class AuthModel implements Model<AuthState> {
  /// 命名空间。
  @override
  String namespace = 'auth';

  /// 初始状态。
  @override
  StoreOfState<AuthState> state = StoreOfState<AuthState>(
    state: AuthState.initial(),
  );

  /// 同步 reducer。
  ///
  /// **注意**：所有 reducer 都必须「就地写入 state 并返回同一个 `StoreOfState` 对象」，
  /// 不能返回新建的 `StoreOfState`。原因是 flutter_dva 允许 effect 内嵌套 dispatch，
  /// 而外层 `RootReducer.runReducer` 在 effect 返回后会把**进入 effect 前捕获的
  /// `StoreOfState` 快照**写回状态表；若 reducer 返回新对象，嵌套 dispatch 产生的新状态
  /// 会被这个陈旧快照覆盖，表现为「effect 里的状态更新全部丢失」。
  /// 就地 `setState` 时快照即当前对象，写回等价于空操作，嵌套更新得以保留。
  @override
  Map<String, ReducerFuction> reducers = <String, ReducerFuction>{
    /// 开始登录：置 loading，清空上一次的错误。
    'loginStart': (StoreOfState<DvaState> state, DvaAction action) {
      state.setState(_of(state).copyWith(loading: true, clearError: true));
      return state;
    },

    /// 登录成功：写入 token 与用户信息。
    'loginSuccess': (StoreOfState<DvaState> state, DvaAction action) {
      final AuthState cur = _of(state);
      final LoginData? data = action.payload is LoginData
          ? action.payload as LoginData
          : null;
      if (data == null) {
        state.setState(cur.copyWith(loading: false, error: '登录响应异常'));
        return state;
      }
      state.setState(
        cur.copyWith(
          token: data.token,
          user: data,
          userId: data.userId,
          name: data.name,
          dataPerm: data.dataPerm,
          loading: false,
          clearError: true,
        ),
      );
      return state;
    },

    /// 登录失败。
    'loginError': (StoreOfState<DvaState> state, DvaAction action) {
      final String message =
          action.payload is String && (action.payload as String).isNotEmpty
              ? action.payload as String
              : '登录失败，请稍后重试';
      state.setState(_of(state).copyWith(loading: false, error: message));
      return state;
    },

    /// 冷启动恢复登录态。
    'restoreSuccess': (StoreOfState<DvaState> state, DvaAction action) {
      final Map<String, dynamic> p = action.payload is Map
          ? Map<String, dynamic>.from(action.payload as Map)
          : <String, dynamic>{};
      state.setState(
        _of(state).copyWith(
          token: p['token'] as String?,
          userId: p['user_id'] as int?,
          name: p['name'] as String?,
          dataPerm: p['data_perm'] as String?,
          loading: false,
          clearError: true,
        ),
      );
      return state;
    },

    /// 退出登录：清空全部登录态。
    ///
    /// 命名为 `logoutSuccess` 而非 `logout`：flutter_dva 的 `DvaReducer.runReducer`
    /// 先匹配 reducers 再匹配 effects，同名 reducer 会**屏蔽同名 effect**，
    /// 导致 `auth/logout` 只清内存状态、不清理本地 token。故 effect 占用 `logout`
    /// 这个名字（对外唯一入口），reducer 改名为 `logoutSuccess`。
    'logoutSuccess': (StoreOfState<DvaState> state, DvaAction action) {
      state.setState(AuthState.initial());
      return state;
    },
  };

  /// 异步 effect。
  @override
  Map<String, EffectFunction> effects = <String, EffectFunction>{
    /// 登录：调用 `/app/auth/login`，成功后持久化 token 并加载菜单。
    'login': (DvaAction action, ModelEffects effects) async {
      await effects.dispatch(DvaAction('loginStart', null));

      final Map<String, dynamic> payload = action.payload is Map
          ? Map<String, dynamic>.from(action.payload as Map)
          : <String, dynamic>{};
      final String userName = (payload['user_name'] ?? '').toString();
      final String password = (payload['password'] ?? '').toString();

      if (userName.isEmpty || password.isEmpty) {
        await effects.dispatch(
          DvaAction('loginError', '请输入用户名和密码'),
        );
        return;
      }

      try {
        final RLogin resp = await AuthController.loginUsingPost(
          LoginReq(userName: userName, password: password),
        );

        if (!resp.isSuccess || resp.loginData == null) {
          await effects.dispatch(
            DvaAction('loginError', resp.errorMessage),
          );
          return;
        }

        final LoginData data = resp.loginData!;

        // 先落盘再更新状态，保证冷启动能读到 token。
        ApiService.setToken(data.token);
        if (SharedPreferenceUtil.isReady) {
          await SharedPreferenceUtil.setToken(data.token);
          await SharedPreferenceUtil.saveUser(
            userId: data.userId,
            userName: userName,
            name: data.name,
            dataPerm: data.dataPerm,
          );
        }

        await effects.dispatch(DvaAction('loginSuccess', data));
        // 登录后拉取移动端菜单（失败不影响登录结果）。
        await effects.dispatch(DvaAction('menu/load', null));
      } catch (e) {
        await effects.dispatch(DvaAction('loginError', '登录异常：$e'));
      }
    },

    /// 退出登录：清本地登录态并回到登录页。
    ///
    /// 完整流程 = 清内存 token → 清 SharedPreferences → 重置状态 → 跳登录页。
    'logout': (DvaAction action, ModelEffects effects) async {
      ApiService.setToken(null);
      await ApiService.clearToken();
      await effects.dispatch(DvaAction('logoutSuccess', null));
      await AppNavigator.toLoginPage();
    },

    /// 冷启动恢复：从 SharedPreferences 读回 token。
    ///
    /// **必须在 `runApp` 之前 await 调用**，否则首帧会因未登录被重定向到 `/login`。
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
  };

  /// 从 [StoreOfState] 中取出 [AuthState]，类型不符时回退初始态。
  static AuthState _of(StoreOfState<DvaState> state) {
    final DvaState? raw = state.getState();
    return raw is AuthState ? raw : AuthState.initial();
  }
}
