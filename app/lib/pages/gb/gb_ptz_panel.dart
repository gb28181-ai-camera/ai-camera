import 'package:flutter/material.dart';

import '../../layout/Gaps.dart';
import '../../services/generated/definitions/AppGb.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';

/// 国标云台控制面板。
///
/// 交互与 Web 端 `PtzControl.tsx` 完全一致：
/// **按住持续下发「开始运动」，松开立即下发「停止运动」**。
/// 这是 GB28181 的语义 —— 方向与停止是两个独立指令（带外 SIP MESSAGE），
/// 如果做成点击式单发，镜头会一直转，只能靠再点一次停止才能停下来。
///
/// 面板是**纯 UI 组件**：只向上抛语义化指令，不直接调接口。
class GbPtzPanel extends StatefulWidget {
  /// 构造云台面板。
  const GbPtzPanel({
    super.key,
    required this.onCommand,
    required this.onAction,
    this.enabled = true,
    this.initialSpeed = 5,
    this.onSpeedChanged,
  });

  /// 方向指令回调（direction 取 [GbPtzDirection]，含 stop）。
  final void Function(String direction, int speed) onCommand;

  /// 变焦动作回调（action 取 [GbPtzAction]）。
  final void Function(String action, int speed) onAction;

  /// 是否可用（未开播 / 通道离线时禁用）。
  final bool enabled;

  /// 初始速度 1–8，默认 5。
  final int initialSpeed;

  /// 速度变化回调（画面拖拽云台需要复用同一档速度）。
  final ValueChanged<int>? onSpeedChanged;

  @override
  State<GbPtzPanel> createState() => _GbPtzPanelState();
}

class _GbPtzPanelState extends State<GbPtzPanel> {
  late int _speed = widget.initialSpeed.clamp(1, 8);

  /// 下发方向（按住开始 / 拖拽切换方向都走这里）。
  void _command(String direction) {
    widget.onCommand(direction, _speed);
  }

  /// 变焦动作（按住放大/缩小）。
  void _action(String action) {
    widget.onAction(action, _speed);
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: <Widget>[
        // ── 八方向方向盘：3×3，中间是停止 ──
        _buildDirectionRow(<Widget>[
          _HoldButton(
            key: const Key('gb-ptz-up-left'),
            icon: Icons.north_west,
            label: '左上',
            enabled: widget.enabled,
            onHold: () => _command(GbPtzDirection.upLeft),
            onRelease: () => _command(GbPtzDirection.stop),
          ),
          _HoldButton(
            key: const Key('gb-ptz-up'),
            icon: Icons.north,
            label: '上',
            enabled: widget.enabled,
            onHold: () => _command(GbPtzDirection.up),
            onRelease: () => _command(GbPtzDirection.stop),
          ),
          _HoldButton(
            key: const Key('gb-ptz-up-right'),
            icon: Icons.north_east,
            label: '右上',
            enabled: widget.enabled,
            onHold: () => _command(GbPtzDirection.upRight),
            onRelease: () => _command(GbPtzDirection.stop),
          ),
        ]),
        SizedBox(height: AppSpacing.sm),
        _buildDirectionRow(<Widget>[
          _HoldButton(
            key: const Key('gb-ptz-left'),
            icon: Icons.west,
            label: '左',
            enabled: widget.enabled,
            onHold: () => _command(GbPtzDirection.left),
            onRelease: () => _command(GbPtzDirection.stop),
          ),
          _StopButton(
            key: const Key('gb-ptz-stop'),
            enabled: widget.enabled,
            onTap: () => _command(GbPtzDirection.stop),
          ),
          _HoldButton(
            key: const Key('gb-ptz-right'),
            icon: Icons.east,
            label: '右',
            enabled: widget.enabled,
            onHold: () => _command(GbPtzDirection.right),
            onRelease: () => _command(GbPtzDirection.stop),
          ),
        ]),
        SizedBox(height: AppSpacing.sm),
        _buildDirectionRow(<Widget>[
          _HoldButton(
            key: const Key('gb-ptz-down-left'),
            icon: Icons.south_west,
            label: '左下',
            enabled: widget.enabled,
            onHold: () => _command(GbPtzDirection.downLeft),
            onRelease: () => _command(GbPtzDirection.stop),
          ),
          _HoldButton(
            key: const Key('gb-ptz-down'),
            icon: Icons.south,
            label: '下',
            enabled: widget.enabled,
            onHold: () => _command(GbPtzDirection.down),
            onRelease: () => _command(GbPtzDirection.stop),
          ),
          _HoldButton(
            key: const Key('gb-ptz-down-right'),
            icon: Icons.south_east,
            label: '右下',
            enabled: widget.enabled,
            onHold: () => _command(GbPtzDirection.downRight),
            onRelease: () => _command(GbPtzDirection.stop),
          ),
        ]),
        SizedBox(height: AppSpacing.md),

        // ── 速度 1–8 ──
        Row(
          children: <Widget>[
            Text('速度', style: AppText.caption),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Slider(
                key: const Key('gb-ptz-speed'),
                value: _speed.toDouble(),
                min: 1,
                max: 8,
                divisions: 7,
                label: '$_speed',
                onChanged: widget.enabled
                    ? (double value) {
                        final int next = value.round().clamp(1, 8);
                        if (next == _speed) return;
                        setState(() => _speed = next);
                        widget.onSpeedChanged?.call(next);
                      }
                    : null,
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            SizedBox(
              width: 20.dp,
              child: Text('$_speed', style: AppText.bodyStrong),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.sm),

        // ── 变焦：按住持续、松开发 stop ──
        Row(
          children: <Widget>[
            Expanded(
              child: _HoldButton(
                key: const Key('gb-ptz-zoom-out'),
                icon: Icons.zoom_out,
                label: '缩小',
                enabled: widget.enabled,
                onHold: () => _action(GbPtzAction.zoomOut),
                onRelease: () => _action(GbPtzAction.zoomStop),
              ),
            ),
            SizedBox(width: AppSpacing.md),
            Text('变焦', style: AppText.caption),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: _HoldButton(
                key: const Key('gb-ptz-zoom-in'),
                icon: Icons.zoom_in,
                label: '放大',
                enabled: widget.enabled,
                onHold: () => _action(GbPtzAction.zoomIn),
                onRelease: () => _action(GbPtzAction.zoomStop),
              ),
            ),
          ],
        ),
      ],
    );
  }

  /// 一行三个等宽按钮。
  Widget _buildDirectionRow(List<Widget> children) {
    return Row(
      children: <Widget>[
        for (int i = 0; i < children.length; i++) ...<Widget>[
          if (i > 0) SizedBox(width: AppSpacing.sm),
          Expanded(child: children[i]),
        ],
      ],
    );
  }
}

/// 按住下发 / 松开停止的按钮。
class _HoldButton extends StatefulWidget {
  const _HoldButton({
    super.key,
    required this.icon,
    required this.label,
    required this.onHold,
    required this.onRelease,
    required this.enabled,
  });

  final IconData icon;
  final String label;
  final VoidCallback onHold;
  final VoidCallback onRelease;
  final bool enabled;

  @override
  State<_HoldButton> createState() => _HoldButtonState();
}

class _HoldButtonState extends State<_HoldButton> {
  bool _holding = false;

  void _start(PointerDownEvent event) {
    if (!widget.enabled || _holding) return;
    setState(() => _holding = true);
    widget.onHold();
  }

  void _end(PointerEvent event) {
    if (!_holding) return;
    setState(() => _holding = false);
    widget.onRelease();
  }

  @override
  void dispose() {
    // 卸载兜底：按住过程中组件被销毁（切分屏 / 关页面）时补发停止，
    // 避免「面板已消失但镜头还在转」。这里不能再 setState。
    if (_holding) {
      _holding = false;
      widget.onRelease();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final Color background =
        _holding ? AppColors.primary : AppColors.fill;
    final Color foreground;
    if (_holding) {
      foreground = AppColors.textOnPrimary;
    } else if (widget.enabled) {
      foreground = AppColors.textSecondary;
    } else {
      foreground = AppColors.textDisabled;
    }

    return Listener(
      behavior: HitTestBehavior.opaque,
      onPointerDown: _start,
      onPointerUp: _end,
      onPointerCancel: _end,
      child: Tooltip(
        message: widget.label,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 100),
          height: 44.dp,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: background,
            borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
          ),
          child: Icon(widget.icon, size: 20.dp, color: foreground),
        ),
      ),
    );
  }
}

/// 中央「停止」按钮：点击单发一次 stop。
class _StopButton extends StatelessWidget {
  const _StopButton({
    super.key,
    required this.enabled,
    required this.onTap,
  });

  final bool enabled;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: enabled ? AppColors.primaryLight : AppColors.fill,
      borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
      child: InkWell(
        onTap: enabled ? onTap : null,
        borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
        child: SizedBox(
          height: 44.dp,
          child: Icon(
            Icons.stop_circle_outlined,
            size: 22.dp,
            color: enabled ? AppColors.primary : AppColors.textDisabled,
          ),
        ),
      ),
    );
  }
}
