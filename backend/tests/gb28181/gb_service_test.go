package gb28181_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标业务 CRUD 层测试
//
// 运行前必须先清空测试库：
//   cd backend && go run ./tests/testutil/resetdb/main.go
// 环境变量指向 ai_camera_test（禁止连开发库）。
// ════════════════════════════════════════════════════════════════

// allScope 超级管理员数据权限。
func allScope() *utils.DataScope {
	return &utils.DataScope{DataPerm: "all", UserName: "tester"}
}

// createDevice 直接写入一台设备。
func createDevice(t *testing.T, ctx context.Context, code, name, gbID string) *model.Device {
	t.Helper()
	active := true
	d := &model.Device{
		Name:       name,
		DeviceCode: code,
		GbDeviceID: gbID,
		Active:     &active,
		Status:     model.DeviceStatusOnline,
	}
	if gbID != "" {
		d.RegisterStatus = model.GbRegisterStatusPending
	}
	if err := bootstrap.DB().WithContext(ctx).Create(d).Error; err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
	return d
}

// createChannel 直接写入一条通道。
func createChannel(t *testing.T, ctx context.Context, device *model.Device, gbChannelID, name, password string) *model.GbChannel {
	t.Helper()
	c := &model.GbChannel{
		DeviceID:    device.ID,
		DeviceCode:  device.DeviceCode,
		GbDeviceID:  device.GbDeviceID,
		GbChannelID: gbChannelID,
		Name:        name,
		Status:      model.GbStatusOn,
		Password:    password,
	}
	if err := bootstrap.DB().WithContext(ctx).Create(c).Error; err != nil {
		t.Fatalf("创建通道失败: %v", err)
	}
	return c
}

// ── 设备分页 ──────────────────────────────────────────────────

func TestGbDevicePageOnlyReturnsGbDevices(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		gbActive := createDevice(t, ctx, "CAM_GB_001", "国标相机A", "34020000001320000001")
		createDevice(t, ctx, "CAM_NORMAL_001", "普通相机", "")
		gbPending := createDevice(t, ctx, "CAM_GB_002", "国标相机B", "34020000001320000002")
		if err := tx.Model(gbPending).Update("register_status", model.GbRegisterStatusActivated).Error; err != nil {
			t.Fatalf("更新注册状态失败: %v", err)
		}

		svc := gbsvc.NewGbDeviceService()

		resp, err := svc.Page(ctx, &request.PageGbDeviceReq{}, allScope())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 2 || len(resp.Items) != 2 {
			t.Fatalf("应只返回 2 台国标设备，实际 total=%d items=%d", resp.Total, len(resp.Items))
		}
		for _, item := range resp.Items {
			if item.GbDeviceID == "" {
				t.Fatalf("返回了未配置国标的设备: %+v", item)
			}
			if item.SipPassword != "" {
				t.Fatalf("空密码不应被脱敏为非空值: %q", item.SipPassword)
			}
		}

		// 注册状态筛选
		resp, err = svc.Page(ctx, &request.PageGbDeviceReq{RegisterStatus: model.GbRegisterStatusPending}, allScope())
		if err != nil {
			t.Fatalf("按注册状态筛选失败: %v", err)
		}
		if resp.Total != 1 || resp.Items[0].ID != gbActive.ID {
			t.Fatalf("按 pending 筛选应命中设备 %d，实际 total=%d", gbActive.ID, resp.Total)
		}

		// 关键字筛选
		resp, err = svc.Page(ctx, &request.PageGbDeviceReq{Keyword: "相机B"}, allScope())
		if err != nil {
			t.Fatalf("关键字筛选失败: %v", err)
		}
		if resp.Total != 1 || resp.Items[0].ID != gbPending.ID {
			t.Fatalf("关键字应命中设备 %d，实际 total=%d", gbPending.ID, resp.Total)
		}
	})
}

// ── 激活凭证 ──────────────────────────────────────────────────

func TestGenerateActivationRuleAndIdempotent(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_ACT_001", "待激活相机", "")
		svc := gbsvc.NewGbDeviceService()

		first, err := svc.GenerateActivation(ctx, d.ID)
		if err != nil {
			t.Fatalf("生成激活凭证失败: %v", err)
		}
		if len(first.GbDeviceID) != 20 {
			t.Fatalf("国标编码应为 20 位，实际 %q(%d)", first.GbDeviceID, len(first.GbDeviceID))
		}
		if !strings.HasPrefix(first.GbDeviceID, "3402000000"+"132") {
			t.Fatalf("国标编码前缀应为 <domain 10><132>，实际 %q", first.GbDeviceID)
		}
		if _, err := strconv.ParseInt(first.GbDeviceID[13:], 10, 64); err != nil {
			t.Fatalf("国标编码后 7 位应为数字: %q", first.GbDeviceID[13:])
		}
		if len(first.SipPassword) != 16 {
			t.Fatalf("SIP 密码应为 16 位，实际 %q(%d)", first.SipPassword, len(first.SipPassword))
		}
		if strings.ContainsAny(first.SipPassword, "0O1lI") {
			t.Fatalf("SIP 密码不应包含易混淆字符: %q", first.SipPassword)
		}
		if first.RegisterStatus != model.GbRegisterStatusPending {
			t.Fatalf("新激活设备注册状态应为 pending，实际 %q", first.RegisterStatus)
		}
		if first.SipDomain != "3402000000" || first.SipServer == "" || first.Transport == "" {
			t.Fatalf("激活凭证字段不完整: %+v", first)
		}

		// 二维码内容必须是合法 JSON，且包含编码与明文密码
		var qr map[string]interface{}
		if err := json.Unmarshal([]byte(first.QrContent), &qr); err != nil {
			t.Fatalf("二维码内容不是合法 JSON: %v (%q)", err, first.QrContent)
		}
		if qr["gb_device_id"] != first.GbDeviceID || qr["sip_password"] != first.SipPassword {
			t.Fatalf("二维码内容与返回字段不一致: %+v", qr)
		}

		// 幂等：第二次调用不应更换编码或密码
		second, err := svc.GenerateActivation(ctx, d.ID)
		if err != nil {
			t.Fatalf("二次生成激活凭证失败: %v", err)
		}
		if second.GbDeviceID != first.GbDeviceID {
			t.Fatalf("二次调用不应更换国标编码: %q -> %q", first.GbDeviceID, second.GbDeviceID)
		}
		if second.SipPassword != first.SipPassword {
			t.Fatalf("二次调用不应重置 SIP 密码")
		}

		// 落库校验
		var saved model.Device
		if err := tx.First(&saved, d.ID).Error; err != nil {
			t.Fatalf("读取设备失败: %v", err)
		}
		if saved.GbDeviceID != first.GbDeviceID || saved.SipPassword != first.SipPassword {
			t.Fatalf("激活凭证未落库: %+v", saved)
		}
		if saved.RegisterStatus != model.GbRegisterStatusPending ||
			saved.SipRealm != "3402000000" ||
			saved.SipTransport != model.GbSipTransportUDP ||
			saved.StreamMode != model.GbStreamModeUDP ||
			saved.Charset != model.GbCharsetGB2312 {
			t.Fatalf("国标接入默认值未补齐: register=%q realm=%q transport=%q mode=%q charset=%q",
				saved.RegisterStatus, saved.SipRealm, saved.SipTransport, saved.StreamMode, saved.Charset)
		}
	})
}

func TestGenerateActivationKeepsExistingCredential(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		d := createDevice(t, ctx, "CAM_ACT_002", "已激活相机", "34020000001320000099")
		if err := bootstrap.DB().WithContext(ctx).Model(d).Updates(map[string]interface{}{
			"sip_password":    "ExistingPwd12345",
			"register_status": model.GbRegisterStatusActivated,
		}).Error; err != nil {
			t.Fatalf("准备设备失败: %v", err)
		}

		resp, err := gbsvc.NewGbDeviceService().GenerateActivation(ctx, d.ID)
		if err != nil {
			t.Fatalf("读取激活凭证失败: %v", err)
		}
		if resp.GbDeviceID != "34020000001320000099" || resp.SipPassword != "ExistingPwd12345" {
			t.Fatalf("已有凭证不应被重写: %+v", resp)
		}
		if resp.RegisterStatus != model.GbRegisterStatusActivated {
			t.Fatalf("已激活设备状态不应回退为 pending，实际 %q", resp.RegisterStatus)
		}
	})
}

// ── 通道 CRUD 与脱敏 ──────────────────────────────────────────

func TestGbChannelCRUDAndMaskedPassword(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CH_001", "通道测试设备", "34020000001320000101")

		region := &model.Region{Name: "测试区域", Sequence: 10}
		if err := tx.Create(region).Error; err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}
		group := &model.GbGroup{Name: "测试分组", Sequence: 10}
		if err := tx.Create(group).Error; err != nil {
			t.Fatalf("创建分组失败: %v", err)
		}

		c := createChannel(t, ctx, d, "34020000001320000101", "旧通道名", "ChanPwd!23")
		svc := gbsvc.NewGbChannelService()

		// 详情：密码脱敏、关联设备名
		detail, err := svc.Get(ctx, c.ID)
		if err != nil {
			t.Fatalf("查询通道详情失败: %v", err)
		}
		if detail.Password != utils.MaskedValue {
			t.Fatalf("通道密码应脱敏为 %q，实际 %q", utils.MaskedValue, detail.Password)
		}
		if detail.DeviceName != "通道测试设备" || detail.GbChannelID != "34020000001320000101" {
			t.Fatalf("详情关联字段缺失: %+v", detail.GbChannelItem)
		}

		// 更新：字符串/指针/敏感字段
		hasAudio := true
		channelType := int64(1)
		ptzType := int64(2)
		regionID := region.ID
		groupID := group.ID
		if err := svc.Update(ctx, c.ID, &request.UpdateGbChannelReq{
			Name:         "新通道名",
			Manufacturer: "厂商A",
			Model:        "M-1",
			Owner:        "归属单位",
			CivilCode:    "340200",
			Address:      "某路 1 号",
			ChannelType:  &channelType,
			PtzType:      &ptzType,
			HasAudio:     &hasAudio,
			Status:       model.GbStatusOn,
			RegionID:     &regionID,
			GroupID:      &groupID,
			StreamID:     "stream-1",
			Password:     utils.MaskedValue, // 脱敏串必须保持原密码
			Remark:       "更新备注",
		}); err != nil {
			t.Fatalf("更新通道失败: %v", err)
		}

		var saved model.GbChannel
		if err := tx.First(&saved, c.ID).Error; err != nil {
			t.Fatalf("读取通道失败: %v", err)
		}
		if saved.Name != "新通道名" || saved.Manufacturer != "厂商A" || saved.Model != "M-1" ||
			saved.Owner != "归属单位" || saved.CivilCode != "340200" || saved.Address != "某路 1 号" ||
			saved.ChannelType != 1 || saved.PtzType != 2 || !boolValue(saved.HasAudio) ||
			saved.Status != model.GbStatusOn || saved.StreamID != "stream-1" || saved.Remark != "更新备注" {
			t.Fatalf("通道字段更新不完整: %+v", saved)
		}
		if saved.Password != "ChanPwd!23" {
			t.Fatalf("提交 **** 时密码应保持原值，实际 %q", saved.Password)
		}
		if saved.RegionID == nil || *saved.RegionID != region.ID || saved.GroupID == nil || *saved.GroupID != group.ID {
			t.Fatalf("区域/分组未更新: region=%v group=%v", saved.RegionID, saved.GroupID)
		}

		// 新密码应生效
		if err := svc.Update(ctx, c.ID, &request.UpdateGbChannelReq{Password: "NewPwd456"}); err != nil {
			t.Fatalf("更新通道密码失败: %v", err)
		}
		if err := tx.First(&saved, c.ID).Error; err != nil {
			t.Fatalf("读取通道失败: %v", err)
		}
		if saved.Password != "NewPwd456" {
			t.Fatalf("新密码未保存，实际 %q", saved.Password)
		}

		// 空密码应保持原值
		if err := svc.Update(ctx, c.ID, &request.UpdateGbChannelReq{Password: ""}); err != nil {
			t.Fatalf("空密码更新失败: %v", err)
		}
		if err := tx.First(&saved, c.ID).Error; err != nil {
			t.Fatalf("读取通道失败: %v", err)
		}
		if saved.Password != "NewPwd456" {
			t.Fatalf("空密码不应覆盖原值，实际 %q", saved.Password)
		}

		// 分页关键字
		page, err := svc.Page(ctx, &request.PageGbChannelReq{Keyword: "新通道名"}, allScope())
		if err != nil {
			t.Fatalf("通道分页失败: %v", err)
		}
		if page.Total != 1 || page.Items[0].DeviceName != "通道测试设备" || page.Items[0].GroupName != "测试分组" {
			t.Fatalf("通道分页关联名称缺失: %+v", page.Items)
		}

		// 重置：清空平台侧字段，但保留密码与通道编码
		if err := svc.Reset(ctx, c.ID); err != nil {
			t.Fatalf("重置通道失败: %v", err)
		}
		if err := tx.First(&saved, c.ID).Error; err != nil {
			t.Fatalf("读取通道失败: %v", err)
		}
		if saved.Name != "" || saved.RegionID != nil || saved.GroupID != nil ||
			saved.StreamID != "" || saved.RecordPlanID != nil || boolValue(saved.EnableBroadcast) ||
			saved.Remark != "" || saved.SyncDT != nil {
			t.Fatalf("重置未清空平台侧字段: %+v", saved)
		}
		if saved.Password != "NewPwd456" || saved.GbChannelID != "34020000001320000101" {
			t.Fatalf("重置不应清空密码或通道编码: %+v", saved)
		}

		// 删除后查不到
		if err := svc.Delete(ctx, c.ID); err != nil {
			t.Fatalf("删除通道失败: %v", err)
		}
		if _, err := svc.Get(ctx, c.ID); !errors.Is(err, gbsvc.ErrGbChannelNotFound) {
			t.Fatalf("删除后应返回通道不存在，实际 %v", err)
		}
	})
}

// ── 媒体节点脱敏 ──────────────────────────────────────────────

func TestGbMediaServerSecretMasking(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := gbsvc.NewGbMediaServerService()
		m, err := svc.Create(ctx, &request.CreateGbMediaServerReq{
			Name:     "ZLM-1",
			ServerID: "zlm-server-1",
			IP:       "127.0.0.1",
			HTTPPort: 8082,
			Secret:   "zlm-secret-value",
		})
		if err != nil {
			t.Fatalf("创建媒体节点失败: %v", err)
		}
		if m.Type != "zlm" || m.RTPPortRange == "" || m.RecordDay != 7 || m.HookAliveInterval != 15 {
			t.Fatalf("节点默认值不正确: %+v", m)
		}

		detail, err := svc.Get(ctx, m.ID)
		if err != nil {
			t.Fatalf("查询节点详情失败: %v", err)
		}
		if detail.Secret != utils.MaskedValue {
			t.Fatalf("secret 应脱敏为 %q，实际 %q", utils.MaskedValue, detail.Secret)
		}

		// 写回脱敏串必须保持原值
		masked := utils.MaskedValue
		if err := svc.Update(ctx, m.ID, &request.UpdateGbMediaServerReq{Secret: &masked}); err != nil {
			t.Fatalf("脱敏串更新失败: %v", err)
		}
		var saved model.GbMediaServer
		if err := tx.First(&saved, m.ID).Error; err != nil {
			t.Fatalf("读取节点失败: %v", err)
		}
		if saved.Secret != "zlm-secret-value" {
			t.Fatalf("提交 **** 时 secret 应保持原值，实际 %q", saved.Secret)
		}

		// 空值同样保持原值
		empty := ""
		if err := svc.Update(ctx, m.ID, &request.UpdateGbMediaServerReq{Secret: &empty}); err != nil {
			t.Fatalf("空值更新失败: %v", err)
		}
		if err := tx.First(&saved, m.ID).Error; err != nil {
			t.Fatalf("读取节点失败: %v", err)
		}
		if saved.Secret != "zlm-secret-value" {
			t.Fatalf("空值不应覆盖 secret，实际 %q", saved.Secret)
		}

		// 新值生效 + 其他字段增量更新
		newSecret := "new-secret"
		newName := "ZLM-主节点"
		if err := svc.Update(ctx, m.ID, &request.UpdateGbMediaServerReq{Secret: &newSecret, Name: &newName}); err != nil {
			t.Fatalf("更新节点失败: %v", err)
		}
		if err := tx.First(&saved, m.ID).Error; err != nil {
			t.Fatalf("读取节点失败: %v", err)
		}
		if saved.Secret != "new-secret" || saved.Name != "ZLM-主节点" {
			t.Fatalf("节点更新不完整: secret=%q name=%q", saved.Secret, saved.Name)
		}

		// 列表脱敏。
		// 注意：不能断言「列表恰好 1 条」—— 种子数据里有一条默认节点
		// （ac_gb_media_server server_id=ai-camera-zlm-01），断言条数会让
		// 这个用例依赖种子内容。改为「本用例创建的节点必须在列，且全部 secret 已脱敏」。
		list, err := svc.List(ctx, allScope())
		if err != nil {
			t.Fatalf("节点列表失败: %v", err)
		}
		found := false
		for _, item := range list {
			if item.Secret != utils.MaskedValue {
				t.Fatalf("列表 secret 未脱敏: id=%d secret=%q", item.ID, item.Secret)
			}
			if item.ID == m.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("列表未包含本用例创建的节点 id=%d", m.ID)
		}
	})
}

// ── 分组删除校验与树 ─────────────────────────────────────────

func TestGbGroupDeleteValidationAndTree(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := gbsvc.NewGbGroupService()

		parent, err := svc.Create(ctx, &request.CreateGbGroupReq{Name: "父分组"})
		if err != nil {
			t.Fatalf("创建父分组失败: %v", err)
		}
		child, err := svc.Create(ctx, &request.CreateGbGroupReq{Name: "子分组", ParentID: &parent.ID})
		if err != nil {
			t.Fatalf("创建子分组失败: %v", err)
		}

		// 有子节点不能删
		if err := svc.Delete(ctx, parent.ID); !errors.Is(err, gbsvc.ErrGbGroupHasChildren) {
			t.Fatalf("有子分组时应拒绝删除，实际 %v", err)
		}

		// 树结构
		tree, err := svc.Tree(ctx, allScope())
		if err != nil {
			t.Fatalf("查询分组树失败: %v", err)
		}
		if len(tree) != 1 || tree[0].ID != parent.ID || len(tree[0].Children) != 1 ||
			tree[0].Children[0].ID != child.ID {
			t.Fatalf("分组树结构不正确: %+v", tree)
		}

		// 成环校验：把父分组的父节点设为子分组
		if err := svc.Update(ctx, parent.ID, &request.UpdateGbGroupReq{ParentID: &child.ID}); !errors.Is(err, gbsvc.ErrGbGroupCycle) {
			t.Fatalf("父子成环时应拒绝更新，实际 %v", err)
		}

		// 被通道引用不能删
		d := createDevice(t, ctx, "CAM_GRP_001", "分组引用设备", "34020000001320000201")
		c := createChannel(t, ctx, d, "34020000001320000201", "引用通道", "")
		if err := tx.Model(c).Update("group_id", child.ID).Error; err != nil {
			t.Fatalf("绑定通道分组失败: %v", err)
		}
		if err := svc.Delete(ctx, child.ID); !errors.Is(err, gbsvc.ErrGbGroupInUse) {
			t.Fatalf("通道引用分组时应拒绝删除，实际 %v", err)
		}

		// 解除引用后可删除；父分组仍因有子节点被拦，先删子再删父
		if err := tx.Model(c).Update("group_id", nil).Error; err != nil {
			t.Fatalf("解绑通道分组失败: %v", err)
		}
		if err := svc.Delete(ctx, child.ID); err != nil {
			t.Fatalf("删除子分组失败: %v", err)
		}
		if err := svc.Delete(ctx, parent.ID); err != nil {
			t.Fatalf("删除父分组失败: %v", err)
		}
	})
}

// ── 信令日志 ──────────────────────────────────────────────────

func TestGbSipLogPageAndStats(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		logs := []model.GbSipLog{
			{GbDeviceID: "34020000001320000301", Direction: "in", Method: "REGISTER", StatusCode: 401, Result: "success"},
			{GbDeviceID: "34020000001320000301", Direction: "out", Method: "MESSAGE", CmdType: "Catalog", StatusCode: 200, Result: "success"},
			{GbDeviceID: "34020000001320000302", Direction: "in", Method: "REGISTER", StatusCode: 403, Result: "rejected"},
		}
		if err := tx.Create(&logs).Error; err != nil {
			t.Fatalf("写入信令日志失败: %v", err)
		}

		svc := gbsvc.NewGbSipLogService()

		page, err := svc.Page(ctx, &request.PageGbSipLogReq{Method: "REGISTER"})
		if err != nil {
			t.Fatalf("日志分页失败: %v", err)
		}
		if page.Total != 2 || len(page.Items) != 2 {
			t.Fatalf("按 REGISTER 筛选应有 2 条，实际 total=%d", page.Total)
		}

		page, err = svc.Page(ctx, &request.PageGbSipLogReq{CmdType: "Catalog", Result: "success"})
		if err != nil {
			t.Fatalf("日志分页失败: %v", err)
		}
		if page.Total != 1 || page.Items[0].Method != "MESSAGE" {
			t.Fatalf("按命令类型筛选结果不正确: %+v", page.Items)
		}

		stats, err := svc.Stats(ctx, &request.GbSipLogStatsReq{})
		if err != nil {
			t.Fatalf("日志统计失败: %v", err)
		}
		if stats.Total != 3 || stats.Success != 2 || stats.Rejected != 1 ||
			stats.Inbound != 2 || stats.Outbound != 1 {
			t.Fatalf("统计计数不正确: %+v", stats)
		}
		if len(stats.ByMethod) != 2 || stats.ByMethod[0].Key != "REGISTER" || stats.ByMethod[0].Count != 2 {
			t.Fatalf("按方法分组不正确: %+v", stats.ByMethod)
		}
		if len(stats.ByCmd) != 1 || stats.ByCmd[0].Key != "Catalog" || stats.ByCmd[0].Count != 1 {
			t.Fatalf("按命令类型分组不正确: %+v", stats.ByCmd)
		}

		got, err := svc.Get(ctx, logs[0].ID)
		if err != nil {
			t.Fatalf("查询日志详情失败: %v", err)
		}
		if got.ID != logs[0].ID {
			t.Fatalf("日志详情 ID 不一致: %d != %d", got.ID, logs[0].ID)
		}
	})
}

// ── 设备删除级联通道 ─────────────────────────────────────────

func TestGbDeviceDeleteCascadesChannels(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_DEL_001", "待删设备", "34020000001320000401")
		c := createChannel(t, ctx, d, "34020000001320000401", "待删通道", "")

		if err := gbsvc.NewGbDeviceService().Delete(ctx, d.ID); err != nil {
			t.Fatalf("删除国标设备失败: %v", err)
		}

		var deviceCount, channelCount int64
		if err := tx.Unscoped().Model(&model.Device{}).Where("id = ?", d.ID).Count(&deviceCount).Error; err != nil {
			t.Fatalf("统计设备失败: %v", err)
		}
		if err := tx.Unscoped().Model(&model.GbChannel{}).Where("id = ?", c.ID).Count(&channelCount).Error; err != nil {
			t.Fatalf("统计通道失败: %v", err)
		}
		// 软删除：记录仍在，但 deleted_at 非空
		if deviceCount != 1 || channelCount != 1 {
			t.Fatalf("软删除不应物理移除记录: device=%d channel=%d", deviceCount, channelCount)
		}
		var savedChannel model.GbChannel
		if err := tx.Unscoped().First(&savedChannel, c.ID).Error; err != nil {
			t.Fatalf("读取通道失败: %v", err)
		}
		if !savedChannel.DeletedAt.Valid {
			t.Fatalf("通道应被软删除")
		}
	})
}

// boolValue 解引用布尔指针。
func boolValue(p *bool) bool { return p != nil && *p }
