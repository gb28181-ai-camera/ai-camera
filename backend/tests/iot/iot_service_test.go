package iot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svcdevice "github.com/boticz/ai-camera-backend/internal/service/device"
	svciot "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理模块测试：产品 / 服务组 / 设备账号 / 设备端 OTA 与激活
//
// 所有用例在事务中运行并在结束后回滚（testutil.WithTx），
// 因此可以安全地直接查询 tx 校验落库结果（含被脱敏字段的真实值）。
// ════════════════════════════════════════════════════════════════

const (
	testUserID   int64  = 1
	testUserName string = "admin"
	testDeptID   int64  = 1
)

func testCtx() context.Context {
	return testutil.SetTestContext(testUserID, testUserName, testDeptID)
}

var uniqCount int64

// uniq 生成进程内唯一后缀，避免唯一索引冲突（Windows 时钟粒度可能重复）。
func uniq(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), atomic.AddInt64(&uniqCount, 1))
}

func boolPtr(b bool) *bool { return &b }

// mustDevice 通过既有设备服务创建设备（IOT 复用 ac_device）。
func mustDevice(t *testing.T, ctx context.Context, code string) *model.Device {
	t.Helper()
	d, err := svcdevice.NewDeviceService().Create(ctx, &request.CreateDeviceReq{
		Name: "IOT-" + code, DeviceCode: code, Location: "测试位置",
	})
	if err != nil {
		t.Fatalf("创建设备 %s 失败: %v", code, err)
	}
	return d
}

// mustProduct 创建测试产品。
//
// 服务组与配网方式都是产品自身配置，测试用蓝牙 + 允许分享作为默认。
func mustProduct(t *testing.T, ctx context.Context, code string) *model.IotProduct {
	t.Helper()
	p, err := svciot.NewProductService().Create(ctx, &request.CreateIotProductReq{
		Name: "产品-" + code, Code: code, Category: "摄像头",
		AutoNetwork: boolPtr(true), NetConfigMode: model.IotNetConfigBluetooth,
		AllowShare: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("创建产品失败: %v", err)
	}
	return p
}

// mustGroup 创建服务组。
func mustGroup(t *testing.T, ctx context.Context, code, groupType string) *model.IotServiceGroup {
	t.Helper()
	detail, err := svciot.NewServiceGroupService().Create(ctx, &request.CreateIotServiceGroupReq{
		Name: "服务组-" + code, Code: code, Type: groupType,
	})
	if err != nil {
		t.Fatalf("创建服务组失败: %v", err)
	}
	return &detail.IotServiceGroup
}

// rawGroup 直接读取落库的服务组（含明文密码）。
func rawGroup(t *testing.T, tx *gorm.DB, id int64) *model.IotServiceGroup {
	t.Helper()
	var g model.IotServiceGroup
	if err := tx.First(&g, id).Error; err != nil {
		t.Fatalf("读取服务组失败: %v", err)
	}
	return &g
}

// ── 产品 CRUD ─────────────────────────────────────────────────

func TestProductCRUD(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svciot.NewProductService()
		code := uniq("PROD_CRUD")

		p, err := svc.Create(ctx, &request.CreateIotProductReq{
			Name: "4G球机", Code: code, Category: "球机", Description: "desc",
			AutoNetwork: boolPtr(true), NetConfigMode: model.IotNetConfigQRCode,
			AllowShare: boolPtr(true), Sequence: 5,
		})
		if err != nil {
			t.Fatalf("创建产品失败: %v", err)
		}
		if p.Name != "4G球机" || p.Code != code || p.Category != "球机" ||
			!p.AutoNetwork || p.NetConfigMode != model.IotNetConfigQRCode || !p.AllowShare ||
			p.Sequence != 5 || p.Active == nil || !*p.Active {
			t.Fatalf("产品字段不完整: %+v", p)
		}
		if p.CreatedBy != testUserName || p.CreatedDT.IsZero() {
			t.Errorf("审计字段异常: %+v", p)
		}

		// 编码唯一
		if _, err := svc.Create(ctx, &request.CreateIotProductReq{Name: "重复", Code: code}); !errors.Is(err, svciot.ErrProductCodeExists) {
			t.Errorf("重复编码应返回 ErrProductCodeExists，got %v", err)
		}
		// 非法配网方式
		if _, err := svc.Create(ctx, &request.CreateIotProductReq{Name: "x", Code: uniq("BAD"), NetConfigMode: "wifi"}); err == nil {
			t.Errorf("非法配网方式应报错")
		}

		// 更新（增量语义）。不再改配网方式：security 只允许 qrcode，
		// 改成 ap 会被配置校验拒绝（见下方非法更新用例）。
		warning, err := svc.Update(ctx, p.ID, &request.UpdateIotProductReq{
			Name: "4G球机-改", Category: "枪机", AutoNetwork: boolPtr(false),
			AllowShare: boolPtr(false), Sequence: 8,
		})
		if err != nil {
			t.Fatalf("更新产品失败: %v", err)
		}
		if warning != "" {
			t.Errorf("无设备绑定时不应有 warning: %q", warning)
		}
		got, err := svc.Get(ctx, p.ID)
		if err != nil {
			t.Fatalf("查询产品失败: %v", err)
		}
		if got.Name != "4G球机-改" || got.Category != "枪机" || got.AutoNetwork ||
			got.NetConfigMode != model.IotNetConfigQRCode || got.AllowShare || got.Sequence != 8 {
			t.Fatalf("更新未生效: %+v", got)
		}

		// 非法更新：配网方式必须是 bluetooth/ap/qrcode 之一
		if _, err := svc.Update(ctx, p.ID, &request.UpdateIotProductReq{
			NetConfigMode: "carrier-pigeon",
		}); err == nil {
			t.Errorf("非法配网方式应被拒绝")
		}

		// 分页关键字
		page, err := svc.Page(ctx, &request.PageIotProductReq{
			PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: code,
		})
		if err != nil || page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("产品分页异常: total=%d err=%v", page.Total, err)
		}

		// 分享二维码
		qr, err := svc.GenerateShareQR(ctx, p.ID)
		if err != nil {
			t.Fatalf("生成分享二维码失败: %v", err)
		}
		if qr.ProductCode != code || !strings.Contains(qr.Content, "iot_product_share") {
			t.Fatalf("二维码内容异常: %+v", qr)
		}
		got, _ = svc.Get(ctx, p.ID)
		if got.ShareQRContent != qr.Content {
			t.Errorf("分享二维码未持久化")
		}

		// 删除
		if err := svc.Delete(ctx, p.ID); err != nil {
			t.Fatalf("删除产品失败: %v", err)
		}
		if _, err := svc.Get(ctx, p.ID); !errors.Is(err, svciot.ErrProductNotFound) {
			t.Errorf("删除后应返回 ErrProductNotFound，got %v", err)
		}
	})
}

// ── 服务组：凭据生成、脱敏、协议白名单 ───────────────────────

func TestServiceGroupCredentialsAndMask(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svciot.NewServiceGroupService()
		code := uniq("GRP_CRED")

		detail, err := svc.Create(ctx, &request.CreateIotServiceGroupReq{
			Name: "GB28181组", Code: code, Type: model.IotServiceGroupGb28181,
			Description: "desc", Active: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("创建服务组失败: %v", err)
		}
		// 读接口脱敏
		if detail.Password != "****" {
			t.Errorf("创建响应密码应脱敏，got %q", detail.Password)
		}
		if detail.Username == "" {
			t.Errorf("用户名应自动生成")
		}
		if detail.Items == nil || len(detail.Items) != 0 {
			t.Errorf("新服务组服务项应为空数组")
		}

		raw := rawGroup(t, tx, detail.ID)
		if len(raw.Password) != 16 {
			t.Errorf("自动生成密码长度 = %d, 期望 16", len(raw.Password))
		}
		if strings.ContainsAny(raw.Password, "0O1lI") {
			t.Errorf("密码不应包含易混淆字符: %q", raw.Password)
		}
		if raw.Username != detail.Username {
			t.Errorf("返回用户名与库中不一致")
		}

		// 分页脱敏
		page, err := svc.Page(ctx, &request.PageIotServiceGroupReq{
			PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: code,
		})
		if err != nil || page.Total != 1 {
			t.Fatalf("服务组分页异常: %v total=%d", err, page.Total)
		}
		if page.Items[0].Password != "****" {
			t.Errorf("分页密码未脱敏: %q", page.Items[0].Password)
		}

		// 重新获取：详情同样脱敏
		got, err := svc.Get(ctx, detail.ID)
		if err != nil {
			t.Fatalf("查询服务组失败: %v", err)
		}
		if got.Password != "****" {
			t.Errorf("详情密码未脱敏: %q", got.Password)
		}

		// 重置凭据：返回明文且与库中一致
		creds, err := svc.ResetCredentials(ctx, detail.ID)
		if err != nil {
			t.Fatalf("重置凭据失败: %v", err)
		}
		if len(creds.Password) != 16 || creds.Username == "" {
			t.Errorf("重置凭据异常: %+v", creds)
		}
		raw = rawGroup(t, tx, detail.ID)
		if raw.Password != creds.Password || raw.Username != creds.Username {
			t.Errorf("重置凭据未落库")
		}
	})
}

func TestServiceGroupProtocolWhitelist(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svciot.NewServiceGroupService()
		g := mustGroup(t, ctx, uniq("GRP_PROTO"), model.IotServiceGroupGb28181)

		// 合法：SIP + RTSP
		items, err := svc.ReplaceItems(ctx, g.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{
				{Protocol: "sip", Host: "sip.example.com"},
				{Protocol: "RTSP", Host: "rtsp.example.com", Port: 8554},
			},
		})
		if err != nil {
			t.Fatalf("合法协议被拒绝: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("服务项数量 = %d, 期望 2", len(items))
		}
		for _, it := range items {
			if it.Protocol == "SIP" && it.Port != 5060 {
				t.Errorf("SIP 默认端口 = %d, 期望 5060", it.Port)
			}
			if it.Protocol == "RTSP" && it.Port != 8554 {
				t.Errorf("RTSP 端口 = %d, 期望 8554", it.Port)
			}
		}

		// 非法：gb28181 不允许 WebSocket
		if _, err := svc.ReplaceItems(ctx, g.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "WebSocket"}},
		}); err == nil {
			t.Errorf("gb28181 配置 WebSocket 应报错")
		}
		// 非法：同组协议重复
		if _, err := svc.ReplaceItems(ctx, g.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "SIP"}, {Protocol: "SIP"}},
		}); err == nil {
			t.Errorf("重复协议应报错")
		}

		// 整体替换为仅 RTSP
		if _, err := svc.ReplaceItems(ctx, g.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "RTSP"}},
		}); err != nil {
			t.Fatalf("替换服务项失败: %v", err)
		}
		items, _ = svc.GetItems(ctx, g.ID)
		if len(items) != 1 || items[0].Protocol != "RTSP" {
			t.Fatalf("整体替换未删除旧项: %+v", items)
		}

		// 硬删除验证：删除 SIP 后再次加入不应撞唯一索引
		if _, err := svc.ReplaceItems(ctx, g.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "RTSP"}, {Protocol: "SIP"}},
		}); err != nil {
			t.Fatalf("重新加入 SIP 失败（说明关联清理不是硬删除）: %v", err)
		}

		// aichat 仅允许 WebSocket
		ai := mustGroup(t, ctx, uniq("GRP_AI"), model.IotServiceGroupAIChat)
		if _, err := svc.ReplaceItems(ctx, ai.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "WebSocket", Path: "/ws"}},
		}); err != nil {
			t.Fatalf("aichat 配置 WebSocket 失败: %v", err)
		}
		if _, err := svc.ReplaceItems(ctx, ai.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "SIP"}},
		}); err == nil {
			t.Errorf("aichat 配置 SIP 应报错")
		}
	})
}

// ── 产品-服务组绑定 ───────────────────────────────────────────

func TestProductServiceBinding(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		productSvc := svciot.NewProductService()
		groupSvc := svciot.NewServiceGroupService()

		// mustProduct 是 elderly 类型，只能绑 webrtc / aichat。
		p := mustProduct(t, ctx, uniq("PROD_BIND"))
		g1 := mustGroup(t, ctx, uniq("GRP_B1"), model.IotServiceGroupWebRTC)
		g2 := mustGroup(t, ctx, uniq("GRP_B2"), model.IotServiceGroupAIChat)

		bound, err := productSvc.BindServiceGroups(ctx, p.ID, []int64{g1.ID, g2.ID, g1.ID})
		if err != nil {
			t.Fatalf("绑定服务组失败: %v", err)
		}
		if len(bound) != 2 {
			t.Fatalf("绑定后数量 = %d, 期望 2（重复 ID 应去重）", len(bound))
		}
		for _, g := range bound {
			if g.Password != "****" {
				t.Errorf("绑定查询密码未脱敏: %q", g.Password)
			}
		}

		// 整体替换为仅 g2
		bound, err = productSvc.BindServiceGroups(ctx, p.ID, []int64{g2.ID})
		if err != nil {
			t.Fatalf("重新绑定失败: %v", err)
		}
		if len(bound) != 1 || bound[0].ID != g2.ID {
			t.Fatalf("整体替换失败: %+v", bound)
		}

		// 不存在的服务组
		if _, err := productSvc.BindServiceGroups(ctx, p.ID, []int64{99999999}); err == nil {
			t.Errorf("绑定不存在的服务组应报错")
		}

		// g1 已解绑，可删除；g2 仍被引用，拒绝删除
		if err := groupSvc.Delete(ctx, g1.ID); err != nil {
			t.Errorf("未引用的服务组应可删除: %v", err)
		}
		if err := groupSvc.Delete(ctx, g2.ID); !errors.Is(err, svciot.ErrServiceGroupInUse) {
			t.Errorf("被引用的服务组应返回 ErrServiceGroupInUse，got %v", err)
		}

		// 解绑后 g2 可删除
		if _, err := productSvc.BindServiceGroups(ctx, p.ID, nil); err != nil {
			t.Fatalf("解绑失败: %v", err)
		}
		if err := groupSvc.Delete(ctx, g2.ID); err != nil {
			t.Errorf("解绑后应可删除: %v", err)
		}
	})
}

// ── 设备筛选与详情 ────────────────────────────────────────────

func TestDeviceFilteringAndDetail(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		deviceSvc := svciot.NewDeviceService()
		productSvc := svciot.NewProductService()
		groupSvc := svciot.NewServiceGroupService()

		p := mustProduct(t, ctx, uniq("PROD_DEV"))
		g := mustGroup(t, ctx, uniq("GRP_DEV"), model.IotServiceGroupAIChat)
		if _, err := groupSvc.ReplaceItems(ctx, g.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "WebSocket", Host: "ai.local", Path: "/ws"}},
		}); err != nil {
			t.Fatalf("配置服务项失败: %v", err)
		}
		if _, err := productSvc.BindServiceGroups(ctx, p.ID, []int64{g.ID}); err != nil {
			t.Fatalf("绑定服务组失败: %v", err)
		}

		d1 := mustDevice(t, ctx, uniq("IOT_D1"))
		d2 := mustDevice(t, ctx, uniq("IOT_D2"))
		d3 := mustDevice(t, ctx, uniq("IOT_D3"))
		for _, d := range []*model.Device{d1, d2} {
			if err := deviceSvc.SetProduct(ctx, d.ID, &p.ID); err != nil {
				t.Fatalf("设置产品失败: %v", err)
			}
		}
		// 给 d1 配置密钥，验证列表脱敏
		if _, err := deviceSvc.ResetSecret(ctx, d1.ID, "127.0.0.1"); err != nil {
			t.Fatalf("重置密钥失败: %v", err)
		}

		page, err := deviceSvc.Page(ctx, &request.PageIotDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 50}, ProductID: &p.ID,
		})
		if err != nil {
			t.Fatalf("设备分页失败: %v", err)
		}
		if page.Total != 2 || len(page.Items) != 2 {
			t.Fatalf("按产品筛选 total=%d items=%d, 期望 2", page.Total, len(page.Items))
		}
		for _, it := range page.Items {
			if it.ProductID == nil || *it.ProductID != p.ID {
				t.Errorf("产品筛选结果包含其它产品: %+v", it)
			}
			if it.DeviceCode == d1.DeviceCode && it.DeviceSecret != "****" {
				t.Errorf("设备接入密钥未脱敏: %q", it.DeviceSecret)
			}
		}

		// 关键字筛选（d3 无产品）
		page, err = deviceSvc.Page(ctx, &request.PageIotDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 50}, Keyword: d3.DeviceCode,
		})
		if err != nil || page.Total != 1 {
			t.Fatalf("关键字筛选异常: total=%d err=%v", page.Total, err)
		}

		// 详情：设备 + 服务组（含服务项）
		detail, err := deviceSvc.Get(ctx, d1.ID)
		if err != nil {
			t.Fatalf("设备详情失败: %v", err)
		}
		if detail.Device.ID != d1.ID || detail.Device.DeviceSecret != "****" {
			t.Errorf("详情设备字段异常: %+v", detail.Device)
		}
		if len(detail.ServiceGroups) != 1 || len(detail.ServiceGroups[0].Items) != 1 {
			t.Fatalf("详情服务组异常: %+v", detail.ServiceGroups)
		}
		if detail.ServiceGroups[0].Password != "****" {
			t.Errorf("详情服务组密码未脱敏")
		}
		if detail.Accounts == nil {
			t.Errorf("账号列表应为空数组而非 null")
		}

		// 解绑
		if err := deviceSvc.SetProduct(ctx, d2.ID, nil); err != nil {
			t.Fatalf("解绑产品失败: %v", err)
		}
		got, _ := deviceSvc.Get(ctx, d2.ID)
		if got.Device.ProductID != nil {
			t.Errorf("解绑未生效")
		}
	})
}

// ── 激活幂等 / 主账号保护 ─────────────────────────────────────

func TestDeviceActivationIdempotent(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		deviceSvc := svciot.NewDeviceService()
		d := mustDevice(t, ctx, uniq("IOT_ACT"))

		first, err := deviceSvc.Activate(ctx, d.ID, "127.0.0.1")
		if err != nil {
			t.Fatalf("首次激活失败: %v", err)
		}
		if first.Username == "" || len(first.Password) != 16 || first.ActivatedAt == nil {
			t.Fatalf("首次激活返回值异常: %+v", first)
		}
		if first.DeviceCode != d.DeviceCode {
			t.Errorf("设备编号不一致")
		}

		second, err := deviceSvc.Activate(ctx, d.ID, "127.0.0.1")
		if err != nil {
			t.Fatalf("重复激活应幂等成功: %v", err)
		}
		if second.Username != first.Username {
			t.Errorf("重复激活用户名变化: %q != %q", second.Username, first.Username)
		}
		if second.Password != "" {
			t.Errorf("重复激活不应再次返回密码")
		}
		// PostgreSQL TIMESTAMPTZ 只保留微秒精度，首次返回值来自内存（纳秒），
		// 二次返回来自数据库；按微秒比较才是正确的等价判断。
		if second.ActivatedAt == nil || second.ActivatedAt.UnixMicro() != first.ActivatedAt.UnixMicro() {
			t.Errorf("重复激活改变了激活时间: first=%v second=%v", first.ActivatedAt, second.ActivatedAt)
		}

		// 主账号不可删除
		detail, _ := deviceSvc.Get(ctx, d.ID)
		var ownerID int64
		for _, a := range detail.Accounts {
			if a.Role == model.IotAccountRoleOwner {
				ownerID = a.ID
			}
		}
		if ownerID == 0 {
			t.Fatalf("未找到主账号")
		}
		if err := deviceSvc.DeleteAccount(ctx, d.ID, ownerID); !errors.Is(err, svciot.ErrOwnerDeleteForbidden) {
			t.Errorf("删除主账号应返回 ErrOwnerDeleteForbidden，got %v", err)
		}

		// 再次通过账号接口创建 owner 应被拒绝
		if _, err := deviceSvc.CreateAccount(ctx, d.ID, &request.CreateIotDeviceAccountReq{Role: "owner"}, "127.0.0.1"); !errors.Is(err, svciot.ErrOwnerExists) {
			t.Errorf("重复创建 owner 应返回 ErrOwnerExists，got %v", err)
		}

		// 事件留痕
		var cnt int64
		tx.Model(&model.IotDeviceEvent{}).Where("device_id = ? AND event = ?", d.ID, model.IotEventActivate).Count(&cnt)
		if cnt != 1 {
			t.Errorf("activate 事件数量 = %d, 期望 1", cnt)
		}
	})
}

// ── 分享账号 ──────────────────────────────────────────────────

func TestSharedAccount(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		deviceSvc := svciot.NewDeviceService()
		d := mustDevice(t, ctx, uniq("IOT_SHARE"))

		// 密码留空 → 服务端生成并一次性返回
		created, err := deviceSvc.CreateAccount(ctx, d.ID, &request.CreateIotDeviceAccountReq{
			Nickname: "家人", Phone: "13800000000",
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("新增分享账号失败: %v", err)
		}
		if created.Account.Role != model.IotAccountRoleShared || created.Account.Status != model.IotAccountStatusActive {
			t.Errorf("分享账号默认值异常: %+v", created.Account)
		}
		if created.Account.Username == "" || len(created.Password) != 16 {
			t.Errorf("自动生成凭据异常: %+v", created)
		}
		if created.Account.ShareToken == nil || *created.Account.ShareToken == "" {
			t.Errorf("分享账号应生成分享令牌")
		}
		if created.Account.ActivatedAt != nil {
			t.Errorf("分享账号不应有激活时间")
		}

		// 显式密码 → 不返回明文，但 bcrypt 可校验
		second, err := deviceSvc.CreateAccount(ctx, d.ID, &request.CreateIotDeviceAccountReq{
			Username: uniq("share_user"), Password: "Passw0rd!",
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("显式密码新增失败: %v", err)
		}
		if second.Password != "" {
			t.Errorf("显式设置密码时不应返回明文")
		}
		var stored model.IotDeviceAccount
		if err := tx.First(&stored, second.Account.ID).Error; err != nil {
			t.Fatalf("读取账号失败: %v", err)
		}
		if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("Passw0rd!")) != nil {
			t.Errorf("密码哈希与明文不匹配")
		}

		// 更新：昵称/状态/密码
		updated, err := deviceSvc.UpdateAccount(ctx, d.ID, second.Account.ID, &request.UpdateIotDeviceAccountReq{
			Nickname: "新昵称", Status: model.IotAccountStatusDisabled, Password: "NewPassw0rd!",
		})
		if err != nil {
			t.Fatalf("更新账号失败: %v", err)
		}
		if updated.Nickname != "新昵称" || updated.Status != model.IotAccountStatusDisabled {
			t.Errorf("账号更新未生效: %+v", updated)
		}
		var after model.IotDeviceAccount
		tx.First(&after, second.Account.ID)
		if bcrypt.CompareHashAndPassword([]byte(after.PasswordHash), []byte("NewPassw0rd!")) != nil {
			t.Errorf("密码更新未生效")
		}

		// 跨设备越权：用另一台设备 ID 操作应 404
		other := mustDevice(t, ctx, uniq("IOT_OTHER"))
		if err := deviceSvc.DeleteAccount(ctx, other.ID, second.Account.ID); !errors.Is(err, svciot.ErrAccountNotFound) {
			t.Errorf("跨设备删除应返回 ErrAccountNotFound，got %v", err)
		}

		// 删除分享账号
		if err := deviceSvc.DeleteAccount(ctx, d.ID, second.Account.ID); err != nil {
			t.Fatalf("删除分享账号失败: %v", err)
		}
		if err := deviceSvc.DeleteAccount(ctx, d.ID, second.Account.ID); !errors.Is(err, svciot.ErrAccountNotFound) {
			t.Errorf("重复删除应返回 ErrAccountNotFound，got %v", err)
		}

		// 事件留痕（2 次创建）
		var cnt int64
		tx.Model(&model.IotDeviceEvent{}).Where("device_id = ? AND event = ?", d.ID, model.IotEventAccountCreate).Count(&cnt)
		if cnt != 2 {
			t.Errorf("account-create 事件数量 = %d, 期望 2", cnt)
		}
	})
}

// ── 设备端 OTA ────────────────────────────────────────────────

func TestDeviceOTA(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		productSvc := svciot.NewProductService()
		groupSvc := svciot.NewServiceGroupService()
		deviceSvc := svciot.NewDeviceService()
		endpoint := svciot.NewDeviceEndpointService()

		// 产品是 elderly 类型：只能绑 webrtc / aichat（gb28181 会被白名单拒绝）。
		p := mustProduct(t, ctx, uniq("PROD_OTA"))
		g1 := mustGroup(t, ctx, uniq("GRP_OTA1"), model.IotServiceGroupWebRTC)
		g2 := mustGroup(t, ctx, uniq("GRP_OTA2"), model.IotServiceGroupAIChat)
		inactive := mustGroup(t, ctx, uniq("GRP_OTA3"), model.IotServiceGroupWebRTC)
		if err := groupSvc.Update(ctx, inactive.ID, &request.UpdateIotServiceGroupReq{Active: boolPtr(false)}); err != nil {
			t.Fatalf("停用服务组失败: %v", err)
		}

		if _, err := groupSvc.ReplaceItems(ctx, g1.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "WebRTC", Host: "webrtc.local"}, {Protocol: "MQTT", Host: "mqtt.local"}},
		}); err != nil {
			t.Fatalf("配置 g1 服务项失败: %v", err)
		}
		if _, err := groupSvc.ReplaceItems(ctx, g2.ID, &request.ReplaceIotServiceItemsReq{
			Items: []request.IotServiceItemReq{{Protocol: "WebSocket", Host: "ai.local", Path: "/ws"}},
		}); err != nil {
			t.Fatalf("配置 g2 服务项失败: %v", err)
		}
		if _, err := productSvc.BindServiceGroups(ctx, p.ID, []int64{g1.ID, g2.ID, inactive.ID}); err != nil {
			t.Fatalf("绑定服务组失败: %v", err)
		}

		d := mustDevice(t, ctx, uniq("IOT_OTA"))
		if err := deviceSvc.SetProduct(ctx, d.ID, &p.ID); err != nil {
			t.Fatalf("设置产品失败: %v", err)
		}
		secretResp, err := deviceSvc.ResetSecret(ctx, d.ID, "127.0.0.1")
		if err != nil {
			t.Fatalf("重置密钥失败: %v", err)
		}

		// 正常 OTA
		resp, err := endpoint.OTA(ctx, d.DeviceCode, secretResp.DeviceSecret, "1.0.0", "10.0.0.1")
		if err != nil {
			t.Fatalf("OTA 失败: %v", err)
		}
		if resp.Product == nil || resp.Product.ID != p.ID {
			t.Errorf("OTA 产品信息缺失: %+v", resp.Product)
		}
		if resp.Device.ID != d.ID || resp.Device.DeviceSecret != "" {
			t.Errorf("OTA 设备信息异常（密钥不应回传）: %+v", resp.Device)
		}
		if resp.ServerTime == "" {
			t.Errorf("server_time 为空")
		}
		// 只返回启用中的服务组
		if len(resp.ServiceGroups) != 2 {
			t.Fatalf("OTA 服务组数量 = %d, 期望 2（停用组应被排除）", len(resp.ServiceGroups))
		}
		raw1 := rawGroup(t, tx, g1.ID)
		for _, g := range resp.ServiceGroups {
			if g.Code == raw1.Code {
				if g.Password != raw1.Password {
					t.Errorf("OTA 应返回明文组密码")
				}
				if g.Username != raw1.Username {
					t.Errorf("OTA 组用户名不一致")
				}
				if len(g.Items) != 2 || g.Items[0].Protocol != "WebRTC" {
					t.Errorf("OTA 服务项异常: %+v", g.Items)
				}
			}
		}
		if len(resp.ServiceGroups[0].Items) == 0 {
			t.Errorf("服务项不应为空")
		}

		// 事件
		var cnt int64
		tx.Model(&model.IotDeviceEvent{}).Where("device_id = ? AND event = ?", d.ID, model.IotEventOTA).Count(&cnt)
		if cnt != 1 {
			t.Errorf("ota 事件数量 = %d, 期望 1", cnt)
		}

		// 鉴权失败
		if _, err := endpoint.OTA(ctx, d.DeviceCode, "wrong-secret", "", "10.0.0.1"); !errors.Is(err, svciot.ErrDeviceSecretInvalid) {
			t.Errorf("错误密钥应返回 ErrDeviceSecretInvalid，got %v", err)
		}
		if _, err := endpoint.OTA(ctx, d.DeviceCode, "", "", "10.0.0.1"); !errors.Is(err, svciot.ErrDeviceCredentialMissing) {
			t.Errorf("缺少密钥应返回 ErrDeviceCredentialMissing，got %v", err)
		}

		// 未配置密钥
		noSecret := mustDevice(t, ctx, uniq("IOT_NOSEC"))
		if _, err := endpoint.OTA(ctx, noSecret.DeviceCode, "anything", "", "10.0.0.1"); !errors.Is(err, svciot.ErrDeviceSecretNotSet) {
			t.Errorf("未配置密钥应返回 ErrDeviceSecretNotSet，got %v", err)
		}

		// 未绑定产品：返回空服务组而不是报错
		unbound := mustDevice(t, ctx, uniq("IOT_UNBOUND"))
		unboundSecret, _ := deviceSvc.ResetSecret(ctx, unbound.ID, "10.0.0.1")
		resp2, err := endpoint.OTA(ctx, unbound.DeviceCode, unboundSecret.DeviceSecret, "", "10.0.0.1")
		if err != nil {
			t.Fatalf("未绑定产品的 OTA 不应失败: %v", err)
		}
		if resp2.Product != nil || len(resp2.ServiceGroups) != 0 {
			t.Errorf("未绑定产品应返回空配置: %+v", resp2)
		}
	})
}

// ── 设备端激活 ────────────────────────────────────────────────

func TestDeviceEndpointActivate(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		deviceSvc := svciot.NewDeviceService()
		endpoint := svciot.NewDeviceEndpointService()
		d := mustDevice(t, ctx, uniq("IOT_EACT"))

		secretResp, err := deviceSvc.ResetSecret(ctx, d.ID, "127.0.0.1")
		if err != nil {
			t.Fatalf("重置密钥失败: %v", err)
		}
		secret := secretResp.DeviceSecret
		if len(secret) != 48 {
			t.Errorf("设备密钥长度 = %d, 期望 48", len(secret))
		}

		activated, err := endpoint.Activate(ctx, d.DeviceCode, secret, &request.IotActivateReq{
			OwnerName: "张三", OwnerPhone: "13900000000",
		}, "10.0.0.1")
		if err != nil {
			t.Fatalf("设备端激活失败: %v", err)
		}
		if activated.Username == "" || len(activated.Password) != 16 {
			t.Fatalf("激活返回值异常: %+v", activated)
		}
		if activated.DeviceCode != d.DeviceCode || activated.ActivatedAt == nil {
			t.Errorf("激活返回值异常: %+v", activated)
		}

		// 账号落库、密码为 bcrypt
		var owner model.IotDeviceAccount
		if err := tx.Where("device_id = ? AND role = ?", d.ID, model.IotAccountRoleOwner).First(&owner).Error; err != nil {
			t.Fatalf("未找到主账号: %v", err)
		}
		if owner.Nickname != "张三" || owner.Phone != "13900000000" {
			t.Errorf("激活资料未落库: %+v", owner)
		}
		if bcrypt.CompareHashAndPassword([]byte(owner.PasswordHash), []byte(activated.Password)) != nil {
			t.Errorf("激活密码哈希不匹配")
		}

		// 设备表 activated_at
		var dev model.Device
		tx.First(&dev, d.ID)
		if dev.ActivatedAt == nil {
			t.Errorf("ac_device.activated_at 未写入")
		}

		// 重复激活 409
		if _, err := endpoint.Activate(ctx, d.DeviceCode, secret, &request.IotActivateReq{}, "10.0.0.1"); !errors.Is(err, svciot.ErrAlreadyActivated) {
			t.Errorf("重复激活应返回 ErrAlreadyActivated，got %v", err)
		}

		// 事件
		var cnt int64
		tx.Model(&model.IotDeviceEvent{}).Where("device_id = ? AND event = ?", d.ID, model.IotEventActivate).Count(&cnt)
		if cnt != 1 {
			t.Errorf("activate 事件数量 = %d, 期望 1", cnt)
		}
	})
}
