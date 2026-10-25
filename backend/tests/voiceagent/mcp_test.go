package voiceagent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

const mcpAuth = "Bearer mcp-secret"

func rpcCall(t *testing.T, svc *voiceagent.MCPService, ctx context.Context, ctxHeader, method string, params interface{}) (int, request.JSONRPCResponse) {
	t.Helper()
	body := map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, _ := json.Marshal(body)
	status, out := svc.Handle(ctx, raw, mcpAuth, ctxHeader, "127.0.0.1")
	var resp request.JSONRPCResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("解析 JSON-RPC 响应失败: %v, body=%s", err, string(out))
	}
	return status, resp
}

func resultContent(t *testing.T, resp request.JSONRPCResponse) (string, bool) {
	t.Helper()
	m, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("result 不是对象: %+v", resp.Result)
	}
	list, ok := m["content"].([]interface{})
	if !ok || len(list) == 0 {
		t.Fatalf("content 缺失: %+v", m)
	}
	first, _ := list[0].(map[string]interface{})
	text, _ := first["text"].(string)
	isErr, _ := m["isError"].(bool)
	return text, isErr
}

func TestMCPInitializeAndToolsList(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := voiceagent.NewMCPService(&staticProvider{cfg: testConfig()})

		status, resp := rpcCall(t, svc, ctx, "", "initialize", nil)
		if status != 200 || resp.Error != nil {
			t.Fatalf("initialize 失败: status=%d err=%+v", status, resp.Error)
		}
		m := resp.Result.(map[string]interface{})
		if m["protocolVersion"] != "2024-11-05" {
			t.Errorf("protocolVersion = %v", m["protocolVersion"])
		}

		status, resp = rpcCall(t, svc, ctx, "", "tools/list", nil)
		if status != 200 || resp.Error != nil {
			t.Fatalf("tools/list 失败: status=%d err=%+v", status, resp.Error)
		}
		tools := resp.Result.(map[string]interface{})["tools"].([]interface{})
		names := map[string]bool{}
		for _, it := range tools {
			names[it.(map[string]interface{})["name"].(string)] = true
		}
		for _, want := range []string{"device.list", "device.status", "alarm.query", "risk.query", "ptz.control", "notify.send"} {
			if !names[want] {
				t.Errorf("缺少工具 %s（实际 %v）", want, names)
			}
		}

		// 数据库里的内置工具应已落库（幂等，不重复）。
		var cnt int64
		if err := tx.WithContext(ctx).Model(&model.VoiceMcpTool{}).Count(&cnt).Error; err != nil {
			t.Fatal(err)
		}
		if cnt != 6 {
			t.Errorf("内置工具数量 = %d, 期望 6", cnt)
		}
	})
}

func TestMCPToolCallAuthAndIdentity(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := voiceagent.NewMCPService(&staticProvider{cfg: testConfig()})

		// 无签名上下文
		_, resp := rpcCall(t, svc, ctx, "", "tools/call", map[string]interface{}{
			"name": "device.list", "arguments": map[string]interface{}{},
		})
		if resp.Error == nil || resp.Error.Code != -32001 {
			t.Fatalf("缺少上下文应返回 -32001，实际 %+v", resp.Error)
		}

		// 参数携带身份字段
		signed := signContext(t, request.ChatContext{UserID: 1, Kind: "web", DataPerm: "all"})
		_, resp = rpcCall(t, svc, ctx, signed, "tools/call", map[string]interface{}{
			"name": "device.list", "arguments": map[string]interface{}{"user_id": 99},
		})
		if resp.Error == nil || resp.Error.Code != request.JSONRPCInvalidParams {
			t.Fatalf("身份参数应被拒绝，实际 %+v", resp.Error)
		}
	})
}

func TestMCPToolDeviceListAndAudit(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		productID, _, _, _ := seedAIChatProduct(t, ctx)
		d1 := seedDevice(t, ctx, productID, uniq("D1"))
		d2 := seedDevice(t, ctx, productID, uniq("D2"))

		svc := voiceagent.NewMCPService(&staticProvider{cfg: testConfig()})
		signed := signContext(t, request.ChatContext{
			SessionID: "sess-1", UserID: 1, UserName: "alice",
			DeptID: 1, RoleCode: "admin", DataPerm: "all", Kind: "web",
		})
		status, resp := rpcCall(t, svc, ctx, signed, "tools/call", map[string]interface{}{
			"name": "device.list", "arguments": map[string]interface{}{"limit": 10},
		})
		if status != 200 || resp.Error != nil {
			t.Fatalf("device.list 失败: status=%d err=%+v", status, resp.Error)
		}
		text, isErr := resultContent(t, resp)
		if isErr {
			t.Fatalf("device.list 返回错误: %s", text)
		}
		if !strings.Contains(text, d1.DeviceCode) || !strings.Contains(text, d2.DeviceCode) {
			t.Errorf("结果缺少设备: %s", text)
		}

		// 审计
		var logs []model.VoiceMcpCallLog
		if err := tx.WithContext(ctx).Where("tool_name = ?", "device.list").Find(&logs).Error; err != nil {
			t.Fatal(err)
		}
		if len(logs) != 1 || logs[0].Status != "ok" {
			t.Fatalf("审计异常: %+v", logs)
		}
		if logs[0].SessionID != "sess-1" || logs[0].UserID == nil || *logs[0].UserID != 1 {
			t.Errorf("审计身份字段异常: %+v", logs[0])
		}

		// 设备上下文只能看到绑定设备
		devCtx := signContext(t, request.ChatContext{
			SessionID: "sess-2", DeviceID: d1.ID, Kind: "device", Scopes: []string{"voice", "device"},
		})
		_, resp = rpcCall(t, svc, ctx, devCtx, "tools/call", map[string]interface{}{
			"name": "device.list", "arguments": map[string]interface{}{},
		})
		text, isErr = resultContent(t, resp)
		if isErr {
			t.Fatalf("设备上下文 device.list 失败: %s", text)
		}
		if !strings.Contains(text, d1.DeviceCode) || strings.Contains(text, d2.DeviceCode) {
			t.Errorf("设备上下文越权: %s", text)
		}
	})
}

func TestMCPAlarmAndRiskQuery(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		productID, _, _, _ := seedAIChatProduct(t, ctx)
		dev := seedDevice(t, ctx, productID, uniq("DAL"))
		at := &model.AlarmType{Name: uniq("告警"), Code: uniq("a"), Level: model.LevelHigh}
		if err := tx.WithContext(ctx).Create(at).Error; err != nil {
			t.Fatal(err)
		}
		rt := &model.RiskType{Name: uniq("风险"), Code: uniq("r"), Level: model.LevelMedium}
		if err := tx.WithContext(ctx).Create(rt).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Create(&model.AlarmRecord{
			AlarmTypeID: at.ID, DeviceID: dev.ID, DeviceCode: dev.DeviceCode,
			AlarmLevel: "high", Content: "设备离线", OccurredDT: time.Now(),
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Create(&model.RiskRecord{
			RiskTypeID: rt.ID, DeviceID: dev.ID, DeviceCode: dev.DeviceCode,
			RiskLevel: "medium", DetectScore: 0.91, OccurredDT: time.Now(),
		}).Error; err != nil {
			t.Fatal(err)
		}

		svc := voiceagent.NewMCPService(&staticProvider{cfg: testConfig()})
		signed := signContext(t, request.ChatContext{UserID: 1, Kind: "web", DataPerm: "all"})

		_, resp := rpcCall(t, svc, ctx, signed, "tools/call", map[string]interface{}{
			"name": "alarm.query", "arguments": map[string]interface{}{"hours": 24},
		})
		text, isErr := resultContent(t, resp)
		if isErr || !strings.Contains(text, at.Name) || !strings.Contains(text, dev.DeviceCode) {
			t.Fatalf("alarm.query 异常: isErr=%v text=%s", isErr, text)
		}

		_, resp = rpcCall(t, svc, ctx, signed, "tools/call", map[string]interface{}{
			"name": "risk.query", "arguments": map[string]interface{}{"hours": 24},
		})
		text, isErr = resultContent(t, resp)
		if isErr || !strings.Contains(text, rt.Name) || !strings.Contains(text, dev.DeviceCode) {
			t.Fatalf("risk.query 异常: isErr=%v text=%s", isErr, text)
		}

		_, resp = rpcCall(t, svc, ctx, signed, "tools/call", map[string]interface{}{
			"name": "device.status", "arguments": map[string]interface{}{"device_code": dev.DeviceCode},
		})
		text, isErr = resultContent(t, resp)
		if isErr || !strings.Contains(text, dev.DeviceCode) {
			t.Fatalf("device.status 异常: isErr=%v text=%s", isErr, text)
		}
	})
}

func TestMCPToolPermissionDenied(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := voiceagent.NewMCPService(&staticProvider{cfg: testConfig()})
		// 触发内置工具落库
		rpcCall(t, svc, ctx, "", "tools/list", nil)

		var tool model.VoiceMcpTool
		if err := tx.WithContext(ctx).Where("name = ?", "ptz.control").First(&tool).Error; err != nil {
			t.Fatal(err)
		}
		active := true
		if err := tx.WithContext(ctx).Create(&model.VoiceToolPermission{
			Active: &active, ToolID: tool.ID, SubjectType: model.VoiceSubjectUser,
			SubjectID: "1", Effect: model.VoiceEffectDeny,
		}).Error; err != nil {
			t.Fatal(err)
		}

		signed := signContext(t, request.ChatContext{UserID: 1, RoleCode: "user", DataPerm: "all", Kind: "web"})
		_, resp := rpcCall(t, svc, ctx, signed, "tools/call", map[string]interface{}{
			"name": "ptz.control", "arguments": map[string]interface{}{
				"channel_id": 1, "direction": "up", "confirm": true,
			},
		})
		text, isErr := resultContent(t, resp)
		if !isErr || !strings.Contains(text, "权限不足") {
			t.Fatalf("应被权限拒绝，实际 isErr=%v text=%s", isErr, text)
		}

		var log model.VoiceMcpCallLog
		if err := tx.WithContext(ctx).Where("tool_name = ?", "ptz.control").First(&log).Error; err != nil {
			t.Fatal(err)
		}
		if log.Status != "denied" {
			t.Errorf("拒绝审计 status = %q", log.Status)
		}
	})
}

func TestMCPWriteToolRequiresConfirm(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := voiceagent.NewMCPService(&staticProvider{cfg: testConfig()})
		signed := signContext(t, request.ChatContext{UserID: 1, Kind: "web", DataPerm: "all"})

		// 未确认
		_, resp := rpcCall(t, svc, ctx, signed, "tools/call", map[string]interface{}{
			"name": "notify.send", "arguments": map[string]interface{}{"content": "hello"},
		})
		text, isErr := resultContent(t, resp)
		if !isErr || !strings.Contains(text, "confirm=true") {
			t.Fatalf("未确认应被拒绝，实际 isErr=%v text=%s", isErr, text)
		}

		// 确认后成功
		_, resp = rpcCall(t, svc, ctx, signed, "tools/call", map[string]interface{}{
			"name": "notify.send", "arguments": map[string]interface{}{"title": "t", "content": "hello", "confirm": true},
		})
		text, isErr = resultContent(t, resp)
		if isErr {
			t.Fatalf("确认后仍失败: %s", text)
		}
		var cnt int64
		if err := tx.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ? AND category = ? AND ref_type = ?", 1, "alarm", "voice").Count(&cnt).Error; err != nil {
			t.Fatal(err)
		}
		if cnt != 1 {
			t.Errorf("站内通知数量 = %d, 期望 1", cnt)
		}
		var logs []model.VoiceMcpCallLog
		if err := tx.WithContext(ctx).Where("tool_name = ?", "notify.send").Order("id ASC").Find(&logs).Error; err != nil {
			t.Fatal(err)
		}
		if len(logs) != 2 || logs[0].Status != "denied" || logs[1].Status != "ok" {
			t.Errorf("审计状态异常: %+v", logs)
		}
	})
}

func TestMCPAuthFailure(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := voiceagent.NewMCPService(&staticProvider{cfg: testConfig()})
		status, _ := svc.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`), "Bearer wrong", "", "")
		if status != 401 {
			t.Fatalf("错误令牌应返回 401，实际 %d", status)
		}
	})
}
