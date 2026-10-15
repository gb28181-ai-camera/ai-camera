import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../models/index.dart';
import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/Definition.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/empty_state.dart';
import '../../widgets/loading_indicator.dart';
import '../../widgets/stat_card.dart';
import '../../layout/Gaps.dart';

/// 首页看板。
///
/// 一屏展示：问候语、四类待办、今日风险/告警、设备在线情况、最近风险。
/// 数据来源 `GET /app/home`。
class HomePage extends StatefulWidget {
  /// 构造首页。
  const HomePage({super.key});

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  AppHome _data = AppHome.empty;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  /// 加载首页数据。
  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final RAppHome resp = await AppController.homeUsingGet();
      if (!mounted) return;
      setState(() {
        _data = resp.isSuccess ? resp.home : AppHome.empty;
        _error = resp.isSuccess ? null : resp.errorMessage;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '加载失败，请下拉重试';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: RefreshIndicator(
          onRefresh: _load,
          child: _buildContent(),
        ),
      ),
    );
  }

  Widget _buildContent() {
    if (_loading && _error == null) {
      return const LoadingIndicator(text: '加载中...');
    }

    if (_error != null && _data.deviceStat.total == 0 && _data.recentRisks.isEmpty) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: <Widget>[
          SizedBox(height: MediaQuery.of(context).size.height * 0.15),
          EmptyState.error(message: _error, onRetry: _load),
        ],
      );
    }

    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: EdgeInsets.only(bottom: AppSpacing.xxl),
      children: <Widget>[
        _buildHeader(),
        _buildTodoSection(),
        _buildTodaySection(),
        _buildDeviceSection(),
        _buildRecentRiskSection(),
      ],
    );
  }

  // ── 顶部问候 ──────────────────────────────────────────────

  Widget _buildHeader() {
    final String name = AppStore.auth.displayName;
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        AppSpacing.md,
      ),
      child: Row(
        children: <Widget>[
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text('你好，$name', style: AppText.title),
                SizedBox(height: AppSpacing.xs),
                Text(
                  '${DateTime.now().toDateString}  ${_greeting()}',
                  style: AppText.hint,
                ),
              ],
            ),
          ),
          IconButton(
            key: const Key('home-notification-entry'),
            onPressed: () => context.push('/notifications'),
            icon: Badge(
              isLabelVisible: _data.unreadCount > 0,
              label: Text('${_data.unreadCount}'),
              child: const Icon(Icons.notifications_none_outlined),
            ),
          ),
        ],
      ),
    );
  }

  /// 按时段生成问候语。
  String _greeting() {
    final int hour = DateTime.now().hour;
    if (hour < 6) return '凌晨好';
    if (hour < 12) return '早上好';
    if (hour < 14) return '中午好';
    if (hour < 18) return '下午好';
    return '晚上好';
  }

  // ── 待办统计 ──────────────────────────────────────────────

  Widget _buildTodoSection() {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          const _SectionTitle(title: '我的待办'),
          GridView.count(
            crossAxisCount: 2,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            mainAxisSpacing: AppSpacing.md,
            crossAxisSpacing: AppSpacing.md,
            childAspectRatio: 1.75,
            children: <Widget>[
              StatCard(
                key: const Key('home-todo-risk'),
                title: '待处理风险',
                value: '${_data.pendingRiskCount}',
                icon: Icons.warning_amber_outlined,
                color: AppColors.danger,
                onTap: () => context.push('/risks?status=pending'),
              ),
              StatCard(
                key: const Key('home-todo-alarm'),
                title: '待处理告警',
                value: '${_data.pendingAlarmCount}',
                icon: Icons.notifications_active_outlined,
                color: AppColors.warning,
                onTap: () => context.push('/alarms?status=pending'),
              ),
              StatCard(
                key: const Key('home-todo-inspect'),
                title: '待巡检任务',
                value: '${_data.pendingInspectCount}',
                icon: Icons.fact_check_outlined,
                color: AppColors.primary,
                onTap: () => context.push('/inspect-tasks'),
              ),
              StatCard(
                key: const Key('home-todo-notification'),
                title: '未读通知',
                value: '${_data.unreadCount}',
                icon: Icons.mark_email_unread_outlined,
                color: AppColors.textSecondary,
                onTap: () => context.push('/notifications'),
              ),
            ],
          ),
        ],
      ),
    );
  }

  // ── 今日风险 / 告警 ───────────────────────────────────────

  Widget _buildTodaySection() {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        0,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          const _SectionTitle(title: '今日概览'),
          Row(
            children: <Widget>[
              Expanded(
                child: StatCard(
                  key: const Key('home-today-risk'),
                  title: '今日风险',
                  value: '${_data.todayRiskCount}',
                  icon: Icons.report_gmailerrorred_outlined,
                  color: AppColors.danger,
                  compact: true,
                  onTap: () => context.push('/risks'),
                ),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: StatCard(
                  key: const Key('home-today-alarm'),
                  title: '今日告警',
                  value: '${_data.todayAlarmCount}',
                  icon: Icons.error_outline,
                  color: AppColors.warning,
                  compact: true,
                  onTap: () => context.push('/alarms'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  // ── 设备在线情况 ──────────────────────────────────────────

  Widget _buildDeviceSection() {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        0,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          const _SectionTitle(title: '设备在线情况'),
          ProgressStatCard(
            key: const Key('home-device-stat'),
            title: '设备总数 ${_data.deviceStat.total}',
            total: _data.deviceStat.total,
            done: _data.deviceStat.online,
            doneLabel: '在线',
            undoneLabel: '离线',
            icon: Icons.videocam_outlined,
            onTap: () => context.push('/devices'),
          ),
        ],
      ),
    );
  }

  // ── 最近风险 ──────────────────────────────────────────────

  Widget _buildRecentRiskSection() {
    final List<RiskRecord> risks = _data.recentRisks;

    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        0,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              const Expanded(child: _SectionTitle(title: '最近风险')),
              TextButton(
                onPressed: () => context.push('/risks'),
                child: const Text('全部'),
              ),
            ],
          ),
          if (risks.isEmpty)
            Container(
              padding: EdgeInsets.symmetric(vertical: AppSpacing.xxl),
              decoration: BoxDecoration(
                color: AppColors.card,
                borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
              ),
              child: const EmptyState(
                icon: Icons.verified_outlined,
                title: '暂无风险记录',
              ),
            )
          else
            ...risks.map(_buildRiskTile),
        ],
      ),
    );
  }

  /// 最近风险条目。
  Widget _buildRiskTile(RiskRecord risk) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Material(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        child: InkWell(
          key: Key('home-risk-${risk.id}'),
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
          onTap: () => context.push('/risks/detail?id=${risk.id}'),
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.md),
            child: Row(
              children: <Widget>[
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Row(
                        children: <Widget>[
                          Expanded(
                            child: Text(
                              risk.riskTypeName.orPlaceholder('未知风险'),
                              style: AppText.bodyStrong,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          StatusTag(
                            text: risk.isHandled ? '已处理' : '待处理',
                            color: AppColors.statusColor(risk.status),
                          ),
                        ],
                      ),
                      SizedBox(height: AppSpacing.xs),
                      Text(
                        '${risk.deviceName.orPlaceholder(risk.deviceCode)} · '
                        '${risk.regionName.orPlaceholder('未分配区域')}',
                        style: AppText.hint,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      SizedBox(height: AppSpacing.xs),
                      Text(risk.occurredDT.fromNow, style: AppText.hint),
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
          ),
        ),
      ),
    );
  }
}

/// 区块标题。
class _SectionTitle extends StatelessWidget {
  const _SectionTitle({required this.title});

  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.md),
      child: Text(title, style: AppText.section),
    );
  }
}
