# AI风险与AI监控 - 页面功能与接口

> 版本: v0.1.0
> 最后更新: 2026-09-21

## 1. 页面列表

| 序号 | 页面名称 | 路由 | 类型 | 说明 |
|:----:|---------|------|:----:|------|
| 1 | AI风险台账 | `/risk/ai-risk` | 列表页 | 全局风险档案：查看全部历史（含已处理）、总览指标、检测确认与处理 |
| 2 | AI监控 | `/risk/ai-monitor` | 监控页 | 选择区域与摄像头，视频与风险信息同屏联动 |

> **与「风险监控处置」的区别**：AI风险台账不默认筛选，保留全部历史并展示
> 总数/待处理/高风险/未读/今日新增/准确率等指标，用于追溯与统计；
> 「风险监控处置」是待处理工作台，默认 `status=pending`，支持自动刷新、
> 详情复核与设备监控跳转。

---

## 2. AI风险

### 2.1 页面功能

- **列表字段**：风险类型、风险图片、设备名称、设备编号、所属区域、设备位置、发生时间、风险等级、
  AI 置信度、检测准确、消息通知、状态、处理人、处理时间。
- **搜索条件**：风险类型（`risk_type_id`）、所属区域（`region_id`）、发生时间范围（`start_dt` ~ `end_dt`）、
  状态（`status`）、设备名称/编号（`keyword` / `device_code`）、风险等级（`risk_level`）、检测准确（`detect_accurate`）。
- **点选筛选**：点击列表中「风险类型」单元格，可直接筛选出该类型的所有数据；
  点击「设备名称」「设备编号」同理，效果与在搜索框中选择一致。
- **查看**：点击「查看」打开风险详情弹窗，展示风险图片、录像、置信度与处理信息；
  打开详情时该条消息通知由**未读转为已读**（`read = true`，记录 `read_dt`）。
- **检测准确确认**：这是风险处理的关键分支，在详情中或列表内直接操作：
  - 点「**否**」（误报）→ 该条数据**直接转为已处理状态**（`status = handled`），
    且消息通知转为已读，**无需再填写处理结果**；
  - 点「**是**」→ 消息通知转为已读，状态仍为未处理（`pending`），
    需继续点击「处理」填写处理结果与处理图片。
- **处理风险**：点击「处理」打开处理弹窗，填写处理结果（`handle_result`）与上传处理图片（`handle_image`），
  提交后状态由**未处理（`pending`）转为已处理（`handled`）**，同时记录处理人（`handler`）与处理时间（`handled_dt`）。
- **查看录像**：点击录像按钮播放该条风险关联的视频录像（`video_url`）。
- **查看实时视频**：点击实时视频按钮，打开弹窗播放该设备的当前实时画面。
- **删除**：点击「删除」并确认后删除该条数据；点「取消」则取消删除。
- **刷新频率**：可设置列表自动刷新频率，取值 **10 秒 / 30 秒 / 60 秒**，默认关闭自动刷新。
- **语音播报标记**：若该风险类型配置了语音，记录中的 `voice_played` 标记是否已播报。

### 2.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/web/risks` | 风险分页列表 |
| GET | `/web/risks/pending-count` | 未处理风险数 |
| GET | `/web/risks/summary` | 台账总览：总数/待处理/高风险/未读/准确分布/今日新增 |
| GET | `/web/risks/:id` | 风险详情 |
| PUT | `/web/risks/:id/read` | 标记消息通知已读 |
| PUT | `/web/risks/:id/accurate` | 确认检测是否准确 |
| PUT | `/web/risks/:id/handle` | 处理风险 |
| DELETE | `/web/risks/:id` | 删除风险 |

### 2.3 接口数据格式

**GET /web/risks**

请求参数：

```json
{
  "page": 1,
  "page_size": 20,
  "keyword": "CAM-0001",
  "risk_type_id": 3,
  "region_id": 1,
  "device_id": 5,
  "device_code": "CAM-0001",
  "risk_level": "high",
  "status": "pending",
  "detect_accurate": "",
  "read": false,
  "start_dt": "2026-09-01 00:00:00",
  "end_dt": "2026-09-21 23:59:59"
}
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|:----:|---|
| `page` / `page_size` | int | 否 | 分页参数，默认 1 / 20 |
| `keyword` | string | 否 | 匹配设备名称或设备编号 |
| `risk_type_id` | int64 | 否 | 风险类型 |
| `region_id` | int64 | 否 | 所属区域 |
| `device_id` | int64 | 否 | 设备 |
| `device_code` | string | 否 | 设备编号精确匹配 |
| `risk_level` | string | 否 | `high` / `medium` / `low` |
| `status` | string | 否 | `pending`（未处理）/ `handled`（已处理） |
| `detect_accurate` | string | 否 | `yes` / `no`，留空表示未确认 |
| `read` | bool | 否 | 消息通知是否已读 |
| `start_dt` / `end_dt` | string | 否 | 发生时间范围（`occurred_dt`） |

响应数据：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 1001,
        "risk_type_id": 3,
        "device_id": 5,
        "device_code": "CAM-0001",
        "region_id": 1,
        "location": "北门入口左侧立杆",
        "image_url": "/uploads/risk/20260921/a.jpg",
        "video_url": "/uploads/risk/20260921/a.mp4",
        "risk_level": "high",
        "detect_score": 0.93,
        "occurred_dt": "2026-09-21T10:30:00+08:00",
        "detect_accurate": "yes",
        "accurate_by": "admin",
        "accurate_dt": "2026-09-21T10:32:00+08:00",
        "read": true,
        "read_dt": "2026-09-21T10:32:00+08:00",
        "status": "pending",
        "handle_result": "",
        "handle_image": "",
        "handler": "",
        "handled_dt": null,
        "voice_played": true,
        "risk_type": { "id": 3, "name": "烟火识别", "code": "fire", "level": "high" },
        "device": { "id": 5, "name": "北门摄像头", "device_code": "CAM-0001" },
        "region": { "id": 1, "name": "北门区域", "code": "REG001" },
        "created_dt": "2026-09-21T10:30:00+08:00"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 20
  }
}
```

> **注意**：`risk_type.name`、`device.name`、`region.name` 为关联名称字段，列表必须展示名称而非仅 ID。

**GET /web/risks/pending-count**

请求参数：无。

响应数据：

```json
{ "code": 200, "message": "success", "data": 25 }
```

**GET /web/risks/:id**

请求参数：路径参数 `id`（风险记录 ID）。

响应数据（`RRiskRecord`）：单条风险对象，结构同列表条目。

> **注意**：详情端点返回图片与录像地址，前端弹窗中的「查看录像」「查看实时视频」分别取
> `video_url` 与所属设备的 `stream_url`。

**PUT /web/risks/:id/read**

请求参数：无请求体（标记当前记录的消息通知为已读）。

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

**PUT /web/risks/:id/accurate**

请求参数：

```json
{ "accurate": "no" }
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|:----:|---|
| `accurate` | string | 是 | `yes` 检测准确 / `no` 误报 |

响应行为：

| 取值 | 状态变化 | 消息通知 |
|---|---|---|
| `no` | `status` 直接置为 `handled`（误报，无需处理） | `read = true` |
| `yes` | `status` 保持 `pending`（待处理） | `read = true` |

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

**PUT /web/risks/:id/handle**

请求参数：

```json
{
  "handle_result": "已到场确认并扑灭，现场无人员受伤",
  "handle_image": "/uploads/handle/20260921/b.jpg"
}
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|:----:|---|
| `handle_result` | string | 是 | 处理结果描述 |
| `handle_image` | string | 否 | 处理图片地址，由 `POST /web/upload` 上传后回填 |

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

> **注意**：处理完成后 `status = handled`，`handler` 取当前登录用户名，`handled_dt` 为服务器时间。
> 已处理的记录重复提交处理会返回 400。

**DELETE /web/risks/:id**

请求参数：路径参数 `id`。

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

---

## 3. AI监控

### 3.1 页面功能

- **区域与摄像头选择**：选择区域与摄像头后，页面同时展示**实时视频监控画面**与该设备产生的**风险信息列表**。
- **风险信息联动**：风险列表按 `device_id` 过滤，仅展示当前所选摄像头的风险记录；视频窗口与列表同屏刷新。
- **查看风险详情**：点击任一风险信息，打开详情弹窗查看风险图片、置信度、发生时间等信息。
- **去处理**：详情弹窗中的「去处理」按钮跳转到「风险监控」页（`/risk/monitor`）并携带该风险 ID，
  在那边完成检测确认与处理。跳转本身不调用接口，处理动作复用 `/web/risks` 组端点。
- **风险快速确认**：可在本页直接对当前风险执行「检测准确」确认（同 AI风险页语义）。
- **加入监控**：可切换当前设备的监控开关，控制其是否出现在「设备监控」列表。
- **自动刷新**：视频窗口与风险列表按同一节奏轮询，轮询频率可设为 10 秒 / 30 秒 / 60 秒。

> **注意**：本页与「AI风险」页共用同一套 `/web/risks` 端点，
> 区别在于本页恒带 `device_id` 过滤且强制先选区域与摄像头。

### 3.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/web/basic/regions/options` | 区域下拉选项 |
| GET | `/web/devices/options` | 设备下拉选项 |
| GET | `/web/devices/:id` | 设备详情（取 `stream_url` 与在线状态） |
| PUT | `/web/devices/:id/monitor` | 加入/移出实时监控 |
| GET | `/web/risks` | 当前摄像头的风险分页列表 |
| GET | `/web/risks/:id` | 风险详情 |
| PUT | `/web/risks/:id/read` | 标记消息通知已读 |
| PUT | `/web/risks/:id/accurate` | 确认检测是否准确 |
| PUT | `/web/risks/:id/handle` | 处理风险 |

### 3.3 接口数据格式

**GET /web/basic/regions/options** / **GET /web/devices/options** / **GET /web/devices/:id** /
**PUT /web/devices/:id/monitor**

请求与响应格式同 `06-实时监控页面与接口.md`。

**GET /web/risks**

请求参数（AI监控页固定携带 `device_id`）：

```json
{
  "page": 1,
  "page_size": 10,
  "region_id": 1,
  "device_id": 5,
  "status": "pending"
}
```

响应数据：同「2.3 接口数据格式 → GET /web/risks」。

**PUT /web/risks/:id/accurate**

请求参数：

```json
{ "accurate": "yes" }
```

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

**PUT /web/risks/:id/handle**

请求参数：

```json
{
  "handle_result": "已现场核实并处理完毕",
  "handle_image": "/uploads/handle/20260921/c.jpg"
}
```

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

---

## 4. 风险状态流转

```
设备识别风险 → 落库
  status = pending, read = false, detect_accurate = ""
        │
        ├── 确认「检测准确 = 否」
        │      → status = handled, read = true（误报结案）
        │
        └── 确认「检测准确 = 是」
               → read = true, status = pending（待处理）
                     │
                     └── 点击「处理」填写处理结果 + 处理图片
                            → status = handled, handler = 当前用户, handled_dt = now
```

| 状态/字段 | 取值 | 说明 |
|---|---|---|
| `status` | `pending` | 未处理 |
| `status` | `handled` | 已处理 |
| `detect_accurate` | `yes` / `no` / 空 | 检测准确 / 误报 / 待确认 |
| `read` | true / false | 消息通知是否已读，影响侧边栏红点 |
