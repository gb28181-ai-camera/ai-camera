import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/LoginReq.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../layout/Gaps.dart';

/// 告警处理页。
///
/// 路由：`/alarms/handle?id=<id>`。
///
/// 与风险处理页结构一致，但告警没有「检测准确」确认环节：
/// 列表点击后直接进入本页，填写处理结果即可。
class AlarmHandlePage extends StatefulWidget {
  /// 构造告警处理页。
  const AlarmHandlePage({super.key});

  @override
  State<AlarmHandlePage> createState() => _AlarmHandlePageState();
}

class _AlarmHandlePageState extends State<AlarmHandlePage> {
  final TextEditingController _resultCtrl = TextEditingController();
  final TextEditingController _imageCtrl = TextEditingController();

  int? _id;
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
      _error = '缺少有效的告警 ID';
    }
  }

  @override
  void dispose() {
    _resultCtrl.dispose();
    _imageCtrl.dispose();
    super.dispose();
  }

  /// 提交处理结果。
  Future<void> _submit() async {
    if (_submitting) return;

    if (_id == null) {
      setState(() => _error = '缺少有效的告警 ID');
      return;
    }

    final String result = _resultCtrl.text.trim();
    if (result.isEmpty) {
      setState(() => _error = '请填写处理结果');
      return;
    }

    setState(() {
      _error = null;
      _submitting = true;
    });

    try {
      final REmptyResponse resp = await AppController.handleAlarmUsingPut(
        _id!,
        HandleReq(
          handleResult: result,
          handleImage: _imageCtrl.text.trim(),
        ),
      );
      if (!mounted) return;

      if (!resp.ok) {
        setState(() {
          _submitting = false;
          _error = resp.errorMessage;
        });
        return;
      }

      context.showToast('处理成功');
      context.pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _error = '提交失败：$e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('处理告警')),
      body: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: <Widget>[
          _buildCard(
            title: '处理结果',
            isRequired: true,
            child: TextField(
              key: const Key('alarm-handle-result'),
              controller: _resultCtrl,
              enabled: !_submitting,
              maxLines: 5,
              minLines: 4,
              maxLength: 500,
              decoration: const InputDecoration(
                hintText: '请描述处置情况，如：已更换电池并重启设备',
                border: InputBorder.none,
              ),
            ),
          ),
          SizedBox(height: AppSpacing.md),
          _buildCard(
            title: '处理图片',
            isRequired: false,
            description: '本版本暂不支持拍照上传，请填写图片地址（可留空）。',
            child: TextField(
              key: const Key('alarm-handle-image'),
              controller: _imageCtrl,
              enabled: !_submitting,
              decoration: const InputDecoration(
                hintText: '/uploads/2026/09/alarm.jpg',
                border: InputBorder.none,
              ),
            ),
          ),
          if (_error != null) ...<Widget>[
            SizedBox(height: AppSpacing.md),
            Row(
              children: <Widget>[
                Icon(
                  Icons.error_outline,
                  size: 16.dp,
                  color: AppColors.danger,
                ),
                SizedBox(width: AppSpacing.xs),
                Expanded(
                  child: Text(
                    _error!,
                    key: const Key('alarm-handle-error'),
                    style: TextStyle(
                      fontSize: 13.dp,
                      color: AppColors.danger,
                    ),
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
      bottomNavigationBar: SafeArea(
        child: Container(
          padding: EdgeInsets.all(AppSpacing.lg),
          decoration: const BoxDecoration(
            color: AppColors.card,
            border: Border(top: BorderSide(color: AppColors.divider)),
          ),
          child: ElevatedButton(
            key: const Key('alarm-handle-submit'),
            onPressed: _submitting ? null : _submit,
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
                : const Text('提交处理'),
          ),
        ),
      ),
    );
  }

  /// 表单卡片。
  Widget _buildCard({
    required String title,
    required bool isRequired,
    required Widget child,
    String? description,
  }) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      padding: EdgeInsets.all(AppSpacing.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Text(title, style: AppText.section),
              if (isRequired) ...<Widget>[
                SizedBox(width: 2.dp),
                const Text('*', style: TextStyle(color: AppColors.danger)),
              ],
            ],
          ),
          if (description != null) ...<Widget>[
            SizedBox(height: AppSpacing.xs),
            Text(description, style: AppText.hint),
          ],
          SizedBox(height: AppSpacing.sm),
          child,
        ],
      ),
    );
  }
}
