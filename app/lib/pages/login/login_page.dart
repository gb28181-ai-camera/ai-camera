import 'package:flutter/material.dart';

import '../../config/api_config.dart';
import '../../models/index.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../widgets/loading_indicator.dart';
import '../../layout/Gaps.dart';

/// 登录页。
///
/// - 用户名 + 密码 + 登录按钮；
/// - 错误信息内联展示（不做弹窗）；
/// - loading 时按钮内嵌 spinner 且禁用；
/// - 登录成功后由 GoRouter 的 redirect 自动跳转首页，本页不做导航。
class LoginPage extends StatefulWidget {
  /// 构造登录页。
  const LoginPage({super.key});

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final TextEditingController _userCtrl = TextEditingController();
  final TextEditingController _pwdCtrl = TextEditingController();
  final FocusNode _pwdFocus = FocusNode();

  /// 本地校验错误（优先于模型返回的错误展示）。
  String? _localError;

  /// 密码是否明文显示。
  bool _obscure = true;

  @override
  void initState() {
    super.initState();
    // 登录态（loading / error）变化时刷新页面。
    authChangeNotifier.addListener(_onAuthChanged);
  }

  @override
  void dispose() {
    authChangeNotifier.removeListener(_onAuthChanged);
    _userCtrl.dispose();
    _pwdCtrl.dispose();
    _pwdFocus.dispose();
    super.dispose();
  }

  void _onAuthChanged() {
    if (!mounted) return;
    setState(() {});
  }

  /// 当前登录态。
  AuthState get _auth => AppStore.auth;

  /// 展示用的错误文案。
  String? get _errorText => _localError ?? _auth.error;

  /// 提交登录。
  Future<void> _submit() async {
    final String userName = _userCtrl.text.trim();
    final String password = _pwdCtrl.text;

    if (userName.isEmpty) {
      setState(() => _localError = '请输入用户名');
      return;
    }
    if (password.isEmpty) {
      setState(() => _localError = '请输入密码');
      return;
    }

    setState(() => _localError = null);
    FocusScope.of(context).unfocus();
    await AppStore.login(userName, password);
  }

  @override
  Widget build(BuildContext context) {
    final bool loading = _auth.loading;
    final String? error = _errorText;

    return Scaffold(
      backgroundColor: AppColors.card,
      body: SafeArea(
        child: SingleChildScrollView(
          padding: EdgeInsets.symmetric(
            horizontal: AppSpacing.xxl,
            vertical: AppSpacing.xxxl,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              SizedBox(height: MediaQuery.of(context).size.height * 0.08),
              _buildLogo(),
              SizedBox(height: AppSpacing.xxxl),
              Text('欢迎登录', style: AppText.title),
              SizedBox(height: AppSpacing.sm),
              Text('AI Camera 智能监控平台', style: AppText.caption),
              SizedBox(height: AppSpacing.xxl),

              // ── 用户名 ──
              TextField(
                key: const Key('login-username'),
                controller: _userCtrl,
                enabled: !loading,
                textInputAction: TextInputAction.next,
                autocorrect: false,
                decoration: InputDecoration(
                  hintText: '请输入用户名',
                  prefixIcon: Icon(Icons.person_outline, size: 20.dp),
                ),
                onSubmitted: (_) => _pwdFocus.requestFocus(),
              ),
              SizedBox(height: AppSpacing.lg),

              // ── 密码 ──
              TextField(
                key: const Key('login-password'),
                controller: _pwdCtrl,
                focusNode: _pwdFocus,
                enabled: !loading,
                obscureText: _obscure,
                textInputAction: TextInputAction.done,
                decoration: InputDecoration(
                  hintText: '请输入密码',
                  prefixIcon: Icon(Icons.lock_outline, size: 20.dp),
                  suffixIcon: IconButton(
                    icon: Icon(
                      _obscure
                          ? Icons.visibility_off_outlined
                          : Icons.visibility_outlined,
                      size: 20.dp,
                      color: AppColors.textTertiary,
                    ),
                    onPressed: () => setState(() => _obscure = !_obscure),
                  ),
                ),
                onSubmitted: (_) => loading ? null : _submit(),
              ),

              // ── 内联错误提示 ──
              if (error != null && error.isNotEmpty) ...<Widget>[
                SizedBox(height: AppSpacing.md),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Icon(
                      Icons.error_outline,
                      size: 16.dp,
                      color: AppColors.danger,
                    ),
                    SizedBox(width: AppSpacing.xs),
                    Expanded(
                      child: Text(
                        error,
                        key: const Key('login-error'),
                        style: TextStyle(
                          fontSize: 13.dp,
                          color: AppColors.danger,
                          height: 1.4,
                        ),
                      ),
                    ),
                  ],
                ),
              ],

              SizedBox(height: AppSpacing.xxl),

              // ── 登录按钮 ──
              ElevatedButton(
                key: const Key('login-submit'),
                onPressed: loading ? null : _submit,
                child: loading
                    ? Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: <Widget>[
                          const InlineSpinner(),
                          SizedBox(width: AppSpacing.sm),
                          const Text('登录中...'),
                        ],
                      )
                    : const Text('登 录'),
              ),

              SizedBox(height: AppSpacing.xxxl),
              Center(
                child: Text(
                  '版本 ${ApiConfig.appVersion}',
                  style: AppText.hint,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// 顶部 Logo 区。
  Widget _buildLogo() {
    return Column(
      children: <Widget>[
        Container(
          width: 72.dp,
          height: 72.dp,
          decoration: BoxDecoration(
            color: AppColors.primaryLight,
            borderRadius: BorderRadius.circular(AppSpacing.radiusCard * 1.6),
          ),
          child: Icon(
            Icons.videocam_outlined,
            size: 38.dp,
            color: AppColors.primary,
          ),
        ),
        SizedBox(height: AppSpacing.md),
        Text('AI Camera', style: AppText.title.copyWith(fontSize: 22.dp)),
      ],
    );
  }
}
