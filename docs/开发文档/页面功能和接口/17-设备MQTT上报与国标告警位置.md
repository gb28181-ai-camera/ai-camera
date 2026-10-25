# 设备 MQTT 上报与国标告警/位置

> 版本：v0.6.0
> 最后更新：2026-10-25

## 1. 两类设备、两条上报链路

| 设备 | 接入 | 告警/位置链路 |
|:--|:--|:--|
| 安防相机 | GB28181 | SIP MESSAGE（MANSCDP `Notify/Alarm`、`Notify/MobilePosition`） |
| 适老化相机 | WebRTC / AIChat | MQTT `iot/<group>/device/<device_code>/report` |

两条链路最终都落到 `ac_alarm_record` / 位置历史，但协议域不同：

- GB28181：`ac_gb_alarm`、`ac_gb_mobile_position`（以 `gb_device_id` 为键）；
- MQTT：`ac_alarm_record`、`ac_iot_device_position`（以 `device_id` 为键），
  同时把最新电量/位置冗余到 `ac_device` 供列表展示。

## 2. MQTT 上报

### 2.1 话题与鉴权

| 方向 | 话题 |
|:--|:--|
| 设备 → 后端 | `iot/<group_code>/device/<device_code>/report` |
| 后端订阅 | `iot/+/device/+/report`（平台账号 `__platform__`） |

- 设备使用「产品 × MQTT 服务」用户名/密码连接 broker；
- broker 已把 ACL 限制在 `iot/<group_code>/**`；
- 后端再校验：设备启用 → 产品启用 → 产品绑定该服务组 → 服务组启用且类型为
  `webrtc` → 组内 MQTT 服务启用。

### 2.2 消息结构

统一 JSON，用 `type` 区分：

```json
{"type":"heartbeat","device_code":"CAM_001","ts":1792920804}

{"type":"battery","device_code":"CAM_001","battery":76,"voltage":"12.4","storage_free":5120}

{"type":"position","device_code":"CAM_001","longitude":120.1693,"latitude":30.2741,
 "speed":1.5,"direction":90,"altitude":10}

{"type":"alarm","device_code":"CAM_001","alarm_type_code":"fall","alarm_level":"high",
 "content":"检测到跌倒","occurred_dt":"2026-10-25 17:33:36"}
```

| type | 落库/更新 |
|:--|:--|
| `heartbeat` | `ac_device.last_beat_dt / status=online` |
| `battery` | `ac_device.battery/voltage/storage_free + last_beat_dt`，并写 `ac_device_history` |
| `position` | `ac_device.latitude/longitude`，并写 `ac_iot_device_position` |
| `alarm` | `ac_alarm_record`（按 `alarm_type_code` 找 `ac_alarm_type`），同时刷新心跳 |

时间戳 `ts` 支持秒/毫秒；缺省用服务端时间。

### 2.3 内置告警类型

| code | 名称 | 等级 |
|:--|:--|:--|
| `offline` | 离线告警 | high |
| `battery` | 电量告警 | medium |
| `traffic` | 流量告警 | medium |
| `fall` | 跌倒告警（适老化） | high |
| `sos` | 一键呼救（适老化） | high |

设备也可以使用管理员在「基础信息 → 告警类型」新增的 code。

### 2.4 联调

```bash
# 模拟器每 2s 上报电量/位置，每 12s 补一条跌倒告警
cd backend
go run ./tools/webrtcsim \
  --broker tcp://127.0.0.1:1883 \
  --group-code webrtc --device-code CAM_001 \
  --stream-file tools/gbsim/testdata/sample.h264 \
  --report-interval 2s
```

检查：

```sql
SELECT battery, voltage, latitude, longitude, last_beat_dt FROM ac_device WHERE device_code='CAM_001';
SELECT count(*) FROM ac_iot_device_position;
SELECT id, alarm_type_id, content, status FROM ac_alarm_record ORDER BY id DESC LIMIT 5;
```

## 3. 国标告警与位置（安防相机）

### 3.1 告警

设备发送：

```xml
<Notify>
  <CmdType>Alarm</CmdType>
  <SN>1</SN>
  <DeviceID>34020000001320000001</DeviceID>
  <AlarmPriority>1</AlarmPriority>
  <AlarmMethod>5</AlarmMethod>
  <AlarmTime>2026-10-25T17:33:36</AlarmTime>
  <AlarmDescription>移动侦测</AlarmDescription>
  <Longitude>120.1693</Longitude>
  <Latitude>30.2741</Latitude>
</Notify>
```

后端 `device.Manager.OnAlarm` 写入 `ac_gb_alarm`，Web「国标视频 → 报警」与
App「国标报警」可查询，SSE `/web/gb/alarms/stream` 实时推送。

### 3.2 位置

```xml
<Notify>
  <CmdType>MobilePosition</CmdType>
  <SN>2</SN>
  <DeviceID>34020000001320000001</DeviceID>
  <Time>2026-10-25T17:33:36</Time>
  <Longitude>120.1693</Longitude>
  <Latitude>30.2741</Latitude>
  <Speed>1.5</Speed>
  <Direction>90</Direction>
  <Altitude>10</Altitude>
</Notify>
```

写入 `ac_gb_mobile_position`；订阅入口：
`POST /web/gb/devices/:id/subscribe`（type=`mobile_position`）。

### 3.3 联调

```bash
cd backend
go run ./tools/gbsim \
  --device-id 34020000001320000001 \
  --sip-server 127.0.0.1:5060 --sip-domain 3402000000 \
  --password '***' --channels 2 \
  --alarm-interval 15s --position-interval 10s
```

- `--alarm-interval` 周期性发 `Notify/Alarm`（默认类型 5 移动侦测）；
- `--position-interval` 周期性发 `Notify/MobilePosition`，经纬度按周期小幅变化；
- 也可用 `--alarm-type` 指定其它国标报警类型码。

## 4. 已知限制

- MQTT 鉴权是产品级凭据，同组设备可以伪造彼此的 `device_code`；
  设备级隔离需要把 ACL 从 group 收窄到 device（后续演进）。
- 位置历史目前只落库，未做轨迹回放/围栏判断。
- 离线推送需要 App 集成个推 SDK 并上报 CID；未配置个推时只记录日志。
