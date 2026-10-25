package iot

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
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

// mustProduct 直接落库创建测试产品。
//
// 业务接口创建产品时要求至少选择一个服务组并同步生成凭据；
// 大量既有用例只关心产品本身，因此这里直接写库，避免被服务组约束干扰。
func mustProduct(t *testing.T, ctx context.Context, code string) *model.IotProduct {
	t.Helper()
	p := &model.IotProduct{
		Active: boolPtr(true), Name: "产品-" + code, Code: code, Category: "摄像头",
		AutoNetwork: true, NetConfigMode: model.IotNetConfigBluetooth, AllowShare: true,
	}
	if err := repository.NewIotProductRepo().Create(ctx, p); err != nil {
		t.Fatalf("创建产品失败: %v", err)
	}
	return p
}

// mustGroup 直接落库创建测试服务组。
//
// 业务上服务组是系统预置的三条固定数据，Create 接口不开放新增；
// 测试需要构造任意编码/类型来验证下游逻辑，因此绕过 service 直接写库。
func mustGroup(t *testing.T, ctx context.Context, code, groupType string) *model.IotServiceGroup {
	t.Helper()
	m := &model.IotServiceGroup{
		Active: boolPtr(true),
		Name:   "服务组-" + code,
		Code:   code,
		Type:   groupType,
	}
	if err := repository.NewIotServiceGroupRepo().Create(ctx, m); err != nil {
		t.Fatalf("创建服务组失败: %v", err)
	}
	return m
}

// mustService 直接落库创建测试服务。
func mustService(t *testing.T, ctx context.Context, groupID int64, code, protocol string) *model.IotService {
	t.Helper()
	port := 0
	if p, ok := map[string]int{
		model.IotProtocolSIP: 5060, model.IotProtocolRTSP: 554,
		model.IotProtocolWebRTC: 443, model.IotProtocolMQTT: 1883,
		model.IotProtocolWebSocket: 443, model.IotProtocolTURN: 3478,
	}[protocol]; ok {
		port = p
	}
	svc := &model.IotService{
		Active: boolPtr(true), GroupID: groupID, Code: code, Name: code,
		Protocol: protocol, Host: strings.ToLower(code) + ".example.com", Port: port,
	}
	if err := repository.NewIotServiceGroupRepo().CreateService(ctx, svc); err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	return svc
}

// serviceIDByCode 按组与编码查服务 ID。
func serviceIDByCode(t *testing.T, ctx context.Context, groupID int64, code string) int64 {
	t.Helper()
	services, err := repository.NewIotServiceGroupRepo().ListServices(ctx, groupID)
	if err != nil {
		t.Fatalf("读取服务失败: %v", err)
	}
	for _, svc := range services {
		if svc.Code == code {
			return svc.ID
		}
	}
	t.Fatalf("未找到服务 %s", code)
	return 0
}

// rawGroup 直接读取落库的服务组。
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
		g := mustGroup(t, ctx, uniq("PROD_CRUD_G"), model.IotServiceGroupGb28181)
		mustService(t, ctx, g.ID, "sip", model.IotProtocolSIP)

		p, err := svc.Create(ctx, &request.CreateIotProductReq{
			Name: "4G球机", Code: code, Category: "球机", Description: "desc",
			AutoNetwork: boolPtr(true), NetConfigMode: model.IotNetConfigQRCode,
			AllowShare: boolPtr(true), Sequence: 5,
			ServiceGroupIDs: []int64{g.ID},
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
		// 绑定服务组后自动生成产品×服务凭据。
		rows, err := repository.NewIotProductServiceRepo().ListByProduct(ctx, p.ID)
		if err != nil || len(rows) != 1 || rows[0].Username == "" || len(rows[0].Password) != 16 {
			t.Fatalf("产品服务凭据生成异常: rows=%+v err=%v", rows, err)
		}

		// 编码唯一
		if _, err := svc.Create(ctx, &request.CreateIotProductReq{
			Name: "重复", Code: code, ServiceGroupIDs: []int64{g.ID},
		}); !errors.Is(err, svciot.ErrProductCodeExists) {
			t.Errorf("重复编码应返回 ErrProductCodeExists，got %v", err)
		}
		// 非法配网方式
		if _, err := svc.Create(ctx, &request.CreateIotProductReq{
			Name: "x", Code: uniq("BAD"), NetConfigMode: "wifi", ServiceGroupIDs: []int64{g.ID},
		}); err == nil {
			t.Errorf("非法配网方式应报错")
		}
		// 未选服务组
		if _, err := svc.Create(ctx, &request.CreateIotProductReq{Name: "无组", Code: uniq("NO_GROUP")}); err == nil {
			t.Errorf("未选服务组应报错")
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

		// 分享二维码：先重新开启分享（上方更新已把 allow_share 置为 false）。
		if _, err := svc.Update(ctx, p.ID, &request.UpdateIotProductReq{AllowShare: boolPtr(true)}); err != nil {
			t.Fatalf("重新开启分享失败: %v", err)
		}
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

// ── 服务组：服务与协议白名单 ─────────────────────────────────

func TestServiceGroupProtocolWhitelist(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svciot.NewServiceGroupService()
		g := mustGroup(t, ctx, uniq("GRP_PROTO"), model.IotServiceGroupGb28181)

		// 合法：SIP + RTSP
		services, err := svc.ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{
				{Code: "sip", Name: "SIP", Protocol: "sip", Host: "sip.example.com"},
				{Code: "rtsp", Name: "RTSP", Protocol: "RTSP", Host: "rtsp.example.com", Port: 8554},
			},
		})
		if err != nil {
			t.Fatalf("合法协议被拒绝: %v", err)
		}
		if len(services) != 2 {
			t.Fatalf("服务数量 = %d, 期望 2", len(services))
		}
		for _, it := range services {
			if it.Protocol == "SIP" && it.Port != 5060 {
				t.Errorf("SIP 默认端口 = %d, 期望 5060", it.Port)
			}
			if it.Protocol == "RTSP" && it.Port != 8554 {
				t.Errorf("RTSP 端口 = %d, 期望 8554", it.Port)
			}
		}

		// 非法：gb28181 不允许 WebSocket
		if _, err := svc.ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "ws", Name: "WS", Protocol: "WebSocket"}},
		}); err == nil {
			t.Errorf("gb28181 配置 WebSocket 应报错")
		}
		// 非法：同组服务编码重复
		if _, err := svc.ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{
				{Code: "sip", Name: "SIP-1", Protocol: "SIP"},
				{Code: "sip", Name: "SIP-2", Protocol: "SIP"},
			},
		}); err == nil {
			t.Errorf("重复服务编码应报错")
		}

		// 整体替换为仅 RTSP
		if _, err := svc.ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "rtsp", Name: "RTSP", Protocol: "RTSP"}},
		}); err != nil {
			t.Fatalf("替换服务失败: %v", err)
		}
		services, _ = svc.GetServices(ctx, g.ID)
		if len(services) != 1 || services[0].Protocol != "RTSP" {
			t.Fatalf("整体替换未删除旧服务: %+v", services)
		}

		// 硬删除验证：删除 SIP 后再次加入不应撞唯一索引
		if _, err := svc.ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{
				{Code: "rtsp", Name: "RTSP", Protocol: "RTSP"},
				{Code: "sip", Name: "SIP", Protocol: "SIP"},
			},
		}); err != nil {
			t.Fatalf("重新加入 SIP 失败（说明关联清理不是硬删除）: %v", err)
		}

		// aichat 仅允许 WebSocket；webrtc 额外允许 TURN。
		ai := mustGroup(t, ctx, uniq("GRP_AI"), model.IotServiceGroupAIChat)
		if _, err := svc.ReplaceServices(ctx, ai.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "ws", Name: "WebSocket", Protocol: "WebSocket", Path: "/ws"}},
		}); err != nil {
			t.Fatalf("aichat 配置 WebSocket 失败: %v", err)
		}
		if _, err := svc.ReplaceServices(ctx, ai.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "sip", Name: "SIP", Protocol: "SIP"}},
		}); err == nil {
			t.Errorf("aichat 配置 SIP 应报错")
		}
		rt := mustGroup(t, ctx, uniq("GRP_RTC"), model.IotServiceGroupWebRTC)
		if _, err := svc.ReplaceServices(ctx, rt.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "turn", Name: "TURN", Protocol: "TURN"}},
		}); err != nil {
			t.Fatalf("webrtc 配置 TURN 失败: %v", err)
		}
	})
}

// TestServiceGroupCreateAndDeleteForbidden 服务组是系统预置的三条固定数据。
func TestServiceGroupCreateAndDeleteForbidden(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svciot.NewServiceGroupService()

		// 不允许新增（无论编码是什么）。
		if _, err := svc.Create(ctx, &request.CreateIotServiceGroupReq{
			Name: "自定义组", Code: "custom_group", Type: model.IotServiceGroupWebRTC,
		}); !errors.Is(err, svciot.ErrServiceGroupCreateForbidden) {
			t.Errorf("新增服务组应返回 ErrServiceGroupCreateForbidden，got %v", err)
		}

		// 固定编码不可删除（测试库已由 initdb 预置三条）。
		for _, code := range model.IotFixedServiceGroupCodes {
			var m model.IotServiceGroup
			if err := tx.Where("code = ?", code).First(&m).Error; err != nil {
				t.Fatalf("读取固定服务组 %s 失败: %v", code, err)
			}
			if err := svc.Delete(ctx, m.ID); !errors.Is(err, svciot.ErrServiceGroupDeleteForbidden) {
				t.Errorf("删除固定服务组 %s 应返回 ErrServiceGroupDeleteForbidden，got %v", code, err)
			}
			// 编码与类型不可修改。
			if err := svc.Update(ctx, m.ID, &request.UpdateIotServiceGroupReq{Code: "changed"}); !errors.Is(err, svciot.ErrServiceGroupCodeImmutable) {
				t.Errorf("修改编码应返回 ErrServiceGroupCodeImmutable，got %v", err)
			}
			if err := svc.Update(ctx, m.ID, &request.UpdateIotServiceGroupReq{Type: "custom"}); !errors.Is(err, svciot.ErrServiceGroupTypeImmutable) {
				t.Errorf("修改类型应返回 ErrServiceGroupTypeImmutable，got %v", err)
			}
		}
	})
}

// TestServiceGroupLockedWhenReferenced 被产品引用的服务组进入只读状态。
func TestServiceGroupLockedWhenReferenced(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		productSvc := svciot.NewProductService()
		groupSvc := svciot.NewServiceGroupService()
		p := mustProduct(t, ctx, uniq("LOCK_P"))
		g := mustGroup(t, ctx, uniq("LOCK_G"), model.IotServiceGroupGb28181)
		if _, err := productSvc.BindServiceGroups(ctx, p.ID, []int64{g.ID}); err != nil {
			t.Fatalf("绑定失败: %v", err)
		}

		// 列表/详情返回引用数量。
		page, err := groupSvc.Page(ctx, &request.PageIotServiceGroupReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: g.Code,
		})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("服务组列表异常: %+v err=%v", page, err)
		}
		if page.Items[0].ReferencedByProducts != 1 {
			t.Errorf("列表引用数量 = %d, 期望 1", page.Items[0].ReferencedByProducts)
		}
		detail, err := groupSvc.Get(ctx, g.ID)
		if err != nil || detail.ReferencedByProducts != 1 {
			t.Errorf("详情引用数量异常: %+v err=%v", detail, err)
		}

		// 所有写操作都被拒绝。
		if err := groupSvc.Update(ctx, g.ID, &request.UpdateIotServiceGroupReq{Name: "改名"}); !errors.Is(err, svciot.ErrServiceGroupInUse) {
			t.Errorf("被引用时更新应返回 ErrServiceGroupInUse，got %v", err)
		}
		if _, err := groupSvc.ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "sip", Name: "SIP", Protocol: "SIP"}},
		}); !errors.Is(err, svciot.ErrServiceGroupInUse) {
			t.Errorf("被引用时改服务应返回 ErrServiceGroupInUse，got %v", err)
		}
		if err := groupSvc.Delete(ctx, g.ID); !errors.Is(err, svciot.ErrServiceGroupInUse) {
			t.Errorf("被引用时删除应返回 ErrServiceGroupInUse，got %v", err)
		}

		// 解绑后恢复可编辑。
		if _, err := productSvc.BindServiceGroups(ctx, p.ID, nil); err != nil {
			t.Fatalf("解绑失败: %v", err)
		}
		if err := groupSvc.Update(ctx, g.ID, &request.UpdateIotServiceGroupReq{Name: "解绑后改名"}); err != nil {
			t.Errorf("解绑后应可更新: %v", err)
		}
		if _, err := groupSvc.ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "sip", Name: "SIP", Protocol: "SIP"}},
		}); err != nil {
			t.Errorf("解绑后应可改服务: %v", err)
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
		mustService(t, ctx, g1.ID, "webrtc", model.IotProtocolWebRTC)
		mustService(t, ctx, g2.ID, "ws", model.IotProtocolWebSocket)

		bound, err := productSvc.BindServiceGroups(ctx, p.ID, []int64{g1.ID, g2.ID, g1.ID})
		if err != nil {
			t.Fatalf("绑定服务组失败: %v", err)
		}
		if len(bound) != 2 {
			t.Fatalf("绑定后数量 = %d, 期望 2（重复 ID 应去重）", len(bound))
		}
		rows, err := repository.NewIotProductServiceRepo().ListByProduct(ctx, p.ID)
		if err != nil || len(rows) != 2 {
			t.Fatalf("产品服务凭据数量 = %d, 期望 2（err=%v）", len(rows), err)
		}

		// 整体替换为仅 g2
		bound, err = productSvc.BindServiceGroups(ctx, p.ID, []int64{g2.ID})
		if err != nil {
			t.Fatalf("重新绑定失败: %v", err)
		}
		if len(bound) != 1 || bound[0].ID != g2.ID {
			t.Fatalf("整体替换失败: %+v", bound)
		}
		rows, err = repository.NewIotProductServiceRepo().ListByProduct(ctx, p.ID)
		if err != nil || len(rows) != 1 {
			t.Fatalf("替换后产品服务凭据数量 = %d, 期望 1", len(rows))
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

// TestTurnUserSync 产品×TURN 凭据同步 coturn userdb。
func TestTurnUserSync(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		t.Setenv("TURN_REALM", "e2e-realm")
		productSvc := svciot.NewProductService()
		p := mustProduct(t, ctx, uniq("TURN_P"))
		g := mustGroup(t, ctx, uniq("TURN_G"), model.IotServiceGroupWebRTC)
		svc := mustService(t, ctx, g.ID, "turn", model.IotProtocolTURN)

		if _, err := productSvc.BindServiceGroups(ctx, p.ID, []int64{g.ID}); err != nil {
			t.Fatalf("绑定失败: %v", err)
		}
		row, err := repository.NewIotProductServiceRepo().Get(ctx, p.ID, svc.ID)
		if err != nil {
			t.Fatalf("读取产品服务凭据失败: %v", err)
		}
		var got model.TurnUserLT
		if err := tx.First(&got, "realm = ? AND name = ?", "e2e-realm", row.Username).Error; err != nil {
			t.Fatalf("coturn 用户未写入: %v", err)
		}
		want := md5HexForTest(row.Username + ":e2e-realm:" + row.Password)
		if got.Hmackey != want {
			t.Errorf("hmackey 异常: got %q want %q", got.Hmackey, want)
		}

		// 轮换后旧账号删除、新账号写入。
		if _, err := productSvc.ResetServiceCredentials(ctx, p.ID, svc.ID); err != nil {
			t.Fatalf("轮换凭据失败: %v", err)
		}
		var cnt int64
		tx.Model(&model.TurnUserLT{}).Where("realm = ? AND name = ?", "e2e-realm", row.Username).Count(&cnt)
		if cnt != 0 {
			t.Errorf("旧 TURN 用户未清理")
		}

		// 删除产品后清理该产品全部 TURN 用户。
		if err := productSvc.Delete(ctx, p.ID); err != nil {
			t.Fatalf("删除产品失败: %v", err)
		}
		tx.Model(&model.TurnUserLT{}).Where("realm = ?", "e2e-realm").Count(&cnt)
		if cnt != 0 {
			t.Errorf("删除产品后 TURN 用户未清理，剩余 %d", cnt)
		}
	})
}

func md5HexForTest(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
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
		if _, err := groupSvc.ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "ws", Name: "WebSocket", Protocol: "WebSocket", Host: "ai.local", Path: "/ws"}},
		}); err != nil {
			t.Fatalf("配置服务失败: %v", err)
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
		if len(detail.ServiceGroups) != 1 || len(detail.ServiceGroups[0].Services) != 1 {
			t.Fatalf("详情服务组异常: %+v", detail.ServiceGroups)
		}
		if detail.ServiceGroups[0].Services[0].Host != "ai.local" {
			t.Errorf("详情服务异常: %+v", detail.ServiceGroups[0].Services[0])
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

// ── 敏感字段脱敏 / 二维码前置条件 ────────────────────────────

// TestDeviceReadMasksSensitiveCredentials IOT 设备读接口必须同时脱敏
// device_secret 与国标 sip_password（设备表被 IOT 复用）。
func TestDeviceReadMasksSensitiveCredentials(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svciot.NewDeviceService()
		d := mustDevice(t, ctx, uniq("IOT_MASK"))
		if err := svc.SetProduct(ctx, d.ID, nil); err != nil {
			t.Fatalf("设置产品失败: %v", err)
		}
		if err := tx.Model(&model.Device{}).Where("id = ?", d.ID).
			Updates(map[string]interface{}{"device_secret": "dev-secret", "sip_password": "sip-secret"}).Error; err != nil {
			t.Fatalf("写入敏感字段失败: %v", err)
		}

		detail, err := svc.Get(ctx, d.ID)
		if err != nil {
			t.Fatalf("设备详情失败: %v", err)
		}
		if detail.Device.DeviceSecret != "****" || detail.Device.SipPassword != "****" {
			t.Errorf("详情敏感字段未脱敏: secret=%q sip=%q",
				detail.Device.DeviceSecret, detail.Device.SipPassword)
		}

		page, err := svc.Page(ctx, &request.PageIotDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200},
			Keyword: d.DeviceCode,
		})
		if err != nil {
			t.Fatalf("设备列表失败: %v", err)
		}
		if len(page.Items) != 1 {
			t.Fatalf("设备列表数量 = %d, 期望 1", len(page.Items))
		}
		if page.Items[0].DeviceSecret != "****" || page.Items[0].SipPassword != "****" {
			t.Errorf("列表敏感字段未脱敏: secret=%q sip=%q",
				page.Items[0].DeviceSecret, page.Items[0].SipPassword)
		}
	})
}

// TestProductShareQRRequiresAllowShare 产品关闭分享后不得再生成分享内容。
func TestProductShareQRRequiresAllowShare(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		g := mustGroup(t, ctx, uniq("NO_SHARE_G"), model.IotServiceGroupWebRTC)
		mustService(t, ctx, g.ID, "webrtc", model.IotProtocolWebRTC)
		p, err := svciot.NewProductService().Create(ctx, &request.CreateIotProductReq{
			Name: "产品-禁止分享", Code: uniq("NO_SHARE"), AllowShare: boolPtr(false),
			ServiceGroupIDs: []int64{g.ID},
		})
		if err != nil {
			t.Fatalf("创建产品失败: %v", err)
		}
		if _, err := svciot.NewProductService().GenerateShareQR(ctx, p.ID); !errors.Is(err, svciot.ErrProductShareDisabled) {
			t.Errorf("关闭分享应返回 ErrProductShareDisabled，got %v", err)
		}
		var stored model.IotProduct
		if err := tx.First(&stored, p.ID).Error; err != nil {
			t.Fatalf("读取产品失败: %v", err)
		}
		if stored.ShareQRContent != "" {
			t.Errorf("被拒绝时不写入 share_qr_content，got %q", stored.ShareQRContent)
		}
	})
}

// TestDeviceQRRequiresProduct 未绑定产品的设备不得生成空产品编码的二维码。
func TestDeviceQRRequiresProduct(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		d := mustDevice(t, ctx, uniq("IOT_NO_PRODUCT"))
		if _, err := svciot.NewShareService().DeviceQR(ctx, d.ID); !errors.Is(err, svciot.ErrDeviceProductRequired) {
			t.Errorf("未绑定产品应返回 ErrDeviceProductRequired，got %v", err)
		}
		detail, err := svciot.NewDeviceService().Get(ctx, d.ID)
		if err != nil {
			t.Fatalf("设备详情失败: %v", err)
		}
		if detail.QRContent != "" {
			t.Errorf("未绑定产品时详情 qr_content 应为空，got %q", detail.QRContent)
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
		// 分享令牌只由 share-token 接口生成；普通分享账号不应带令牌，
		// 否则「创建账号」会意外产生一个可被认领的入口。
		if created.Account.ShareToken != nil {
			t.Errorf("普通分享账号不应自动生成分享令牌，got %q", *created.Account.ShareToken)
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

// ── 后台解绑 ──────────────────────────────────────────────────

// TestUnbindDevice 解绑删除全部账号、清除激活时间，且不影响设备接入配置。
func TestUnbindDevice(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svciot.NewDeviceService()
		p := mustProduct(t, ctx, uniq("UNBIND_P"))
		d := mustDevice(t, ctx, uniq("UNBIND_D"))
		if err := svc.SetProduct(ctx, d.ID, &p.ID); err != nil {
			t.Fatalf("设置产品失败: %v", err)
		}
		secret, err := svc.ResetSecret(ctx, d.ID, "127.0.0.1")
		if err != nil {
			t.Fatalf("重置密钥失败: %v", err)
		}
		if _, err := svc.Activate(ctx, d.ID, "127.0.0.1"); err != nil {
			t.Fatalf("激活失败: %v", err)
		}
		if _, err := svc.CreateAccount(ctx, d.ID, &request.CreateIotDeviceAccountReq{Nickname: "家人"}, "127.0.0.1"); err != nil {
			t.Fatalf("创建分享账号失败: %v", err)
		}

		// 确认解绑前有 owner + shared 两个账号，且设备已激活。
		before, err := svc.Get(ctx, d.ID)
		if err != nil {
			t.Fatalf("解绑前详情失败: %v", err)
		}
		if len(before.Accounts) != 2 || before.Device.ActivatedAt == nil {
			t.Fatalf("解绑前状态异常: accounts=%d activated=%v", len(before.Accounts), before.Device.ActivatedAt)
		}

		res, err := svc.Unbind(ctx, d.ID, "127.0.0.1")
		if err != nil {
			t.Fatalf("解绑失败: %v", err)
		}
		if res.DeviceID != d.ID || res.Accounts != 2 {
			t.Errorf("解绑结果异常: %+v", res)
		}

		after, err := svc.Get(ctx, d.ID)
		if err != nil {
			t.Fatalf("解绑后详情失败: %v", err)
		}
		if len(after.Accounts) != 0 || after.Device.ActivatedAt != nil {
			t.Errorf("解绑后仍有账号或激活时间: accounts=%d activated=%v",
				len(after.Accounts), after.Device.ActivatedAt)
		}
		// 接入配置保持不变。
		if after.Device.ProductID == nil || *after.Device.ProductID != p.ID {
			t.Errorf("解绑不应清除产品绑定: %+v", after.Device.ProductID)
		}
		var rawDev model.Device
		if err := tx.First(&rawDev, d.ID).Error; err != nil {
			t.Fatalf("读取设备失败: %v", err)
		}
		if rawDev.DeviceSecret != secret.DeviceSecret || rawDev.DeviceSecret == "" {
			t.Errorf("解绑不应清除接入密钥: got %q want %q", rawDev.DeviceSecret, secret.DeviceSecret)
		}

		// 账号行被硬删除（含软删除行）。
		var rows int64
		tx.Unscoped().Model(&model.IotDeviceAccount{}).Where("device_id = ?", d.ID).Count(&rows)
		if rows != 0 {
			t.Errorf("解绑后账号行应被硬删除，剩余 %d", rows)
		}

		// 事件留痕，且重复解绑保持幂等。
		res2, err := svc.Unbind(ctx, d.ID, "127.0.0.1")
		if err != nil || res2.Accounts != 0 {
			t.Errorf("重复解绑应幂等返回 0，got res=%+v err=%v", res2, err)
		}
		var cnt int64
		tx.Model(&model.IotDeviceEvent{}).Where("device_id = ? AND event = ?", d.ID, model.IotEventUnbind).Count(&cnt)
		if cnt != 2 {
			t.Errorf("unbind 事件数量 = %d, 期望 2", cnt)
		}

		// 解绑后可重新激活。
		if _, err := svc.Activate(ctx, d.ID, "127.0.0.1"); err != nil {
			t.Errorf("解绑后应可重新激活: %v", err)
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

		if _, err := groupSvc.ReplaceServices(ctx, g1.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{
				{Code: "webrtc", Name: "WebRTC", Protocol: "WebRTC", Host: "webrtc.local"},
				{Code: "mqtt", Name: "MQTT", Protocol: "MQTT", Host: "mqtt.local"},
			},
		}); err != nil {
			t.Fatalf("配置 g1 服务失败: %v", err)
		}
		if _, err := groupSvc.ReplaceServices(ctx, g2.ID, &request.ReplaceIotServicesReq{
			Services: []request.IotServiceReq{{Code: "ws", Name: "WebSocket", Protocol: "WebSocket", Host: "ai.local", Path: "/ws"}},
		}); err != nil {
			t.Fatalf("配置 g2 服务失败: %v", err)
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
		// 设备同时具备国标身份时，OTA 也不得回传 sip_password。
		if err := tx.Model(&model.Device{}).Where("id = ?", d.ID).
			Update("sip_password", "sip-secret-for-ota").Error; err != nil {
			t.Fatalf("写入 sip_password 失败: %v", err)
		}

		// 正常 OTA
		resp, err := endpoint.OTA(ctx, d.DeviceCode, secretResp.DeviceSecret, "1.0.0", "10.0.0.1")
		if err != nil {
			t.Fatalf("OTA 失败: %v", err)
		}
		if resp.Product == nil || resp.Product.ID != p.ID {
			t.Errorf("OTA 产品信息缺失: %+v", resp.Product)
		}
		if resp.Device.ID != d.ID || resp.Device.DeviceSecret != "" || resp.Device.SipPassword != "" {
			t.Errorf("OTA 设备信息异常（device_secret / sip_password 不应回传）: %+v", resp.Device)
		}
		if resp.ServerTime == "" {
			t.Errorf("server_time 为空")
		}
		// 只返回启用中的服务组
		if len(resp.ServiceGroups) != 2 {
			t.Fatalf("OTA 服务组数量 = %d, 期望 2（停用组应被排除）", len(resp.ServiceGroups))
		}
		rows, err := repository.NewIotProductServiceRepo().ListByProduct(ctx, p.ID)
		if err != nil {
			t.Fatalf("读取产品服务凭据失败: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("产品服务凭据数量 = %d, 期望 3", len(rows))
		}
		credByService := make(map[int64]model.IotProductService, len(rows))
		for _, row := range rows {
			credByService[row.ServiceID] = row
		}
		for _, g := range resp.ServiceGroups {
			if g.Code != g1.Code {
				continue
			}
			if len(g.Services) != 2 {
				t.Fatalf("OTA 服务数量 = %d, 期望 2", len(g.Services))
			}
			for _, svc := range g.Services {
				svcID := serviceIDByCode(t, ctx, g1.ID, svc.Code)
				cred, ok := credByService[svcID]
				if !ok {
					t.Fatalf("OTA 服务 %s 缺少凭据", svc.Code)
				}
				if svc.Username != cred.Username || svc.Password != cred.Password || svc.Password == "****" {
					t.Errorf("OTA 应返回该产品在该服务上的明文凭据: %+v", svc)
				}
			}
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
