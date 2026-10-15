# APP移动端 - 页面功能与接口

> 版本: v0.1.0
> 最后更新: 2026-09-21

## 1. 页面列表

移动端（Flutter）全部接口挂载在 `/app` 前缀下，与管理后台的 `/web` 前缀并列。
两者共用同一份 Service 与数据权限逻辑，但菜单按 `platform = mobile` 过滤。

| 序号 | 页面名称 | 路由 | 类型 | 说明 |
|:----:|---------|------|:----:|------|
| 1 | 登录 | `/app/login` | 表单页 | 移动端账号密码登录 |
| 2 | 首页 | `/app/home` | 看板 | 关键指标与待办概览 |
| 3 | 待办 | `/app/todos` | 列表页 | 未处理风险 / 告警 / 巡检任务聚合 |
| 4 | 风险 | `/app/risks` | 列表页 | 风险列表、详情、检测确认与处理 |
| 5 | 告警 | `/app/alarms` | 列表页 | 告警列表与处理 |
| 6 | 巡检 | `/app/inspect-tasks` | 列表页 | 我收到的巡检任务与提交 |
| 7 | 设备 | `/app/devices` | 列表页 | 设备台账与运行状态 |
| 8 | 通知 | `/app/notifications` | 列表页 | 消息通知（告警/风险/巡检） |
| 9 | 我的 | `/app/profile` | 表单页 | 个人资料、修改密码、退出登录 |

> **注意**：移动端**没有**写接口以外的管理端点——区域、设备、固件、角色等配置类数据
> 只在管理后台维护，移动端仅做只读查看与业务处理。

---

## 2. 登录

### 2.1 页面功能

- **账号密码登录**：输入登录名与密码，调用 `POST /app/auth/login` 获取 JWT。
- **与管理后台一致**：账号体系、密码校验、禁用判定完全一致，Token 可跨端使用。
- **菜单加载**：登录成功后调用 `GET /app/menus` 拉取 `platform = mobile` 的菜单树，驱动底部导航与首页宫格。
- **本机绑定**：登录后可绑定个推 `client_id`，用于接收 `notify_type = app` 的推送（绑定动作由客户端完成）。

### 2.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/app/auth/login` | 移动端登录 |
| GET | `/app/menus` | 移动端菜单树 |

### 2.3 接口数据格式

**POST /app/auth/login**

请求参数：

```json
{
  "user_name": "zhangsan",
  "password": "Zs@2026"
}
```

响应数据（`RLogin`，结构同管理后台登录）：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user_id": 2,
    "name": "张三",
    "dept_id": 2,
    "data_perm": "dept",
    "cross_dept_ids": "",
    "permissions": ["risk:monitor:handle", "inspect:task:finish"]
  }
}
```

**GET /app/menus**

请求参数：无（`platform` 固定为 `mobile`）。

响应数据（`RAppMenuList`）：

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": 41,
      "active": true,
      "sequence": 10,
      "name": "风险",
      "parent_id": null,
      "level": 1,
      "path": "/app/risks",
      "icon": "alert",
      "component": "RiskListPage",
      "type": "menu",
      "platform": "mobile",
      "permission_code": "risk:monitor:list",
      "hidden": false
    }
  ]
}
```

---

## 3. 首页

### 3.1 页面功能

- **指标概览**：展示今日风险数、今日告警数、待处理总数、设备在线率等关键指标。
- **待办概览**：展示待处理风险、待处理告警、待巡检任务的条数，点击跳转到「待办」页。
- **快捷入口**：风险 / 告警 / 巡检 / 设备 四个快捷入口，点击进入对应列表。
- **未读角标**：底部导航「通知」项展示未读消息数红点。
- **下拉刷新**：下拉重新拉取首页聚合数据。

### 3.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/app/home` | 首页聚合数据 |
| GET | `/app/todos` | 待办聚合列表 |
| GET | `/app/notifications` | 通知列表（用于未读角标） |

### 3.3 接口数据格式

**GET /app/home**

请求参数：无（用户与数据权限取自 Token）。

响应数据（`RAppHome`）：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "today_risk": 12,
    "today_alarm": 3,
    "pending_risk": 25,
    "pending_alarm": 7,
    "pending_inspect": 4,
    "device_summary": {
      "total": 120,
      "online": 98,
      "offline": 22,
      "online_rate": 81.7
    },
    "latest_risks": [
      {
        "id": 1001,
        "risk_type_name": "烟火识别",
        "risk_level": "high",
        "device_name": "北门摄像头",
        "region_name": "北门区域",
        "occurred_dt": "2026-09-21T10:30:00+08:00",
        "status": "pending"
      }
    ]
  }
}
```

> **注意**：`latest_risks[].risk_type_name`、`device_name`、`region_name` 均为**关联名称字段**，
> 移动端首页直接展示名称，无需二次请求。

**GET /app/todos**

请求参数：无。

响应数据（`RAppTodoList`）：

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "category": "risk",
      "title": "待处理风险",
      "count": 25,
      "ref_type": "risk",
      "path": "/app/risks"
    },
    {
      "category": "alarm",
      "title": "待处理告警",
      "count": 7,
      "ref_type": "alarm",
      "path": "/app/alarms"
    },
    {
      "category": "inspect",
      "title": "待巡检任务",
      "count": 4,
      "ref_type": "inspect",
      "path": "/app/inspect-tasks"
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `category` | string | `risk` / `alarm` / `inspect` |
| `title` | string | 待办分组标题 |
| `count` | int64 | 待办数量 |
| `path` | string | 点击跳转的路由 |

---

## 4. 待办

### 4.1 页面功能

- **聚合展示**：将待处理风险、待处理告警、待巡检任务三类待办按分组聚合展示。
- **分组折叠**：每个分组可展开/折叠，展示该分组的前 N 条明细。
- **点击跳转**：点击某条待办跳转到对应的详情页（风险详情 / 告警列表 / 巡检任务详情）。
- **数量同步**：分组标题右侧显示该分组的待办总数，处理后数量实时刷新。
- **下拉刷新**。

> **注意**：待办数据来自 `GET /app/todos`，明细数据由各业务模块的列表接口提供。

### 4.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/app/todos` | 待办聚合列表 |
| GET | `/app/risks` | 待处理风险明细 |
| GET | `/app/alarms` | 待处理告警明细 |
| GET | `/app/inspect-tasks` | 待巡检任务明细 |

### 4.3 接口数据格式

**GET /app/todos**

格式见「3.3 接口数据格式 → GET /app/todos」。

**GET /app/risks**（待办页固定携带 `status = pending`）

请求参数：

```json
{ "page": 1, "page_size": 10, "status": "pending" }
```

响应数据：同「5.3 接口数据格式」。

---

## 5. 风险

### 5.1 页面功能

- **列表字段**：风险图片缩略图、风险类型、风险等级、设备名称、所属区域、发生时间、状态、消息通知。
- **搜索条件**：风险类型（`risk_type_id`）、状态（`status`）、所属区域（`region_id`）、发生时间范围。
- **筛选 Tab**：全部 / 未处理 / 已处理，对应 `status` 传空 / `pending` / `handled`。
- **查看详情**：点击列表项进入详情页，展示大图、风险等级、AI 置信度、设备与位置、发生时间；
  进入详情时该消息通知**由未读转为已读**。
- **检测准确确认**：详情页提供「是 / 否」两个按钮：
  - 点「**否**」→ 该条数据**直接转为已处理**，消息通知转为已读（误报结案）；
  - 点「**是**」→ 消息通知转为已读，状态保持未处理，可继续处理。
- **处理风险**：点「处理」填写处理结果并拍照/选择处理图片，提交后状态**由未处理转为已处理**。
- **图片上传**：处理图片先调用 `POST /web/upload` 上传，得到 `url` 后回填 `handle_image`。
- **上拉加载更多**：分页加载，滚动到底部自动加载下一页。

### 5.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/app/risks` | 风险分页列表 |
| GET | `/app/risks/:id` | 风险详情 |
| PUT | `/app/risks/:id/accurate` | 确认检测是否准确 |
| PUT | `/app/risks/:id/handle` | 处理风险 |
| POST | `/web/upload` | 上传处理图片 / 风险图片 |

### 5.3 接口数据格式

**GET /app/risks**

请求参数：

```json
{
  "page": 1,
  "page_size": 10,
  "risk_type_id": 3,
  "region_id": 1,
  "status": "pending",
  "start_dt": "2026-09-01 00:00:00",
  "end_dt": "2026-09-21 23:59:59"
}
```

响应数据（`RPageRiskRecord`，与管理后台同结构）：

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
        "detect_accurate": "",
        "read": false,
        "status": "pending",
        "handle_result": "",
        "handler": "",
        "handled_dt": null,
        "risk_type": { "id": 3, "name": "烟火识别", "code": "fire", "level": "high" },
        "device": { "id": 5, "name": "北门摄像头", "device_code": "CAM-0001" },
        "region": { "id": 1, "name": "北门区域", "code": "REG001" }
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

**GET /app/risks/:id**

请求参数：路径参数 `id`（风险记录 ID）。

响应数据（`RRiskRecord`）：单条风险对象，结构同列表条目，额外带 `handle_image` / `accurate_by` / `accurate_dt`。

**PUT /app/risks/:id/accurate**

请求参数：

```json
{ "accurate": "no" }
```

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

**PUT /app/risks/:id/handle**

请求参数：

```json
{
  "handle_result": "已到场确认并处理完毕",
  "handle_image": "/uploads/handle/20260921/f.jpg"
}
```

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

---

## 6. 告警

### 6.1 页面功能

- **列表字段**：告警类型、告警等级、设备名称、设备编号、所属区域、设备位置、告警内容、告警时间、状态。
- **搜索条件**：告警类型（`alarm_type_id`）、状态（`status`）、所属区域（`region_id`）、操作时间范围。
- **筛选 Tab**：全部 / 未处理 / 已处理。
- **处理告警**：点击「处理」填写处理结果并上传处理图片，提交后状态**由未处理转为已处理**。
- **查看详情**：点击列表项查看告警详细信息（含冗余的告警内容 `content` 与设备位置）。
- **上拉加载更多**。

> **注意**：移动端告警模块**没有**「检测准确」操作——该字段只属于风险记录。

### 6.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/app/alarms` | 告警分页列表 |
| PUT | `/app/alarms/:id/handle` | 处理告警 |
| POST | `/web/upload` | 上传处理图片 |

### 6.3 接口数据格式

**GET /app/alarms**

请求参数：

```json
{
  "page": 1,
  "page_size": 10,
  "alarm_type_id": 2,
  "region_id": 1,
  "status": "pending",
  "start_dt": "2026-09-01 00:00:00",
  "end_dt": "2026-09-21 23:59:59"
}
```

响应数据（`RPageAlarmRecord`）：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 2001,
        "alarm_type_id": 2,
        "device_id": 5,
        "device_code": "CAM-0001",
        "region_id": 1,
        "location": "北门入口左侧立杆",
        "alarm_level": "high",
        "content": "设备电量低于 20%，当前 18%",
        "occurred_dt": "2026-09-21T10:31:00+08:00",
        "read": false,
        "read_dt": null,
        "status": "pending",
        "handle_result": "",
        "handler": "",
        "handled_dt": null,
        "alarm_type": { "id": 2, "name": "电量告警", "code": "battery", "level": "high" },
        "device": { "id": 5, "name": "北门摄像头", "device_code": "CAM-0001" },
        "region": { "id": 1, "name": "北门区域", "code": "REG001" }
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

**PUT /app/alarms/:id/handle**

请求参数：

```json
{
  "handle_result": "已更换电池，设备恢复正常",
  "handle_image": "/uploads/handle/20260921/g.jpg"
}
```

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

---

## 7. 巡检

### 7.1 页面功能

- **任务列表**：仅展示**指派给当前登录用户**（`inspector_id = 当前用户`）的巡检任务。
- **列表字段**：任务名称、所属区域、巡检位置、巡检内容、巡检时间、状态。
- **筛选 Tab**：待巡检（`published`）/ 已完成（`finished`）。
- **任务详情**：查看任务名称、所属区域、巡检位置、巡检内容与计划巡检时间。
- **完成巡检**：点击「完成巡检」，填写**处理结果**（`result`）并上传**处理结果图**（`result_image`），
  提交后任务 `status = finished` 并从任务列表移除，同时生成一条**巡检记录**
  （管理后台「巡检记录」页可见）。
- **消息通知**：任务被指派时生成一条 `category = inspect` 的系统通知，处理完成后通知状态同步更新。

> **注意**：`PUT /app/inspect-tasks/:id/finish` 是**巡检记录的唯一产生入口**，
> 管理后台不提供完成操作。

### 7.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/app/inspect-tasks` | 我的巡检任务分页列表 |
| GET | `/app/inspect-tasks/:id` | 巡检任务详情 |
| PUT | `/app/inspect-tasks/:id/finish` | 完成巡检并生成记录 |
| POST | `/web/upload` | 上传处理结果图 |

### 7.3 接口数据格式

**GET /app/inspect-tasks**

请求参数：

```json
{
  "page": 1,
  "page_size": 10,
  "status": "published",
  "region_id": 1,
  "start_dt": "2026-09-01 00:00:00",
  "end_dt": "2026-09-30 23:59:59"
}
```

响应数据（`RPageInspectTask`，`inspector_id` 由后端强制为当前用户）：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 301,
        "active": true,
        "name": "北门区域周巡检",
        "region_id": 1,
        "location": "北门入口及周边立杆",
        "inspector_id": 2,
        "content": "检查设备外观、供电、镜头遮挡与线缆固定情况",
        "plan_dt": "2026-09-22T09:00:00+08:00",
        "status": "published",
        "read": false,
        "remark": "",
        "region": { "id": 1, "name": "北门区域", "code": "REG001" }
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

**GET /app/inspect-tasks/:id**

请求参数：路径参数 `id`（任务 ID）。

响应数据（`RInspectTask`）：单条巡检任务对象，结构同列表条目。

**PUT /app/inspect-tasks/:id/finish**

请求参数：

```json
{
  "result": "设备外观完好，供电正常，镜头已清洁",
  "result_image": "/uploads/inspect/20260922/b.jpg"
}
```

| 参数 | 类型 | 必填 | 说明 |
|---|---|:----:|---|
| `result` | string | 是 | 处理结果描述 |
| `result_image` | string | 否 | 处理结果图地址，由 `POST /web/upload` 上传后回填 |

响应数据（`RInspectRecord`，返回新生成的巡检记录）：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 501,
    "task_id": 301,
    "task_name": "北门区域周巡检",
    "region_id": 1,
    "location": "北门入口及周边立杆",
    "inspector_id": 2,
    "inspector": "张三",
    "content": "检查设备外观、供电、镜头遮挡与线缆固定情况",
    "plan_dt": "2026-09-22T09:00:00+08:00",
    "result": "设备外观完好，供电正常，镜头已清洁",
    "result_image": "/uploads/inspect/20260922/b.jpg",
    "handled_dt": "2026-09-22T09:25:00+08:00",
    "status": "finished"
  }
}
```

> **注意**：完成动作是**幂等保护**的——任务已为 `finished` 时重复提交返回 400。
> 记录中的 `inspector` 为提交时的姓名快照。

---

## 8. 设备

### 8.1 页面功能

- **列表字段**：设备名称、设备编号、所属区域、设备位置、运行状态、电量、剩余存储、最近心跳时间。
- **搜索条件**：运行状态（`status`）、所属区域（`region_id`）、设备名称/编号关键字（`keyword`）。
- **设备详情**：查看设备完整信息，包括固件版本、模型版本、系统版本、设备模式、工作模式、
  信息上传间隔、物联网卡卡号、经纬度位置（可跳转地图）。
- **在线状态标识**：`online` 绿色、`offline` 灰色。
- **只读**：移动端不提供设备新增/编辑/删除，配置类操作只在管理后台进行。
- **上拉加载更多**。

### 8.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/app/devices` | 设备分页列表 |
| GET | `/app/devices/:id` | 设备详情 |

### 8.3 接口数据格式

**GET /app/devices**

请求参数：

```json
{
  "page": 1,
  "page_size": 10,
  "keyword": "CAM-0001",
  "region_id": 1,
  "status": "online"
}
```

响应数据（`RPageDevice`）：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 5,
        "name": "北门摄像头",
        "device_code": "CAM-0001",
        "region_id": 1,
        "location": "北门入口左侧立杆",
        "latitude": "30.274150",
        "longitude": "120.155150",
        "firmware_id": 2,
        "firmware_ver": "1.3.0",
        "model_id": 3,
        "model_ver": "3.1.0",
        "system_ver": "v2.1.5",
        "work_mode": "normal",
        "device_mode": "full",
        "info_interval": 60,
        "iot_card_id": 4,
        "status": "online",
        "battery": 86,
        "voltage": "3.92",
        "storage_total": 32768,
        "storage_free": 12040,
        "last_beat_dt": "2026-09-21T10:31:00+08:00",
        "monitor_enabled": true,
        "region": { "id": 1, "name": "北门区域", "code": "REG001" },
        "iot_card": { "id": 4, "name": "移动物联卡A", "card_no": "89860012345678901234" },
        "firmware": { "id": 2, "name": "主板固件", "version": "1.3.0" },
        "ai_model": { "id": 3, "name": "烟火识别模型", "version": "3.1.0" }
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

**GET /app/devices/:id**

请求参数：路径参数 `id`（设备 ID）。

响应数据（`RDevice`）：单条设备对象，结构同列表条目。

---

## 9. 通知

### 9.1 页面功能

- **消息类型**：统一展示设备告警（`category = alarm`）、风险通知（`risk`）、巡检通知（`inspect`）三类消息。
- **筛选 Tab**：全部 / 告警 / 风险 / 巡检，对应 `category` 传空 / `alarm` / `risk` / `inspect`。
- **查看消息**：点击消息标记该条为已读（`PUT /app/notifications/:id/read`），
  并按 `ref_type` / `ref_id` 跳转到对应业务详情页。
- **未读红点**：底部导航「通知」项显示未读数量红点，数据来自列表接口返回的未读统计。
- **下拉刷新 + 上拉加载更多**。

> **注意**：通知按 `user_id` 隔离，用户只能看到与操作自己的消息。
> 通知频率由「设备告警设置」「风险通知设置」控制，详见 `11-升级管理与告警设置页面与接口.md`。

### 9.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/app/notifications` | 通知分页列表 |
| PUT | `/app/notifications/:id/read` | 标记单条已读 |

### 9.3 接口数据格式

**GET /app/notifications**

请求参数：

```json
{
  "page": 1,
  "page_size": 10,
  "category": "risk",
  "read": false,
  "keyword": ""
}
```

响应数据（`RPageNotification`）：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 3001,
        "user_id": 2,
        "category": "risk",
        "title": "风险通知",
        "content": "北门摄像头识别到烟火，请及时处理",
        "ref_type": "risk",
        "ref_id": 1001,
        "read": false,
        "read_dt": null,
        "created_dt": "2026-09-21T10:30:05+08:00"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

> **注意**：`ref_type` 与 `ref_id` 是跳转依据：
> `ref_type = risk` 跳 `/app/risks/:ref_id`，`alarm` 跳 `/app/alarms`，`inspect` 跳 `/app/inspect-tasks/:ref_id`。

**PUT /app/notifications/:id/read**

请求参数：路径参数 `id`（通知 ID），无请求体。

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

---

## 10. 我的

### 10.1 页面功能

- **个人资料**：展示与修改姓名、邮箱、手机号、头像；登录名与所属部门不可自行修改。
- **头像上传**：调用 `POST /web/upload` 上传图片后回填 `avatar`。
- **修改密码**：输入原密码与新密码，原密码错误时拒绝修改；修改成功后需重新登录。
- **清除缓存 / 关于**：客户端本地功能，不涉及后端接口。
- **退出登录**：清除本地 Token 并跳回登录页。

### 10.2 接口列表

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/app/profile` | 获取个人资料 |
| PUT | `/app/profile` | 修改个人资料 |
| PUT | `/app/profile/password` | 修改密码 |
| POST | `/web/upload` | 上传头像 |

### 10.3 接口数据格式

**GET /app/profile**

请求参数：无。

响应数据（`RUser`，结构与管理后台个人中心一致）：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 2,
    "active": true,
    "name": "张三",
    "user_name": "zhangsan",
    "email": "zhangsan@example.com",
    "phone": "13900000001",
    "avatar": "/uploads/avatar/zhangsan.png",
    "dept_id": 2,
    "post_id": 5,
    "is_admin": false,
    "last_login": "2026-09-21T09:12:33+08:00",
    "remark": "",
    "dept": { "id": 2, "name": "华东大区", "code": "EAST" },
    "post": { "id": 5, "name": "巡检员", "code": "inspector" },
    "roles": [{ "id": 2, "name": "区域管理员", "code": "region_admin" }]
  }
}
```

**PUT /app/profile**

请求参数：

```json
{
  "name": "张三",
  "email": "zhangsan@example.com",
  "phone": "13900000001",
  "avatar": "/uploads/avatar/zhangsan.png",
  "remark": ""
}
```

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

**PUT /app/profile/password**

请求参数：

```json
{
  "old_password": "Zs@2026",
  "new_password": "Zs@2027"
}
```

响应数据：

```json
{ "code": 200, "message": "success", "data": null }
```

---

## 11. 移动端接口总表

| 分组 | 方法 | 路径 | 说明 |
|---|---|---|---|
| 认证 | POST | `/app/auth/login` | 登录 |
| 认证 | GET | `/app/menus` | 移动端菜单树 |
| 首页 | GET | `/app/home` | 首页聚合数据 |
| 首页 | GET | `/app/todos` | 待办聚合 |
| 风险 | GET | `/app/risks` | 风险列表 |
| 风险 | GET | `/app/risks/:id` | 风险详情 |
| 风险 | PUT | `/app/risks/:id/handle` | 处理风险 |
| 风险 | PUT | `/app/risks/:id/accurate` | 确认检测是否准确 |
| 告警 | GET | `/app/alarms` | 告警列表 |
| 告警 | PUT | `/app/alarms/:id/handle` | 处理告警 |
| 巡检 | GET | `/app/inspect-tasks` | 我的巡检任务 |
| 巡检 | GET | `/app/inspect-tasks/:id` | 巡检任务详情 |
| 巡检 | PUT | `/app/inspect-tasks/:id/finish` | 完成巡检并生成记录 |
| 设备 | GET | `/app/devices` | 设备列表 |
| 设备 | GET | `/app/devices/:id` | 设备详情 |
| 通知 | GET | `/app/notifications` | 通知列表 |
| 通知 | PUT | `/app/notifications/:id/read` | 标记已读 |
| 我的 | GET | `/app/profile` | 个人资料 |
| 我的 | PUT | `/app/profile` | 修改资料 |
| 我的 | PUT | `/app/profile/password` | 修改密码 |

> **注意**：移动端共 20 个业务端点，全部挂在 `/app` 前缀下，需携带 JWT。
> 图片统一通过 `/web/upload` 上传，音频通过 `/web/upload/audio` 上传。
