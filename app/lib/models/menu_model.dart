import 'package:flutter_dva/flutter_dva.dart';

import '../services/generated/AppController.dart';
import '../services/generated/definitions/Definition.dart';
import '../services/generated/definitions/Response.dart';

/// 菜单与权限状态。
///
/// 跨页共享：侧边栏/首页宫格取菜单树，按钮级权限判断走 [hasPerm]。
class MenuState extends DvaState {
  /// 构造菜单状态。
  MenuState({
    this.menus = const <MenuNode>[],
    this.permissions = const <String>[],
    this.loading = false,
    this.error,
  });

  /// 初始状态。
  MenuState.initial()
      : menus = const <MenuNode>[],
        permissions = const <String>[],
        loading = false,
        error = null;

  /// 菜单树（仅 platform=mobile）。
  final List<MenuNode> menus;

  /// 扁平化后的权限标识集合。
  final List<String> permissions;

  /// 是否正在加载。
  final bool loading;

  /// 错误信息。
  final String? error;

  /// 是否已加载过。
  bool get isLoaded => menus.isNotEmpty;

  /// 是否拥有某权限标识（如 `risk:handle`）。
  ///
  /// 未加载到任何菜单时返回 false；超级管理员由后端直接下发全部菜单，
  /// 因此无需在此特判。
  bool hasPerm(String code) {
    if (code.isEmpty) return false;
    return permissions.contains(code);
  }

  /// 同时拥有多个权限标识。
  bool hasAnyPerm(List<String> codes) => codes.any(hasPerm);

  /// 拷贝并覆盖字段。
  MenuState copyWith({
    List<MenuNode>? menus,
    List<String>? permissions,
    bool? loading,
    String? error,
    bool clearError = false,
  }) {
    return MenuState(
      menus: menus ?? this.menus,
      permissions: permissions ?? this.permissions,
      loading: loading ?? this.loading,
      error: clearError ? null : (error ?? this.error),
    );
  }

  @override
  Map<String, dynamic> toJson() => <String, dynamic>{
        'permissions': permissions,
      };

  @override
  void fromJson(Map<String, dynamic> json) {}
}

/// 菜单模型（DVA 命名空间 `menu`）。
class MenuModel implements Model<MenuState> {
  /// 命名空间。
  @override
  String namespace = 'menu';

  /// 初始状态。
  @override
  StoreOfState<MenuState> state = StoreOfState<MenuState>(
    state: MenuState.initial(),
  );

  /// 同步 reducer。
  ///
  /// 与 [AuthModel] 一致：必须就地 `setState` 并返回同一个 `StoreOfState`，
  /// 否则 effect 内嵌套 dispatch 的状态会被外层陈旧快照覆盖（详见 auth_model.dart 注释）。
  @override
  Map<String, ReducerFuction> reducers = <String, ReducerFuction>{
    /// 开始加载。
    'loadStart': (StoreOfState<DvaState> state, DvaAction action) {
      state.setState(_of(state).copyWith(loading: true, clearError: true));
      return state;
    },

    /// 加载成功：写入菜单树并扁平化权限标识。
    'loadSuccess': (StoreOfState<DvaState> state, DvaAction action) {
      final List<MenuNode> menus = action.payload is List
          ? (action.payload as List)
              .whereType<MenuNode>()
              .toList(growable: false)
          : const <MenuNode>[];
      final List<String> perms = <String>[];
      for (final MenuNode m in menus) {
        perms.addAll(m.permissionCodes);
      }
      state.setState(
        _of(state).copyWith(
          menus: menus,
          permissions: perms,
          loading: false,
          clearError: true,
        ),
      );
      return state;
    },

    /// 加载失败。
    'loadError': (StoreOfState<DvaState> state, DvaAction action) {
      final String message =
          action.payload is String && (action.payload as String).isNotEmpty
              ? action.payload as String
              : '菜单加载失败';
      state.setState(_of(state).copyWith(loading: false, error: message));
      return state;
    },

    /// 清空（退出登录时调用）。
    'reset': (StoreOfState<DvaState> state, DvaAction action) {
      state.setState(MenuState.initial());
      return state;
    },
  };

  /// 异步 effect。
  @override
  Map<String, EffectFunction> effects = <String, EffectFunction>{
    /// 拉取移动端菜单树 `GET /app/menus`。
    'load': (DvaAction action, ModelEffects effects) async {
      await effects.dispatch(DvaAction('loadStart', null));
      try {
        final RAppMenuList resp = await AppController.menusUsingGet();
        if (!resp.isSuccess) {
          await effects.dispatch(DvaAction('loadError', resp.errorMessage));
          return;
        }
        await effects.dispatch(DvaAction('loadSuccess', resp.menus));
      } catch (e) {
        await effects.dispatch(DvaAction('loadError', '菜单加载异常：$e'));
      }
    },
  };

  /// 从 [StoreOfState] 中取出 [MenuState]，类型不符时回退初始态。
  static MenuState _of(StoreOfState<DvaState> state) {
    final DvaState? raw = state.getState();
    return raw is MenuState ? raw : MenuState.initial();
  }
}
