import 'package:ai_camera_app/models/index.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:ai_camera_app/utils/SharedPreferenceUtil.dart';
import 'package:flutter_dva/flutter_dva.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'test_dio_adapter.dart';

/// 两级移动端菜单树：目录 + 菜单 + 按钮级权限。
List<dynamic> menuTreeJson() => <dynamic>[
      <String, dynamic>{
        'id': 1,
        'name': '风险中心',
        'parent_id': null,
        'level': 1,
        'path': '/risks',
        'icon': 'risk',
        'type': 'directory',
        'platform': 'mobile',
        'permission_code': 'risk:view',
        'hidden': false,
        'children': <dynamic>[
          <String, dynamic>{
            'id': 2,
            'name': '处理风险',
            'parent_id': 1,
            'level': 2,
            'path': '/risks/handle',
            'icon': 'warning',
            'type': 'button',
            'platform': 'mobile',
            'permission_code': 'risk:handle',
            'hidden': false,
            'children': <dynamic>[],
          },
        ],
      },
      <String, dynamic>{
        'id': 3,
        'name': '设备',
        'parent_id': null,
        'level': 1,
        'path': '/devices',
        'icon': 'camera',
        'type': 'menu',
        'platform': 'mobile',
        'permission_code': '',
        'hidden': false,
        'children': <dynamic>[],
      },
    ];

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late Dva dva;

  setUp(() async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    SharedPreferenceUtil.resetForTest();
    await SharedPreferenceUtil.init();
    ApiService.setToken('tk-menu');
    dva = await bootDva(<Model<dynamic>>[MenuModel()]);
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  group('MenuModel', () {
    test('加载成功：解析菜单树并扁平化权限标识', () async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{'/app/menus': wrapSuccess(menuTreeJson())},
      );
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'menu/load');

      final MenuState state = readState<MenuState>(dva, 'menu');

      expect(state.loading, isFalse);
      expect(state.error, isNull);
      expect(state.isLoaded, isTrue);

      // 顶层两个菜单，第一个带一个子节点。
      expect(state.menus, hasLength(2));
      expect(state.menus.first.name, '风险中心');
      expect(state.menus.first.children, hasLength(1));
      expect(state.menus.first.children.first.isButton, isTrue);

      // 权限标识已扁平化（含子节点）。
      expect(state.permissions, containsAll(<String>['risk:view', 'risk:handle']));
      expect(state.hasPerm('risk:handle'), isTrue);
      expect(state.hasPerm('risk:view'), isTrue);

      // 请求带上了 Bearer token。
      expect(adapter.lastRequest!.headers['Authorization'], 'Bearer tk-menu');
    });

    test('hasPerm：未授权的权限标识返回 false', () async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{'/app/menus': wrapSuccess(menuTreeJson())},
      );
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'menu/load');
      final MenuState state = readState<MenuState>(dva, 'menu');

      expect(state.hasPerm('risk:handle'), isTrue);
      expect(state.hasPerm('alarm:handle'), isFalse);
      expect(state.hasPerm(''), isFalse);
      // hasAnyPerm：任一命中即为 true。
      expect(state.hasAnyPerm(<String>['alarm:handle', 'risk:view']), isTrue);
      expect(state.hasAnyPerm(<String>['alarm:handle', 'device:edit']), isFalse);
    });

    test('未加载时 hasPerm 恒为 false', () {
      final MenuState state = readState<MenuState>(dva, 'menu');
      expect(state.isLoaded, isFalse);
      expect(state.hasPerm('risk:view'), isFalse);
    });

    test('加载失败：写入 error 且菜单为空', () async {
      final FixedAdapter adapter = FixedAdapter(
        wrapError(500, '菜单服务异常'),
      );
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'menu/load');

      final MenuState state = readState<MenuState>(dva, 'menu');
      expect(state.loading, isFalse);
      expect(state.error, '菜单服务异常');
      expect(state.menus, isEmpty);
      expect(state.permissions, isEmpty);
    });

    test('reset：清空已加载的菜单与权限', () async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{'/app/menus': wrapSuccess(menuTreeJson())},
      );
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'menu/load');
      expect(readState<MenuState>(dva, 'menu').isLoaded, isTrue);

      await dispatchAction(dva, 'menu/reset');

      final MenuState state = readState<MenuState>(dva, 'menu');
      expect(state.menus, isEmpty);
      expect(state.permissions, isEmpty);
      expect(state.hasPerm('risk:view'), isFalse);
    });
  });
}
