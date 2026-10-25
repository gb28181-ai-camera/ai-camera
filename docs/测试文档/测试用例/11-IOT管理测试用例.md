# IOT 管理测试用例

> 版本: v0.3.0
> 最后更新: 2026-10-24
> 覆盖模块: IOT 产品、服务组/服务项、设备账号与激活、OTA、设备/分享二维码、
> 凭据复制与脱敏、后台解绑、MQTT 鉴权/ACL、TURN 凭据、App 绑定

## 核心测试要点

> IOT 模块的测试重点是**凭据边界**与**绑定生命周期**：
> 组凭据只在 OTA/App 服务接口返回明文，Web 读接口必须脱敏；
> 分享令牌必须一次性且可撤销；设备解绑必须能收回全部账号。

| # | 测试要点 | 测试方法 / 手段 | 状态 |
|:--:|:---|:---|:---:|
| 1 | 产品 CRUD、编码唯一、配网方式校验 | `TestProductCRUD` | 🟡 |
| 2 | 产品关闭分享时拒绝生成分享码 | `TestProductShareQRRequiresAllowShare` + E2E `12a` | 🟡 |
| 3 | 产品×服务凭据自动生成、读接口脱敏、重置一次性明文 | `TestProductCRUD`、`TestCredentialsPlaintextAndMaskBoundary` + E2E `12a` | 🟡 |
| 4 | 协议白名单（gb28181/webrtc/aichat，webrtc 含 TURN）与服务编码唯一 | `TestServiceGroupProtocolWhitelist` + E2E `12b` | 🟡 |
| 5 | 产品绑定服务组整体替换并同步产品服务凭据 | `TestProductServiceBinding` + E2E `12a` | 🟡 |
| 6 | 设备筛选、详情、`device_secret` 与 `sip_password` 脱敏 | `TestDeviceFilteringAndDetail`、`TestDeviceReadMasksSensitiveCredentials` | 🟡 |
| 7 | 激活幂等、主账号不可删除 | `TestDeviceActivationIdempotent`、`TestSharedAccount` + E2E `12c` | 🟡 |
| 8 | 设备 OTA 只返回启用组/服务，下发产品×服务明文凭据，清空设备密钥 | `TestDeviceOTA` | 🟡 |
| 9 | 未绑定产品不生成二维码（400），详情 `qr_content` 为空 | `TestDeviceQRRequiresProduct`、`TestDeviceQRAndDetail` | 🟡 |
| 10 | 分享令牌一次性、撤销后不可认领 | `TestShareTokenClaimOnce` + E2E `12c` | 🟡 |
| 11 | App 认领/绑定：owner/shared、幂等、用户隔离 | `TestAppBindOwnerIdempotentShared`、`TestAppDeviceListAndScope` | 🟡 |
| 12 | 扫码绑定必须产品编码匹配 | `TestAppBindByQR` | 🟡 |
| 13 | SIP 首绑：写入、幂等、异编码冲突、产品不匹配 | `TestBindBySIP` | 🟡 |
| 14 | 后台解绑：删除全部账号、清空激活、保留接入配置、幂等 | `TestUnbindDevice` + E2E `12c` | 🟡 |
| 15 | 账号新增/编辑不接受调用方 `share_token` | DTO 移除 + E2E `12c` 账号流程 | 🟡 |
| 16 | MQTT 鉴权/ACL、平台账号隔离 | `TestMqttAuthenticate`、`TestMqttACL` | 🟡 |
| 17 | TURN REST 凭据签名向量与 RPC fail-closed | `service/turn` 单测、`handler/rpc` 单测 | 🟡 |
| 18 | 产品×TURN 凭据同步 coturn userdb | `TestTurnUserSync`（新增） | 🟡 |
| 19 | ICE 未配置时返回空数组 | `TestIceServers` | 🟡 |
| 20 | 契约路由与注册一致（Web 29 + App 8） | `TestRouteTable_MatchesContract` | 🟡 |
| 21 | 三个 IOT 页面 CRUD/弹窗/权限码 | Playwright `12a`/`12b`/`12c` | 🟡 |
| 22 | 服务组固定三条、禁止增删、编码/类型不可改 | `TestServiceGroupCreateAndDeleteForbidden` + E2E `12b` | 🟡 |
| 23 | 被产品引用的服务组只读，解绑后恢复可编辑 | `TestServiceGroupLockedWhenReferenced` + E2E `12b` | 🟡 |

## 端到端用例映射

| spec | 覆盖 |
|---|---|
| `frontend/e2e/smoke/12a-iot-product.spec.ts` | 种子产品、CRUD、选择服务组、分享二维码、产品×服务明文复制、权限码 |
| `frontend/e2e/smoke/12b-iot-service-group.spec.ts` | 固定三条、禁止增删、服务编辑、协议白名单、编码只读、引用锁定 |
| `frontend/e2e/smoke/12c-iot-device.spec.ts` | 产品筛选、服务授权、分享开关、激活、账号管理、二维码、分享码撤销、重置密钥、解绑 |
| `frontend/e2e/smoke/99-all-list-page.spec.ts` | 三个 IOT 路由渲染与菜单可达性 |

> 执行顺序：`resetdb` → `initdb` → 启动测试后端（Casbin 策略启动时加载）→
> `npx playwright test --project=smoke --workers=1`。
