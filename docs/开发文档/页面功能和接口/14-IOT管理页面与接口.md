# 14. IOT 管理页面与接口（v0.3.0）

> 版本：v0.3.0
> 最后更新：2026-10-24
> 接口契约：[后端开发/00-接口契约.md](../后端开发/00-接口契约.md) §10.14（Web）/ §10.15（设备端）/ §10.16（App）
> 数据库：[10-IOT管理.sql](../数据库设计/sql/ddl/10-IOT管理.sql)
> 设计依据：[docs/敏捷开发/v0.3.0 IOT与多协议接入/00-新增需求与设计.md](../../敏捷开发/v0.3.0%20IOT与多协议接入/00-新增需求与设计.md)

---

## 1. 页面清单

三个页面均位于 `frontend/src/admin/pages/iot/`，菜单由 `ac_menu` 驱动：

| 路由 | 页面组件 | 菜单 / 权限码 | E2E |
|---|---|---|---|
| `/iot/product` | `ProductList.tsx` | 151 / `iot:product:list` | `12a-iot-product.spec.ts` |
| `/iot/service-group` | `ServiceGroupList.tsx` | 152 / `iot:service-group:list` | `12b-iot-service-group.spec.ts` |
| `/iot/device` | `DeviceList.tsx` | 153 / `iot:device:list` | `12c-iot-device.spec.ts` |

一级目录：15 `/iot`，权限码 `iot:view`。

---

## 2. 产品管理 `/iot/product`

### 2.1 功能项

| 功能 | 说明 |
|---|---|
| 列表 | 产品名称、编码、分类、自动联网（4G）、配网方式、允许分享、绑定服务组数、启用状态 |
| 筛选 | 关键字（名称/编码/分类）、启用状态 |
| 新增 / 编辑 | 名称、编码（唯一）、分类、描述、排序、自动联网、配网方式（蓝牙/AP/二维码）、允许分享、启用 |
| 删除 | 软删除；已绑定设备的产品需先解绑设备 |
| 分享二维码 | 仅 `allow_share=true` 可用；生成 `iot_product_share` JSON 并持久化 `share_qr_content` |
| 选择服务组 | 新增/编辑产品时多选系统预置服务组，随产品保存（`service_group_ids`）；后端为组内每个服务生成产品级凭据 |
| 复制鉴权 | 弹窗按「服务组 → 服务」展示产品×服务明文用户名/密码、MQTT URL 与 TURN 信息；支持复制 JSON / 复制文本 / 密码显隐 |

### 2.2 接口

| 方法 | 路径 | 权限码 |
|---|---|---|
| GET | `/web/iot/products` | `iot:product:list` |
| GET | `/web/iot/products/:id` | `iot:product:list` |
| POST | `/web/iot/products` | `iot:product:create` |
| PUT | `/web/iot/products/:id` | `iot:product:update` |
| DELETE | `/web/iot/products/:id` | `iot:product:delete` |
| POST | `/web/iot/products/:id/share-qr` | `iot:product:update` |
| GET | `/web/iot/products/:id/credentials` | `iot:product:update` |
| GET | `/web/iot/products/:id/services` | `iot:product:list` |
| POST | `/web/iot/products/:id/services/:serviceId/reset-credentials` | `iot:product:update` |
| GET | `/web/iot/products/:id/service-groups` | `iot:product:list` |
| PUT | `/web/iot/products/:id/service-groups` | `iot:product:update` |

> `share-qr` 在 `allow_share=false` 时返回 400「该产品未开启分享」。
> `credentials` 是 Web 侧唯一返回明文产品×服务凭据的接口；
> 创建/更新产品时通过 `service_group_ids` 同步绑定并生成凭据。

---

## 3. 服务组 `/iot/service-group`

### 3.1 功能项

| 功能 | 说明 |
|---|---|
| 列表 | 名称、编码、类型、被引用、启用 |
| 筛选 | 关键字（名称/编码）、类型 |
| 固定三条 | 系统预置 gb28181 / webrtc / aichat；**不提供新增/删除**，编码与类型不可修改 |
| 编辑 | 仅名称、描述、启用状态 |
| 编辑服务 | Form.List 动态增删：编码（组内唯一）、名称、协议、host、port（0=默认）、path、extra（JSON 校验）、排序、启用 |
| 协议白名单 | `gb28181` → SIP/RTSP；`webrtc` → WebRTC/MQTT/RTSP/TURN；`aichat` → WebSocket |
| 引用锁定 | 被任一产品绑定后，编辑/编辑服务全部禁用；需先在产品上解绑 |
| 凭据 | 服务组不持有凭据；凭据在产品页按「产品×服务」维护 |

### 3.2 接口

| 方法 | 路径 | 权限码 |
|---|---|---|
| GET | `/web/iot/service-groups` | `iot:service-group:list` |
| GET | `/web/iot/service-groups/:id` | `iot:service-group:list` |
| POST | `/web/iot/service-groups` | 固定数据，恒返回 400 |
| PUT | `/web/iot/service-groups/:id` | `iot:service-group:update` |
| DELETE | `/web/iot/service-groups/:id` | 固定编码恒返回 400 |
| PUT | `/web/iot/service-groups/:id/services` | `iot:service-group:update` |

---

## 4. 设备管理 `/iot/device`

### 4.1 功能项

| 功能 | 说明 |
|---|---|
| 列表 | 设备名称、编号、所属产品、在线状态、激活状态、主账号、启用 |
| 筛选 | 关键字（名称/编号/位置）、所属产品、在线状态 |
| 设置产品 | 选择/清空产品；清空后设备不再下发服务组 |
| 激活设备 | 首次创建 owner 并返回一次性密码；重复点击提示「已激活」 |
| 账号管理 | 账号列表（owner/shared）、新增分享账号（密码可留空自动生成）、编辑、删除；owner 不可删除 |
| 服务授权 | 展示产品绑定的服务组与服务列表（OTA 下发的同一份数据，不含明文凭据） |
| 设备二维码 | `https://boticz.cn/qrDevice/<product_code>/<device_code>`；未绑定产品时后端 400 |
| 分享码 | 生成一次性 `https://boticz.cn/deviceShareToken/<token>`；产品 `allow_share=false` 时按钮禁用 |
| 撤销分享码 | 清空该设备所有未认领令牌 |
| 重置密钥 | 返回一次性 `device_secret` |
| 解绑设备 | 删除全部账号（含 owner）、清空激活状态；设备接入配置保持不变 |

### 4.2 接口

| 方法 | 路径 | 权限码 |
|---|---|---|
| GET | `/web/iot/devices` | `iot:device:list` |
| GET | `/web/iot/devices/:id` | `iot:device:list` |
| POST | `/web/iot/devices/:id/accounts` | `iot:device:account:create` |
| PUT | `/web/iot/devices/:id/accounts/:accountId` | `iot:device:account:update` |
| DELETE | `/web/iot/devices/:id/accounts/:accountId` | `iot:device:account:delete` |
| POST | `/web/iot/devices/:id/activate` | `iot:device:activate` |
| POST | `/web/iot/devices/:id/reset-secret` | `iot:device:secret-reset` |
| PUT | `/web/iot/devices/:id/product` | `iot:device:update` |
| POST | `/web/iot/devices/:id/qr` | `iot:device:update` |
| POST | `/web/iot/devices/:id/share-token` | `iot:device:account:create` |
| DELETE | `/web/iot/devices/:id/share-token` | `iot:device:account:delete` |
| POST | `/web/iot/devices/:id/unbind` | `iot:device:unbind` |

### 4.3 关键响应

```json
// GET /web/iot/devices/:id
{
  "device": { "id": 1, "device_code": "CAM_001", "product_id": 2,
              "device_secret": "****", "sip_password": "****", "activated_at": "..." },
  "accounts": [{ "id": 1, "username": "owner_xxx", "role": "owner", "status": "active" }],
  "service_groups": [{ "code": "webrtc", "type": "webrtc",
                       "services": [{ "code": "mqtt", "name": "MQTT 信令", "protocol": "MQTT",
                                      "host": "mqtt.example.com", "port": 1883 }] }],
  "qr_content": "https://boticz.cn/qrDevice/elderly_camera/CAM_001"
}
```

```json
// POST /web/iot/devices/:id/unbind
{ "device_id": 1, "accounts": 2 }
```

---

## 5. 设备端与 App 接口（摘要）

| 分组 | 方法 | 路径 | 鉴权 |
|---|---|---|---|
| 设备端 | POST | `/iot/ota` | `X-Device-Code` + `X-Device-Secret` |
| 设备端 | POST | `/iot/activate` | 同上 |
| App | POST | `/app/iot/devices/claim` | JWT + Casbin |
| App | POST | `/app/iot/devices/bind` | 同上 |
| App | POST | `/app/iot/devices/bind-qr` | 同上 |
| App | GET | `/app/iot/devices` | 同上 |
| App | GET | `/app/iot/devices/:id` | 同上 |
| App | GET | `/app/iot/devices/:id/services` | 同上 |
| App | POST | `/app/iot/devices/:id/share-token` | 同上（仅 owner） |
| App | GET | `/app/rtc/ice-servers` | 同上 |

- OTA 按 `service_groups[].services[]` 下发每个服务的产品级明文凭据，
  `device_secret` / `sip_password` 为空。
- App `/services` 结构相同；MQTT 使用该服务自己的用户名/密码。
- `/app/rtc/ice-servers` 未配置 TURN 时返回 `[]`，客户端按直连处理；
  设备 TURN 凭据走产品×TURN 服务并同步 coturn userdb。

---

## 6. E2E 类名与用例

| 页面 | Modal / 容器类名 | 用例 |
|---|---|---|
| 产品 | `e2e-iot-product-table`、`e2e-iot-product-modal`、`e2e-iot-product-share-modal`、`e2e-iot-product-groups-select`、`e2e-iot-credentials-modal`、`e2e-iot-credentials-services-table` | `12a` |
| 服务组 | `e2e-iot-service-group-table`、`e2e-iot-service-group-modal`、`e2e-iot-service-group-items-modal` | `12b` |
| 设备 | `e2e-iot-device-table`、`e2e-iot-device-product-modal`、`e2e-iot-device-account-modal`、`e2e-iot-device-account-table`、`e2e-iot-device-account-form-modal`、`e2e-iot-device-account-credential-modal`、`e2e-iot-device-services-modal`、`e2e-iot-device-services-table`、`e2e-iot-device-secret-modal`、`e2e-iot-device-qr-modal`、`e2e-iot-share-qr-modal` | `12c` |

- 公共夹具：`frontend/e2e/helpers/iot.ts`（造数、清理、弹窗关闭、非搜索型 Select）。
- `99-all-list-page.spec.ts` 覆盖三个 IOT 路由的渲染与菜单可达性。
- 冒烟顺序：先 `resetdb` + `initdb`，再启动测试后端（Casbin 策略在启动时加载）。
