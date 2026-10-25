package request

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）—— Wave 3 运维增强请求
// ════════════════════════════════════════════════════════════════

// ── 日志检索 ──────────────────────────────────────────────────

// GbLogQueryReq 内存日志检索。
//
// Level 采用「最低级别」语义：level=warn 返回 warn/error/fatal。
// 空值表示不过滤。Keyword 在 message 与结构化字段中做不区分大小写的子串匹配。
type GbLogQueryReq struct {
	PageReq
	Level   string `form:"level" json:"level" example:"warn"`
	Keyword string `form:"keyword" json:"keyword"`
}

// ── 指标历史 ──────────────────────────────────────────────────

// GbMetricsHistoryReq 指标历史范围。
type GbMetricsHistoryReq struct {
	// Range 仅支持 1h / 24h，缺省 1h。
	Range string `form:"range" json:"range" example:"1h"`
}

// ── 云端录像合并 ──────────────────────────────────────────────

// GbMergeTaskCreateReq 创建合并任务。
type GbMergeTaskCreateReq struct {
	// IDs 为 ac_gb_cloud_record 主键，顺序即合并顺序；服务端会按录像开始时间排序。
	IDs  []int64 `json:"ids" binding:"required,min=2"`
	Name string  `json:"name" example:"园区东门_20261025"`
}

// PageGbMergeTaskReq 合并任务分页筛选。
type PageGbMergeTaskReq struct {
	PageReq
	Status  string `form:"status" json:"status" example:"success"`
	Keyword string `form:"keyword" json:"keyword"`
}

// ── 免登录分享 ────────────────────────────────────────────────

// GbShareTokenCreateReq 创建分享令牌。
type GbShareTokenCreateReq struct {
	// TTLSeconds 缺省 3600，允许范围 [60, 86400]。
	TTLSeconds int    `json:"ttl_seconds" example:"3600"`
	Remark     string `json:"remark"`
}

// GbSharePlayReq 匿名分享点播。
type GbSharePlayReq struct {
	// Protocol 仅支持 webrtc/flv/hls，缺省 webrtc。
	Protocol string `json:"protocol" example:"webrtc"`
}

// ── 推流导入 ──────────────────────────────────────────────────

// GbStreamPushImportReq 推流导入（multipart 表单说明，文件字段名 file）。
//
// 支持 CSV（逗号分隔）与 TSV（制表符分隔）。表头可选，支持中英文别名：
//
//	name / 名称 / 通道名称
//	app / 应用名
//	stream / 流ID / 流名称
//	media_server_id / 媒体节点
//	push_url / 推流地址（rtmp://host:port/app/stream）
//	gb_channel_id / 通道编码
//	remark / 备注
//
// 无表头时按 name,app,stream,media_server_id,remark 的顺序解析。
type GbStreamPushImportReq struct {
	// File 通过 c.FormFile("file") 读取，Swagger 仅用于展示字段。
	File string `form:"file" json:"file"`
}
