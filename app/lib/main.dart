import 'package:flutter/material.dart';
import 'package:flutter_dva/flutter_dva.dart';

import 'constants/DefaultTheme.dart';
import 'models/index.dart';
import 'router/app_router.dart';
import 'utils/SharedPreferenceUtil.dart';

/// 应用入口。
///
/// 启动顺序（**不可调换**）：
/// 1. `WidgetsFlutterBinding.ensureInitialized()`；
/// 2. `await SharedPreferenceUtil.init()` 预热本地存储；
/// 3. `Dva.start()` 建立 store（auth / menu 两个模型）；
/// 4. `await dispatch('auth/restoreToken')` 在 `runApp` **之前**恢复登录态，
///    否则首帧会因未登录被 redirect 到 `/login`；
/// 5. `runApp()`。
Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // 1. 预热本地存储，保证随后可同步读取 token。
  await SharedPreferenceUtil.init();

  // 2. 建立 DVA store；驱动路由的 refreshListenable。
  //
  // 同时挂 onAction 与 onStateChange：reducer 就地写入状态时，若两次 dispatch
  // 落在同一毫秒，flutter_dva 的时间戳比较会判定「未变化」而不触发 onStateChange，
  // 导致路由错过重定向时机；onAction 对每个 action 必触发，可作为兜底。
  final Dva dva = Dva(
    DvaOpts(
      models: <Model<dynamic>>[AuthModel(), MenuModel()],
      onAction: (DvaAction action) => _notifyAuthChanged(),
      onStateChange: (StoreOfState prev, DvaAction action) =>
          _notifyAuthChanged(),
      // 默认未注册 onError，异常会被静默吞掉；此处至少打印出来便于排查。
      onError: (dynamic error, Map<String, dynamic> context) {
        debugPrint('[dva] action=${context['action']} error=$error');
      },
    ),
  );

  // 3. 建立 store 并拿到根组件构造器。
  final WidgetCreatorFunction app = await dva.start(
    () => const AiCameraApp(),
    () async {},
  );

  // 4. 恢复登录态（刷新页面热重载时同样生效）。
  await AppStore.dispatch('auth/restoreToken');

  runApp(app());
}

/// 通知 GoRouter 重新执行 redirect。
///
/// 只 bump 计数，不做判断；`_redirect` 内部会读取最新登录态自行决定跳转。
void _notifyAuthChanged() {
  authChangeNotifier.value++;
}

/// 根组件。
class AiCameraApp extends StatelessWidget {
  /// 构造根组件。
  const AiCameraApp({super.key});

  @override
  Widget build(BuildContext context) {
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
            textScaler: mq.textScaler.clamp(
              minScaleFactor: 0.9,
              maxScaleFactor: 1.2,
            ),
          ),
          child: child ?? const SizedBox.shrink(),
        );
      },
    );
  }
}
