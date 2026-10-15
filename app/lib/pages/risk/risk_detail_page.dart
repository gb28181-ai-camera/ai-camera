import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/Definition.dart';
import '../../services/generated/definitions/LoginReq.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/empty_state.dart';
import '../../widgets/entity_list_page.dart';
import '../../widgets/loading_indicator.dart';
import '../../widgets/stat_card.dart';
import '../../layout/Gaps.dart';

/// 风险详情页。
///
/// 路由：`/risks/detail?id=<id>`。
///
/// 底部操作：
/// - 「标记误报」→ `accurate=no`，后端直接置为已处理；
/// - 「确认并处理」→ `accurate=yes`，随后进入处理页填写处理结果。
class RiskDetailPage extends StatefulWidget {
  /// 构造风险详情页。
  const RiskDetailPage({super.key});

  @override
  State<RiskDetailPage> createState() => _RiskDetailPageState();
}

class _RiskDetailPageState extends State<RiskDetailPage> {
  int? _id;
  RiskRecord? _risk;
  bool _loading = true;
  bool _submitting = false;
  String? _error;
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    _id = int.tryParse(
      GoRouterState.of(context).uri.queryParameters['id'] ?? '',
    );
    if (_id == null) {
      _loading = false;
      _error = '缺少有效的风险 ID';
      return;
    }
    _load();
  }

  /// 加载风险详情。
  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final RRiskRecord resp = await AppController.riskDetailUsingGet(_id!);
      if (!mounted) return;
      setState(() {
        _risk = resp.isSuccess ? resp.value : null;
        _error = resp.isSuccess ? null : resp.errorMessage;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '加载失败，请重试';
      });
    }
  }

  /// 确认检测准确性。
  ///
  /// [accurate] 为 `no` 时标记误报并返回；为 `yes` 时进入处理页。
  Future<void> _confirmAccurate(String accurate) async {
    if (_submitting || _id == null) return;
    setState(() => _submitting = true);

    try {
      final REmptyResponse resp = await AppController.updateRiskAccurateUsingPut(
        _id!,
        UpdateAccurateReq(accurate: accurate),
      );
      if (!mounted) return;

      if (!resp.ok) {
        setState(() => _submitting = false);
        context.showToast(resp.errorMessage);
        return;
      }

      if (accurate == 'no') {
        context.showToast('已标记为误报');
        context.pop(true);
        return;
      }

      // 确认为真实风险 → 进入处理页。
      final bool? handled = await context.push<bool>('/risks/handle?id=$_id');
      if (!mounted) return;
      setState(() => _submitting = false);
      if (handled == true) {
        context.pop(true);
      } else {
        await _load();
      }
    } catch (e) {
      if (!mounted) return;
      setState(() => _submitting = false);
      context.showToast('操作失败：$e');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('风险详情')),
      body: _buildBody(),
      bottomNavigationBar: _risk == null ? null : _buildActions(),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const LoadingIndicator(text: '加载中...');
    }

    final RiskRecord? risk = _risk;
    if (risk == null) {
      return EmptyState.error(message: _error, onRetry: _load);
    }

    return ListView(
      padding: EdgeInsets.only(bottom: AppSpacing.xxl),
      children: <Widget>[
        _buildImage(risk),
        _buildBasicCard(risk),
        _buildDetectCard(risk),
        if (risk.isHandled) _buildHandleResultCard(risk),
      ],
    );
  }

  /// 大图预览。
  Widget _buildImage(RiskRecord risk) {
    final String full = absoluteImageUrl(risk.imageUrl);
    return GestureDetector(
      onTap: full.isEmpty ? null : () => _previewImage(full),
      child: Container(
        color: AppColors.card,
        width: double.infinity,
        height: AppSpacing.detailImageHeight,
        child: RemoteImage(
          url: risk.imageUrl,
          width: double.infinity,
          height: AppSpacing.detailImageHeight,
          radius: 0,
          fit: BoxFit.contain,
        ),
      ),
    );
  }

  /// 全屏预览图片。
  void _previewImage(String url) {
    Navigator.of(context).push(
      PageRouteBuilder<void>(
        opaque: false,
        barrierColor: Colors.black87,
        pageBuilder: (BuildContext context, _, __) {
          return Scaffold(
            backgroundColor: Colors.transparent,
            body: GestureDetector(
              onTap: () => Navigator.of(context).pop(),
              child: Center(
                child: InteractiveViewer(
                  child: Image.network(url),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  /// 基本信息卡片。
  Widget _buildBasicCard(RiskRecord risk) {
    return _card(
      title: '风险信息',
      children: <Widget>[
        InfoRow(
          label: '风险类型',
          value: risk.riskTypeName,
          valueWidget: Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  risk.riskTypeName.orPlaceholder('--'),
                  style: AppText.body,
                ),
              ),
              StatusTag(
                text: _levelText(risk.riskLevel),
                color: AppColors.levelColor(risk.riskLevel),
              ),
            ],
          ),
        ),
        InfoRow(label: '设备名称', value: risk.deviceName),
        InfoRow(label: '设备编号', value: risk.deviceCode),
        InfoRow(label: '所属区域', value: risk.regionName),
        InfoRow(label: '设备位置', value: risk.location),
        InfoRow(
          label: '发生时间',
          value: risk.occurredDT.orPlaceholder('--'),
        ),
        InfoRow(
          label: '处理状态',
          valueWidget: StatusTag(
            text: risk.isHandled ? '已处理' : '待处理',
            color: AppColors.statusColor(risk.status),
          ),
        ),
      ],
    );
  }

  /// 检测信息卡片。
  Widget _buildDetectCard(RiskRecord risk) {
    return _card(
      title: 'AI 检测',
      children: <Widget>[
        InfoRow(
          label: '检测置信度',
          valueWidget: Row(
            children: <Widget>[
              Expanded(
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(AppSpacing.radiusTag),
                  child: LinearProgressIndicator(
                    value: risk.detectScore.clamp(0.0, 1.0),
                    minHeight: 8,
                    backgroundColor: AppColors.fill,
                    valueColor: AlwaysStoppedAnimation<Color>(
                      AppColors.levelColor(risk.riskLevel),
                    ),
                  ),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Text(risk.detectScoreText, style: AppText.caption),
            ],
          ),
        ),
        InfoRow(
          label: '检测准确',
          valueWidget: StatusTag(
            text: _accurateText(risk.detectAccurate),
            color: _accurateColor(risk.detectAccurate),
          ),
        ),
        if (risk.accurateBy.isNotEmpty)
          InfoRow(label: '确认人', value: risk.accurateBy),
        if (risk.accurateDT != null)
          InfoRow(
            label: '确认时间',
            value: risk.accurateDT.orPlaceholder('--'),
          ),
      ],
    );
  }

  /// 处理结果卡片（已处理时展示）。
  Widget _buildHandleResultCard(RiskRecord risk) {
    return _card(
      title: '处理结果',
      children: <Widget>[
        InfoRow(label: '处理人', value: risk.handler),
        InfoRow(
          label: '处理时间',
          value: risk.handledDT.orPlaceholder('--'),
        ),
        InfoRow(
          label: '处理说明',
          value: risk.handleResult,
          valueWidget: Text(
            risk.handleResult.orPlaceholder('--'),
            style: AppText.body,
          ),
        ),
        if (risk.handleImage.isNotEmpty) ...<Widget>[
          SizedBox(height: AppSpacing.sm),
          RemoteImage(
            url: risk.handleImage,
            width: double.infinity,
            height: 160.dp,
          ),
        ],
      ],
    );
  }

  /// 底部操作栏。
  Widget _buildActions() {
    final RiskRecord risk = _risk!;

    // 已处理：仅展示关闭。
    if (risk.isHandled) {
      return _bottomBar(
        child: ElevatedButton(
          onPressed: () => context.pop(true),
          child: const Text('返回'),
        ),
      );
    }

    final bool confirmedYes = risk.detectAccurate == 'yes';

    return _bottomBar(
      child: Row(
        children: <Widget>[
          Expanded(
            child: OutlinedButton(
              key: const Key('risk-mark-false'),
              onPressed: _submitting ? null : () => _confirmAccurate('no'),
              child: const Text('标记误报'),
            ),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: ElevatedButton(
              key: const Key('risk-confirm-handle'),
              onPressed:
                  _submitting ? null : () => _confirmAccurate('yes'),
              child: _submitting
                  ? SizedBox(
                      width: 16.dp,
                      height: 16.dp,
                      child: const CircularProgressIndicator(
                        strokeWidth: 2,
                        valueColor: AlwaysStoppedAnimation<Color>(
                          AppColors.textOnPrimary,
                        ),
                      ),
                    )
                  : Text(confirmedYes ? '去处理' : '确认并处理'),
            ),
          ),
        ],
      ),
    );
  }

  /// 底部统一容器。
  Widget _bottomBar({required Widget child}) {
    return SafeArea(
      child: Container(
        padding: EdgeInsets.all(AppSpacing.lg),
        decoration: const BoxDecoration(
          color: AppColors.card,
          border: Border(top: BorderSide(color: AppColors.divider)),
        ),
        child: child,
      ),
    );
  }

  /// 统一卡片外观。
  Widget _card({required String title, required List<Widget> children}) {
    return Container(
      margin: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        0,
      ),
      padding: EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(title, style: AppText.section),
          SizedBox(height: AppSpacing.sm),
          ...children,
        ],
      ),
    );
  }

  static String _levelText(String level) {
    switch (level) {
      case 'high':
        return '高风险';
      case 'medium':
        return '中风险';
      case 'low':
        return '低风险';
      default:
        return '未分级';
    }
  }

  static String _accurateText(String accurate) {
    switch (accurate) {
      case 'yes':
        return '准确';
      case 'no':
        return '误报';
      default:
        return '待确认';
    }
  }

  static Color _accurateColor(String accurate) {
    switch (accurate) {
      case 'yes':
        return AppColors.success;
      case 'no':
        return AppColors.textTertiary;
      default:
        return AppColors.warning;
    }
  }
}
