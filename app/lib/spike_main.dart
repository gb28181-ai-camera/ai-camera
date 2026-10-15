// ⚠️⚠️ 临时 Spike 入口（验证完可整文件删除）⚠️⚠️
//
// 只启动播放器共存 Spike 页面：**不经路由、不登录、不依赖后端**，
// 便于在真机/模拟器上快速验证 `flutter_webrtc` 与 `media_kit` 的同页共存风险。
//
// 运行方式：
//   cd app && flutter run -t lib/spike_main.dart
//
// 页面会在首帧后自动「同时启动」WebRTC（本地摄像头）与 media_kit（网络流）。
// 观察控制台 `[SPIKE]` 日志与画面：是否出现
// `Platform channel messages must be sent on the platform thread` 报错、
// 黑屏或解码失败。
//
// 相关文件（一并删除即可）：
//   - lib/spike_main.dart（本文件）
//   - lib/pages/device/player_spike_page.dart
//   - lib/router/app_router.dart 中的 `/spike/player` 路由与 import
import 'package:flutter/material.dart';
import 'package:media_kit/media_kit.dart';

import 'pages/device/player_spike_page.dart';

/// Spike 独立入口。
void main() {
  // media_kit 的 libmpv 需要初始化（幂等）。返回 Future，这里不阻塞启动。
  MediaKit.ensureInitialized();
  runApp(const SpikeApp());
}

/// 仅承载 Spike 页面的最小 App。
class SpikeApp extends StatelessWidget {
  /// 构造 Spike App。
  const SpikeApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '播放器共存 Spike',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.blue),
      ),
      home: const PlayerSpikePage(autoStart: true),
    );
  }
}
