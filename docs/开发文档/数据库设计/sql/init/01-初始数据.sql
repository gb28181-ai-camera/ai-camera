-- ================================================================
-- 01-初始数据 - AI Camera 智能监控平台 · v0.1.0
-- 生成时间基准：2026-09-21
--
-- 幂等性：全部语句使用 INSERT ... SELECT ... WHERE NOT EXISTS 形式，
--         可重复执行且不会产生重复数据。
--
-- 执行顺序（必须先建表，再灌数据）：
--   psql -f sql/ddl/01..08-*.sql
--   psql -f sql/init/01-初始数据.sql
--
-- 内容清单：
--   1. 部门（总公司）
--   2. 超级管理员用户 admin / 超级管理员角色 super_admin / 关联关系
--   3. 完整 PC 菜单树（含按钮权限）+ 移动端菜单树
--   4. ac_menu_api：契约文档路由总表中的全部鉴权端点（Casbin p 策略唯一来源）
--   5. ac_role_menu_rel：super_admin 的角色-菜单授权
--   6. casbin_rule：p 策略（super_admin × 全部接口）+ g 策略（admin → super_admin）
--   7. 内置告警类型 3 条 / 内置设备模式 3 条
--   8. 示例区域 3 条 / 示例风险类型 2 条 / 示例固件类型 2 条 / 示例系统参数
--
-- 注意：本文件不使用 $$ 美元引用与存储过程、且注释中不含分号，
--       可被 tests/testutil/initdb 直接按分号分句逐条执行。
-- ================================================================


-- ════════════════════════════════════════════════════════════════
-- 1. 系统基础：部门
-- ════════════════════════════════════════════════════════════════
INSERT INTO ac_dept (id, parent_id, level, name, code, path, sequence, active, created_by, created_dt, updated_by, updated_dt)
SELECT 1, NULL, 1, '总公司', 'ROOT', '/1/', 1, TRUE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_dept WHERE id = 1);


-- ════════════════════════════════════════════════════════════════
-- 2. 超级管理员用户与角色
--    账号 admin，明文密码 admin123，哈希为 bcrypt cost=10。
--    该哈希已用 golang.org/x/crypto/bcrypt（后端使用的库）
--    与 bcryptjs 双向校验通过（CompareHashAndPassword == nil）。
--    登录后请立即在「个人中心 → 修改密码」更换初始密码。
-- ════════════════════════════════════════════════════════════════
INSERT INTO ac_user (id, active, name, user_name, password_hash, email, phone, is_admin, dept_id, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, '系统管理员', 'admin', '$2a$10$KZABO4S46mqPVXqbZu3pN.wOEpES4QJE9faWabkgtFy6EaF7tKV2.', '', '', TRUE, 1, '内置超级管理员账号，初始密码 admin123', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_user WHERE user_name = 'admin');

INSERT INTO ac_role (id, active, sequence, name, code, description, data_permission, cross_dept_ids, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, 1, '超级管理员', 'super_admin', '拥有全部菜单与接口权限', 'all', '', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_role WHERE code = 'super_admin');

INSERT INTO ac_user_role_rel (id, user_id, role_id)
SELECT 1, 1, 1
WHERE NOT EXISTS (SELECT 1 FROM ac_user_role_rel WHERE user_id = 1 AND role_id = 1);


-- ════════════════════════════════════════════════════════════════
-- 3. 菜单树
--    type：directory 目录 / menu 菜单 / button 按钮
--    platform：pc 管理后台 / mobile 移动端
--    说明：菜单 id 采用层级编码（父 811 → 按钮 8111），便于人工维护。
-- ════════════════════════════════════════════════════════════════

INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1, NULL, 1, '首页', '/dashboard', 'HomeOutlined', 'dashboard/index', 'menu', 'pc', 'dashboard:view', '菜单', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 2, NULL, 1, '实时监控', '/monitor/realtime', 'VideoCameraOutlined', 'monitor/index', 'menu', 'pc', 'monitor:view', '菜单', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 2);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 3, NULL, 1, 'AI风险', '/risk/ai-risk', 'WarningOutlined', 'risk/index', 'menu', 'pc', 'risk:view', '菜单', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 3);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 4, NULL, 1, 'AI监控', '/risk/ai-monitor', 'RobotOutlined', 'ai-monitor/index', 'menu', 'pc', 'ai-monitor:view', '菜单', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 4);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 5, NULL, 1, '风险统计', '/risk-stats', 'AreaChartOutlined', '', 'directory', 'pc', 'risk-stats:view', '目录', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 5);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 6, NULL, 1, '巡检中心', '/inspect', 'SafetyCertificateOutlined', '', 'directory', 'pc', 'inspect:view', '目录', 6, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 6);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 7, NULL, 1, '统计分析', '/analysis', 'BarChartOutlined', '', 'directory', 'pc', 'analysis:view', '目录', 7, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 7);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8, NULL, 1, '系统设置', '/system', 'SettingOutlined', '', 'directory', 'pc', 'system:view', '目录', 8, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9, NULL, 1, '通用能力', '/common', 'AppstoreOutlined', '', 'directory', 'pc', 'common:view', '目录', 9, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 51, '5', 2, '风险监控', '/risk/monitor', 'WarningOutlined', 'risk-stats/risk-monitor', 'menu', 'pc', 'risk-monitor:view', '菜单', 51, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 51);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 52, '5', 2, '设备监控', '/device/monitor', 'DesktopOutlined', 'risk-stats/device-monitor', 'menu', 'pc', 'device-monitor:view', '菜单', 52, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 52);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 53, '5', 2, 'GIS查看', '/device/gis', 'GlobalOutlined', 'risk-stats/gis', 'menu', 'pc', 'gis:view', '菜单', 53, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 53);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 54, '5', 2, '设备历史记录', '/device/history', 'HistoryOutlined', 'risk-stats/device-history', 'menu', 'pc', 'device-history:view', '菜单', 54, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 54);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 55, '5', 2, '设备告警', '/alarm/list', 'BellOutlined', 'risk-stats/alarm', 'menu', 'pc', 'alarm:view', '菜单', 55, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 55);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 56, '5', 2, '物联网卡监控', '/device/iot-card-monitor', 'SimOutlined', 'risk-stats/iot-card-monitor', 'menu', 'pc', 'iot-card-monitor:view', '菜单', 56, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 56);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 61, '6', 2, '巡检任务管理', '/inspect/task', 'ScheduleOutlined', 'inspect/tasks', 'menu', 'pc', 'inspect-task:view', '菜单', 61, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 61);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 62, '6', 2, '巡检记录', '/inspect/record', 'FileTextOutlined', 'inspect/records', 'menu', 'pc', 'inspect-record:view', '菜单', 62, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 62);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 71, '7', 2, '风险统计分析', '/stats/risk', 'LineChartOutlined', 'analysis/risk', 'menu', 'pc', 'stats-risk:view', '菜单', 71, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 71);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 72, '7', 2, '设备告警分析', '/stats/alarm', 'PieChartOutlined', 'analysis/alarm', 'menu', 'pc', 'stats-alarm:view', '菜单', 72, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 72);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 73, '7', 2, '可视化大屏', '/screen', 'FundProjectionScreenOutlined', 'analysis/screen', 'menu', 'pc', 'screen:view', '菜单', 73, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 73);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 81, '8', 2, '资源管理', '/system/resource', 'ClusterOutlined', '', 'directory', 'pc', 'resource:view', '目录', 81, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 81);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 82, '8', 2, '告警设置', '/system/alarm-setting', 'AlertOutlined', '', 'directory', 'pc', 'alarm-setting-dir:view', '目录', 82, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 82);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 83, '8', 2, '升级管理', '/system/upgrade', 'CloudUploadOutlined', '', 'directory', 'pc', 'upgrade:view', '目录', 83, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 83);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 84, '8', 2, '基础信息管理', '/system/basic', 'ProfileOutlined', '', 'directory', 'pc', 'basic:view', '目录', 84, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 84);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 85, '8', 2, '第三方平台管理', '/system/thirdparty', 'ApiOutlined', '', 'directory', 'pc', 'thirdparty:view', '目录', 85, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 85);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 86, '8', 2, '系统管理', '/system/management', 'TeamOutlined', '', 'directory', 'pc', 'management:view', '目录', 86, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 86);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 811, '81', 3, '设备管理', '/settings/device', 'DesktopOutlined', 'system/resource/devices', 'menu', 'pc', 'device:view', '菜单', 811, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 811);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 812, '81', 3, '语音配置管理', '/settings/voice', 'SoundOutlined', 'system/resource/voice-configs', 'menu', 'pc', 'voice-config:view', '菜单', 812, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 812);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 813, '81', 3, '物联网卡管理', '/settings/iot-card', 'SimOutlined', 'system/resource/iot-cards', 'menu', 'pc', 'iot-card:view', '菜单', 813, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 813);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 814, '81', 3, '物联网卡更换记录', '/settings/iot-card-replace', 'SwapOutlined', 'system/resource/iot-card-replace-logs', 'menu', 'pc', 'iot-card-replace:view', '菜单', 814, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 814);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 815, '81', 3, '设备模式管理', '/settings/device-mode', 'ControlOutlined', 'system/resource/device-modes', 'menu', 'pc', 'device-mode:view', '菜单', 815, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 815);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 821, '82', 3, '设备告警设置', '/settings/alarm', 'BellOutlined', 'system/alarm-setting/alarm-settings', 'menu', 'pc', 'alarm-setting:view', '菜单', 821, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 821);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 822, '82', 3, '风险通知设置', '/settings/risk-notify', 'NotificationOutlined', 'system/alarm-setting/risk-notify-settings', 'menu', 'pc', 'risk-notify:view', '菜单', 822, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 822);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 831, '83', 3, '设备升级管理', '/settings/upgrade', 'CloudUploadOutlined', 'system/upgrade/devices', 'menu', 'pc', 'upgrade-device:view', '菜单', 831, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 831);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 832, '83', 3, '固件管理', '/settings/firmware', 'CodeOutlined', 'system/upgrade/firmwares', 'menu', 'pc', 'firmware:view', '菜单', 832, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 832);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 833, '83', 3, '模型管理', '/settings/model', 'ExperimentOutlined', 'system/upgrade/models', 'menu', 'pc', 'ai-model:view', '菜单', 833, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 833);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 834, '83', 3, '固件类型管理', '/settings/firmware-type', 'TagsOutlined', 'system/upgrade/firmware-types', 'menu', 'pc', 'firmware-type:view', '菜单', 834, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 834);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 841, '84', 3, '区域管理', '/basic/region', 'EnvironmentOutlined', 'system/basic/regions', 'menu', 'pc', 'region:view', '菜单', 841, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 841);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 842, '84', 3, '风险类型管理', '/basic/risk-type', 'WarningOutlined', 'system/basic/risk-types', 'menu', 'pc', 'risk-type:view', '菜单', 842, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 842);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 843, '84', 3, '告警类型管理', '/basic/alarm-type', 'BellOutlined', 'system/basic/alarm-types', 'menu', 'pc', 'alarm-type:view', '菜单', 843, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 843);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 851, '85', 3, '个推账号绑定', '/settings/getui', 'MobileOutlined', 'system/thirdparty/getui', 'menu', 'pc', 'getui:view', '菜单', 851, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 851);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 852, '85', 3, '腾讯云短信配置', '/settings/sms', 'MessageOutlined', 'system/thirdparty/sms', 'menu', 'pc', 'sms:view', '菜单', 852, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 852);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 861, '86', 3, '个人中心', '/profile', 'UserOutlined', 'system/management/profile', 'menu', 'pc', 'profile:view', '菜单', 861, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 861);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 862, '86', 3, '用户管理', '/system/user', 'UserAddOutlined', 'system/management/users', 'menu', 'pc', 'user:view', '菜单', 862, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 862);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 863, '86', 3, '菜单管理', '/system/menu', 'MenuOutlined', 'system/management/menus', 'menu', 'pc', 'menu:view', '菜单', 863, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 863);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 864, '86', 3, '角色管理', '/system/role', 'SafetyCertificateOutlined', 'system/management/roles', 'menu', 'pc', 'role:view', '菜单', 864, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 864);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 865, '86', 3, '部门管理', '/system/dept', 'ApartmentOutlined', 'system/management/depts', 'menu', 'pc', 'dept:view', '菜单', 865, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 865);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 866, '86', 3, '岗位管理', '/system/post', 'IdcardOutlined', 'system/management/posts', 'menu', 'pc', 'post:view', '菜单', 866, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 866);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 867, '86', 3, '操作日志', '/system/operation-log', 'FileSearchOutlined', 'system/management/logs', 'menu', 'pc', 'log:view', '菜单', 867, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 867);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 868, '86', 3, '系统通知', '/system/notification', 'NotificationOutlined', 'system/management/notifications', 'menu', 'pc', 'notification:view', '菜单', 868, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 868);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 869, '86', 3, '参数配置', '/system/config', 'ToolOutlined', 'system/management/configs', 'menu', 'pc', 'config:view', '菜单', 869, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 869);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 91, '9', 2, '文件上传', '', '', '', 'menu', 'pc', 'upload:file', '菜单', 91, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 91);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 92, '9', 2, '音频上传', '', '', '', 'menu', 'pc', 'upload:audio', '菜单', 92, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 92);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 511, '51', 4, '处理风险', '', '', '', 'button', 'pc', 'risk:handle', '按钮', 511, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 511);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 512, '51', 4, '检测准确性', '', '', '', 'button', 'pc', 'risk:accurate', '按钮', 512, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 512);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 513, '51', 4, '删除风险', '', '', '', 'button', 'pc', 'risk:delete', '按钮', 513, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 513);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 514, '51', 4, '导出风险', '', '', '', 'button', 'pc', 'risk:export', '按钮', 514, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 514);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 551, '55', 4, '处理告警', '', '', '', 'button', 'pc', 'alarm:handle', '按钮', 551, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 551);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 552, '55', 4, '删除告警', '', '', '', 'button', 'pc', 'alarm:delete', '按钮', 552, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 552);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 553, '55', 4, '导出告警', '', '', '', 'button', 'pc', 'alarm:export', '按钮', 553, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 553);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 561, '56', 4, '导出物联网卡监控', '', '', '', 'button', 'pc', 'iot-card-monitor:export', '按钮', 561, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 561);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 611, '61', 4, '新增巡检任务', '', '', '', 'button', 'pc', 'inspect-task:create', '按钮', 611, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 611);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 612, '61', 4, '编辑巡检任务', '', '', '', 'button', 'pc', 'inspect-task:update', '按钮', 612, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 612);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 613, '61', 4, '删除巡检任务', '', '', '', 'button', 'pc', 'inspect-task:delete', '按钮', 613, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 613);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 614, '61', 4, '导出巡检任务', '', '', '', 'button', 'pc', 'inspect-task:export', '按钮', 614, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 614);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 621, '62', 4, '导出巡检记录', '', '', '', 'button', 'pc', 'inspect-record:export', '按钮', 621, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 621);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 711, '71', 4, '导出风险统计', '', '', '', 'button', 'pc', 'stats-risk:export', '按钮', 711, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 711);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 721, '72', 4, '导出告警分析', '', '', '', 'button', 'pc', 'stats-alarm:export', '按钮', 721, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 721);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8111, '811', 4, '新增设备', '', '', '', 'button', 'pc', 'device:create', '按钮', 8111, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8111);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8112, '811', 4, '编辑设备', '', '', '', 'button', 'pc', 'device:update', '按钮', 8112, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8112);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8113, '811', 4, '删除设备', '', '', '', 'button', 'pc', 'device:delete', '按钮', 8113, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8113);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8114, '811', 4, '切换设备模式', '', '', '', 'button', 'pc', 'device:mode', '按钮', 8114, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8114);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8115, '811', 4, '同步设备时间', '', '', '', 'button', 'pc', 'device:sync-time', '按钮', 8115, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8115);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8116, '811', 4, '实时监控开关', '', '', '', 'button', 'pc', 'device:monitor', '按钮', 8116, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8116);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8117, '811', 4, '导出设备', '', '', '', 'button', 'pc', 'device:export', '按钮', 8117, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8117);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8121, '812', 4, '新增语音配置', '', '', '', 'button', 'pc', 'voice-config:create', '按钮', 8121, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8121);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8122, '812', 4, '编辑语音配置', '', '', '', 'button', 'pc', 'voice-config:update', '按钮', 8122, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8122);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8123, '812', 4, '删除语音配置', '', '', '', 'button', 'pc', 'voice-config:delete', '按钮', 8123, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8123);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8131, '813', 4, '新增物联网卡', '', '', '', 'button', 'pc', 'iot-card:create', '按钮', 8131, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8131);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8132, '813', 4, '编辑物联网卡', '', '', '', 'button', 'pc', 'iot-card:update', '按钮', 8132, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8132);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8133, '813', 4, '删除物联网卡', '', '', '', 'button', 'pc', 'iot-card:delete', '按钮', 8133, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8133);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8134, '813', 4, '导出物联网卡', '', '', '', 'button', 'pc', 'iot-card:export', '按钮', 8134, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8134);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8141, '814', 4, '更换物联网卡', '', '', '', 'button', 'pc', 'iot-card-replace:create', '按钮', 8141, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8141);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8142, '814', 4, '导出更换记录', '', '', '', 'button', 'pc', 'iot-card-replace:export', '按钮', 8142, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8142);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8151, '815', 4, '新增设备模式', '', '', '', 'button', 'pc', 'device-mode:create', '按钮', 8151, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8151);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8152, '815', 4, '编辑设备模式', '', '', '', 'button', 'pc', 'device-mode:update', '按钮', 8152, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8152);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8153, '815', 4, '删除设备模式', '', '', '', 'button', 'pc', 'device-mode:delete', '按钮', 8153, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8153);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8211, '821', 4, '新增告警设置', '', '', '', 'button', 'pc', 'alarm-setting:create', '按钮', 8211, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8211);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8212, '821', 4, '编辑告警设置', '', '', '', 'button', 'pc', 'alarm-setting:update', '按钮', 8212, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8212);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8213, '821', 4, '删除告警设置', '', '', '', 'button', 'pc', 'alarm-setting:delete', '按钮', 8213, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8213);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8214, '821', 4, '关闭全部告警通知', '', '', '', 'button', 'pc', 'alarm-setting:disable-all', '按钮', 8214, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8214);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8221, '822', 4, '新增风险通知', '', '', '', 'button', 'pc', 'risk-notify:create', '按钮', 8221, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8221);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8222, '822', 4, '编辑风险通知', '', '', '', 'button', 'pc', 'risk-notify:update', '按钮', 8222, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8222);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8223, '822', 4, '删除风险通知', '', '', '', 'button', 'pc', 'risk-notify:delete', '按钮', 8223, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8223);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8224, '822', 4, '关闭全部风险通知', '', '', '', 'button', 'pc', 'risk-notify:disable-all', '按钮', 8224, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8224);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8311, '831', 4, '下发升级', '', '', '', 'button', 'pc', 'upgrade-device:create', '按钮', 8311, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8311);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8312, '831', 4, '取消升级计划', '', '', '', 'button', 'pc', 'upgrade-device:cancel', '按钮', 8312, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8312);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8313, '831', 4, '导出升级记录', '', '', '', 'button', 'pc', 'upgrade-device:export', '按钮', 8313, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8313);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8321, '832', 4, '新增固件', '', '', '', 'button', 'pc', 'firmware:create', '按钮', 8321, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8321);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8322, '832', 4, '编辑固件', '', '', '', 'button', 'pc', 'firmware:update', '按钮', 8322, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8322);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8323, '832', 4, '删除固件', '', '', '', 'button', 'pc', 'firmware:delete', '按钮', 8323, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8323);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8331, '833', 4, '新增模型', '', '', '', 'button', 'pc', 'ai-model:create', '按钮', 8331, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8331);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8332, '833', 4, '编辑模型', '', '', '', 'button', 'pc', 'ai-model:update', '按钮', 8332, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8332);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8333, '833', 4, '删除模型', '', '', '', 'button', 'pc', 'ai-model:delete', '按钮', 8333, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8333);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8341, '834', 4, '新增固件类型', '', '', '', 'button', 'pc', 'firmware-type:create', '按钮', 8341, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8341);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8342, '834', 4, '编辑固件类型', '', '', '', 'button', 'pc', 'firmware-type:update', '按钮', 8342, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8342);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8343, '834', 4, '删除固件类型', '', '', '', 'button', 'pc', 'firmware-type:delete', '按钮', 8343, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8343);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8411, '841', 4, '新增区域', '', '', '', 'button', 'pc', 'region:create', '按钮', 8411, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8411);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8412, '841', 4, '编辑区域', '', '', '', 'button', 'pc', 'region:update', '按钮', 8412, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8412);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8413, '841', 4, '删除区域', '', '', '', 'button', 'pc', 'region:delete', '按钮', 8413, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8413);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8414, '841', 4, '导出区域', '', '', '', 'button', 'pc', 'region:export', '按钮', 8414, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8414);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8421, '842', 4, '新增风险类型', '', '', '', 'button', 'pc', 'risk-type:create', '按钮', 8421, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8421);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8422, '842', 4, '编辑风险类型', '', '', '', 'button', 'pc', 'risk-type:update', '按钮', 8422, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8422);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8423, '842', 4, '删除风险类型', '', '', '', 'button', 'pc', 'risk-type:delete', '按钮', 8423, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8423);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8431, '843', 4, '新增告警类型', '', '', '', 'button', 'pc', 'alarm-type:create', '按钮', 8431, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8431);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8432, '843', 4, '编辑告警类型', '', '', '', 'button', 'pc', 'alarm-type:update', '按钮', 8432, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8432);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8433, '843', 4, '删除告警类型', '', '', '', 'button', 'pc', 'alarm-type:delete', '按钮', 8433, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8433);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8511, '851', 4, '修改个推配置', '', '', '', 'button', 'pc', 'getui:update', '按钮', 8511, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8511);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8521, '852', 4, '修改短信配置', '', '', '', 'button', 'pc', 'sms:update', '按钮', 8521, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8521);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8611, '861', 4, '修改个人资料', '', '', '', 'button', 'pc', 'profile:update', '按钮', 8611, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8611);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8612, '861', 4, '修改密码', '', '', '', 'button', 'pc', 'profile:password', '按钮', 8612, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8612);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8621, '862', 4, '新增用户', '', '', '', 'button', 'pc', 'user:create', '按钮', 8621, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8621);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8622, '862', 4, '编辑用户', '', '', '', 'button', 'pc', 'user:update', '按钮', 8622, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8622);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8623, '862', 4, '删除用户', '', '', '', 'button', 'pc', 'user:delete', '按钮', 8623, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8623);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8624, '862', 4, '重置密码', '', '', '', 'button', 'pc', 'user:reset-password', '按钮', 8624, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8624);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8625, '862', 4, '启用停用用户', '', '', '', 'button', 'pc', 'user:enable', '按钮', 8625, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8625);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8626, '862', 4, '导出用户', '', '', '', 'button', 'pc', 'user:export', '按钮', 8626, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8626);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8631, '863', 4, '新增菜单', '', '', '', 'button', 'pc', 'menu:create', '按钮', 8631, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8631);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8632, '863', 4, '编辑菜单', '', '', '', 'button', 'pc', 'menu:update', '按钮', 8632, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8632);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8633, '863', 4, '删除菜单', '', '', '', 'button', 'pc', 'menu:delete', '按钮', 8633, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8633);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8634, '863', 4, '菜单接口绑定', '', '', '', 'button', 'pc', 'menu:api', '按钮', 8634, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8634);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8641, '864', 4, '新增角色', '', '', '', 'button', 'pc', 'role:create', '按钮', 8641, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8641);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8642, '864', 4, '编辑角色', '', '', '', 'button', 'pc', 'role:update', '按钮', 8642, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8642);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8643, '864', 4, '删除角色', '', '', '', 'button', 'pc', 'role:delete', '按钮', 8643, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8643);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8644, '864', 4, '分配权限', '', '', '', 'button', 'pc', 'role:assign', '按钮', 8644, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8644);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8651, '865', 4, '新增部门', '', '', '', 'button', 'pc', 'dept:create', '按钮', 8651, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8651);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8652, '865', 4, '编辑部门', '', '', '', 'button', 'pc', 'dept:update', '按钮', 8652, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8652);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8653, '865', 4, '删除部门', '', '', '', 'button', 'pc', 'dept:delete', '按钮', 8653, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8653);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8661, '866', 4, '新增岗位', '', '', '', 'button', 'pc', 'post:create', '按钮', 8661, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8661);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8662, '866', 4, '编辑岗位', '', '', '', 'button', 'pc', 'post:update', '按钮', 8662, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8662);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8663, '866', 4, '删除岗位', '', '', '', 'button', 'pc', 'post:delete', '按钮', 8663, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8663);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8671, '867', 4, '导出日志', '', '', '', 'button', 'pc', 'log:export', '按钮', 8671, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8671);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8672, '867', 4, '删除日志', '', '', '', 'button', 'pc', 'log:delete', '按钮', 8672, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8672);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8673, '867', 4, '清空日志', '', '', '', 'button', 'pc', 'log:clear', '按钮', 8673, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8673);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8681, '868', 4, '标记已读', '', '', '', 'button', 'pc', 'notification:read', '按钮', 8681, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8681);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8682, '868', 4, '删除通知', '', '', '', 'button', 'pc', 'notification:delete', '按钮', 8682, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8682);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8691, '869', 4, '新增参数', '', '', '', 'button', 'pc', 'config:create', '按钮', 8691, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8691);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8692, '869', 4, '编辑参数', '', '', '', 'button', 'pc', 'config:update', '按钮', 8692, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8692);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8693, '869', 4, '删除参数', '', '', '', 'button', 'pc', 'config:delete', '按钮', 8693, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8693);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8694, '869', 4, '启用停用参数', '', '', '', 'button', 'pc', 'config:enable', '按钮', 8694, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8694);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 101, NULL, 1, '首页', '/m/home', 'home', 'pages/home/index', 'menu', 'mobile', 'app:home', '菜单', 101, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 101);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 102, NULL, 1, '风险', '/m/risks', 'warning', 'pages/risk/index', 'menu', 'mobile', 'app:risk', '菜单', 102, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 102);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 103, NULL, 1, '告警', '/m/alarms', 'bell', 'pages/alarm/index', 'menu', 'mobile', 'app:alarm', '菜单', 103, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 103);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 104, NULL, 1, '巡检', '/m/inspect', 'clipboard', 'pages/inspect/index', 'menu', 'mobile', 'app:inspect', '菜单', 104, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 104);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 105, NULL, 1, '设备', '/m/devices', 'camera', 'pages/device/index', 'menu', 'mobile', 'app:device', '菜单', 105, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 105);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 106, NULL, 1, '通知', '/m/notifications', 'message', 'pages/notification/index', 'menu', 'mobile', 'app:notice', '菜单', 106, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 106);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 107, NULL, 1, '我的', '/m/profile', 'user', 'pages/profile/index', 'menu', 'mobile', 'app:profile', '菜单', 107, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 107);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 108, NULL, 1, '国标监控', '/m/gb', 'video-camera', 'pages/gb/index', 'menu', 'mobile', 'app:gb', '菜单', 108, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 108);

-- 通用能力目录及上传菜单在侧边栏隐藏（仅作权限载体）
UPDATE ac_menu SET hidden = TRUE WHERE id IN (9, 91, 92);


-- ════════════════════════════════════════════════════════════════
-- 4. 菜单-接口绑定（ac_menu_api）
--    本表是 Casbin p 策略的唯一来源，必须覆盖契约文档路由总表中
--    全部需要鉴权的端点。以下端点不在此表内（不经 Casbin 校验）：
--      POST /web/auth/login          公开接口（登录）
--      POST /app/auth/login          公开接口（移动端登录）
--      GET  /health                  公开接口（健康检查）
--      POST /web/risks/report        设备回调（ApiKeyAuth 分组）
--      POST /web/alarms/report       设备回调（ApiKeyAuth 分组）
--      POST /web/devices/:id/heartbeat 设备心跳（ApiKeyAuth 分组）
--    同一接口可绑定到多个菜单（如实时监控与设备管理都要读设备列表），
--    Casbin p 策略按 (角色, 路径, 方法) 唯一，重复绑定不会放大权限。
-- ════════════════════════════════════════════════════════════════

INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 1, 861, 'GET', '/web/profile', '获取个人信息', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 861 AND method = 'GET' AND path = '/web/profile');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 2, 8611, 'PUT', '/web/profile', '修改个人信息', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8611 AND method = 'PUT' AND path = '/web/profile');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 3, 8612, 'PUT', '/web/profile/password', '修改密码', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8612 AND method = 'PUT' AND path = '/web/profile/password');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 4, 861, 'GET', '/web/profile/menus', '获取当前用户菜单树', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 861 AND method = 'GET' AND path = '/web/profile/menus');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 5, 861, 'GET', '/web/profile/api-keys', '查询API密钥列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 861 AND method = 'GET' AND path = '/web/profile/api-keys');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 6, 861, 'POST', '/web/profile/api-keys', '生成API密钥', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 861 AND method = 'POST' AND path = '/web/profile/api-keys');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 7, 861, 'DELETE', '/web/profile/api-keys/:id', '删除API密钥', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 861 AND method = 'DELETE' AND path = '/web/profile/api-keys/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 8, 862, 'GET', '/web/system/users', '分页查询用户', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 862 AND method = 'GET' AND path = '/web/system/users');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 9, 862, 'GET', '/web/system/users/options', '用户下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 862 AND method = 'GET' AND path = '/web/system/users/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 10, 862, 'GET', '/web/system/users/:id', '查询用户详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 862 AND method = 'GET' AND path = '/web/system/users/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 11, 8621, 'POST', '/web/system/users', '新增用户', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8621 AND method = 'POST' AND path = '/web/system/users');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 12, 8622, 'PUT', '/web/system/users/:id', '编辑用户', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8622 AND method = 'PUT' AND path = '/web/system/users/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 13, 8623, 'DELETE', '/web/system/users/:id', '删除用户', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8623 AND method = 'DELETE' AND path = '/web/system/users/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 14, 8625, 'PUT', '/web/system/users/:id/enable', '启用用户', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8625 AND method = 'PUT' AND path = '/web/system/users/:id/enable');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 15, 8625, 'PUT', '/web/system/users/:id/disable', '停用用户', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8625 AND method = 'PUT' AND path = '/web/system/users/:id/disable');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 16, 8624, 'PUT', '/web/system/users/:id/reset-password', '重置用户密码', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8624 AND method = 'PUT' AND path = '/web/system/users/:id/reset-password');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 17, 864, 'GET', '/web/system/roles', '分页查询角色', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 864 AND method = 'GET' AND path = '/web/system/roles');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 18, 864, 'GET', '/web/system/roles/options', '角色下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 864 AND method = 'GET' AND path = '/web/system/roles/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 19, 864, 'GET', '/web/system/roles/:id', '查询角色详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 864 AND method = 'GET' AND path = '/web/system/roles/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 20, 8641, 'POST', '/web/system/roles', '新增角色', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8641 AND method = 'POST' AND path = '/web/system/roles');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 21, 8642, 'PUT', '/web/system/roles/:id', '编辑角色', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8642 AND method = 'PUT' AND path = '/web/system/roles/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 22, 8643, 'DELETE', '/web/system/roles/:id', '删除角色', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8643 AND method = 'DELETE' AND path = '/web/system/roles/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 23, 8644, 'GET', '/web/system/roles/:id/menus', '查询角色已授权菜单', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8644 AND method = 'GET' AND path = '/web/system/roles/:id/menus');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 24, 8644, 'PUT', '/web/system/roles/:id/menus', '分配角色菜单', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8644 AND method = 'PUT' AND path = '/web/system/roles/:id/menus');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 25, 8644, 'GET', '/web/system/roles/:id/depts', '查询角色已关联部门', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8644 AND method = 'GET' AND path = '/web/system/roles/:id/depts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 26, 8644, 'PUT', '/web/system/roles/:id/depts', '分配角色部门', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8644 AND method = 'PUT' AND path = '/web/system/roles/:id/depts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 27, 8644, 'GET', '/web/system/roles/:id/posts', '查询角色已关联岗位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8644 AND method = 'GET' AND path = '/web/system/roles/:id/posts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 28, 8644, 'PUT', '/web/system/roles/:id/posts', '分配角色岗位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8644 AND method = 'PUT' AND path = '/web/system/roles/:id/posts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 29, 8644, 'GET', '/web/system/roles/:id/users', '查询角色已关联用户', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8644 AND method = 'GET' AND path = '/web/system/roles/:id/users');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 30, 8644, 'PUT', '/web/system/roles/:id/users', '分配角色用户', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8644 AND method = 'PUT' AND path = '/web/system/roles/:id/users');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 31, 863, 'GET', '/web/system/menus', '查询菜单树', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 863 AND method = 'GET' AND path = '/web/system/menus');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 32, 863, 'GET', '/web/system/menus/:id', '查询菜单详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 863 AND method = 'GET' AND path = '/web/system/menus/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 33, 8631, 'POST', '/web/system/menus', '新增菜单', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8631 AND method = 'POST' AND path = '/web/system/menus');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 34, 8632, 'PUT', '/web/system/menus/:id', '编辑菜单', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8632 AND method = 'PUT' AND path = '/web/system/menus/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 35, 8633, 'DELETE', '/web/system/menus/:id', '删除菜单', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8633 AND method = 'DELETE' AND path = '/web/system/menus/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 36, 8634, 'GET', '/web/system/menus/:id/apis', '查询菜单绑定接口', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8634 AND method = 'GET' AND path = '/web/system/menus/:id/apis');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 37, 8634, 'POST', '/web/system/menus/:id/apis', '新增菜单接口绑定', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8634 AND method = 'POST' AND path = '/web/system/menus/:id/apis');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 38, 8634, 'DELETE', '/web/system/menus/apis/:apiId', '删除菜单接口绑定', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8634 AND method = 'DELETE' AND path = '/web/system/menus/apis/:apiId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 39, 865, 'GET', '/web/system/depts', '查询部门树', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 865 AND method = 'GET' AND path = '/web/system/depts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 40, 865, 'GET', '/web/system/depts/options', '部门下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 865 AND method = 'GET' AND path = '/web/system/depts/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 41, 865, 'GET', '/web/system/depts/:id', '查询部门详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 865 AND method = 'GET' AND path = '/web/system/depts/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 42, 8651, 'POST', '/web/system/depts', '新增部门', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8651 AND method = 'POST' AND path = '/web/system/depts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 43, 8652, 'PUT', '/web/system/depts/:id', '编辑部门', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8652 AND method = 'PUT' AND path = '/web/system/depts/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 44, 8653, 'DELETE', '/web/system/depts/:id', '删除部门', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8653 AND method = 'DELETE' AND path = '/web/system/depts/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 45, 866, 'GET', '/web/system/posts', '分页查询岗位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 866 AND method = 'GET' AND path = '/web/system/posts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 46, 866, 'GET', '/web/system/posts/options', '岗位下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 866 AND method = 'GET' AND path = '/web/system/posts/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 47, 866, 'GET', '/web/system/posts/:id', '查询岗位详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 866 AND method = 'GET' AND path = '/web/system/posts/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 48, 8661, 'POST', '/web/system/posts', '新增岗位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8661 AND method = 'POST' AND path = '/web/system/posts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 49, 8662, 'PUT', '/web/system/posts/:id', '编辑岗位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8662 AND method = 'PUT' AND path = '/web/system/posts/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 50, 8663, 'DELETE', '/web/system/posts/:id', '删除岗位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8663 AND method = 'DELETE' AND path = '/web/system/posts/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 51, 869, 'GET', '/web/system/configs', '查询参数列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 869 AND method = 'GET' AND path = '/web/system/configs');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 52, 8691, 'POST', '/web/system/configs', '新增参数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8691 AND method = 'POST' AND path = '/web/system/configs');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 53, 8692, 'PUT', '/web/system/configs/:id', '编辑参数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8692 AND method = 'PUT' AND path = '/web/system/configs/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 54, 8693, 'DELETE', '/web/system/configs/:id', '删除参数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8693 AND method = 'DELETE' AND path = '/web/system/configs/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 55, 8694, 'PUT', '/web/system/configs/:id/enable', '启用参数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8694 AND method = 'PUT' AND path = '/web/system/configs/:id/enable');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 56, 8694, 'PUT', '/web/system/configs/:id/disable', '停用参数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8694 AND method = 'PUT' AND path = '/web/system/configs/:id/disable');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 57, 867, 'GET', '/web/system/logs', '分页查询操作日志', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 867 AND method = 'GET' AND path = '/web/system/logs');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 58, 867, 'GET', '/web/system/logs/:id', '查询操作日志详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 867 AND method = 'GET' AND path = '/web/system/logs/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 59, 8672, 'DELETE', '/web/system/logs/:id', '删除操作日志', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8672 AND method = 'DELETE' AND path = '/web/system/logs/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 60, 8673, 'DELETE', '/web/system/logs/clear', '清空操作日志', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8673 AND method = 'DELETE' AND path = '/web/system/logs/clear');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 61, 8671, 'GET', '/web/system/logs/export', '导出操作日志', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8671 AND method = 'GET' AND path = '/web/system/logs/export');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 62, 868, 'GET', '/web/system/notifications', '分页查询系统通知', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 868 AND method = 'GET' AND path = '/web/system/notifications');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 63, 868, 'GET', '/web/system/notifications/unread-count', '查询未读通知数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 868 AND method = 'GET' AND path = '/web/system/notifications/unread-count');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 64, 8681, 'PUT', '/web/system/notifications/:id/read', '标记通知已读', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8681 AND method = 'PUT' AND path = '/web/system/notifications/:id/read');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 65, 8681, 'PUT', '/web/system/notifications/read-all', '全部标记已读', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8681 AND method = 'PUT' AND path = '/web/system/notifications/read-all');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 66, 8682, 'DELETE', '/web/system/notifications/:id', '删除通知', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8682 AND method = 'DELETE' AND path = '/web/system/notifications/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 67, 841, 'GET', '/web/basic/regions', '分页查询区域', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 841 AND method = 'GET' AND path = '/web/basic/regions');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 68, 841, 'GET', '/web/basic/regions/options', '区域下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 841 AND method = 'GET' AND path = '/web/basic/regions/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 69, 841, 'GET', '/web/basic/regions/:id', '查询区域详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 841 AND method = 'GET' AND path = '/web/basic/regions/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 70, 8411, 'POST', '/web/basic/regions', '新增区域', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8411 AND method = 'POST' AND path = '/web/basic/regions');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 71, 8412, 'PUT', '/web/basic/regions/:id', '编辑区域', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8412 AND method = 'PUT' AND path = '/web/basic/regions/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 72, 8413, 'DELETE', '/web/basic/regions/:id', '删除区域', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8413 AND method = 'DELETE' AND path = '/web/basic/regions/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 73, 842, 'GET', '/web/basic/risk-types', '分页查询风险类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 842 AND method = 'GET' AND path = '/web/basic/risk-types');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 74, 842, 'GET', '/web/basic/risk-types/options', '风险类型下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 842 AND method = 'GET' AND path = '/web/basic/risk-types/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 75, 842, 'GET', '/web/basic/risk-types/:id', '查询风险类型详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 842 AND method = 'GET' AND path = '/web/basic/risk-types/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 76, 8421, 'POST', '/web/basic/risk-types', '新增风险类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8421 AND method = 'POST' AND path = '/web/basic/risk-types');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 77, 8422, 'PUT', '/web/basic/risk-types/:id', '编辑风险类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8422 AND method = 'PUT' AND path = '/web/basic/risk-types/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 78, 8423, 'DELETE', '/web/basic/risk-types/:id', '删除风险类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8423 AND method = 'DELETE' AND path = '/web/basic/risk-types/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 79, 843, 'GET', '/web/basic/alarm-types', '分页查询告警类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 843 AND method = 'GET' AND path = '/web/basic/alarm-types');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 80, 843, 'GET', '/web/basic/alarm-types/options', '告警类型下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 843 AND method = 'GET' AND path = '/web/basic/alarm-types/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 81, 843, 'GET', '/web/basic/alarm-types/:id', '查询告警类型详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 843 AND method = 'GET' AND path = '/web/basic/alarm-types/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 82, 8431, 'POST', '/web/basic/alarm-types', '新增告警类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8431 AND method = 'POST' AND path = '/web/basic/alarm-types');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 83, 8432, 'PUT', '/web/basic/alarm-types/:id', '编辑告警类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8432 AND method = 'PUT' AND path = '/web/basic/alarm-types/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 84, 8433, 'DELETE', '/web/basic/alarm-types/:id', '删除告警类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8433 AND method = 'DELETE' AND path = '/web/basic/alarm-types/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 85, 811, 'GET', '/web/devices', '分页查询设备', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 811 AND method = 'GET' AND path = '/web/devices');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 86, 811, 'GET', '/web/devices/options', '设备下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 811 AND method = 'GET' AND path = '/web/devices/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 87, 811, 'GET', '/web/devices/:id', '查询设备详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 811 AND method = 'GET' AND path = '/web/devices/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 88, 8111, 'POST', '/web/devices', '新增设备', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8111 AND method = 'POST' AND path = '/web/devices');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 89, 8112, 'PUT', '/web/devices/:id', '编辑设备', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8112 AND method = 'PUT' AND path = '/web/devices/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 90, 8113, 'DELETE', '/web/devices/:id', '删除设备', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8113 AND method = 'DELETE' AND path = '/web/devices/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 91, 8114, 'PUT', '/web/devices/:id/mode', '切换设备模式', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8114 AND method = 'PUT' AND path = '/web/devices/:id/mode');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 92, 8115, 'PUT', '/web/devices/:id/sync-time', '同步设备时间', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8115 AND method = 'PUT' AND path = '/web/devices/:id/sync-time');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 93, 8116, 'PUT', '/web/devices/:id/monitor', '开关实时监控', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8116 AND method = 'PUT' AND path = '/web/devices/:id/monitor');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 94, 54, 'GET', '/web/devices/history', '分页查询设备历史记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 54 AND method = 'GET' AND path = '/web/devices/history');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 95, 54, 'GET', '/web/devices/:id/history-chart', '设备历史曲线', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 54 AND method = 'GET' AND path = '/web/devices/:id/history-chart');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 96, 815, 'GET', '/web/devices/modes', '分页查询设备模式', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 815 AND method = 'GET' AND path = '/web/devices/modes');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 97, 815, 'GET', '/web/devices/modes/options', '设备模式下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 815 AND method = 'GET' AND path = '/web/devices/modes/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 98, 8151, 'POST', '/web/devices/modes', '新增设备模式', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8151 AND method = 'POST' AND path = '/web/devices/modes');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 99, 8152, 'PUT', '/web/devices/modes/:id', '编辑设备模式', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8152 AND method = 'PUT' AND path = '/web/devices/modes/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 100, 8153, 'DELETE', '/web/devices/modes/:id', '删除设备模式', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8153 AND method = 'DELETE' AND path = '/web/devices/modes/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 101, 813, 'GET', '/web/devices/iot-cards', '分页查询物联网卡', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 813 AND method = 'GET' AND path = '/web/devices/iot-cards');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 102, 813, 'GET', '/web/devices/iot-cards/options', '物联网卡下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 813 AND method = 'GET' AND path = '/web/devices/iot-cards/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 103, 8131, 'POST', '/web/devices/iot-cards', '新增物联网卡', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8131 AND method = 'POST' AND path = '/web/devices/iot-cards');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 104, 8132, 'PUT', '/web/devices/iot-cards/:id', '编辑物联网卡', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8132 AND method = 'PUT' AND path = '/web/devices/iot-cards/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 105, 8133, 'DELETE', '/web/devices/iot-cards/:id', '删除物联网卡', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8133 AND method = 'DELETE' AND path = '/web/devices/iot-cards/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 106, 814, 'GET', '/web/devices/iot-card-replace-logs', '分页查询物联网卡更换记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 814 AND method = 'GET' AND path = '/web/devices/iot-card-replace-logs');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 107, 8141, 'POST', '/web/devices/iot-card-replace-logs', '更换物联网卡', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8141 AND method = 'POST' AND path = '/web/devices/iot-card-replace-logs');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 108, 812, 'GET', '/web/devices/voice-configs', '分页查询语音配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 812 AND method = 'GET' AND path = '/web/devices/voice-configs');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 109, 8121, 'POST', '/web/devices/voice-configs', '新增语音配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8121 AND method = 'POST' AND path = '/web/devices/voice-configs');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 110, 8122, 'PUT', '/web/devices/voice-configs/:id', '编辑语音配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8122 AND method = 'PUT' AND path = '/web/devices/voice-configs/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 111, 8123, 'DELETE', '/web/devices/voice-configs/:id', '删除语音配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8123 AND method = 'DELETE' AND path = '/web/devices/voice-configs/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 112, 3, 'GET', '/web/risks', 'AI风险列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 3 AND method = 'GET' AND path = '/web/risks');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 113, 3, 'GET', '/web/risks/pending-count', 'AI风险待处理数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 3 AND method = 'GET' AND path = '/web/risks/pending-count');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 114, 3, 'GET', '/web/risks/:id', 'AI风险详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 3 AND method = 'GET' AND path = '/web/risks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 115, 51, 'GET', '/web/risks', '分页查询风险记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 51 AND method = 'GET' AND path = '/web/risks');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 116, 51, 'GET', '/web/risks/pending-count', '风险待处理数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 51 AND method = 'GET' AND path = '/web/risks/pending-count');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 117, 51, 'GET', '/web/risks/:id', '查询风险详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 51 AND method = 'GET' AND path = '/web/risks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 118, 51, 'PUT', '/web/risks/:id/read', '标记风险消息已读', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 51 AND method = 'PUT' AND path = '/web/risks/:id/read');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 119, 512, 'PUT', '/web/risks/:id/accurate', '确认检测准确性', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 512 AND method = 'PUT' AND path = '/web/risks/:id/accurate');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 120, 511, 'PUT', '/web/risks/:id/handle', '处理风险', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 511 AND method = 'PUT' AND path = '/web/risks/:id/handle');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 121, 513, 'DELETE', '/web/risks/:id', '删除风险记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 513 AND method = 'DELETE' AND path = '/web/risks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 122, 55, 'GET', '/web/alarms', '分页查询设备告警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 55 AND method = 'GET' AND path = '/web/alarms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 123, 55, 'GET', '/web/alarms/pending-count', '告警待处理数', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 55 AND method = 'GET' AND path = '/web/alarms/pending-count');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 124, 55, 'GET', '/web/alarms/:id', '查询告警详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 55 AND method = 'GET' AND path = '/web/alarms/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 125, 55, 'PUT', '/web/alarms/:id/read', '标记告警消息已读', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 55 AND method = 'PUT' AND path = '/web/alarms/:id/read');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 126, 551, 'PUT', '/web/alarms/:id/handle', '处理告警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 551 AND method = 'PUT' AND path = '/web/alarms/:id/handle');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 127, 552, 'DELETE', '/web/alarms/:id', '删除告警记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 552 AND method = 'DELETE' AND path = '/web/alarms/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 128, 61, 'GET', '/web/inspects/tasks', '分页查询巡检任务', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 61 AND method = 'GET' AND path = '/web/inspects/tasks');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 129, 61, 'GET', '/web/inspects/tasks/:id', '查询巡检任务详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 61 AND method = 'GET' AND path = '/web/inspects/tasks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 130, 611, 'POST', '/web/inspects/tasks', '新增巡检任务', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 611 AND method = 'POST' AND path = '/web/inspects/tasks');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 131, 612, 'PUT', '/web/inspects/tasks/:id', '编辑巡检任务', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 612 AND method = 'PUT' AND path = '/web/inspects/tasks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 132, 613, 'DELETE', '/web/inspects/tasks/:id', '删除巡检任务', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 613 AND method = 'DELETE' AND path = '/web/inspects/tasks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 212, 61, 'PUT', '/web/inspects/tasks/:id/read', '标记巡检任务消息通知已读', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 61 AND method = 'PUT' AND path = '/web/inspects/tasks/:id/read');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 133, 62, 'GET', '/web/inspects/records', '分页查询巡检记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 62 AND method = 'GET' AND path = '/web/inspects/records');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 134, 62, 'GET', '/web/inspects/records/:id', '查询巡检记录详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 62 AND method = 'GET' AND path = '/web/inspects/records/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 135, 834, 'GET', '/web/upgrades/firmware-types', '分页查询固件类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 834 AND method = 'GET' AND path = '/web/upgrades/firmware-types');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 136, 834, 'GET', '/web/upgrades/firmware-types/options', '固件类型下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 834 AND method = 'GET' AND path = '/web/upgrades/firmware-types/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 137, 8341, 'POST', '/web/upgrades/firmware-types', '新增固件类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8341 AND method = 'POST' AND path = '/web/upgrades/firmware-types');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 138, 8342, 'PUT', '/web/upgrades/firmware-types/:id', '编辑固件类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8342 AND method = 'PUT' AND path = '/web/upgrades/firmware-types/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 139, 8343, 'DELETE', '/web/upgrades/firmware-types/:id', '删除固件类型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8343 AND method = 'DELETE' AND path = '/web/upgrades/firmware-types/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 140, 832, 'GET', '/web/upgrades/firmwares', '分页查询固件', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 832 AND method = 'GET' AND path = '/web/upgrades/firmwares');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 141, 832, 'GET', '/web/upgrades/firmwares/options', '固件下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 832 AND method = 'GET' AND path = '/web/upgrades/firmwares/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 142, 8321, 'POST', '/web/upgrades/firmwares', '新增固件', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8321 AND method = 'POST' AND path = '/web/upgrades/firmwares');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 143, 8322, 'PUT', '/web/upgrades/firmwares/:id', '编辑固件', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8322 AND method = 'PUT' AND path = '/web/upgrades/firmwares/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 144, 8323, 'DELETE', '/web/upgrades/firmwares/:id', '删除固件', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8323 AND method = 'DELETE' AND path = '/web/upgrades/firmwares/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 145, 833, 'GET', '/web/upgrades/models', '分页查询AI模型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 833 AND method = 'GET' AND path = '/web/upgrades/models');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 146, 833, 'GET', '/web/upgrades/models/options', 'AI模型下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 833 AND method = 'GET' AND path = '/web/upgrades/models/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 147, 8331, 'POST', '/web/upgrades/models', '新增AI模型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8331 AND method = 'POST' AND path = '/web/upgrades/models');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 148, 8332, 'PUT', '/web/upgrades/models/:id', '编辑AI模型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8332 AND method = 'PUT' AND path = '/web/upgrades/models/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 149, 8333, 'DELETE', '/web/upgrades/models/:id', '删除AI模型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8333 AND method = 'DELETE' AND path = '/web/upgrades/models/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 150, 8311, 'POST', '/web/upgrades/devices', '下发设备升级(现在/定时)', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8311 AND method = 'POST' AND path = '/web/upgrades/devices');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 151, 831, 'GET', '/web/upgrades/plans', '分页查询升级计划', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 831 AND method = 'GET' AND path = '/web/upgrades/plans');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 152, 8312, 'PUT', '/web/upgrades/plans/:id/cancel', '取消单个升级计划', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8312 AND method = 'PUT' AND path = '/web/upgrades/plans/:id/cancel');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 153, 8312, 'PUT', '/web/upgrades/plans/cancel', '批量取消升级计划', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8312 AND method = 'PUT' AND path = '/web/upgrades/plans/cancel');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 154, 831, 'GET', '/web/upgrades/records', '分页查询升级记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 831 AND method = 'GET' AND path = '/web/upgrades/records');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 155, 821, 'GET', '/web/settings/alarm-settings', '分页查询设备告警设置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 821 AND method = 'GET' AND path = '/web/settings/alarm-settings');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 156, 8211, 'POST', '/web/settings/alarm-settings', '新增设备告警设置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8211 AND method = 'POST' AND path = '/web/settings/alarm-settings');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 157, 8212, 'PUT', '/web/settings/alarm-settings/:id', '编辑设备告警设置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8212 AND method = 'PUT' AND path = '/web/settings/alarm-settings/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 158, 8213, 'DELETE', '/web/settings/alarm-settings/:id', '删除设备告警设置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8213 AND method = 'DELETE' AND path = '/web/settings/alarm-settings/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 159, 8214, 'PUT', '/web/settings/alarm-settings/disable-all', '关闭全部告警通知', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8214 AND method = 'PUT' AND path = '/web/settings/alarm-settings/disable-all');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 160, 822, 'GET', '/web/settings/risk-notify-settings', '分页查询风险通知设置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 822 AND method = 'GET' AND path = '/web/settings/risk-notify-settings');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 161, 8221, 'POST', '/web/settings/risk-notify-settings', '新增风险通知设置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8221 AND method = 'POST' AND path = '/web/settings/risk-notify-settings');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 162, 8222, 'PUT', '/web/settings/risk-notify-settings/:id', '编辑风险通知设置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8222 AND method = 'PUT' AND path = '/web/settings/risk-notify-settings/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 163, 8223, 'DELETE', '/web/settings/risk-notify-settings/:id', '删除风险通知设置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8223 AND method = 'DELETE' AND path = '/web/settings/risk-notify-settings/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 164, 8224, 'PUT', '/web/settings/risk-notify-settings/disable-all', '关闭全部风险通知', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8224 AND method = 'PUT' AND path = '/web/settings/risk-notify-settings/disable-all');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 165, 851, 'GET', '/web/settings/getui', '查询个推账号配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 851 AND method = 'GET' AND path = '/web/settings/getui');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 166, 8511, 'PUT', '/web/settings/getui', '保存个推账号配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8511 AND method = 'PUT' AND path = '/web/settings/getui');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 167, 852, 'GET', '/web/settings/sms', '查询腾讯云短信配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 852 AND method = 'GET' AND path = '/web/settings/sms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 168, 8521, 'PUT', '/web/settings/sms', '保存腾讯云短信配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8521 AND method = 'PUT' AND path = '/web/settings/sms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 169, 1, 'GET', '/web/stats/dashboard', '首页看板统计', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 1 AND method = 'GET' AND path = '/web/stats/dashboard');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 170, 71, 'GET', '/web/stats/risk', '风险统计分析', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 71 AND method = 'GET' AND path = '/web/stats/risk');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 171, 72, 'GET', '/web/stats/alarm', '设备告警分析', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 72 AND method = 'GET' AND path = '/web/stats/alarm');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 172, 73, 'GET', '/web/stats/screen', '可视化大屏数据', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 73 AND method = 'GET' AND path = '/web/stats/screen');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 173, 56, 'GET', '/web/stats/traffic', '物联网卡流量统计', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 56 AND method = 'GET' AND path = '/web/stats/traffic');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 174, 2, 'GET', '/web/stats/device-status', '设备在线状态统计', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 2 AND method = 'GET' AND path = '/web/stats/device-status');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 175, 52, 'GET', '/web/stats/device-status', '设备监控状态统计', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 52 AND method = 'GET' AND path = '/web/stats/device-status');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 176, 2, 'GET', '/web/devices', '实时监控设备列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 2 AND method = 'GET' AND path = '/web/devices');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 177, 53, 'GET', '/web/devices/options', 'GIS查看设备下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 53 AND method = 'GET' AND path = '/web/devices/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 178, 91, 'POST', '/web/upload', '通用文件上传', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 91 AND method = 'POST' AND path = '/web/upload');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 179, 92, 'POST', '/web/upload/audio', '语音文件上传', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 92 AND method = 'POST' AND path = '/web/upload/audio');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 180, 101, 'GET', '/app/menus', '移动端菜单树', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 101 AND method = 'GET' AND path = '/app/menus');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 181, 101, 'GET', '/app/home', '移动端首页', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 101 AND method = 'GET' AND path = '/app/home');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 182, 101, 'GET', '/app/todos', '移动端待办', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 101 AND method = 'GET' AND path = '/app/todos');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 183, 102, 'GET', '/app/risks', '移动端风险列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 102 AND method = 'GET' AND path = '/app/risks');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 184, 102, 'GET', '/app/risks/:id', '移动端风险详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 102 AND method = 'GET' AND path = '/app/risks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 185, 102, 'PUT', '/app/risks/:id/handle', '移动端处理风险', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 102 AND method = 'PUT' AND path = '/app/risks/:id/handle');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 186, 102, 'PUT', '/app/risks/:id/accurate', '移动端确认检测准确性', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 102 AND method = 'PUT' AND path = '/app/risks/:id/accurate');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 187, 103, 'GET', '/app/alarms', '移动端告警列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 103 AND method = 'GET' AND path = '/app/alarms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 188, 103, 'PUT', '/app/alarms/:id/handle', '移动端处理告警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 103 AND method = 'PUT' AND path = '/app/alarms/:id/handle');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 189, 104, 'GET', '/app/inspect-tasks', '移动端巡检任务列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 104 AND method = 'GET' AND path = '/app/inspect-tasks');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 190, 104, 'GET', '/app/inspect-tasks/:id', '移动端巡检任务详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 104 AND method = 'GET' AND path = '/app/inspect-tasks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 191, 104, 'PUT', '/app/inspect-tasks/:id/finish', '移动端完成巡检任务', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 104 AND method = 'PUT' AND path = '/app/inspect-tasks/:id/finish');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 192, 105, 'GET', '/app/devices', '移动端设备列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 105 AND method = 'GET' AND path = '/app/devices');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 193, 105, 'GET', '/app/devices/:id', '移动端设备详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 105 AND method = 'GET' AND path = '/app/devices/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 194, 106, 'GET', '/app/notifications', '移动端通知列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 106 AND method = 'GET' AND path = '/app/notifications');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 195, 106, 'PUT', '/app/notifications/:id/read', '移动端标记通知已读', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 106 AND method = 'PUT' AND path = '/app/notifications/:id/read');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 196, 107, 'GET', '/app/profile', '移动端个人信息', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 107 AND method = 'GET' AND path = '/app/profile');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 197, 107, 'PUT', '/app/profile', '移动端修改个人信息', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 107 AND method = 'PUT' AND path = '/app/profile');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 198, 107, 'PUT', '/app/profile/password', '移动端修改密码', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 107 AND method = 'PUT' AND path = '/app/profile/password');


-- ════════════════════════════════════════════════════════════════
-- 5. 角色-菜单授权：super_admin 拥有全部菜单
-- ════════════════════════════════════════════════════════════════
INSERT INTO ac_role_menu_rel (role_id, menu_id)
SELECT 1, m.id FROM ac_menu m
WHERE NOT EXISTS (SELECT 1 FROM ac_role_menu_rel r WHERE r.role_id = 1 AND r.menu_id = m.id);


-- ════════════════════════════════════════════════════════════════
-- 6. Casbin 策略（casbin_rule）
--    p 策略：super_admin → 全部菜单接口（与 ac_menu_api 同源）
--    g 策略：admin → super_admin
--    先按角色清理再重建，保证重复执行结果一致。
-- ════════════════════════════════════════════════════════════════
DELETE FROM casbin_rule WHERE p_type = 'p' AND v0 = 'super_admin';

INSERT INTO casbin_rule (p_type, v0, v1, v2)
SELECT DISTINCT 'p', 'super_admin', path, method FROM ac_menu_api WHERE deleted_at IS NULL;

DELETE FROM casbin_rule WHERE p_type = 'g' AND v0 = 'admin';

INSERT INTO casbin_rule (p_type, v0, v1)
SELECT 'g', 'admin', 'super_admin'
WHERE NOT EXISTS (SELECT 1 FROM casbin_rule WHERE p_type = 'g' AND v0 = 'admin' AND v1 = 'super_admin');


-- ════════════════════════════════════════════════════════════════
-- 7. 内置字典：告警类型 / 设备模式
-- ════════════════════════════════════════════════════════════════
INSERT INTO ac_alarm_type (id, active, sequence, name, code, level, description, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, 1, '离线告警', 'offline', 'high', '设备心跳超时判定为离线', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_alarm_type WHERE code = 'offline');

INSERT INTO ac_alarm_type (id, active, sequence, name, code, level, description, created_by, created_dt, updated_by, updated_dt)
SELECT 2, TRUE, 2, '电量告警', 'battery', 'medium', '设备电量低于阈值时告警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_alarm_type WHERE code = 'battery');

INSERT INTO ac_alarm_type (id, active, sequence, name, code, level, description, created_by, created_dt, updated_by, updated_dt)
SELECT 3, TRUE, 3, '流量告警', 'traffic', 'medium', '物联网卡流量低于阈值时告警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_alarm_type WHERE code = 'traffic');

INSERT INTO ac_device_mode (id, active, sequence, name, code, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, 1, '充电模式', 'charging', '设备仅维持心跳与充电，关闭实时视频', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_device_mode WHERE code = 'charging');

INSERT INTO ac_device_mode (id, active, sequence, name, code, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 2, TRUE, 2, '全开模式', 'full', 'AI 识别、实时视频、上报全部开启', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_device_mode WHERE code = 'full');

INSERT INTO ac_device_mode (id, active, sequence, name, code, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 3, TRUE, 3, '省电模式', 'power_saving', '降低上报频率与视频码率以延长续航', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_device_mode WHERE code = 'power_saving');


-- ════════════════════════════════════════════════════════════════
-- 8. 示例业务数据：区域 / 风险类型 / 固件类型 / 系统参数
-- ════════════════════════════════════════════════════════════════
INSERT INTO ac_region (id, active, sequence, name, code, remark, dept_id, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, 1, '一号厂区', 'AREA_A', '示例区域，可删除', 1, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_region WHERE code = 'AREA_A');

INSERT INTO ac_region (id, active, sequence, name, code, remark, dept_id, created_by, created_dt, updated_by, updated_dt)
SELECT 2, TRUE, 2, '二号厂区', 'AREA_B', '示例区域，可删除', 1, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_region WHERE code = 'AREA_B');

INSERT INTO ac_region (id, active, sequence, name, code, remark, dept_id, created_by, created_dt, updated_by, updated_dt)
SELECT 3, TRUE, 3, '仓储中心', 'AREA_C', '示例区域，可删除', 1, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_region WHERE code = 'AREA_C');

INSERT INTO ac_risk_type (id, active, sequence, name, code, level, description, dept_id, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, 1, '未佩戴安全帽', 'no_helmet', 'high', '示例风险类型，可删除', 1, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_risk_type WHERE code = 'no_helmet');

INSERT INTO ac_risk_type (id, active, sequence, name, code, level, description, dept_id, created_by, created_dt, updated_by, updated_dt)
SELECT 2, TRUE, 2, '违规闯入', 'intrusion', 'high', '示例风险类型，可删除', 1, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_risk_type WHERE code = 'intrusion');

INSERT INTO ac_firmware_type (id, active, sequence, name, code, description, dept_id, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, 1, '主控固件', 'main', '示例固件类型，可删除', 1, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_firmware_type WHERE code = 'main');

INSERT INTO ac_firmware_type (id, active, sequence, name, code, description, dept_id, created_by, created_dt, updated_by, updated_dt)
SELECT 2, TRUE, 2, '摄像头固件', 'camera', '示例固件类型，可删除', 1, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_firmware_type WHERE code = 'camera');

INSERT INTO ac_system_config (id, active, name, code, "value", group_name, data_type, sequence, description, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, '设备心跳超时(秒)', 'device_heartbeat_timeout', '180', 'device', 'int', 1, '超过该时长未收到心跳则判定离线', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_system_config WHERE code = 'device_heartbeat_timeout');

INSERT INTO ac_system_config (id, active, name, code, "value", group_name, data_type, sequence, description, created_by, created_dt, updated_by, updated_dt)
SELECT 2, TRUE, '电量告警阈值(%)', 'battery_alarm_threshold', '20', 'device', 'int', 2, '设备电量低于该值触发电量告警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_system_config WHERE code = 'battery_alarm_threshold');

INSERT INTO ac_system_config (id, active, name, code, "value", group_name, data_type, sequence, description, created_by, created_dt, updated_by, updated_dt)
SELECT 3, TRUE, '流量告警阈值(MB)', 'traffic_alarm_threshold', '100', 'device', 'int', 3, '物联网卡剩余流量低于该值触发流量告警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_system_config WHERE code = 'traffic_alarm_threshold');

INSERT INTO ac_sequence (id, code, name, prefix, current_val, step, date_pattern, reset_by_day, created_by, created_dt, updated_by, updated_dt)
SELECT 1, 'device_code', '设备编号', 'AC', 0, 1, '20060102', TRUE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_sequence WHERE code = 'device_code');

INSERT INTO ac_sequence (id, code, name, prefix, current_val, step, date_pattern, reset_by_day, created_by, created_dt, updated_by, updated_dt)
SELECT 2, 'inspect_task_code', '巡检任务编号', 'IT', 0, 1, '20060102', TRUE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_sequence WHERE code = 'inspect_task_code');


-- ════════════════════════════════════════════════════════════════
-- 9. 修正序列值
--    以上 INSERT 显式指定了主键，BIGSERIAL 的序列不会自动推进，
--    此处将各序列对齐到当前最大 id，避免后续 insert 主键冲突。
-- ════════════════════════════════════════════════════════════════
SELECT setval(pg_get_serial_sequence('ac_dept', 'id'),          COALESCE((SELECT MAX(id) FROM ac_dept), 1));
SELECT setval(pg_get_serial_sequence('ac_user', 'id'),          COALESCE((SELECT MAX(id) FROM ac_user), 1));
SELECT setval(pg_get_serial_sequence('ac_role', 'id'),          COALESCE((SELECT MAX(id) FROM ac_role), 1));
SELECT setval(pg_get_serial_sequence('ac_menu', 'id'),          COALESCE((SELECT MAX(id) FROM ac_menu), 1));
SELECT setval(pg_get_serial_sequence('ac_menu_api', 'id'),      COALESCE((SELECT MAX(id) FROM ac_menu_api), 1));
SELECT setval(pg_get_serial_sequence('ac_user_role_rel', 'id'), COALESCE((SELECT MAX(id) FROM ac_user_role_rel), 1));
SELECT setval(pg_get_serial_sequence('ac_role_menu_rel', 'id'), COALESCE((SELECT MAX(id) FROM ac_role_menu_rel), 1));
SELECT setval(pg_get_serial_sequence('ac_region', 'id'),        COALESCE((SELECT MAX(id) FROM ac_region), 1));
SELECT setval(pg_get_serial_sequence('ac_risk_type', 'id'),     COALESCE((SELECT MAX(id) FROM ac_risk_type), 1));
SELECT setval(pg_get_serial_sequence('ac_alarm_type', 'id'),    COALESCE((SELECT MAX(id) FROM ac_alarm_type), 1));
SELECT setval(pg_get_serial_sequence('ac_device_mode', 'id'),   COALESCE((SELECT MAX(id) FROM ac_device_mode), 1));
SELECT setval(pg_get_serial_sequence('ac_firmware_type', 'id'), COALESCE((SELECT MAX(id) FROM ac_firmware_type), 1));
SELECT setval(pg_get_serial_sequence('ac_system_config', 'id'), COALESCE((SELECT MAX(id) FROM ac_system_config), 1));
SELECT setval(pg_get_serial_sequence('ac_sequence', 'id'),      COALESCE((SELECT MAX(id) FROM ac_sequence), 1));


-- ════════════════════════════════════════════════════════════════
-- 9. 消息推送 WebHook（飞书 / 钉钉 / 企业微信 / 自定义）
--    挂在「85 第三方平台管理」下。
--    注意：本段置于文件末尾，需在菜单与接口写入后重建 Casbin p 策略，
--    否则新增接口不会进入 super_admin 的授权集合。
-- ════════════════════════════════════════════════════════════════

-- 菜单
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 853, '85', 3, '消息推送配置', '/settings/webhooks', 'NotificationOutlined', 'system/thirdparty/webhooks', 'menu', 'pc', 'webhook:view', '菜单', 853, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 853);

-- 按钮权限
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8531, '853', 4, '新增配置', '', '', '', 'button', 'pc', 'webhook:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8531);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8532, '853', 4, '编辑配置', '', '', '', 'button', 'pc', 'webhook:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8532);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8533, '853', 4, '删除配置', '', '', '', 'button', 'pc', 'webhook:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8533);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 8534, '853', 4, '测试推送', '', '', '', 'button', 'pc', 'webhook:test', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 8534);

-- 接口绑定（Casbin p 策略的来源）
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 200, 8531, 'POST', '/web/settings/webhooks', '新增WebHook配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8531 AND method = 'POST' AND path = '/web/settings/webhooks');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 201, 8531, 'POST', '/web/settings/webhooks/test', '即时测试WebHook配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8531 AND method = 'POST' AND path = '/web/settings/webhooks/test');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 202, 853, 'GET', '/web/settings/webhooks', '分页查询WebHook配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 853 AND method = 'GET' AND path = '/web/settings/webhooks');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 203, 853, 'GET', '/web/settings/webhooks/platforms', '查询WebHook平台选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 853 AND method = 'GET' AND path = '/web/settings/webhooks/platforms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 204, 853, 'GET', '/web/settings/webhooks/:id', '查询WebHook配置详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 853 AND method = 'GET' AND path = '/web/settings/webhooks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 205, 8532, 'PUT', '/web/settings/webhooks/:id', '编辑WebHook配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8532 AND method = 'PUT' AND path = '/web/settings/webhooks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 206, 8532, 'PUT', '/web/settings/webhooks/:id/enable', '启用WebHook配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8532 AND method = 'PUT' AND path = '/web/settings/webhooks/:id/enable');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 207, 8532, 'PUT', '/web/settings/webhooks/:id/disable', '停用WebHook配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8532 AND method = 'PUT' AND path = '/web/settings/webhooks/:id/disable');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 208, 8533, 'DELETE', '/web/settings/webhooks/:id', '删除WebHook配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8533 AND method = 'DELETE' AND path = '/web/settings/webhooks/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 209, 8534, 'POST', '/web/settings/webhooks/:id/test', '发送WebHook测试消息', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 8534 AND method = 'POST' AND path = '/web/settings/webhooks/:id/test');

-- 角色授权：super_admin 拥有新菜单
INSERT INTO ac_role_menu_rel (role_id, menu_id)
SELECT 1, m.id FROM ac_menu m
WHERE m.id IN (853, 8531, 8532, 8533, 8534)
  AND NOT EXISTS (SELECT 1 FROM ac_role_menu_rel r WHERE r.role_id = 1 AND r.menu_id = m.id);

-- 重建 Casbin p 策略（新增接口必须重新派生，否则 super_admin 无法访问）
DELETE FROM casbin_rule WHERE p_type = 'p' AND v0 = 'super_admin';
INSERT INTO casbin_rule (p_type, v0, v1, v2)
SELECT DISTINCT 'p', 'super_admin', path, method FROM ac_menu_api WHERE deleted_at IS NULL;


-- ════════════════════════════════════════════════════════════════
-- 10. 按钮级权限码（补齐页面 permCodes 检查的标识）
--     前端页面用 permCodes.includes('<模块>:<实体>:<动作>') 控制按钮显隐，
--     这些标识必须作为 type='button' 的菜单行存在，且已授予对应角色，
--     否则 /web/profile/menus 不会返回它们，按钮对超管也不渲染。
-- ════════════════════════════════════════════════════════════════
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9001, '841', 4, '区域-新增', '', '', '', 'button', 'pc', 'basic:region:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9001);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9002, '841', 4, '区域-编辑', '', '', '', 'button', 'pc', 'basic:region:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9002);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9003, '841', 4, '区域-删除', '', '', '', 'button', 'pc', 'basic:region:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9003);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9004, '842', 4, '风险类型-新增', '', '', '', 'button', 'pc', 'basic:risk-type:create', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9004);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9005, '842', 4, '风险类型-编辑', '', '', '', 'button', 'pc', 'basic:risk-type:update', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9005);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9006, '842', 4, '风险类型-删除', '', '', '', 'button', 'pc', 'basic:risk-type:delete', '按钮', 6, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9006);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9007, '843', 4, '告警类型-新增', '', '', '', 'button', 'pc', 'basic:alarm-type:create', '按钮', 7, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9007);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9008, '843', 4, '告警类型-编辑', '', '', '', 'button', 'pc', 'basic:alarm-type:update', '按钮', 8, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9008);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9009, '843', 4, '告警类型-删除', '', '', '', 'button', 'pc', 'basic:alarm-type:delete', '按钮', 9, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9009);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9010, '811', 4, '设备-切换模式', '', '', '', 'button', 'pc', 'device:switch-mode', '按钮', 10, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9010);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9011, '814', 4, '物联网卡-更换登记', '', '', '', 'button', 'pc', 'iot-card:replace', '按钮', 11, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9011);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9012, '52', 4, '设备监控-查看', '', '', '', 'button', 'pc', 'device:monitor:view', '按钮', 12, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9012);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9013, '52', 4, '设备监控-视频', '', '', '', 'button', 'pc', 'device:monitor:video', '按钮', 13, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9013);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9014, '61', 4, '巡检任务-新增', '', '', '', 'button', 'pc', 'inspect:task:create', '按钮', 14, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9014);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9015, '61', 4, '巡检任务-编辑', '', '', '', 'button', 'pc', 'inspect:task:update', '按钮', 15, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9015);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9016, '61', 4, '巡检任务-删除', '', '', '', 'button', 'pc', 'inspect:task:delete', '按钮', 16, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9016);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9017, '821', 4, '告警设置-新增', '', '', '', 'button', 'pc', 'setting:alarm:create', '按钮', 17, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9017);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9018, '821', 4, '告警设置-编辑', '', '', '', 'button', 'pc', 'setting:alarm:update', '按钮', 18, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9018);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9019, '821', 4, '告警设置-删除', '', '', '', 'button', 'pc', 'setting:alarm:delete', '按钮', 19, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9019);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9020, '821', 4, '告警设置-关闭全部', '', '', '', 'button', 'pc', 'setting:alarm:disable-all', '按钮', 20, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9020);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9021, '822', 4, '风险通知-新增', '', '', '', 'button', 'pc', 'setting:risk-notify:create', '按钮', 21, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9021);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9022, '822', 4, '风险通知-编辑', '', '', '', 'button', 'pc', 'setting:risk-notify:update', '按钮', 22, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9022);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9023, '822', 4, '风险通知-删除', '', '', '', 'button', 'pc', 'setting:risk-notify:delete', '按钮', 23, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9023);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9024, '822', 4, '风险通知-关闭全部', '', '', '', 'button', 'pc', 'setting:risk-notify:disable-all', '按钮', 24, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9024);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9025, '851', 4, '个推-保存', '', '', '', 'button', 'pc', 'setting:thirdparty:getui:update', '按钮', 25, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9025);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9026, '852', 4, '短信-保存', '', '', '', 'button', 'pc', 'setting:thirdparty:sms:update', '按钮', 26, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9026);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9027, '831', 4, '升级-下发', '', '', '', 'button', 'pc', 'upgrade:issue', '按钮', 27, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9027);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9028, '831', 4, '升级-取消', '', '', '', 'button', 'pc', 'upgrade:cancel', '按钮', 28, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9028);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9029, '832', 4, '固件-新增', '', '', '', 'button', 'pc', 'upgrade:firmware:create', '按钮', 29, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9029);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9030, '832', 4, '固件-编辑', '', '', '', 'button', 'pc', 'upgrade:firmware:update', '按钮', 30, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9030);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9031, '832', 4, '固件-删除', '', '', '', 'button', 'pc', 'upgrade:firmware:delete', '按钮', 31, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9031);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9032, '833', 4, '模型-新增', '', '', '', 'button', 'pc', 'upgrade:model:create', '按钮', 32, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9032);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9033, '833', 4, '模型-编辑', '', '', '', 'button', 'pc', 'upgrade:model:update', '按钮', 33, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9033);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9034, '833', 4, '模型-删除', '', '', '', 'button', 'pc', 'upgrade:model:delete', '按钮', 34, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9034);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9035, '834', 4, '固件类型-新增', '', '', '', 'button', 'pc', 'upgrade:firmware-type:create', '按钮', 35, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9035);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9036, '834', 4, '固件类型-编辑', '', '', '', 'button', 'pc', 'upgrade:firmware-type:update', '按钮', 36, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9036);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9037, '834', 4, '固件类型-删除', '', '', '', 'button', 'pc', 'upgrade:firmware-type:delete', '按钮', 37, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9037);

-- 授予 super_admin（角色 1）
INSERT INTO ac_role_menu_rel (role_id, menu_id)
SELECT 1, m.id FROM ac_menu m
WHERE m.id BETWEEN 9001 AND 9100
  AND NOT EXISTS (SELECT 1 FROM ac_role_menu_rel r WHERE r.role_id = 1 AND r.menu_id = m.id);

-- ════════════════════════════════════════════════════════════════
-- 11. 固件 / AI 模型详情接口（v0.1.0 补实现）
--     此前 GET /web/upgrades/firmwares/:id、/web/upgrades/models/:id 未实现（404）。
--     路由已补齐，此处同步补 ac_menu_api 绑定并重建 Casbin p 策略
--     （ac_menu_api 是 p 策略的唯一来源，缺失则 super_admin 也会被 403 拒绝）。
-- ════════════════════════════════════════════════════════════════
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 210, 832, 'GET', '/web/upgrades/firmwares/:id', '查询固件详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 832 AND method = 'GET' AND path = '/web/upgrades/firmwares/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 211, 833, 'GET', '/web/upgrades/models/:id', '查询AI模型详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE menu_id = 833 AND method = 'GET' AND path = '/web/upgrades/models/:id');

-- 重建 Casbin p 策略（新增接口必须重新派生，否则 super_admin 无法访问）
DELETE FROM casbin_rule WHERE p_type = 'p' AND v0 = 'super_admin';
INSERT INTO casbin_rule (p_type, v0, v1, v2)
SELECT DISTINCT 'p', 'super_admin', path, method FROM ac_menu_api WHERE deleted_at IS NULL;

-- 补充：system 一组按钮权限码（页面通过 can() 辅助函数读取，首轮排查遗漏）
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9101, '862', 4, '用户-新增', '', '', '', 'button', 'pc', 'system:user:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9101);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9102, '862', 4, '用户-编辑', '', '', '', 'button', 'pc', 'system:user:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9102);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9103, '862', 4, '用户-删除', '', '', '', 'button', 'pc', 'system:user:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9103);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9104, '862', 4, '用户-重置密码', '', '', '', 'button', 'pc', 'system:user:reset', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9104);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9105, '864', 4, '角色-新增', '', '', '', 'button', 'pc', 'system:role:create', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9105);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9106, '864', 4, '角色-编辑', '', '', '', 'button', 'pc', 'system:role:update', '按钮', 6, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9106);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9107, '864', 4, '角色-删除', '', '', '', 'button', 'pc', 'system:role:delete', '按钮', 7, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9107);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9108, '864', 4, '角色-授权', '', '', '', 'button', 'pc', 'system:role:assign', '按钮', 8, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9108);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9109, '864', 4, '角色-数据权限', '', '', '', 'button', 'pc', 'system:role:data', '按钮', 9, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9109);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9110, '863', 4, '菜单-新增', '', '', '', 'button', 'pc', 'system:menu:create', '按钮', 10, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9110);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9111, '863', 4, '菜单-编辑', '', '', '', 'button', 'pc', 'system:menu:update', '按钮', 11, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9111);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9112, '863', 4, '菜单-删除', '', '', '', 'button', 'pc', 'system:menu:delete', '按钮', 12, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9112);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9113, '863', 4, '菜单-绑定接口', '', '', '', 'button', 'pc', 'system:menu:api', '按钮', 13, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9113);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9114, '865', 4, '部门-新增', '', '', '', 'button', 'pc', 'system:dept:create', '按钮', 14, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9114);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9115, '865', 4, '部门-编辑', '', '', '', 'button', 'pc', 'system:dept:update', '按钮', 15, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9115);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9116, '865', 4, '部门-删除', '', '', '', 'button', 'pc', 'system:dept:delete', '按钮', 16, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9116);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9117, '866', 4, '岗位-新增', '', '', '', 'button', 'pc', 'system:post:create', '按钮', 17, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9117);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9118, '866', 4, '岗位-编辑', '', '', '', 'button', 'pc', 'system:post:update', '按钮', 18, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9118);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9119, '866', 4, '岗位-删除', '', '', '', 'button', 'pc', 'system:post:delete', '按钮', 19, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9119);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9120, '869', 4, '参数-新增', '', '', '', 'button', 'pc', 'system:config:create', '按钮', 20, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9120);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9121, '869', 4, '参数-编辑', '', '', '', 'button', 'pc', 'system:config:update', '按钮', 21, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9121);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9122, '869', 4, '参数-删除', '', '', '', 'button', 'pc', 'system:config:delete', '按钮', 22, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9122);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9123, '867', 4, '日志-删除', '', '', '', 'button', 'pc', 'system:log:delete', '按钮', 23, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9123);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9124, '867', 4, '日志-清空', '', '', '', 'button', 'pc', 'system:log:clear', '按钮', 24, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9124);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9125, '867', 4, '日志-导出', '', '', '', 'button', 'pc', 'system:log:export', '按钮', 25, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9125);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9126, '868', 4, '通知-标记已读', '', '', '', 'button', 'pc', 'system:notify:mark', '按钮', 26, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9126);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 9127, '868', 4, '通知-删除', '', '', '', 'button', 'pc', 'system:notify:delete', '按钮', 27, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 9127);

INSERT INTO ac_role_menu_rel (role_id, menu_id)
SELECT 1, m.id FROM ac_menu m
WHERE m.id BETWEEN 9101 AND 9200
  AND NOT EXISTS (SELECT 1 FROM ac_role_menu_rel r WHERE r.role_id = 1 AND r.menu_id = m.id);


-- ════════════════════════════════════════════════════════════════
-- 11. 示例设备与物联网卡
--     设备/风险/告警/巡检/升级/统计等多数页面与 E2E 用例都依赖一台
--     可查询的示例设备（东门摄像头 / CAM_001），缺失会导致干净库上
--     设备相关用例全部落空。此处补齐，使 resetdb + initdb 后的库自足。
-- ════════════════════════════════════════════════════════════════

INSERT INTO ac_iot_card (id, active, name, card_no, region_id, device_id, traffic_total, traffic_used, traffic_left, stat_dt, expire_dt, status, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, '示例物联网卡', '89860000000000000001', 1, NULL, 30720, 4096, 26624, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '365 days', 'normal', '示例数据', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_iot_card WHERE id = 1 OR card_no = '89860000000000000001');

INSERT INTO ac_device (id, active, name, device_code, region_id, location, latitude, longitude, firmware_ver, model_ver, system_ver, work_mode, device_mode, info_interval, iot_card_id, status, battery, voltage, storage_total, storage_free, online_dt, monitor_enabled, stream_url, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, '东门摄像头', 'CAM_001', 1, '东门入口', '30.5728', '104.0668', '', '', 'v1.0.0', '全开模式', '全开模式', 60, 1, 'online', 86, '12.4', 65536, 51200, CURRENT_TIMESTAMP, TRUE, '', '示例数据', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_device WHERE id = 1 OR device_code = 'CAM_001');

UPDATE ac_iot_card SET device_id = 1 WHERE card_no = '89860000000000000001' AND device_id IS NULL;

-- 一台设备的历史采样点，供「设备历史记录」页面与曲线用例使用
INSERT INTO ac_device_history (device_id, device_code, iot_card_no, region_id, location, status, battery, voltage, storage_free, sampled_dt, created_by, created_dt, updated_by, updated_dt)
SELECT 1, 'CAM_001', '89860000000000000001', 1, '东门入口', 'online', 90 - (n.i * 2), '12.4', 60000 - (n.i * 100), CURRENT_TIMESTAMP - (n.i * INTERVAL '1 hour'), 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
FROM generate_series(0, 11) AS n(i)
WHERE NOT EXISTS (SELECT 1 FROM ac_device_history WHERE device_id = 1);

-- 示例风险与告警各一条，使风险监控/设备告警页面与筛选用例有数据可查
INSERT INTO ac_risk_record (id, risk_type_id, device_id, device_code, region_id, location, image_url, video_url, risk_level, detect_score, occurred_dt, detect_accurate, read, status, voice_played, created_by, created_dt, updated_by, updated_dt)
SELECT 1, 1, 1, 'CAM_001', 1, '东门入口', '', '', 'high', 0.93, CURRENT_TIMESTAMP - INTERVAL '2 hours', '', FALSE, 'pending', FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_risk_record WHERE id = 1);

INSERT INTO ac_alarm_record (id, alarm_type_id, device_id, device_code, region_id, location, alarm_level, content, occurred_dt, read, status, created_by, created_dt, updated_by, updated_dt)
SELECT 1, 1, 1, 'CAM_001', 1, '东门入口', 'high', '设备心跳超时，已离线', CURRENT_TIMESTAMP - INTERVAL '1 hour', FALSE, 'pending', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_alarm_record WHERE id = 1);


-- ════════════════════════════════════════════════════════════════
-- 12. 国标视频（GB/T 28181）权限数据（v0.2.0 新增）
--     设计依据：docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md §5.2、§10、§11.1
--     表结构：docs/开发文档/数据库设计/sql/ddl/09-国标视频.sql
--     菜单 id 层级编码：目录 12 → 二级 121 → 页面 1211 → 按钮 12111。
--     权限码规范 模块:实体:动作，本模块统一前缀 gb:。
--     ⚠️ 权限码必须真实存在于 ac_menu 且已授予角色，否则前端 permCodes.includes()
--        对超管也会静默失效（见 CLAUDE.md 按钮权限码规范）。
--     注意：本文件由 initdb 按分号分句执行，注释与数据中不得出现分号。
-- ════════════════════════════════════════════════════════════════

-- ── 12.1 一级目录 ─────────────────────────────────────────────
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12, NULL, 1, '国标视频', '/gb', 'VideoCameraAddOutlined', '', 'directory', 'pc', 'gb:view', '目录', 12, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12);

-- ── 12.2 二级目录 ─────────────────────────────────────────────
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 121, '12', 2, '设备与通道', '/gb/device', 'DeploymentUnitOutlined', '', 'directory', 'pc', 'gb:device-dir:view', '目录', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 121);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 122, '12', 2, '播放与录像', '/gb/media', 'PlayCircleOutlined', '', 'directory', 'pc', 'gb:media-dir:view', '目录', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 122);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 123, '12', 2, '报警与位置', '/gb/alarm', 'CompassOutlined', '', 'directory', 'pc', 'gb:alarm-dir:view', '目录', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 123);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 124, '12', 2, '流媒体运维', '/gb/ops', 'CloudServerOutlined', '', 'directory', 'pc', 'gb:ops-dir:view', '目录', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 124);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 125, '12', 2, '国标级联', '/gb/platform', 'ShareAltOutlined', '', 'directory', 'pc', 'gb:platform-dir:view', '目录', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 125);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 126, '12', 2, '组织与语音', '/gb/org', 'PartitionOutlined', '', 'directory', 'pc', 'gb:org-dir:view', '目录', 6, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 126);

-- ── 12.3 页面（type=menu）─────────────────────────────────────
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1211, '121', 3, '国标设备', '/gb/device/list', 'DesktopOutlined', 'gb/GbDeviceList', 'menu', 'pc', 'gb:device:list', '菜单', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1211);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1212, '121', 3, '国标通道', '/gb/device/channel', 'ProfileOutlined', 'gb/GbChannelList', 'menu', 'pc', 'gb:channel:list', '菜单', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1212);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1213, '121', 3, '设备激活', '/gb/device/activation', 'QrcodeOutlined', 'gb/DeviceActivationList', 'menu', 'pc', 'gb:activation:list', '菜单', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1213);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1214, '121', 3, '订阅记录', '/gb/device/subscribe', 'NotificationOutlined', 'gb/GbSubscribeList', 'menu', 'pc', 'gb:subscription:list', '菜单', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1214);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1221, '122', 3, '实时播放', '/gb/media/live', 'PlayCircleOutlined', 'gb/LivePlay', 'menu', 'pc', 'gb:live:list', '菜单', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1221);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1222, '122', 3, '录像回放', '/gb/media/playback', 'HistoryOutlined', 'gb/Playback', 'menu', 'pc', 'gb:playback:list', '菜单', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1222);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1223, '122', 3, '录像下载', '/gb/media/download', 'DownloadOutlined', 'gb/RecordDownload', 'menu', 'pc', 'gb:record-download:list', '菜单', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1223);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1224, '122', 3, '云端录像', '/gb/media/cloud-record', 'CloudDownloadOutlined', 'gb/CloudRecordList', 'menu', 'pc', 'gb:cloud-record:list', '菜单', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1224);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1225, '122', 3, '录像计划', '/gb/media/record-plan', 'ScheduleOutlined', 'gb/RecordPlanList', 'menu', 'pc', 'gb:record-plan:list', '菜单', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1225);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1231, '123', 3, '国标报警', '/gb/alarm/list', 'BellOutlined', 'gb/GbAlarmList', 'menu', 'pc', 'gb:alarm:list', '菜单', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1231);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1232, '123', 3, '移动位置', '/gb/alarm/position', 'GlobalOutlined', 'gb/MobilePosition', 'menu', 'pc', 'gb:position:list', '菜单', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1232);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1241, '124', 3, '媒体节点', '/gb/ops/media-server', 'CloudServerOutlined', 'gb/MediaServerList', 'menu', 'pc', 'gb:media-server:list', '菜单', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1241);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1242, '124', 3, '拉流代理', '/gb/ops/stream-proxy', 'ApiOutlined', 'gb/StreamProxyList', 'menu', 'pc', 'gb:stream-proxy:list', '菜单', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1242);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1243, '124', 3, '推流', '/gb/ops/stream-push', 'UploadOutlined', 'gb/StreamPushList', 'menu', 'pc', 'gb:stream-push:list', '菜单', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1243);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1244, '124', 3, '信令日志', '/gb/ops/sip-log', 'FileSearchOutlined', 'gb/SipLogList', 'menu', 'pc', 'gb:sip-log:list', '菜单', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1244);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1251, '125', 3, '上级平台', '/gb/platform/list', 'ShareAltOutlined', 'gb/PlatformList', 'menu', 'pc', 'gb:platform:list', '菜单', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1251);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1252, '125', 3, '级联通道', '/gb/platform/channel', 'LinkOutlined', 'gb/PlatformChannelList', 'menu', 'pc', 'gb:platform-channel:list', '菜单', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1252);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1261, '126', 3, '业务分组', '/gb/org/group', 'ApartmentOutlined', 'gb/GbGroupTree', 'menu', 'pc', 'gb:group:list', '菜单', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1261);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1263, '126', 3, '语音 Agent', '/gb/org/voice-agent', 'AudioOutlined', 'gb/VoiceAgentConfig', 'menu', 'pc', 'voice:agent:view', '菜单', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1263);

-- ── 12.4 按钮（type=button，path/component 为空）──────────────
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12111, '1211', 4, '同步通道', '', '', '', 'button', 'pc', 'gb:device:sync-channel', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12111);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12112, '1211', 4, '订阅目录', '', '', '', 'button', 'pc', 'gb:device:subscribe', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12112);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12114, '1211', 4, '删除国标设备', '', '', '', 'button', 'pc', 'gb:device:delete', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12114);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12115, '1211', 4, '生成激活凭证', '', '', '', 'button', 'pc', 'gb:device:activation', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12115);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12116, '1211', 4, '设备诊断', '', '', '', 'button', 'pc', 'gb:device:diagnose', '按钮', 6, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12116);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12121, '1212', 4, '编辑通道', '', '', '', 'button', 'pc', 'gb:channel:update', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12121);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12122, '1212', 4, '设置分组', '', '', '', 'button', 'pc', 'gb:channel:group', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12122);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12123, '1212', 4, '设置区域', '', '', '', 'button', 'pc', 'gb:channel:region', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12123);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12124, '1212', 4, '重置通道', '', '', '', 'button', 'pc', 'gb:channel:reset', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12124);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12128, '1212', 4, '删除通道', '', '', '', 'button', 'pc', 'gb:channel:delete', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12128);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12141, '1214', 4, '新增订阅', '', '', '', 'button', 'pc', 'gb:subscription:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12141);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12142, '1214', 4, '删除订阅', '', '', '', 'button', 'pc', 'gb:subscription:delete', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12142);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12211, '1221', 4, '云台控制', '', '', '', 'button', 'pc', 'gb:live:ptz', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12211);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12212, '1221', 4, '语音广播', '', '', '', 'button', 'pc', 'gb:live:broadcast', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12212);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12213, '1221', 4, '语音对讲', '', '', '', 'button', 'pc', 'gb:live:talk', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12213);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12214, '1221', 4, '视频抓图', '', '', '', 'button', 'pc', 'gb:live:snap', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12214);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12216, '1221', 4, '开始停止播放', '', '', '', 'button', 'pc', 'gb:live:play', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12216);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12221, '1222', 4, '回放控制', '', '', '', 'button', 'pc', 'gb:playback:control', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12221);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12223, '1222', 4, '开始停止回放', '', '', '', 'button', 'pc', 'gb:playback:play', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12223);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12231, '1223', 4, '开始下载', '', '', '', 'button', 'pc', 'gb:record-download:start', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12231);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12232, '1223', 4, '取消下载', '', '', '', 'button', 'pc', 'gb:record-download:cancel', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12232);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12241, '1224', 4, '播放录像', '', '', '', 'button', 'pc', 'gb:cloud-record:play', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12241);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12242, '1224', 4, '下载录像', '', '', '', 'button', 'pc', 'gb:cloud-record:download', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12242);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12243, '1224', 4, '收藏录像', '', '', '', 'button', 'pc', 'gb:cloud-record:collect', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12243);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12244, '1224', 4, '删除录像', '', '', '', 'button', 'pc', 'gb:cloud-record:delete', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12244);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12251, '1225', 4, '新增计划', '', '', '', 'button', 'pc', 'gb:record-plan:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12251);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12252, '1225', 4, '编辑计划', '', '', '', 'button', 'pc', 'gb:record-plan:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12252);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12253, '1225', 4, '删除计划', '', '', '', 'button', 'pc', 'gb:record-plan:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12253);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12254, '1225', 4, '关联通道', '', '', '', 'button', 'pc', 'gb:record-plan:bind', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12254);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12311, '1231', 4, '处理报警', '', '', '', 'button', 'pc', 'gb:alarm:handle', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12311);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12312, '1231', 4, '报警抓图', '', '', '', 'button', 'pc', 'gb:alarm:snap', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12312);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12313, '1231', 4, '清空报警', '', '', '', 'button', 'pc', 'gb:alarm:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12313);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12321, '1232', 4, '订阅位置', '', '', '', 'button', 'pc', 'gb:position:subscribe', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12321);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12411, '1241', 4, '新增节点', '', '', '', 'button', 'pc', 'gb:media-server:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12411);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12412, '1241', 4, '编辑节点', '', '', '', 'button', 'pc', 'gb:media-server:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12412);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12413, '1241', 4, '删除节点', '', '', '', 'button', 'pc', 'gb:media-server:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12413);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12414, '1241', 4, '在线检查', '', '', '', 'button', 'pc', 'gb:media-server:check', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12414);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12421, '1242', 4, '新增代理', '', '', '', 'button', 'pc', 'gb:stream-proxy:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12421);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12422, '1242', 4, '编辑代理', '', '', '', 'button', 'pc', 'gb:stream-proxy:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12422);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12423, '1242', 4, '删除代理', '', '', '', 'button', 'pc', 'gb:stream-proxy:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12423);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12424, '1242', 4, '启动拉流', '', '', '', 'button', 'pc', 'gb:stream-proxy:start', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12424);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12425, '1242', 4, '停止拉流', '', '', '', 'button', 'pc', 'gb:stream-proxy:stop', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12425);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12431, '1243', 4, '新增推流', '', '', '', 'button', 'pc', 'gb:stream-push:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12431);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12432, '1243', 4, '编辑推流', '', '', '', 'button', 'pc', 'gb:stream-push:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12432);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12433, '1243', 4, '删除推流', '', '', '', 'button', 'pc', 'gb:stream-push:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12433);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12434, '1243', 4, '启动推流', '', '', '', 'button', 'pc', 'gb:stream-push:start', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12434);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12435, '1243', 4, '停止推流', '', '', '', 'button', 'pc', 'gb:stream-push:stop', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12435);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12511, '1251', 4, '新增平台', '', '', '', 'button', 'pc', 'gb:platform:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12511);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12512, '1251', 4, '编辑平台', '', '', '', 'button', 'pc', 'gb:platform:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12512);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12513, '1251', 4, '删除平台', '', '', '', 'button', 'pc', 'gb:platform:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12513);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12516, '1251', 4, '推送分组', '', '', '', 'button', 'pc', 'gb:platform:groups', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12516);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12517, '1251', 4, '推送区域', '', '', '', 'button', 'pc', 'gb:platform:regions', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12517);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12521, '1252', 4, '添加通道', '', '', '', 'button', 'pc', 'gb:platform-channel:add', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12521);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12522, '1252', 4, '定制通道', '', '', '', 'button', 'pc', 'gb:platform-channel:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12522);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12523, '1252', 4, '移除通道', '', '', '', 'button', 'pc', 'gb:platform-channel:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12523);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12611, '1261', 4, '新增分组', '', '', '', 'button', 'pc', 'gb:group:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12611);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12612, '1261', 4, '编辑分组', '', '', '', 'button', 'pc', 'gb:group:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12612);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12613, '1261', 4, '删除分组', '', '', '', 'button', 'pc', 'gb:group:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12613);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12631, '1263', 4, '保存配置', '', '', '', 'button', 'pc', 'voice:agent:update', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12631);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12632, '1263', 4, '测试连接', '', '', '', 'button', 'pc', 'voice:agent:test', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12632);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12633, '1263', 4, '应用配置', '', '', '', 'button', 'pc', 'voice:agent:apply', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 12633);


-- ── 12.4b 清理早期草稿遗留的权限数据（幂等）─────────────────
--     旧版按「规划中但未实现」的路由写入了 ac_menu_api（id 300-401），
--     部分按钮也已随接口调整迁移到「播放与录像」页面或直接取消。
--     先清理再重建，保证菜单授权与真实路由严格一致。
DELETE FROM ac_menu_api WHERE id BETWEEN 300 AND 401;
DELETE FROM ac_role_menu_rel WHERE menu_id IN (12113, 12125, 12126, 12127, 12131, 12132, 12215, 12222, 12322, 12415, 12436, 12441, 12442, 12514, 12515);
DELETE FROM ac_menu WHERE id IN (12113, 12125, 12126, 12127, 12131, 12132, 12215, 12222, 12322, 12415, 12436, 12441, 12442, 12514, 12515);


-- ── 12.5 接口绑定（ac_menu_api 是 Casbin p 策略的唯一来源）────
--     本段由 internal/api/router/{web,gb_live,gb_ops,app}.go 的**实际注册路由**反推，
--     与 docs/开发文档/后端开发/00-接口契约.md §10 逐条对应。
--     /app/* 组同样挂载了 casbinMW，移动端国标接口也必须在此登记（见本段末尾）。
--     规则：路由 (method, path) 全局唯一 —— 读接口挂 menu 行，写接口挂 button 行，
--           个别返回取流地址的 GET（云端录像播放/下载）按「动作」挂在 button 行。
--     注意：本文件由 initdb 按分号分句执行，注释与数据中不得出现分号。
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 500, 1211, 'GET', '/web/gb/devices', '分页查询国标设备', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/devices');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 501, 1211, 'GET', '/web/gb/devices/:id', '查询国标设备详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/devices/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 502, 12111, 'POST', '/web/gb/devices/:id/sync-channel', '同步设备通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/devices/:id/sync-channel');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 503, 12112, 'POST', '/web/gb/devices/:id/subscribe', '订阅目录与报警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/devices/:id/subscribe');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 504, 12112, 'DELETE', '/web/gb/devices/:id/subscribe', '取消订阅', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/devices/:id/subscribe');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 505, 12115, 'POST', '/web/gb/devices/:id/activation', '生成设备激活凭证', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/devices/:id/activation');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 506, 12114, 'DELETE', '/web/gb/devices/:id', '删除国标设备', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/devices/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 507, 1212, 'GET', '/web/gb/channels', '分页查询国标通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/channels');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 508, 1212, 'GET', '/web/gb/channels/options', '国标通道下拉选项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/channels/options');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 509, 1212, 'GET', '/web/gb/channels/:id', '查询国标通道详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/channels/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 510, 12121, 'PUT', '/web/gb/channels/:id', '编辑国标通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/channels/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 511, 12128, 'DELETE', '/web/gb/channels/:id', '删除国标通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/channels/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 512, 12123, 'PUT', '/web/gb/channels/:id/region', '设置通道区域', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/channels/:id/region');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 513, 12122, 'PUT', '/web/gb/channels/:id/group', '设置通道业务分组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/channels/:id/group');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 514, 12124, 'POST', '/web/gb/channels/:id/reset', '重置通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/channels/:id/reset');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 515, 1214, 'GET', '/web/gb/subscribes', '分页查询订阅记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/subscribes');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 516, 12141, 'POST', '/web/gb/subscribes', '新增订阅记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/subscribes');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 517, 12142, 'DELETE', '/web/gb/subscribes/:id', '删除订阅记录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/subscribes/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 518, 1221, 'GET', '/web/gb/play/:channelId/status', '查询实时播放状态', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/play/:channelId/status');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 519, 1221, 'GET', '/web/gb/ptz/:channelId/presets', '查询云台预置位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/ptz/:channelId/presets');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 520, 12216, 'POST', '/web/gb/play/:channelId', '开始实时播放', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/play/:channelId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 521, 12216, 'POST', '/web/gb/play/:channelId/stop', '停止实时播放', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/play/:channelId/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 522, 12214, 'POST', '/web/gb/play/:channelId/snapshot', '实时视频抓图', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/play/:channelId/snapshot');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 523, 12211, 'POST', '/web/gb/ptz/:channelId', '云台控制', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/ptz/:channelId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 524, 12211, 'POST', '/web/gb/ptz/:channelId/presets', '新增云台预置位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/ptz/:channelId/presets');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 525, 12211, 'DELETE', '/web/gb/ptz/:channelId/presets/:presetId', '删除云台预置位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/ptz/:channelId/presets/:presetId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 526, 12211, 'POST', '/web/gb/ptz/:channelId/cruise', '云台巡航', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/ptz/:channelId/cruise');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 527, 12211, 'POST', '/web/gb/ptz/:channelId/scan', '云台扫描', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/ptz/:channelId/scan');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 528, 12211, 'POST', '/web/gb/ptz/:channelId/wiper', '云台雨刷', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/ptz/:channelId/wiper');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 529, 12212, 'POST', '/web/gb/voice/:channelId/broadcast/start', '开始语音广播', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/voice/:channelId/broadcast/start');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 530, 12212, 'POST', '/web/gb/voice/:channelId/broadcast/stop', '停止语音广播', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/voice/:channelId/broadcast/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 531, 12213, 'POST', '/web/gb/voice/:channelId/talk/start', '开始语音对讲', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/voice/:channelId/talk/start');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 532, 12213, 'POST', '/web/gb/voice/:channelId/talk/stop', '停止语音对讲', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/voice/:channelId/talk/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 533, 1222, 'GET', '/web/gb/playback/records', '查询录像文件', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/playback/records');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 534, 12223, 'POST', '/web/gb/playback', '开始录像回放', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/playback');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 535, 12223, 'POST', '/web/gb/playback/stop', '停止录像回放', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/playback/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 536, 12221, 'POST', '/web/gb/playback/control', '回放控制', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/playback/control');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 537, 1223, 'GET', '/web/gb/playback/download', '分页查询下载任务', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/playback/download');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 538, 12231, 'POST', '/web/gb/playback/download', '开始下载录像', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/playback/download');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 539, 12232, 'DELETE', '/web/gb/playback/download/:id', '取消下载录像', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/playback/download/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 540, 1224, 'GET', '/web/gb/cloud-records/dates', '查询云端录像日期', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/cloud-records/dates');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 541, 1224, 'GET', '/web/gb/cloud-records', '分页查询云端录像', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/cloud-records');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 542, 12241, 'GET', '/web/gb/cloud-records/:id/play', '播放云端录像', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/cloud-records/:id/play');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 543, 12242, 'GET', '/web/gb/cloud-records/:id/download', '下载云端录像', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/cloud-records/:id/download');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 544, 12243, 'PUT', '/web/gb/cloud-records/collect', '收藏云端录像', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/cloud-records/collect');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 545, 12244, 'DELETE', '/web/gb/cloud-records', '删除云端录像', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/cloud-records');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 546, 1225, 'GET', '/web/gb/record-plans', '分页查询录像计划', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/record-plans');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 547, 1225, 'GET', '/web/gb/record-plans/:id', '查询录像计划详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/record-plans/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 548, 12251, 'POST', '/web/gb/record-plans', '新增录像计划', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/record-plans');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 549, 12252, 'PUT', '/web/gb/record-plans/:id', '编辑录像计划', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/record-plans/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 550, 12253, 'DELETE', '/web/gb/record-plans/:id', '删除录像计划', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/record-plans/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 551, 12254, 'PUT', '/web/gb/record-plans/:id/channels', '关联录像计划通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/record-plans/:id/channels');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 552, 1231, 'GET', '/web/gb/alarms', '分页查询国标报警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/alarms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 553, 1231, 'GET', '/web/gb/alarms/:id', '查询国标报警详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/alarms/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 554, 12311, 'PUT', '/web/gb/alarms/:id/handle', '处理国标报警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/alarms/:id/handle');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 555, 12312, 'POST', '/web/gb/alarms/:id/snapshot', '报警关联抓图', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/alarms/:id/snapshot');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 556, 12313, 'DELETE', '/web/gb/alarms', '清空国标报警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/alarms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 557, 1232, 'GET', '/web/gb/positions/latest', '查询最新位置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/positions/latest');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 558, 1232, 'GET', '/web/gb/positions/realtime', '查询实时位置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/positions/realtime');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 559, 1232, 'GET', '/web/gb/positions/history', '查询历史轨迹', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/positions/history');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 560, 12321, 'POST', '/web/gb/positions/subscribe', '订阅移动位置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/positions/subscribe');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 561, 1241, 'GET', '/web/gb/media-servers', '查询媒体节点列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/media-servers');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 562, 1241, 'GET', '/web/gb/media-servers/:id', '查询媒体节点详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/media-servers/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 563, 1241, 'GET', '/web/gb/media-servers/:id/info', '查询媒体节点运行信息', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/media-servers/:id/info');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 564, 12411, 'POST', '/web/gb/media-servers', '新增媒体节点', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/media-servers');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 565, 12412, 'PUT', '/web/gb/media-servers/:id', '编辑媒体节点', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/media-servers/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 566, 12413, 'DELETE', '/web/gb/media-servers/:id', '删除媒体节点', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/media-servers/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 567, 12414, 'GET', '/web/gb/media-servers/:id/check', '媒体节点在线检查', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/media-servers/:id/check');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 568, 1242, 'GET', '/web/gb/stream-proxies', '分页查询拉流代理', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/stream-proxies');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 569, 12421, 'POST', '/web/gb/stream-proxies', '新增拉流代理', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/stream-proxies');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 570, 12422, 'PUT', '/web/gb/stream-proxies/:id', '编辑拉流代理', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/stream-proxies/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 571, 12423, 'DELETE', '/web/gb/stream-proxies/:id', '删除拉流代理', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/stream-proxies/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 572, 12424, 'POST', '/web/gb/stream-proxies/:id/start', '启动拉流', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/stream-proxies/:id/start');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 573, 12425, 'POST', '/web/gb/stream-proxies/:id/stop', '停止拉流', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/stream-proxies/:id/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 574, 1243, 'GET', '/web/gb/stream-pushes', '分页查询推流', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/stream-pushes');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 575, 12431, 'POST', '/web/gb/stream-pushes', '新增推流', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/stream-pushes');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 576, 12432, 'PUT', '/web/gb/stream-pushes/:id', '编辑推流', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/stream-pushes/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 577, 12433, 'DELETE', '/web/gb/stream-pushes/:id', '删除推流', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/stream-pushes/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 578, 12434, 'POST', '/web/gb/stream-pushes/:id/start', '启动推流', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/stream-pushes/:id/start');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 579, 12435, 'POST', '/web/gb/stream-pushes/:id/stop', '停止推流', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/stream-pushes/:id/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 580, 1244, 'GET', '/web/gb/sip-logs', '分页查询信令日志', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/sip-logs');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 581, 1244, 'GET', '/web/gb/sip-logs/stats', '信令日志统计', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/sip-logs/stats');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 582, 1244, 'GET', '/web/gb/sip-logs/:id', '查询信令日志详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/sip-logs/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 583, 1251, 'GET', '/web/gb/platforms', '分页查询上级平台', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/platforms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 584, 1251, 'GET', '/web/gb/platforms/:id', '查询上级平台详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/platforms/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 585, 12511, 'POST', '/web/gb/platforms', '新增上级平台', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/platforms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 586, 12512, 'PUT', '/web/gb/platforms/:id', '编辑上级平台', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/platforms/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 587, 12513, 'DELETE', '/web/gb/platforms/:id', '删除上级平台', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/platforms/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 588, 12516, 'PUT', '/web/gb/platforms/:id/groups', '设置级联推送分组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/platforms/:id/groups');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 589, 12517, 'PUT', '/web/gb/platforms/:id/regions', '设置级联推送区域', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/platforms/:id/regions');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 590, 1252, 'GET', '/web/gb/platforms/:id/channels', '分页查询级联通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/platforms/:id/channels');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 591, 12521, 'POST', '/web/gb/platforms/:id/channels', '添加级联通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/platforms/:id/channels');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 592, 12523, 'DELETE', '/web/gb/platforms/:id/channels', '移除级联通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/platforms/:id/channels');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 593, 12522, 'PUT', '/web/gb/platforms/:id/channels/:channelId', '定制级联通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/platforms/:id/channels/:channelId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 594, 1261, 'GET', '/web/gb/groups/tree', '查询业务分组树', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/groups/tree');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 595, 1261, 'GET', '/web/gb/groups', '查询业务分组列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/groups');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 596, 12611, 'POST', '/web/gb/groups', '新增业务分组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/groups');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 597, 12612, 'PUT', '/web/gb/groups/:id', '编辑业务分组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/gb/groups/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 598, 12613, 'DELETE', '/web/gb/groups/:id', '删除业务分组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/gb/groups/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 599, 1263, 'GET', '/web/settings/voice-agent', '读取语音Agent配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/settings/voice-agent');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 600, 1263, 'GET', '/web/settings/voice-agent/providers', '读取语音provider字典', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/settings/voice-agent/providers');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 601, 12631, 'PUT', '/web/settings/voice-agent', '保存语音Agent配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/settings/voice-agent');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 602, 12632, 'POST', '/web/settings/voice-agent/test', '语音Agent测试连接', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/settings/voice-agent/test');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 603, 12633, 'POST', '/web/settings/voice-agent/apply', '应用语音Agent配置', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/settings/voice-agent/apply');

-- 移动端国标接口（契约 §10.12）：aAuth 组同样挂载 casbinMW，
-- 因此这些路由必须进入 ac_menu_api，否则连超管也会被 403 拒绝。
-- 移动端没有按钮级菜单，沿用既有做法统一挂到移动端页面菜单 108。
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 604, 108, 'GET', '/app/gb/devices', '移动端分页查询国标设备', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/app/gb/devices');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 605, 108, 'GET', '/app/gb/channels', '移动端分页查询国标通道', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/app/gb/channels');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 606, 108, 'POST', '/app/gb/play/:channelId', '移动端开始实时播放', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/play/:channelId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 607, 108, 'POST', '/app/gb/play/:channelId/stop', '移动端停止实时播放', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/play/:channelId/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 608, 108, 'POST', '/app/gb/play/:channelId/snapshot', '移动端实时抓图', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/play/:channelId/snapshot');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 609, 108, 'POST', '/app/gb/ptz/:channelId', '移动端云台控制', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/ptz/:channelId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 610, 108, 'GET', '/app/gb/playback/records', '移动端查询录像文件', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/app/gb/playback/records');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 611, 108, 'POST', '/app/gb/playback', '移动端开始录像回放', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/playback');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 612, 108, 'POST', '/app/gb/playback/control', '移动端回放控制', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/playback/control');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 613, 108, 'POST', '/app/gb/playback/stop', '移动端停止录像回放', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/playback/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 614, 108, 'POST', '/app/gb/talk/:channelId/start', '移动端开始语音对讲', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/talk/:channelId/start');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 615, 108, 'POST', '/app/gb/talk/:channelId/stop', '移动端停止语音对讲', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/app/gb/talk/:channelId/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 616, 108, 'GET', '/app/gb/alarms', '移动端分页查询国标报警', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/app/gb/alarms');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 617, 12211, 'POST', '/web/gb/ptz/:channelId/presets/:presetId/goto', '调用预置位', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/ptz/:channelId/presets/:presetId/goto');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 618, 12116, 'POST', '/web/gb/devices/:id/diagnose', '设备可达性与 NAT 诊断', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/devices/:id/diagnose');

-- ── 12.6 角色授权：super_admin（角色 1）──────────────────────
--     国标菜单 / 按钮的 permission_code 以 gb: 前缀标识，
--     语音 Agent 按契约 §10.9 使用 voice:agent: 前缀，两者都要授权。
INSERT INTO ac_role_menu_rel (role_id, menu_id)
SELECT 1, m.id FROM ac_menu m
WHERE (m.permission_code LIKE 'gb:%' OR m.permission_code LIKE 'voice:agent:%')
  AND NOT EXISTS (SELECT 1 FROM ac_role_menu_rel r WHERE r.role_id = 1 AND r.menu_id = m.id);


-- ── 12.7 语音 provider 字典（ac_voice_provider）───────────────
--     只声明「有哪些 provider、各自需要哪些参数」，驱动前端动态表单。
--     具体参数值存 ac_thirdparty_config（Category='voice_agent'），不在此表。
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, 1, 'stt', 'local_whisper', '本地 Whisper', '[{"key":"model","label":"模型","type":"text","required":true},{"key":"language","label":"识别语言","type":"text","required":false},{"key":"endpoint","label":"服务地址","type":"text","required":false}]', '{"model":"base","language":"zh","endpoint":""}', '本地部署的 Whisper STT', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'stt' AND code = 'local_whisper');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 2, TRUE, 2, 'stt', 'openai', 'OpenAI Whisper', '[{"key":"api_key","label":"API Key","type":"password","required":true,"secret":true},{"key":"model","label":"模型","type":"text","required":true},{"key":"endpoint","label":"服务地址","type":"text","required":false},{"key":"language","label":"识别语言","type":"text","required":false}]', '{"model":"whisper-1","endpoint":"https://api.openai.com/v1","language":"zh"}', 'OpenAI 云端 STT', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'stt' AND code = 'openai');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 3, TRUE, 3, 'stt', 'deepgram', 'Deepgram', '[{"key":"api_key","label":"API Key","type":"password","required":true,"secret":true},{"key":"model","label":"模型","type":"text","required":false},{"key":"language","label":"识别语言","type":"text","required":false}]', '{"model":"nova-2","language":"zh"}', 'Deepgram 云端 STT', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'stt' AND code = 'deepgram');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 4, TRUE, 1, 'llm', 'anthropic', 'Anthropic Claude', '[{"key":"api_key","label":"API Key","type":"password","required":true,"secret":true},{"key":"model","label":"模型","type":"text","required":true},{"key":"system_prompt","label":"系统提示词","type":"textarea","required":false},{"key":"temperature","label":"温度","type":"number","required":false},{"key":"max_tokens","label":"最大输出长度","type":"number","required":false}]', '{"model":"claude-3-5-sonnet","system_prompt":"你是一个安防语音助手","temperature":0.7,"max_tokens":1024}', 'Anthropic Claude 对话模型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'llm' AND code = 'anthropic');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 5, TRUE, 2, 'llm', 'openai', 'OpenAI GPT', '[{"key":"api_key","label":"API Key","type":"password","required":true,"secret":true},{"key":"model","label":"模型","type":"text","required":true},{"key":"endpoint","label":"服务地址","type":"text","required":false},{"key":"system_prompt","label":"系统提示词","type":"textarea","required":false},{"key":"temperature","label":"温度","type":"number","required":false},{"key":"max_tokens","label":"最大输出长度","type":"number","required":false}]', '{"model":"gpt-4o-mini","endpoint":"https://api.openai.com/v1","system_prompt":"你是一个安防语音助手","temperature":0.7,"max_tokens":1024}', 'OpenAI 对话模型', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'llm' AND code = 'openai');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 6, TRUE, 1, 'tts', 'edge_tts', 'Edge TTS', '[{"key":"voice","label":"音色","type":"text","required":true},{"key":"speed","label":"语速","type":"number","required":false}]', '{"voice":"zh-CN-XiaoxiaoNeural","speed":1.0}', '微软 Edge 免费 TTS', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'tts' AND code = 'edge_tts');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 7, TRUE, 2, 'tts', 'openai', 'OpenAI TTS', '[{"key":"api_key","label":"API Key","type":"password","required":true,"secret":true},{"key":"model","label":"模型","type":"text","required":true},{"key":"voice","label":"音色","type":"text","required":true},{"key":"speed","label":"语速","type":"number","required":false}]', '{"model":"tts-1","voice":"alloy","speed":1.0}', 'OpenAI 云端 TTS', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'tts' AND code = 'openai');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 8, TRUE, 3, 'tts', 'azure', 'Azure TTS', '[{"key":"api_key","label":"API Key","type":"password","required":true,"secret":true},{"key":"region","label":"区域","type":"text","required":true},{"key":"voice","label":"音色","type":"text","required":true}]', '{"region":"eastasia","voice":"zh-CN-XiaoxiaoNeural"}', 'Azure 语音合成', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'tts' AND code = 'azure');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 9, TRUE, 1, 'vad', 'webrtcvad', 'WebRTC VAD', '[{"key":"aggressiveness","label":"灵敏度","type":"number","required":false}]', '{"aggressiveness":2}', '轻量级 VAD，无需模型文件', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'vad' AND code = 'webrtcvad');
INSERT INTO ac_voice_provider (id, active, sequence, category, code, name, param_schema, default_params, remark, created_by, created_dt, updated_by, updated_dt)
SELECT 10, TRUE, 2, 'vad', 'silero', 'Silero VAD', '[{"key":"threshold","label":"阈值","type":"number","required":false},{"key":"min_silence_ms","label":"最小静音时长(毫秒)","type":"number","required":false}]', '{"threshold":0.5,"min_silence_ms":300}', '基于神经网络的 VAD', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_voice_provider WHERE category = 'vad' AND code = 'silero');


-- ── 12.7b 默认流媒体节点（ac_gb_media_server）────────────────
--     没有这一行时：ZLM 的 10 个 hook 全部因「找不到匹配 secret 的节点」被拒，
--     表现为「ZLM 日志显示回调成功、平台侧却一条都没收到」——必须先有节点记录。
--
--     地址约定（本地开发形态：ZLM 在容器、backend 在宿主机）：
--       ip         = 127.0.0.1  + http_port   = 8082  → backend 调 ZLM API
--       stream_ip  = 127.0.0.1  + flv_port    = 8082  → 浏览器拉流
--     compose full 形态下应改为 ip='zlm' + http_port=80（容器内网），
--     或在后台「媒体节点」页面直接修改。
--
--     rtp_enable=FALSE 是**单端口模式**的标记（设计 §2.2）：
--     不调 openRtpServer，设备统一向 rtp_proxy_port 打流。
INSERT INTO ac_gb_media_server (id, active, name, server_id, type, ip, hook_ip, sdp_ip, stream_ip,
    http_port, flv_port, ws_flv_port, secret, rtp_enable, rtp_proxy_port,
    record_path, record_day, online, hook_alive_interval, default_server, auto_config,
    remark, created_by, created_dt, updated_by, updated_dt)
SELECT 1, TRUE, '本地 ZLMediaKit', 'ai-camera-zlm-01', 'zlm', '127.0.0.1', 'host.docker.internal', '127.0.0.1', '127.0.0.1',
    8082, 8082, 8082, 'aicamera-zlm-dev-secret-0123456789', FALSE, 10000,
    '/opt/media/bin/www/record', 7, FALSE, 15, TRUE, FALSE,
    'seed 默认节点，与 deploy/zlm/config.ini 的 mediaServerId 对应', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_gb_media_server WHERE server_id = 'ai-camera-zlm-01');

-- ── 12.7d 级联运行时动作（启动 / 停止 / 手动推送目录）────────
--     级联从「配置 CRUD」升级为可用后新增的 4 个端点。
--     status 挂在上级平台列表菜单（1251）下；其余三个是按钮型菜单。
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12518, '1251', 4, '启动级联', '', '', '', 'button', 'pc', 'gb:platform:cascade-start', '按钮', 6, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE permission_code = 'gb:platform:cascade-start');
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12519, '1251', 4, '停止级联', '', '', '', 'button', 'pc', 'gb:platform:cascade-stop', '按钮', 7, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE permission_code = 'gb:platform:cascade-stop');
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 12520, '1251', 4, '推送目录', '', '', '', 'button', 'pc', 'gb:platform:catalog-push', '按钮', 8, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE permission_code = 'gb:platform:catalog-push');

INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 619, 1251, 'GET', '/web/gb/platforms/cascade/status', '查询级联运行状态', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/gb/platforms/cascade/status');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 620, 12518, 'POST', '/web/gb/platforms/cascade/start', '启动级联服务', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/platforms/cascade/start');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 621, 12519, 'POST', '/web/gb/platforms/cascade/stop', '停止级联服务', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/platforms/cascade/stop');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 622, 12520, 'POST', '/web/gb/platforms/:id/catalog-push', '手动推送级联目录', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/gb/platforms/:id/catalog-push');

-- 新增按钮同样要授权给 super_admin，否则前端 permCodes 里没有对应权限码。
INSERT INTO ac_role_menu_rel (role_id, menu_id)
SELECT 1, m.id FROM ac_menu m
WHERE m.id IN (12518, 12519, 12520)
  AND NOT EXISTS (SELECT 1 FROM ac_role_menu_rel r WHERE r.role_id = 1 AND r.menu_id = m.id);

-- ── 12.7e IOT 管理权限数据（v0.3.0 新增）─────────────────────
--     设计依据：docs/需求文档/12-IOT管理模块.md
--     契约：docs/开发文档/后端开发/00-接口契约.md §10.14 / §10.15
--     表结构：docs/开发文档/数据库设计/sql/ddl/10-IOT管理.sql
--     权限码统一为 iot:<实体>:<动作>；按钮型菜单 path/component 为空。

-- 一级目录
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 15, NULL, 1, 'IOT 管理', '/iot', 'ClusterOutlined', '', 'directory', 'pc', 'iot:view', '目录', 15, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 15);

-- 3 个页面
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 151, '15', 2, '产品管理', '/iot/product', 'AppstoreOutlined', 'iot/ProductList', 'menu', 'pc', 'iot:product:list', '菜单', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 151);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 152, '15', 2, '服务组', '/iot/service-group', 'ClusterOutlined', 'iot/ServiceGroupList', 'menu', 'pc', 'iot:service-group:list', '菜单', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 152);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 153, '15', 2, '设备管理', '/iot/device', 'MobileOutlined', 'iot/IotDeviceList', 'menu', 'pc', 'iot:device:list', '菜单', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 153);

-- 产品按钮
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1511, '151', 3, '新增产品', '', '', '', 'button', 'pc', 'iot:product:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1511);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1512, '151', 3, '编辑产品', '', '', '', 'button', 'pc', 'iot:product:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1512);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1513, '151', 3, '删除产品', '', '', '', 'button', 'pc', 'iot:product:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1513);

-- 服务组按钮
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1521, '152', 3, '新增服务组', '', '', '', 'button', 'pc', 'iot:service-group:create', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1521);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1522, '152', 3, '编辑服务组', '', '', '', 'button', 'pc', 'iot:service-group:update', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1522);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1523, '152', 3, '删除服务组', '', '', '', 'button', 'pc', 'iot:service-group:delete', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1523);

-- 设备按钮
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1531, '153', 3, '设置产品', '', '', '', 'button', 'pc', 'iot:device:update', '按钮', 1, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1531);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1532, '153', 3, '新增账号', '', '', '', 'button', 'pc', 'iot:device:account:create', '按钮', 2, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1532);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1533, '153', 3, '编辑账号', '', '', '', 'button', 'pc', 'iot:device:account:update', '按钮', 3, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1533);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1534, '153', 3, '删除账号', '', '', '', 'button', 'pc', 'iot:device:account:delete', '按钮', 4, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1534);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1535, '153', 3, '激活设备', '', '', '', 'button', 'pc', 'iot:device:activate', '按钮', 5, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1535);
INSERT INTO ac_menu (id, parent_id, level, name, path, icon, component, type, platform, permission_code, description, sequence, active, hidden, created_by, created_dt, updated_by, updated_dt)
SELECT 1536, '153', 3, '重置密钥', '', '', '', 'button', 'pc', 'iot:device:secret-reset', '按钮', 6, TRUE, FALSE, 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu WHERE id = 1536);

-- ac_menu_api：Web 端 23 条端点（设备端 /iot/* 不走 Casbin，不登记）
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 700, 151, 'GET', '/web/iot/products', 'IOT产品分页列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/iot/products');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 701, 151, 'GET', '/web/iot/products/:id', 'IOT产品详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/iot/products/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 702, 1511, 'POST', '/web/iot/products', '新增IOT产品', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/iot/products');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 703, 1512, 'PUT', '/web/iot/products/:id', '更新IOT产品', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/iot/products/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 704, 1513, 'DELETE', '/web/iot/products/:id', '删除IOT产品', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/iot/products/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 705, 151, 'POST', '/web/iot/products/:id/share-qr', '生成产品分享二维码内容', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/iot/products/:id/share-qr');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 706, 151, 'GET', '/web/iot/products/:id/service-groups', '查询产品已绑定服务组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/iot/products/:id/service-groups');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 707, 1512, 'PUT', '/web/iot/products/:id/service-groups', '绑定产品服务组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/iot/products/:id/service-groups');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 708, 152, 'GET', '/web/iot/service-groups', 'IOT服务组分页列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/iot/service-groups');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 709, 152, 'GET', '/web/iot/service-groups/:id', 'IOT服务组详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/iot/service-groups/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 710, 1521, 'POST', '/web/iot/service-groups', '新增IOT服务组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/iot/service-groups');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 711, 1522, 'PUT', '/web/iot/service-groups/:id', '更新IOT服务组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/iot/service-groups/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 712, 1523, 'DELETE', '/web/iot/service-groups/:id', '删除IOT服务组', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/iot/service-groups/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 713, 1522, 'POST', '/web/iot/service-groups/:id/reset-credentials', '重置服务组凭据', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/iot/service-groups/:id/reset-credentials');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 714, 1522, 'PUT', '/web/iot/service-groups/:id/items', '整体替换服务组服务项', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/iot/service-groups/:id/items');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 715, 153, 'GET', '/web/iot/devices', 'IOT设备分页列表', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/iot/devices');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 716, 153, 'GET', '/web/iot/devices/:id', 'IOT设备详情', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'GET' AND path = '/web/iot/devices/:id');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 717, 1532, 'POST', '/web/iot/devices/:id/accounts', '新增设备账号', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/iot/devices/:id/accounts');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 718, 1533, 'PUT', '/web/iot/devices/:id/accounts/:accountId', '更新设备账号', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/iot/devices/:id/accounts/:accountId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 719, 1534, 'DELETE', '/web/iot/devices/:id/accounts/:accountId', '删除设备账号', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'DELETE' AND path = '/web/iot/devices/:id/accounts/:accountId');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 720, 1535, 'POST', '/web/iot/devices/:id/activate', '手动激活设备主账号', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/iot/devices/:id/activate');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 721, 1536, 'POST', '/web/iot/devices/:id/reset-secret', '重置设备接入密钥', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'POST' AND path = '/web/iot/devices/:id/reset-secret');
INSERT INTO ac_menu_api (id, menu_id, method, path, description, created_by, created_dt, updated_by, updated_dt)
SELECT 722, 1531, 'PUT', '/web/iot/devices/:id/product', '设置设备所属产品', 'system', CURRENT_TIMESTAMP, 'system', CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM ac_menu_api WHERE method = 'PUT' AND path = '/web/iot/devices/:id/product');

-- 授权给 super_admin（角色 1）
INSERT INTO ac_role_menu_rel (role_id, menu_id)
SELECT 1, m.id FROM ac_menu m
WHERE m.permission_code LIKE 'iot:%'
  AND NOT EXISTS (SELECT 1 FROM ac_role_menu_rel r WHERE r.role_id = 1 AND r.menu_id = m.id);

-- ── 12.8 重建 Casbin p 策略 + 序列对齐 ───────────────────────
--     ac_menu_api 是 p 策略的唯一来源，新增接口必须重新派生，
--     否则 super_admin 访问 /web/gb/* 与 /app/gb/* 都会被 403 拒绝。
DELETE FROM casbin_rule WHERE p_type = 'p' AND v0 = 'super_admin';
INSERT INTO casbin_rule (p_type, v0, v1, v2)
SELECT DISTINCT 'p', 'super_admin', path, method FROM ac_menu_api WHERE deleted_at IS NULL;

SELECT setval(pg_get_serial_sequence('ac_menu', 'id'),           COALESCE((SELECT MAX(id) FROM ac_menu), 1));
SELECT setval(pg_get_serial_sequence('ac_menu_api', 'id'),       COALESCE((SELECT MAX(id) FROM ac_menu_api), 1));
SELECT setval(pg_get_serial_sequence('ac_role_menu_rel', 'id'),  COALESCE((SELECT MAX(id) FROM ac_role_menu_rel), 1));
SELECT setval(pg_get_serial_sequence('ac_voice_provider', 'id'), COALESCE((SELECT MAX(id) FROM ac_voice_provider), 1));
SELECT setval(pg_get_serial_sequence('ac_gb_media_server', 'id'), COALESCE((SELECT MAX(id) FROM ac_gb_media_server), 1));
