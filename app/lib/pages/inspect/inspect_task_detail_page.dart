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

/// 巡检任务详情页。
///
/// 路由：`/inspect-tasks/detail?id=<id>`。
///
/// 展示任务信息 + 「完成巡检」表单（处理结果必填 + 结果图 URL）。
/// 提交成功后任务变为 finished，返回列表时从「待巡检」中消失。
class InspectTaskDetailPage extends StatefulWidget {
  /// 构造巡检任务详情页。
  const InspectTaskDetailPage({super.key});

  @override
  State<InspectTaskDetailPage> createState() => _InspectTaskDetailPageState();
}

class _InspectTaskDetailPageState extends State<InspectTaskDetailPage> {
  final TextEditingController _resultCtrl = TextEditingController();
  final TextEditingController _imageCtrl = TextEditingController();

  int? _id;
  InspectTask? _task;
  bool _loading = true;
  bool _submitting = false;
  String? _error;
  String? _formError;
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
      _error = '缺少有效的任务 ID';
      return;
    }
    _load();
  }

  @override
  void dispose() {
    _resultCtrl.dispose();
    _imageCtrl.dispose();
    super.dispose();
  }

  /// 加载任务详情。
  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final RInspectTask resp = await AppController.inspectTaskDetailUsingGet(_id!);
      if (!mounted) return;
      setState(() {
        _task = resp.isSuccess ? resp.value : null;
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

  /// 提交巡检结果。
  Future<void> _submit() async {
    if (_submitting || _id == null) return;

    final String result = _resultCtrl.text.trim();
    if (result.isEmpty) {
      setState(() => _formError = '请填写巡检结果');
      return;
    }

    setState(() {
      _formError = null;
      _submitting = true;
    });

    try {
      final REmptyResponse resp = await AppController.finishInspectTaskUsingPut(
        _id!,
        FinishInspectTaskReq(
          result: result,
          resultImage: _imageCtrl.text.trim(),
        ),
      );
      if (!mounted) return;

      if (!resp.ok) {
        setState(() {
          _submitting = false;
          _formError = resp.errorMessage;
        });
        return;
      }

      context.showToast('巡检已完成');
      context.pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _formError = '提交失败：$e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('巡检任务详情')),
      body: _buildBody(),
      bottomNavigationBar:
          (_task != null && !_task!.isFinished) ? _buildBottomBar() : null,
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const LoadingIndicator(text: '加载中...');
    }

    final InspectTask? task = _task;
    if (task == null) {
      return EmptyState.error(message: _error, onRetry: _load);
    }

    return ListView(
      padding: EdgeInsets.only(bottom: AppSpacing.xxl),
      children: <Widget>[
        _buildTaskCard(task),
        _buildTaskContentCard(task),
        if (task.isFinished) _buildFinishedHint(),
        if (!task.isFinished) _buildForm(),
      ],
    );
  }

  /// 任务基本信息。
  Widget _buildTaskCard(InspectTask task) {
    return _card(
      title: '任务信息',
      children: <Widget>[
        InfoRow(
          label: '任务名称',
          valueWidget: Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  task.name.orPlaceholder('--'),
                  style: AppText.body,
                ),
              ),
              StatusTag(
                text: task.statusText,
                color: task.isFinished ? AppColors.success : AppColors.primary,
              ),
            ],
          ),
        ),
        InfoRow(label: '所属区域', value: task.regionName),
        InfoRow(label: '巡检位置', value: task.location),
        InfoRow(label: '巡检时间', value: task.planDT.orPlaceholder('--')),
        if (task.remark.isNotEmpty)
          InfoRow(label: '备注', value: task.remark),
      ],
    );
  }

  /// 巡检内容。
  Widget _buildTaskContentCard(InspectTask task) {
    return _card(
      title: '巡检内容',
      children: <Widget>[
        Text(
          task.content.orPlaceholder('无具体要求'),
          style: AppText.body,
        ),
      ],
    );
  }

  /// 已完成提示。
  Widget _buildFinishedHint() {
    return _card(
      title: '巡检状态',
      children: <Widget>[
        Row(
          children: <Widget>[
            Icon(
              Icons.check_circle_outline,
              size: 18.dp,
              color: AppColors.success,
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Text('该任务已完成巡检', style: AppText.body),
            ),
          ],
        ),
      ],
    );
  }

  /// 完成巡检表单。
  Widget _buildForm() {
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
          Row(
            children: <Widget>[
              Text('完成巡检', style: AppText.section),
              SizedBox(width: 2.dp),
              const Text('*', style: TextStyle(color: AppColors.danger)),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          TextField(
            key: const Key('inspect-result'),
            controller: _resultCtrl,
            enabled: !_submitting,
            maxLines: 5,
            minLines: 4,
            maxLength: 500,
            decoration: const InputDecoration(
              hintText: '请填写巡检结果，如：设备运行正常，无异常发热',
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          TextField(
            key: const Key('inspect-result-image'),
            controller: _imageCtrl,
            enabled: !_submitting,
            decoration: InputDecoration(
              hintText: '结果图地址（可留空）',
              prefixIcon: Icon(Icons.image_outlined, size: 20.dp),
            ),
          ),
          if (_formError != null) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Text(
              _formError!,
              key: const Key('inspect-form-error'),
              style: TextStyle(fontSize: 13.dp, color: AppColors.danger),
            ),
          ],
        ],
      ),
    );
  }

  /// 底部提交按钮。
  Widget _buildBottomBar() {
    return SafeArea(
      child: Container(
        padding: EdgeInsets.all(AppSpacing.lg),
        decoration: const BoxDecoration(
          color: AppColors.card,
          border: Border(top: BorderSide(color: AppColors.divider)),
        ),
        child: ElevatedButton(
          key: const Key('inspect-submit'),
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
              : const Text('完成巡检'),
        ),
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
}
