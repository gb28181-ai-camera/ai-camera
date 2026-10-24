import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../layout/Gaps.dart';
import '../../services/IotAppController.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';

/// 添加设备入口页。
///
/// 目前平台只有两类产品，添加流程完全不同，因此按**产品类型**平铺两个入口，
/// 而不是做成「先扫描再猜」：
///
/// | 产品 | 配网 | 绑定 | 分享 |
/// |:--|:--|:--|:--|
/// | 安防相机（`security`） | 出厂即联网，**无需配网** | 扫描机身二维码绑定 | 不支持 |
/// | 适老化智能相机（`elderly`） | 需要蓝牙写入 Wi-Fi | 配网成功后绑定 | owner 可生成分享码 |
///
/// 为什么安防相机不走配网：它出厂时已经预置网络（4G / 已由安装人员配置），
/// 机身二维码是唯一的绑定入口；让用户做一次蓝牙配网既无必要，也会因为搜不到
/// 设备（设备根本没进配网模式）而失败。适老化相机面向家庭场景，需要家属现场
/// 用手机把家里的 Wi-Fi 凭据传给设备，所以必须先配网再绑定。
class AddDevicePage extends StatelessWidget {
  /// 构造添加设备页。
  const AddDevicePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('添加设备')),
      backgroundColor: AppColors.background,
      body: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: <Widget>[
          Text('请选择要添加的产品', style: AppText.section),
          SizedBox(height: AppSpacing.xs),
          Text(
            '两类产品的添加方式不同：安防相机已联网，扫码即可绑定；'
            '适老化相机需要先用蓝牙帮设备连接 Wi-Fi，再完成绑定。',
            style: AppText.hint,
          ),
          SizedBox(height: AppSpacing.lg),
          _EntryCard(
            key: const Key('iot-add-security'),
            icon: Icons.qr_code_scanner,
            title: '安防相机',
            subtitle: '无需配网，扫描机身二维码即可绑定',
            highlights: const <String>['无需配网', '扫描机身码', '不支持分享'],
            // 产品类型 → 入口路由集中在 IotDeviceType.addRoute 中维护，
            // 页面不重复写路由字符串，便于后续新增产品时一处调整。
            onTap: () => context.push(IotDeviceType.security.addRoute),
          ),
          SizedBox(height: AppSpacing.md),
          _EntryCard(
            key: const Key('iot-add-elderly'),
            icon: Icons.bluetooth_searching,
            title: '适老化智能相机',
            subtitle: '请先为设备连接 Wi-Fi（蓝牙配网）',
            highlights: const <String>['蓝牙配网', '配网后绑定', '可分享给家人'],
            onTap: () => context.push(IotDeviceType.elderly.addRoute),
          ),
          SizedBox(height: AppSpacing.xl),
          Material(
            key: const Key('iot-add-my-devices'),
            color: AppColors.card,
            borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
            child: InkWell(
              borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
              onTap: () => context.push('/iot/devices'),
              child: Padding(
                padding: EdgeInsets.all(AppSpacing.lg),
                child: Row(
                  children: <Widget>[
                    Icon(
                      Icons.videocam_outlined,
                      size: 20.dp,
                      color: AppColors.primary,
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Text('查看我的设备', style: AppText.bodyStrong),
                    ),
                    Icon(
                      Icons.chevron_right,
                      size: 20.dp,
                      color: AppColors.textTertiary,
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// 添加入口卡片。
class _EntryCard extends StatelessWidget {
  const _EntryCard({
    super.key,
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.highlights,
    required this.onTap,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final List<String> highlights;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.card,
      borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      child: InkWell(
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        onTap: onTap,
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Row(
                children: <Widget>[
                  Container(
                    width: 44.dp,
                    height: 44.dp,
                    decoration: BoxDecoration(
                      color: AppColors.primaryLight,
                      borderRadius:
                          BorderRadius.circular(AppSpacing.radiusControl),
                    ),
                    child: Icon(icon, size: 24.dp, color: AppColors.primary),
                  ),
                  SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        Text(title, style: AppText.bodyStrong),
                        SizedBox(height: 2.dp),
                        Text(subtitle, style: AppText.hint),
                      ],
                    ),
                  ),
                  Icon(
                    Icons.chevron_right,
                    size: 20.dp,
                    color: AppColors.textTertiary,
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.xs,
                children: <Widget>[
                  for (final String item in highlights)
                    Container(
                      padding: EdgeInsets.symmetric(
                        horizontal: AppSpacing.sm,
                        vertical: 2.dp,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.fill,
                        borderRadius:
                            BorderRadius.circular(AppSpacing.radiusTag),
                      ),
                      child: Text(item, style: AppText.tag),
                    ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
