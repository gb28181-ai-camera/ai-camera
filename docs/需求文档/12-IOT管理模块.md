# 12. IOT 管理模块

> 版本：v0.3.0
> 接口契约：[docs/开发文档/后端开发/00-接口契约.md](../开发文档/后端开发/00-接口契约.md) §10.14（Web）/ §10.15（设备端）
> 数据库：[docs/开发文档/数据库设计/sql/ddl/10-IOT管理.sql](../开发文档/数据库设计/sql/ddl/10-IOT管理.sql)

## 1. 模块目标

以「产品」为聚合根管理 AI 摄像头的接入配置：产品决定设备的联网/配网方式、
可分享策略以及可用服务组；设备通过 OTA 拉取产品配置与服务凭据，
通过激活创建主账号。设备数据**复用 `ac_device`**，不引入第二套设备体系。

## 2. 功能清单

### 2.1 产品管理

- 产品基础信息：名称、编码（唯一）、分类、描述、排序、启用状态。
- 联网与配网：
  - `auto_network`：是否自动联网（4G 设备）；
  - `net_config_mode`：`bluetooth`（蓝牙配网）/ `ap`（AP 配网）/ `qrcode`（二维码配网）。
- 分享策略：`allow_share` 是否允许分享；`share_qr_content` 保存分享二维码内容
  （JSON 字符串，由「生成/刷新」动作写入）。
- 服务组配置：一个产品可绑定多个服务组（整体替换），设备 OTA 时据此下发。

### 2.2 服务管理

服务组 / 服务两级：

**服务组是系统预置的三条固定数据**，编码与类型一致，不开放新增/删除，
编码与类型不可修改：

| 服务组编码 | 允许的服务协议 |
|---|---|
| `gb28181` | SIP、RTSP |
| `webrtc` | WebRTC、MQTT、RTSP、TURN |
| `aichat` | WebSocket |

**引用锁定**：服务组一旦被任一产品绑定，即进入只读状态——禁止编辑名称/描述、
修改服务、删除；必须先在产品上解除绑定。

**凭据是产品 × 服务级**（`ac_iot_product_service`）：

- 产品选择服务组后，平台为组内每个服务自动生成独立的 `username` / `password`；
- 密码 16 位，字符集排除易混淆的 `0 O 1 l I`；
- Web 读接口密码脱敏为 `****`；「重置凭据」按产品×服务返回一次性明文；
- 服务字段：编码（组内唯一）、名称、协议、host、port、path、extra、排序、启用。

### 2.3 设备与账号

- 设备列表：数据源 `ac_device`，支持关键字与产品筛选；显示所属产品。
- **独立 GB28181 设备**：没有产品编码/设备编码的国标设备不绑定任何产品，
  只按 `gb_device_id` 走国标白名单；产品绑定是可选项，OTA 对无产品设备返回空服务组。
- 设备详情：设备信息 + 账号列表 + 已授权服务组（含服务列表）。
- 账号：
  - 主账号（`owner`）：由「激活」创建，一台设备最多一个；
  - 分享账号（`shared`）：管理端可增删改，可生成分享令牌。
- 密码只存 bcrypt 哈希；明文仅在创建/激活响应中返回一次。
- 主账号不允许删除。

### 2.4 设备端

- `POST /iot/ota`：返回产品信息、设备信息、服务组授权配置（含明文组凭据）。
- `POST /iot/activate`：激活设备主账号，返回一次性明文密码；重复激活 409。
- 鉴权头：`X-Device-Code` + `X-Device-Secret`。`device_secret` 在平台重置后
  明文展示一次；未配置密钥时 OTA/激活返回 401。

## 3. 关键设计决策

1. **设备复用 `ac_device`**：设备已被风险、告警、巡检、升级、国标等模块引用，
   新建 IOT 设备表会产生两套主键与两套生命周期。仅追加
   `product_id` / `device_secret` / `activated_at` 三列。
2. **凭据是产品 × 服务级**：同一服务组下不同产品、同一产品的不同服务都有独立账号；
   既避免设备级 N 倍维护，又允许 MQTT/TURN 等外部服务按产品分配账号。
3. **服务组固定三条、不开放增删**：编码即业务分类，避免分类膨胀与协议白名单漂移。
4. **设备端不走 JWT**：设备无用户会话；`device_secret` 一机一份、可单独轮换，
   泄露影响面最小。
5. **OTA 返回明文服务凭据**：设备已完成 `device_secret` 鉴权，脱敏会导致无法登录；
   Web 端读接口始终脱敏。
6. **TURN 用 coturn SQL userdb**：产品×TURN 凭据写入 `turnusers_lt`，
   coturn 原生查询；不维护 coturn 私有分支。

## 4. 权限码

| 权限码 | 说明 |
|---|---|
| `iot:product:list` | 产品列表/详情 |
| `iot:product:create` | 新增产品 |
| `iot:product:update` | 编辑产品、分享二维码、绑定服务组 |
| `iot:product:delete` | 删除产品 |
| `iot:service-group:list` | 服务组列表/详情 |
| `iot:service-group:create` | 保留（服务组固定预置，接口恒拒绝新增） |
| `iot:service-group:update` | 编辑服务组、替换服务 |
| `iot:service-group:delete` | 保留（固定编码不允许删除） |
| `iot:device:list` | 设备列表/详情 |
| `iot:device:update` | 设置设备所属产品 |
| `iot:device:account:create` | 新增设备账号 |
| `iot:device:account:update` | 编辑设备账号 |
| `iot:device:account:delete` | 删除设备账号 |
| `iot:device:activate` | 手动激活设备 |
| `iot:device:secret-reset` | 重置设备接入密钥 |
| `iot:device:unbind` | 后台解绑设备（删除全部账号并清除激活状态） |
