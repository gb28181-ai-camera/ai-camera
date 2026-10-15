import 'package:flutter/material.dart';

import '../../models/index.dart';
import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/Definition.dart';
import '../../services/generated/definitions/LoginReq.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/empty_state.dart';
import '../../widgets/loading_indicator.dart';
import '../../widgets/stat_card.dart';
import '../../layout/Gaps.dart';

/// 我的（个人中心）。
///
/// 展示姓名 / 用户名 / 部门 / 岗位，提供修改密码与退出登录入口。
class ProfilePage extends StatefulWidget {
  /// 构造个人中心页。
  const ProfilePage({super.key});

  @override
  State<ProfilePage> createState() => _ProfilePageState();
}

class _ProfilePageState extends State<ProfilePage> {
  UserProfile? _profile;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    authChangeNotifier.addListener(_onAuthChanged);
    _load();
  }

  @override
  void dispose() {
    authChangeNotifier.removeListener(_onAuthChanged);
    super.dispose();
  }

  void _onAuthChanged() {
    if (!mounted) return;
    setState(() {});
  }

  /// 加载个人资料。
  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final RUser resp = await AppController.profileUsingGet();
      if (!mounted) return;
      setState(() {
        _profile = resp.isSuccess ? resp.value : null;
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

  /// 退出登录确认。
  Future<void> _confirmLogout() async {
    final bool? ok = await showDialog<bool>(
      context: context,
      builder: (BuildContext ctx) => AlertDialog(
        title: const Text('退出登录'),
        content: const Text('确定要退出当前账号吗？'),
        actions: <Widget>[
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('确定退出'),
          ),
        ],
      ),
    );

    if (ok != true) return;
    await AppStore.logout();
  }

  /// 打开修改密码弹窗。
  Future<void> _openChangePassword() async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: AppColors.card,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppSpacing.radiusCard * 1.5),
        ),
      ),
      builder: (BuildContext ctx) => const _ChangePasswordSheet(),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('我的')),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _buildBody(),
      ),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const LoadingIndicator(text: '加载中...');
    }

    if (_profile == null && _error != null) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: <Widget>[
          SizedBox(height: MediaQuery.of(context).size.height * 0.12),
          EmptyState.error(message: _error, onRetry: _load),
        ],
      );
    }

    final AuthState auth = AppStore.auth;
    final UserProfile? p = _profile;

    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: EdgeInsets.only(bottom: AppSpacing.xxl),
      children: <Widget>[
        _buildHeader(p, auth),
        _buildInfoCard(p),
        _buildActionCard(),
      ],
    );
  }

  /// 顶部头像与姓名。
  Widget _buildHeader(UserProfile? p, AuthState auth) {
    final String name = p?.displayName ?? auth.displayName;
    final String userName = p?.userName ?? '';

    return Container(
      width: double.infinity,
      color: AppColors.card,
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.xl,
      ),
      child: Row(
        children: <Widget>[
          Container(
            width: 60.dp,
            height: 60.dp,
            decoration: const BoxDecoration(
              color: AppColors.primaryLight,
              shape: BoxShape.circle,
            ),
            alignment: Alignment.center,
            child: Text(
              name.isEmpty ? '?' : name.characters.first,
              style: AppText.title.copyWith(color: AppColors.primary),
            ),
          ),
          SizedBox(width: AppSpacing.lg),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(name, style: AppText.title),
                SizedBox(height: AppSpacing.xs),
                Text(
                  userName.isEmpty ? '--' : '@$userName',
                  style: AppText.hint,
                ),
                if (p != null && p.isAdmin) ...<Widget>[
                  SizedBox(height: AppSpacing.sm),
                  const StatusTag(
                    text: '超级管理员',
                    color: AppColors.primary,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// 个人信息卡片。
  Widget _buildInfoCard(UserProfile? p) {
    return Container(
      margin: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        0,
      ),
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: AppSpacing.sm,
      ),
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      child: Column(
        children: <Widget>[
          ProfileRow(label: '姓名', value: p?.name ?? ''),
          const Divider(height: 1),
          ProfileRow(label: '用户名', value: p?.userName ?? ''),
          const Divider(height: 1),
          ProfileRow(label: '部门', value: p?.deptName ?? ''),
          const Divider(height: 1),
          ProfileRow(label: '岗位', value: p?.postName ?? ''),
          const Divider(height: 1),
          ProfileRow(label: '手机号', value: p?.phone ?? ''),
        ],
      ),
    );
  }

  /// 操作入口卡片。
  Widget _buildActionCard() {
    return Container(
      margin: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        0,
      ),
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      child: Column(
        children: <Widget>[
          ListTile(
            key: const Key('profile-change-password'),
            leading: Icon(
              Icons.lock_outline,
              size: 20.dp,
              color: AppColors.textSecondary,
            ),
            title: const Text('修改密码'),
            trailing: Icon(
              Icons.chevron_right,
              size: 20.dp,
              color: AppColors.textTertiary,
            ),
            onTap: _openChangePassword,
          ),
          const Divider(height: 1),
          ListTile(
            key: const Key('profile-logout'),
            leading: Icon(
              Icons.logout_outlined,
              size: 20.dp,
              color: AppColors.danger,
            ),
            title: const Text(
              '退出登录',
              style: TextStyle(color: AppColors.danger),
            ),
            onTap: _confirmLogout,
          ),
        ],
      ),
    );
  }
}

/// 个人信息行。
class ProfileRow extends StatelessWidget {
  /// 构造信息行。
  const ProfileRow({super.key, required this.label, required this.value});

  /// 标签。
  final String label;

  /// 值。
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
      child: Row(
        children: <Widget>[
          SizedBox(
            width: 72.dp,
            child: Text(label, style: AppText.caption),
          ),
          Expanded(
            child: Text(
              value.isEmpty ? '--' : value,
              style: AppText.body,
              textAlign: TextAlign.right,
            ),
          ),
        ],
      ),
    );
  }
}

/// 修改密码底部弹窗。
class _ChangePasswordSheet extends StatefulWidget {
  const _ChangePasswordSheet();

  @override
  State<_ChangePasswordSheet> createState() => _ChangePasswordSheetState();
}

class _ChangePasswordSheetState extends State<_ChangePasswordSheet> {
  final TextEditingController _oldCtrl = TextEditingController();
  final TextEditingController _newCtrl = TextEditingController();
  final TextEditingController _confirmCtrl = TextEditingController();

  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _oldCtrl.dispose();
    _newCtrl.dispose();
    _confirmCtrl.dispose();
    super.dispose();
  }

  /// 提交修改密码。
  Future<void> _submit() async {
    if (_submitting) return;

    final String oldPwd = _oldCtrl.text;
    final String newPwd = _newCtrl.text;
    final String confirm = _confirmCtrl.text;

    if (oldPwd.isEmpty) {
      setState(() => _error = '请输入原密码');
      return;
    }
    if (newPwd.isEmpty) {
      setState(() => _error = '请输入新密码');
      return;
    }
    if (newPwd.length < 6) {
      setState(() => _error = '新密码至少 6 位');
      return;
    }
    if (newPwd != confirm) {
      setState(() => _error = '两次输入的新密码不一致');
      return;
    }

    setState(() {
      _error = null;
      _submitting = true;
    });

    final REmptyResponse resp = await AppController.changePasswordUsingPut(
      ChangePasswordReq(oldPassword: oldPwd, newPassword: newPwd),
    );
    if (!mounted) return;

    if (!resp.ok) {
      setState(() {
        _submitting = false;
        _error = resp.errorMessage;
      });
      return;
    }

    Navigator.of(context).pop();
    context.showToast('密码修改成功，请重新登录');
    await AppStore.logout();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(
        left: AppSpacing.pagePadding,
        right: AppSpacing.pagePadding,
        top: AppSpacing.lg,
        bottom: MediaQuery.of(context).viewInsets.bottom + AppSpacing.lg,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Center(
            child: Container(
              width: 36.dp,
              height: 4.dp,
              decoration: BoxDecoration(
                color: AppColors.divider,
                borderRadius: BorderRadius.circular(2.dp),
              ),
            ),
          ),
          SizedBox(height: AppSpacing.lg),
          Text('修改密码', style: AppText.title),
          SizedBox(height: AppSpacing.lg),
          TextField(
            key: const Key('password-old'),
            controller: _oldCtrl,
            obscureText: true,
            enabled: !_submitting,
            decoration: const InputDecoration(hintText: '请输入原密码'),
          ),
          SizedBox(height: AppSpacing.md),
          TextField(
            key: const Key('password-new'),
            controller: _newCtrl,
            obscureText: true,
            enabled: !_submitting,
            decoration: const InputDecoration(hintText: '请输入新密码（至少 6 位）'),
          ),
          SizedBox(height: AppSpacing.md),
          TextField(
            key: const Key('password-confirm'),
            controller: _confirmCtrl,
            obscureText: true,
            enabled: !_submitting,
            decoration: const InputDecoration(hintText: '请再次输入新密码'),
          ),
          if (_error != null) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Text(
              _error!,
              key: const Key('password-error'),
              style: TextStyle(fontSize: 13.dp, color: AppColors.danger),
            ),
          ],
          SizedBox(height: AppSpacing.xl),
          ElevatedButton(
            key: const Key('password-submit'),
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
                : const Text('确认修改'),
          ),
        ],
      ),
    );
  }
}
