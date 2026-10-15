import 'package:flutter/material.dart';

import '../theme/app_colors.dart';
import '../theme/app_spacing.dart';
import '../theme/app_text.dart';
import '../utils/Extensions.dart';
import 'empty_state.dart';
import 'loading_indicator.dart';
import '../layout/Gaps.dart';

/// 分页取数函数：给定页码与每页条数，返回分页载荷。
typedef EntityPageFetcher<T> = Future<PageDataLike<T>> Function(
  int page,
  int pageSize,
);

/// 分页载荷的抽象。
///
/// 与生成代码中的 `PageData<T>` 结构一致；此处定义接口是为了让
/// [EntityListPage] 不直接依赖 generated 目录，便于单独测试。
abstract class PageDataLike<T> {
  /// 总条数。
  int get total;

  /// 当前页数据。
  List<T> get items;

  /// 是否还有下一页。
  bool get hasMore;
}

/// 把生成代码的 `PageData<T>` 适配为 [PageDataLike]。
class PageDataAdapter<T> implements PageDataLike<T> {
  /// 构造适配器。
  const PageDataAdapter({
    required this.total,
    required this.items,
    required this.hasMore,
  });

  @override
  final int total;

  @override
  final List<T> items;

  @override
  final bool hasMore;
}

/// 通用分页列表页。
///
/// 统一处理：首屏 loading、空态、失败重试、下拉刷新、上滑加载更多。
/// 各业务列表页（风险/告警/巡检/设备/通知）只需提供 [fetcher] 与 [itemBuilder]。
class EntityListPage<T extends Object> extends StatefulWidget {
  /// 构造分页列表页。
  const EntityListPage({
    super.key,
    required this.fetcher,
    required this.itemBuilder,
    this.title,
    this.header,
    this.emptyText = '暂无数据',
    this.emptyIcon = Icons.inbox_outlined,
    this.pageSize = 20,
    this.actions,
    this.separator,
    this.reloadToken,
    this.padding,
  });

  /// 分页取数。
  final EntityPageFetcher<T> fetcher;

  /// 列表项构造。
  final Widget Function(BuildContext context, T item, int index) itemBuilder;

  /// AppBar 标题；为 null 时不渲染 AppBar。
  final String? title;

  /// 列表顶部固定区域（筛选条）。
  final Widget? header;

  /// 空态文案。
  final String emptyText;

  /// 空态图标。
  final IconData emptyIcon;

  /// 每页条数。
  final int pageSize;

  /// AppBar 右侧操作。
  final List<Widget>? actions;

  /// 列表项分隔。
  final Widget? separator;

  /// 刷新令牌：值变化时重新加载（父页面切换筛选条件时传入）。
  final Object? reloadToken;

  /// 列表内边距。
  final EdgeInsetsGeometry? padding;

  @override
  State<EntityListPage<T>> createState() => _EntityListPageState<T>();
}

class _EntityListPageState<T extends Object>
    extends State<EntityListPage<T>> {
  final ScrollController _controller = ScrollController();

  final List<T> _items = <T>[];

  int _page = 1;
  int _total = 0;
  bool _loading = false;
  bool _firstLoaded = false;
  String? _error;

  bool get _hasMore => _items.length < _total;

  @override
  void initState() {
    super.initState();
    _controller.addListener(_onScroll);
    _loadFirstPage();
  }

  @override
  void didUpdateWidget(EntityListPage<T> oldWidget) {
    super.didUpdateWidget(oldWidget);
    // 筛选条件变化（fetcher 闭包捕获了新条件）时重置并重新加载。
    if (widget.reloadToken != oldWidget.reloadToken) {
      _loadFirstPage();
    }
  }

  @override
  void dispose() {
    _controller.removeListener(_onScroll);
    _controller.dispose();
    super.dispose();
  }

  /// 触底加载下一页。
  void _onScroll() {
    if (!_controller.hasClients) return;
    final double threshold =
        _controller.position.maxScrollExtent - 160;
    if (_controller.position.pixels >= threshold) {
      _loadMore();
    }
  }

  /// 重新加载第一页。
  Future<void> _loadFirstPage() async {
    setState(() {
      _page = 1;
      _total = 0;
      _error = null;
      _items.clear();
      _firstLoaded = false;
    });
    await _fetch();
  }

  /// 下拉刷新。
  Future<void> _refresh() async {
    _page = 1;
    _total = 0;
    _error = null;
    await _fetch();
  }

  /// 加载下一页（并发保护）。
  Future<void> _loadMore() async {
    if (_loading || !_hasMore) return;
    _page += 1;
    await _fetch();
  }

  /// 实际取数。
  Future<void> _fetch() async {
    if (_loading) return;
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final PageDataLike<T> result =
          await widget.fetcher(_page, widget.pageSize);
      if (!mounted) return;
      setState(() {
        _total = result.total;
        if (_page == 1) {
          _items
            ..clear()
            ..addAll(result.items);
        } else {
          _items.addAll(result.items);
        }
        _loading = false;
        _firstLoaded = true;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _firstLoaded = true;
        _error = '加载失败，请重试';
        // 加载下一页失败时回退页码，避免跳页丢数据。
        if (_page > 1) _page -= 1;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final Widget body = _buildBody();

    if (widget.title == null) {
      return body;
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(widget.title!),
        actions: widget.actions,
      ),
      body: body,
    );
  }

  Widget _buildBody() {
    final Widget? header = widget.header;

    // 首屏加载中。
    if (!_firstLoaded && _loading) {
      return Column(
        children: <Widget>[
          if (header != null) header,
          const Expanded(child: LoadingIndicator(text: '加载中...')),
        ],
      );
    }

    // 首屏加载失败且无数据。
    if (_firstLoaded && _items.isEmpty && _error != null) {
      return Column(
        children: <Widget>[
          if (header != null) header,
          Expanded(
            child: EmptyState.error(
              message: _error,
              onRetry: _loadFirstPage,
            ),
          ),
        ],
      );
    }

    // 空列表。
    if (_items.isEmpty) {
      return Column(
        children: <Widget>[
          if (header != null) header,
          Expanded(
            child: RefreshIndicator(
              onRefresh: _refresh,
              child: ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: <Widget>[
                  SizedBox(height: MediaQuery.of(context).size.height * 0.12),
                  EmptyState(
                    icon: widget.emptyIcon,
                    title: widget.emptyText,
                  ),
                ],
              ),
            ),
          ),
        ],
      );
    }

    return Column(
      children: <Widget>[
        if (header != null) header,
        Expanded(
          child: RefreshIndicator(
            onRefresh: _refresh,
            child: ListView.separated(
              controller: _controller,
              physics: const AlwaysScrollableScrollPhysics(),
              padding: widget.padding ??
                  EdgeInsets.symmetric(vertical: AppSpacing.sm),
              itemCount: _items.length + 1,
              separatorBuilder: (BuildContext context, int index) =>
                  widget.separator ?? const SizedBox.shrink(),
              itemBuilder: (BuildContext context, int index) {
                if (index == _items.length) {
                  return ListFooter(
                    loading: _loading,
                    hasMore: _hasMore,
                    error: _error,
                    onRetry: _loadMore,
                    emptyCount: _items.length,
                  );
                }
                return widget.itemBuilder(context, _items[index], index);
              },
            ),
          ),
        ),
      ],
    );
  }
}

/// 列表项卡片容器：统一圆角、留白与点击反馈。
class EntityCard extends StatelessWidget {
  /// 构造列表项卡片。
  const EntityCard({
    super.key,
    required this.child,
    this.onTap,
    this.margin,
  });

  /// 内容。
  final Widget child;

  /// 点击回调。
  final VoidCallback? onTap;

  /// 外边距；为 null 时使用默认值 `EdgeInsets.symmetric(horizontal: pagePadding, vertical: xs)`。
  ///
  /// 默认值不能写成实参默认值 —— 设计令牌依赖运行时屏幕尺寸，不是编译期常量。
  final EdgeInsetsGeometry? margin;

  @override
  Widget build(BuildContext context) {
    final EdgeInsetsGeometry effectiveMargin = margin ??
        EdgeInsets.symmetric(
          horizontal: AppSpacing.pagePadding,
          vertical: AppSpacing.xs,
        );

    return Padding(
      padding: effectiveMargin,
      child: Material(
        color: Theme.of(context).cardTheme.color,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.md),
            child: child,
          ),
        ),
      ),
    );
  }
}

/// 网络图片：统一占位、错误兜底与圆角。
///
/// 后端图片字段为相对路径，内部经 `absoluteImageUrl` 补全为绝对地址。
class RemoteImage extends StatelessWidget {
  /// 构造网络图片。
  const RemoteImage({
    super.key,
    required this.url,
    this.width,
    this.height,
    this.radius,
    this.fit = BoxFit.cover,
  });

  /// 图片地址（可为相对路径）。
  final String? url;

  /// 宽度。
  final double? width;

  /// 高度。
  final double? height;

  /// 圆角；为 null 时使用 [AppSpacing.radiusControl]。
  ///
  /// 默认值不能写成实参默认值 —— 设计令牌依赖运行时屏幕尺寸，不是编译期常量。
  final double? radius;

  /// 填充方式。
  final BoxFit fit;

  @override
  Widget build(BuildContext context) {
    final String full = absoluteImageUrl(url);
    final double effectiveRadius = radius ?? AppSpacing.radiusControl;
    final Widget placeholder = Container(
      width: width,
      height: height,
      color: AppColors.fill,
      alignment: Alignment.center,
      child: Icon(
        Icons.image_not_supported_outlined,
        size: 20.dp,
        color: AppColors.textDisabled,
      ),
    );

    return ClipRRect(
      borderRadius: BorderRadius.circular(effectiveRadius),
      child: full.isEmpty
          ? placeholder
          : Image.network(
              full,
              width: width,
              height: height,
              fit: fit,
              errorBuilder: (
                BuildContext context,
                Object error,
                StackTrace? stackTrace,
              ) =>
                  placeholder,
              loadingBuilder: (
                BuildContext context,
                Widget child,
                ImageChunkEvent? progress,
              ) {
                if (progress == null) return child;
                return Container(
                  width: width,
                  height: height,
                  color: AppColors.fill,
                  alignment: Alignment.center,
                  child: SizedBox(
                    width: 16.dp,
                    height: 16.dp,
                    child: const CircularProgressIndicator(strokeWidth: 2),
                  ),
                );
              },
            ),
    );
  }
}

/// 详情页信息行：左侧标签、右侧值。
class InfoRow extends StatelessWidget {
  /// 构造信息行。
  const InfoRow({
    super.key,
    required this.label,
    this.value = '',
    this.valueColor,
    this.valueWidget,
  });

  /// 标签。
  final String label;

  /// 值文本；提供了 [valueWidget] 时忽略。
  final String value;

  /// 值颜色。
  final Color? valueColor;

  /// 自定义值组件（优先于 [value]）。
  final Widget? valueWidget;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          SizedBox(
            width: 88.dp,
            child: Text(label, style: AppText.caption),
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: valueWidget ??
                Text(
                  value.isEmpty ? '--' : value,
                  style: AppText.body.copyWith(color: valueColor),
                ),
          ),
        ],
      ),
    );
  }
}
