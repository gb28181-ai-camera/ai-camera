package voiceagent

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	svcgb "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	sysservice "github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// MCP 协议版本（设计 §4）。
const mcpProtocolVersion = "2024-11-05"

// MCP 扩展错误码（工具调用身份/权限问题）。
const (
	mcpErrUnauthorized = -32001
	mcpErrDenied       = -32003
)

// MCPService 内部 MCP 服务端（JSON-RPC 2.0）。
type MCPService struct {
	provider ConfigProvider
	tools    *repository.VoiceMcpToolRepo
	perms    *repository.VoiceToolPermissionRepo
	logs     *repository.VoiceMcpCallLogRepo
	signer   *ContextSigner
}

// NewMCPService 构造 MCP 服务。
func NewMCPService(provider ConfigProvider) *MCPService {
	return &MCPService{
		provider: provider,
		tools:    repository.NewVoiceMcpToolRepo(),
		perms:    repository.NewVoiceToolPermissionRepo(),
		logs:     repository.NewVoiceMcpCallLogRepo(),
		signer:   NewContextSigner(ContextSecret()),
	}
}

// CheckInternalToken 校验 /internal/* 的内部令牌。
//
// 供 MCP 与记忆接口共用；令牌来源与 MCP 一致（环境变量优先，其次配置表）。
func CheckInternalToken(ctx context.Context, authHeader string) error {
	cfg := BuildRuntimeConfig(nil)
	if p := DefaultConfigProvider(); p != nil {
		if c, err := p.Runtime(ctx); err == nil && c != nil {
			cfg = c
		}
	}
	if strings.TrimSpace(cfg.InternalMCPToken) == "" {
		return errors.New("未配置 INTERNAL_MCP_TOKEN")
	}
	if !bearerEqual(authHeader, cfg.InternalMCPToken) {
		return errors.New("内部令牌无效")
	}
	return nil
}

// Handle 处理一次 JSON-RPC 请求，返回 HTTP 状态码与响应体。
//
// authHeader 为 Authorization；contextHeader 为 X-Voice-Context。
// 协议错误一律返回 HTTP 200 + JSON-RPC error（MCP 约定），
// 鉴权失败返回 401 / 503。
func (s *MCPService) Handle(
	ctx context.Context,
	body []byte,
	authHeader, contextHeader, ip string,
) (int, []byte) {
	cfg := s.runtime(ctx)
	if !cfg.MCPEnabled {
		return 503, mustJSON(request.JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &request.JSONRPCError{Code: request.JSONRPCInternalError, Message: "MCP 未启用"},
		})
	}
	if strings.TrimSpace(cfg.InternalMCPToken) == "" {
		return 503, mustJSON(request.JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &request.JSONRPCError{Code: request.JSONRPCInternalError, Message: "未配置 INTERNAL_MCP_TOKEN"},
		})
	}
	if !bearerEqual(authHeader, cfg.InternalMCPToken) {
		return 401, mustJSON(request.JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &request.JSONRPCError{Code: mcpErrUnauthorized, Message: "内部令牌无效"},
		})
	}

	var req request.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return 200, mustJSON(request.JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &request.JSONRPCError{Code: request.JSONRPCParseError, Message: "JSON 解析失败"},
		})
	}
	if req.ID == nil {
		req.ID = json.RawMessage("null")
	}

	switch req.Method {
	case "initialize":
		_ = s.tools.EnsureBuiltin(ctx, builtinTools())
		return 200, mustJSON(request.JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]interface{}{
				"protocolVersion": mcpProtocolVersion,
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":      map[string]interface{}{"name": "ai-camera", "version": "v0.6.0"},
			},
		})

	case "tools/list":
		_ = s.tools.EnsureBuiltin(ctx, builtinTools())
		items, err := s.tools.ListEnabled(ctx)
		if err != nil {
			return 200, rpcError(req.ID, request.JSONRPCInternalError, "读取工具失败: "+err.Error())
		}
		tools := make([]map[string]interface{}, 0, len(items))
		for _, t := range items {
			tools = append(tools, map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": rawOrEmpty(t.InputSchema),
			})
		}
		return 200, mustJSON(request.JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]interface{}{"tools": tools},
		})

	case "tools/call":
		return 200, s.handleToolCall(ctx, req, contextHeader, ip)

	default:
		return 200, rpcError(req.ID, request.JSONRPCMethodNotFound, "不支持的方法: "+req.Method)
	}
}

// handleToolCall 执行工具调用并写审计。
func (s *MCPService) handleToolCall(
	ctx context.Context,
	req request.JSONRPCRequest,
	contextHeader, ip string,
) []byte {
	var params request.McpToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil || strings.TrimSpace(params.Name) == "" {
		return rpcError(req.ID, request.JSONRPCInvalidParams, "tools/call 参数无效")
	}
	ident, err := s.signer.Verify(contextHeader)
	if err != nil {
		return rpcError(req.ID, mcpErrUnauthorized, "X-Voice-Context 无效: "+err.Error())
	}
	if params.Arguments == nil {
		params.Arguments = map[string]interface{}{}
	}
	// 身份只能来自签名上下文，禁止 LLM 通过参数覆盖。
	if bad := forbiddenIdentityArgs(params.Arguments); len(bad) > 0 {
		return rpcError(req.ID, request.JSONRPCInvalidParams,
			"工具参数不允许携带身份字段: "+strings.Join(bad, ","))
	}

	// tools/call 可能早于 tools/list 到达（Pipecat 重启后直接调用），
	// 因此这里同样确保内置工具已落库。
	_ = s.tools.EnsureBuiltin(ctx, builtinTools())

	tool, err := s.tools.GetByName(ctx, params.Name)
	if err != nil || tool == nil || !tool.Enabled {
		return rpcError(req.ID, request.JSONRPCInvalidParams, "工具不存在或未启用: "+params.Name)
	}

	allowed, reason := s.checkPermission(ctx, tool, ident)
	if !allowed {
		s.writeLog(ctx, ident, tool, params.Arguments, "denied", reason, 0, ip, isConfirmed(params.Arguments))
		return toolError(req.ID, "权限不足: "+reason)
	}
	if requiresConfirm(tool) && !isConfirmed(params.Arguments) {
		s.writeLog(ctx, ident, tool, params.Arguments, "denied", "需要 confirm=true", 0, ip, false)
		return toolError(req.ID, "该工具为写操作，必须携带 confirm=true 才能执行")
	}

	start := time.Now()
	result, toolErr := s.execute(ctx, ident, tool, params.Arguments)
	elapsed := time.Since(start).Milliseconds()
	if toolErr != nil {
		s.writeLog(ctx, ident, tool, params.Arguments, "error", toolErr.Error(), elapsed, ip, isConfirmed(params.Arguments))
		return toolError(req.ID, toolErr.Error())
	}
	s.writeLog(ctx, ident, tool, params.Arguments, "ok", summarize(result), elapsed, ip, isConfirmed(params.Arguments))
	return mustJSON(request.JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": result}},
			"isError": false,
		},
	})
}

// execute 工具分发。
func (s *MCPService) execute(ctx context.Context, ident *request.ChatContext, tool *model.VoiceMcpTool, args map[string]interface{}) (string, error) {
	switch tool.Name {
	case "device.list":
		return s.toolDeviceList(ctx, ident, args)
	case "device.status":
		return s.toolDeviceStatus(ctx, ident, args)
	case "alarm.query":
		return s.toolAlarmQuery(ctx, ident, args)
	case "risk.query":
		return s.toolRiskQuery(ctx, ident, args)
	case "ptz.control":
		return s.toolPtzControl(ctx, ident, args)
	case "notify.send":
		return s.toolNotifySend(ctx, ident, args)
	default:
		return "", fmt.Errorf("工具未实现: %s", tool.Name)
	}
}

// ── 工具实现 ──────────────────────────────────────────────────

func (s *MCPService) toolDeviceList(ctx context.Context, ident *request.ChatContext, args map[string]interface{}) (string, error) {
	limit := clampInt(argInt(args, "limit", 20), 1, 100)
	q := bootstrap.DB().WithContext(ctx).Model(&model.Device{}).
		Where("(active IS NULL OR active = true)")
	q = s.applyDeviceScope(q, ident)
	if kw := strings.TrimSpace(argString(args, "keyword")); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(name ILIKE ? OR device_code ILIKE ?)", like, like)
	}
	if group := strings.TrimSpace(argString(args, "service_group")); group != "" {
		q = q.Where(`product_id IN (
			SELECT psg.product_id FROM ac_iot_product_service_group psg
			 JOIN ac_iot_service_group g ON g.id = psg.service_group_id
			WHERE psg.deleted_at IS NULL AND g.code = ?)`, group)
	}
	var rows []model.Device
	if err := q.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return "", err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, d := range rows {
		out = append(out, map[string]interface{}{
			"id":           d.ID,
			"device_code":  d.DeviceCode,
			"name":         d.Name,
			"status":       d.Status,
			"battery":      d.Battery,
			"product_id":   d.ProductID,
			"region_id":    d.RegionID,
			"last_beat_dt": d.LastBeatDT,
		})
	}
	return jsonText(map[string]interface{}{"total": len(out), "items": out})
}

func (s *MCPService) toolDeviceStatus(ctx context.Context, ident *request.ChatContext, args map[string]interface{}) (string, error) {
	var dev *model.Device
	if id := argInt(args, "device_id", 0); id > 0 {
		q := bootstrap.DB().WithContext(ctx).Where("id = ?", id)
		q = s.applyDeviceScope(q, ident)
		var d model.Device
		if err := q.First(&d).Error; err != nil {
			return "", errors.New("设备不存在或无权访问")
		}
		dev = &d
	} else if code := strings.TrimSpace(argString(args, "device_code")); code != "" {
		q := bootstrap.DB().WithContext(ctx).Where("device_code = ?", code)
		q = s.applyDeviceScope(q, ident)
		var d model.Device
		if err := q.First(&d).Error; err != nil {
			return "", errors.New("设备不存在或无权访问")
		}
		dev = &d
	} else {
		return "", errors.New("device_id 或 device_code 至少提供一个")
	}
	return jsonText(map[string]interface{}{
		"id":           dev.ID,
		"device_code":  dev.DeviceCode,
		"name":         dev.Name,
		"status":       dev.Status,
		"battery":      dev.Battery,
		"voltage":      dev.Voltage,
		"storage_free": dev.StorageFree,
		"last_beat_dt": dev.LastBeatDT,
		"online_dt":    dev.OnlineDT,
		"offline_dt":   dev.OfflineDT,
	})
}

func (s *MCPService) toolAlarmQuery(ctx context.Context, ident *request.ChatContext, args map[string]interface{}) (string, error) {
	hours := clampInt(argInt(args, "hours", 24), 1, 720)
	limit := clampInt(argInt(args, "limit", 20), 1, 100)
	q := bootstrap.DB().WithContext(ctx).Model(&model.AlarmRecord{}).
		Where("occurred_dt >= ?", time.Now().Add(-time.Duration(hours)*time.Hour))
	q = s.applyRecordScope(q, ident, "ac_alarm_record.device_id", "ac_alarm_record.dept_id")
	if id := argInt(args, "device_id", 0); id > 0 {
		q = q.Where("ac_alarm_record.device_id = ?", id)
	}
	var rows []struct {
		model.AlarmRecord
		AlarmTypeName string `gorm:"column:alarm_type_name"`
	}
	if err := q.Select("ac_alarm_record.*, at.name AS alarm_type_name").
		Joins("LEFT JOIN ac_alarm_type at ON at.id = ac_alarm_record.alarm_type_id").
		Order("ac_alarm_record.occurred_dt DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return "", err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"id":              r.ID,
			"device_id":       r.DeviceID,
			"device_code":     r.DeviceCode,
			"alarm_type_id":   r.AlarmTypeID,
			"alarm_type_name": r.AlarmTypeName,
			"alarm_level":     r.AlarmLevel,
			"content":         r.Content,
			"status":          r.Status,
			"occurred_dt":     r.OccurredDT,
		})
	}
	return jsonText(map[string]interface{}{"total": len(out), "items": out})
}

func (s *MCPService) toolRiskQuery(ctx context.Context, ident *request.ChatContext, args map[string]interface{}) (string, error) {
	hours := clampInt(argInt(args, "hours", 24), 1, 720)
	limit := clampInt(argInt(args, "limit", 20), 1, 100)
	q := bootstrap.DB().WithContext(ctx).Model(&model.RiskRecord{}).
		Where("occurred_dt >= ?", time.Now().Add(-time.Duration(hours)*time.Hour))
	q = s.applyRecordScope(q, ident, "ac_risk_record.device_id", "ac_risk_record.dept_id")
	if id := argInt(args, "device_id", 0); id > 0 {
		q = q.Where("ac_risk_record.device_id = ?", id)
	}
	var rows []struct {
		model.RiskRecord
		RiskTypeName string `gorm:"column:risk_type_name"`
	}
	if err := q.Select("ac_risk_record.*, rt.name AS risk_type_name").
		Joins("LEFT JOIN ac_risk_type rt ON rt.id = ac_risk_record.risk_type_id").
		Order("ac_risk_record.occurred_dt DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return "", err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]interface{}{
			"id":             r.ID,
			"device_id":      r.DeviceID,
			"device_code":    r.DeviceCode,
			"risk_type_id":   r.RiskTypeID,
			"risk_type_name": r.RiskTypeName,
			"risk_level":     r.RiskLevel,
			"detect_score":   r.DetectScore,
			"status":         r.Status,
			"occurred_dt":    r.OccurredDT,
		})
	}
	return jsonText(map[string]interface{}{"total": len(out), "items": out})
}

func (s *MCPService) toolPtzControl(ctx context.Context, ident *request.ChatContext, args map[string]interface{}) (string, error) {
	channelID := int64(argInt(args, "channel_id", 0))
	if channelID <= 0 {
		return "", errors.New("channel_id 不能为空")
	}
	// 通道必须属于可访问设备。
	var ch model.GbChannel
	if err := bootstrap.DB().WithContext(ctx).First(&ch, channelID).Error; err != nil {
		return "", errors.New("通道不存在")
	}
	q := bootstrap.DB().WithContext(ctx).Model(&model.Device{}).Where("id = ?", ch.DeviceID)
	q = s.applyDeviceScope(q, ident)
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return "", err
	}
	if cnt == 0 {
		return "", errors.New("通道所属设备无权访问")
	}

	svc := svcgb.NewGbPtzService()
	if presetID := argInt(args, "preset_id", 0); presetID > 0 {
		if err := svc.GotoPreset(ctx, channelID, presetID); err != nil {
			return "", err
		}
		return jsonText(map[string]interface{}{"ok": true, "channel_id": channelID, "preset_id": presetID})
	}
	req := &request.GbPtzReq{
		Direction: strings.TrimSpace(argString(args, "direction")),
		Action:    strings.TrimSpace(argString(args, "action")),
		Speed:     argInt(args, "speed", 5),
	}
	if req.Direction == "" && req.Action == "" {
		return "", errors.New("direction 或 action 至少提供一个")
	}
	if err := svc.Control(ctx, channelID, req); err != nil {
		return "", err
	}
	return jsonText(map[string]interface{}{
		"ok": true, "channel_id": channelID,
		"direction": req.Direction, "action": req.Action, "speed": req.Speed,
	})
}

func (s *MCPService) toolNotifySend(ctx context.Context, ident *request.ChatContext, args map[string]interface{}) (string, error) {
	title := strings.TrimSpace(argString(args, "title"))
	content := strings.TrimSpace(argString(args, "content"))
	if content == "" {
		return "", errors.New("content 不能为空")
	}
	if title == "" {
		title = "语音助手通知"
	}
	target := ident.UserID
	if v := int64(argInt(args, "target_user_id", 0)); v > 0 {
		if v != ident.UserID && !isAdminRole(ident.RoleCode) {
			return "", errors.New("无权向其他用户发送通知")
		}
		target = v
	}
	if target <= 0 {
		return "", errors.New("无法确定通知接收人")
	}
	// 复用既有通知分类 alarm：Notification 的分页筛选只接受
	// alarm/risk/inspect（request.PageNotificationReq 的 oneof），
	// 新增 "voice" 分类会让既有前端筛选与统计失配。
	// ref_type=voice 用于区分来源。
	if err := sysservice.NewNotificationService().Push(
		ctx, target, sysservice.NotifyCategoryAlarm, title, content, "voice", 0,
	); err != nil {
		return "", err
	}
	return jsonText(map[string]interface{}{"ok": true, "user_id": target})
}

// ── 权限与审计 ────────────────────────────────────────────────

func (s *MCPService) checkPermission(ctx context.Context, tool *model.VoiceMcpTool, ident *request.ChatContext) (bool, string) {
	rules, err := s.perms.ListByTool(ctx, tool.ID)
	if err != nil {
		return false, "读取权限规则失败"
	}
	if len(rules) == 0 {
		return true, ""
	}
	allow, deny := false, false
	for _, r := range rules {
		if !ruleMatches(r, ident) {
			continue
		}
		switch strings.ToLower(r.Effect) {
		case model.VoiceEffectDeny:
			deny = true
		case model.VoiceEffectAllow:
			allow = true
		}
	}
	if deny {
		return false, "命中 deny 规则"
	}
	if !allow {
		return false, "未命中 allow 规则（默认拒绝）"
	}
	return true, ""
}

func ruleMatches(r model.VoiceToolPermission, ident *request.ChatContext) bool {
	switch strings.ToLower(r.SubjectType) {
	case model.VoiceSubjectRole:
		return ident.RoleCode != "" && r.SubjectID == ident.RoleCode
	case model.VoiceSubjectUser:
		return ident.UserID > 0 && r.SubjectID == strconv.FormatInt(ident.UserID, 10)
	case model.VoiceSubjectDept:
		return ident.DeptID > 0 && r.SubjectID == strconv.FormatInt(ident.DeptID, 10)
	case model.VoiceSubjectDevice:
		return ident.DeviceID > 0 && r.SubjectID == strconv.FormatInt(ident.DeviceID, 10)
	default:
		return false
	}
}

func (s *MCPService) writeLog(
	ctx context.Context,
	ident *request.ChatContext,
	tool *model.VoiceMcpTool,
	args map[string]interface{},
	status, message string,
	elapsed int64,
	ip string,
	confirmed bool,
) {
	raw, _ := json.Marshal(args)
	log := &model.VoiceMcpCallLog{
		ToolName:  tool.Name,
		RiskLevel: tool.RiskLevel,
		Confirmed: confirmed,
		Status:    status,
		Args:      truncate(string(raw), 2000),
		Result:    truncate(message, 1000),
		ErrorMsg:  truncate(message, 1000),
		ElapsedMS: elapsed,
		IP:        ip,
	}
	if ident != nil {
		log.SessionID = ident.SessionID
		if ident.UserID > 0 {
			uid := ident.UserID
			log.UserID = &uid
		}
		if ident.DeviceID > 0 {
			did := ident.DeviceID
			log.DeviceID = &did
		}
		if ident.DeptID > 0 {
			d := ident.DeptID
			log.DeptID = &d
		}
		if ident.UserName != "" {
			ctx = model.WithUsername(ctx, ident.UserName)
		}
	}
	if status == "ok" {
		log.ErrorMsg = ""
	}
	_ = s.logs.Create(ctx, log)
}

// applyDeviceScope 设备查询的数据范围。
func (s *MCPService) applyDeviceScope(q *gorm.DB, ident *request.ChatContext) *gorm.DB {
	if ident == nil {
		return q.Where("1 = 0")
	}
	if ident.DeviceID > 0 {
		return q.Where("id = ?", ident.DeviceID)
	}
	return dataScope(ident).ApplyToQuery(q, "ac_device.dept_id")
}

// applyRecordScope 记录类（告警/风险）的数据范围。
//
// 设备上下文优先按 device_id 限制；否则按记录自身的 dept_id 走通用数据权限。
func (s *MCPService) applyRecordScope(q *gorm.DB, ident *request.ChatContext, deviceCol, deptCol string) *gorm.DB {
	if ident == nil {
		return q.Where("1 = 0")
	}
	if ident.DeviceID > 0 {
		return q.Where(deviceCol+" = ?", ident.DeviceID)
	}
	return dataScope(ident).ApplyToQuery(q, deptCol)
}

func dataScope(ident *request.ChatContext) *utils.DataScope {
	ds := &utils.DataScope{
		UserID:       ident.UserID,
		UserName:     ident.UserName,
		DataPerm:     ident.DataPerm,
		CrossDeptIDs: ident.CrossDeptIDs,
	}
	if ident.DeptID > 0 {
		d := ident.DeptID
		ds.DeptID = &d
	}
	if ds.DataPerm == "" {
		ds.DataPerm = "personal"
	}
	return ds
}

// ── 内置工具定义 ──────────────────────────────────────────────

func builtinTools() []model.VoiceMcpTool {
	return []model.VoiceMcpTool{
		{
			Source: model.VoiceToolSourceInternal,
			Name:   "device.list", WireName: "device.list", DisplayName: "查询设备",
			Description: "按关键字/服务组查询有权限的设备列表",
			InputSchema: `{"type":"object","properties":{` +
				`"keyword":{"type":"string","description":"设备名称或编号关键字"},` +
				`"service_group":{"type":"string","description":"服务组编码，如 aichat/gb28181/webrtc"},` +
				`"limit":{"type":"integer","minimum":1,"maximum":100,"default":20}}}`,
			RiskLevel: model.VoiceRiskLow,
		},
		{
			Source: model.VoiceToolSourceInternal,
			Name:   "device.status", WireName: "device.status", DisplayName: "查询设备状态",
			Description: "查询设备在线状态、电量与最后心跳时间",
			InputSchema: `{"type":"object","properties":{` +
				`"device_id":{"type":"integer","description":"设备ID"},` +
				`"device_code":{"type":"string","description":"设备编号"}}}`,
			RiskLevel: model.VoiceRiskLow,
		},
		{
			Source: model.VoiceToolSourceInternal,
			Name:   "alarm.query", WireName: "alarm.query", DisplayName: "查询告警",
			Description: "查询最近一段时间内有权限的设备告警",
			InputSchema: `{"type":"object","properties":{` +
				`"device_id":{"type":"integer"},` +
				`"hours":{"type":"integer","minimum":1,"maximum":720,"default":24},` +
				`"limit":{"type":"integer","minimum":1,"maximum":100,"default":20}}}`,
			RiskLevel: model.VoiceRiskLow,
		},
		{
			Source: model.VoiceToolSourceInternal,
			Name:   "risk.query", WireName: "risk.query", DisplayName: "查询风险",
			Description: "查询最近一段时间内有权限的设备风险",
			InputSchema: `{"type":"object","properties":{` +
				`"device_id":{"type":"integer"},` +
				`"hours":{"type":"integer","minimum":1,"maximum":720,"default":24},` +
				`"limit":{"type":"integer","minimum":1,"maximum":100,"default":20}}}`,
			RiskLevel: model.VoiceRiskLow,
		},
		{
			Source: model.VoiceToolSourceInternal,
			Name:   "ptz.control", WireName: "ptz.control", DisplayName: "云台控制",
			Description: "控制国标通道云台方向/变焦，或调用预置位（写操作）",
			InputSchema: `{"type":"object","properties":{` +
				`"channel_id":{"type":"integer","description":"国标通道ID"},` +
				`"direction":{"type":"string","enum":["up","down","left","right","up-left","up-right","down-left","down-right","stop"]},` +
				`"action":{"type":"string"},` +
				`"speed":{"type":"integer","minimum":1,"maximum":8,"default":5},` +
				`"preset_id":{"type":"integer"},` +
				`"confirm":{"type":"boolean","description":"写操作必须为 true"}}}`,
			RiskLevel: model.VoiceRiskMedium, RequireConfirm: true,
		},
		{
			Source: model.VoiceToolSourceInternal,
			Name:   "notify.send", WireName: "notify.send", DisplayName: "发送通知",
			Description: "向当前用户（管理员可指定用户）发送站内通知（写操作）",
			InputSchema: `{"type":"object","properties":{` +
				`"title":{"type":"string"},` +
				`"content":{"type":"string"},` +
				`"target_user_id":{"type":"integer"},` +
				`"confirm":{"type":"boolean","description":"写操作必须为 true"}},"required":["content"]}`,
			RiskLevel: model.VoiceRiskMedium, RequireConfirm: true,
		},
	}
}

// ── 小工具 ────────────────────────────────────────────────────

func (s *MCPService) runtime(ctx context.Context) *RuntimeConfig {
	if s.provider == nil {
		return BuildRuntimeConfig(nil)
	}
	cfg, err := s.provider.Runtime(ctx)
	if err != nil || cfg == nil {
		return BuildRuntimeConfig(nil)
	}
	return cfg
}

func bearerEqual(header, token string) bool {
	h := strings.TrimSpace(header)
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(parts[1])), []byte(token)) == 1
}

func forbiddenIdentityArgs(args map[string]interface{}) []string {
	forbidden := []string{"user_id", "dept_id", "role_code", "username", "user_name", "company_id", "tenant_id", "data_perm", "cross_dept_ids"}
	var bad []string
	for _, k := range forbidden {
		if _, ok := args[k]; ok {
			bad = append(bad, k)
		}
	}
	return bad
}

// requiresConfirm 写操作确认策略：显式 require_confirm，或风险等级为 medium/high。
func requiresConfirm(tool *model.VoiceMcpTool) bool {
	if tool == nil {
		return false
	}
	if tool.RequireConfirm {
		return true
	}
	switch strings.ToLower(tool.RiskLevel) {
	case model.VoiceRiskMedium, model.VoiceRiskHigh:
		return true
	}
	return false
}

func isConfirmed(args map[string]interface{}) bool {
	v, ok := args["confirm"]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	case float64:
		return t == 1
	}
	return false
}

func isAdminRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin", "super_admin", "administrator", "superadmin", "root":
		return true
	}
	return false
}

func argString(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func argInt(args map[string]interface{}, key string, def int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err == nil {
			return n
		}
	}
	return def
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func rawOrEmpty(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	if s == "" {
		return json.RawMessage("{}")
	}
	if !json.Valid([]byte(s)) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(s)
}

func jsonText(v interface{}) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}

func summarize(s string) string { return truncate(s, 1000) }

func mustJSON(v interface{}) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

func rpcError(id json.RawMessage, code int, msg string) []byte {
	return mustJSON(request.JSONRPCResponse{
		JSONRPC: "2.0", ID: id,
		Error: &request.JSONRPCError{Code: code, Message: msg},
	})
}

func toolError(id json.RawMessage, msg string) []byte {
	return mustJSON(request.JSONRPCResponse{
		JSONRPC: "2.0", ID: id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": msg}},
			"isError": true,
		},
	})
}
