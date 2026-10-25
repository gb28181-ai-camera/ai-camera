package iot

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	svciot "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT v0.3.1：复制鉴权信息 / 分享令牌 / App 认领与绑定 / ICE
// ════════════════════════════════════════════════════════════════

// setupProductWithMQTT 创建产品 + webrtc 服务组（MQTT/WebRTC 服务）并绑定。
func setupProductWithMQTT(t *testing.T, ctx context.Context) (*model.IotProduct, *model.IotServiceGroup, *model.IotService) {
	t.Helper()
	p := mustProduct(t, ctx, uniq("CRED_P"))
	g := mustGroup(t, ctx, uniq("CRED_G"), model.IotServiceGroupWebRTC)
	// 先配置服务再绑定：被产品引用后服务组进入只读状态。
	services, err := svciot.NewServiceGroupService().ReplaceServices(ctx, g.ID, &request.ReplaceIotServicesReq{
		Services: []request.IotServiceReq{
			{Code: "mqtt", Name: "MQTT", Protocol: model.IotProtocolMQTT, Host: "mqtt.example.com", Port: 1883},
			{Code: "webrtc", Name: "WebRTC", Protocol: model.IotProtocolWebRTC, Host: "rtc.example.com", Port: 443},
		},
	})
	if err != nil {
		t.Fatalf("替换服务失败: %v", err)
	}
	if _, err := svciot.NewProductService().BindServiceGroups(ctx, p.ID, []int64{g.ID}); err != nil {
		t.Fatalf("绑定服务组失败: %v", err)
	}
	var mqtt *model.IotService
	for i := range services {
		if services[i].Protocol == model.IotProtocolMQTT {
			mqtt = &services[i]
		}
	}
	if mqtt == nil {
		t.Fatal("未创建 MQTT 服务")
	}
	return p, g, mqtt
}

// TestCredentialsPlaintextAndMaskBoundary 明文只出现在 credentials 接口。
func TestCredentialsPlaintextAndMaskBoundary(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		t.Setenv("TURN_URLS", "turn:turn.example.com:3478?transport=udp")
		t.Setenv("TURN_REALM", "camera-test")

		p, g, mqtt := setupProductWithMQTT(t, ctx)
		rows, err := repository.NewIotProductServiceRepo().ListByProduct(ctx, p.ID)
		if err != nil || len(rows) != 2 {
			t.Fatalf("产品服务凭据数量 = %d, 期望 2", len(rows))
		}
		rawByService := make(map[int64]model.IotProductService, len(rows))
		for _, row := range rows {
			rawByService[row.ServiceID] = row
		}

		cred := svciot.NewCredentialsService()
		got, err := cred.ProductCredentials(ctx, p.ID)
		if err != nil {
			t.Fatalf("产品鉴权信息失败: %v", err)
		}
		if got.Product.Code != p.Code || got.Product.Name != p.Name {
			t.Errorf("产品摘要异常: %+v", got.Product)
		}
		if len(got.Groups) != 1 {
			t.Fatalf("服务组数量 = %d, 期望 1", len(got.Groups))
		}
		gr := got.Groups[0]
		if gr.Code != g.Code {
			t.Errorf("服务组标识异常: %+v", gr)
		}
		if len(gr.Services) != 2 {
			t.Errorf("服务数量 = %d, 期望 2", len(gr.Services))
		}
		raw := rawByService[mqtt.ID]
		var credMqtt *response.IotCredentialsService
		for i := range gr.Services {
			if gr.Services[i].Protocol == model.IotProtocolMQTT {
				credMqtt = &gr.Services[i]
			}
		}
		if credMqtt == nil {
			t.Fatal("凭据响应缺少 MQTT 服务")
		}
		if credMqtt.Username != raw.Username || credMqtt.Password != raw.Password {
			t.Errorf("应为产品×服务的明文凭据: %+v", credMqtt)
		}
		if credMqtt.Password == "****" || credMqtt.Password == "" {
			t.Errorf("明文接口不应返回脱敏/空密码: %q", credMqtt.Password)
		}
		if got.MQTT == nil || got.MQTT.URL != "mqtt://mqtt.example.com:1883" {
			t.Errorf("MQTT 连接信息异常: %+v", got.MQTT)
		}
		if got.MQTT.TopicPrefix != "iot/"+g.Code {
			t.Errorf("topic_prefix = %q, want %q", got.MQTT.TopicPrefix, "iot/"+g.Code)
		}
		if got.Turn == nil || len(got.Turn.URLs) != 1 || got.Turn.Realm != "camera-test" {
			t.Errorf("TURN 接入信息异常: %+v", got.Turn)
		}

		// 边界：普通读接口仍必须脱敏
		list, err := svciot.NewProductService().GetProductServices(ctx, p.ID)
		if err != nil {
			t.Fatalf("产品服务凭据列表失败: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("普通读接口服务数量 = %d, 期望 2", len(list))
		}
		for _, row := range list {
			if row.Password != "****" {
				t.Errorf("普通读接口必须脱敏，got %+v", row)
			}
		}
	})
}

// TestCredentialsTurnNilWhenUnconfigured 未配置 TURN 时 turn 必须为 null。
func TestCredentialsTurnNilWhenUnconfigured(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		t.Setenv("TURN_URLS", "")
		t.Setenv("TURN_STATIC_AUTH_SECRET", "")
		p, _, _ := setupProductWithMQTT(t, ctx)

		got, err := svciot.NewCredentialsService().ProductCredentials(ctx, p.ID)
		if err != nil {
			t.Fatalf("产品鉴权信息失败: %v", err)
		}
		if got.Turn != nil {
			t.Errorf("未配置 TURN 时 turn 应为 null，got %+v", got.Turn)
		}
	})
}

// TestShareTokenClaimOnce 分享令牌生成 → 认领成功 → 二次认领失败。
func TestShareTokenClaimOnce(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		dev := mustDevice(t, ctx, uniq("SHARE_D"))
		shareSvc := svciot.NewShareService()
		appSvc := svciot.NewAppDeviceService()

		st, err := shareSvc.CreateShareToken(ctx, dev.ID, "127.0.0.1")
		if err != nil {
			t.Fatalf("生成分享令牌失败: %v", err)
		}
		if !strings.HasPrefix(st.Token, "share_") || st.QRContent == "" {
			t.Fatalf("令牌/二维码内容异常: %+v", st)
		}
		// 与参考 App 对齐的 URL 格式：boticz.cn/deviceShareToken/{token}
		if want := "https://boticz.cn/deviceShareToken/" + st.Token; st.QRContent != want {
			t.Errorf("分享二维码格式异常: got %q want %q", st.QRContent, want)
		}

		// 未认领账号必须是 shared + platform_user_id IS NULL
		var acc model.IotDeviceAccount
		if err := tx.First(&acc, st.AccountID).Error; err != nil {
			t.Fatalf("读取分享账号失败: %v", err)
		}
		if acc.Role != model.IotAccountRoleShared || acc.PlatformUserID != nil || acc.ShareToken == nil {
			t.Fatalf("分享账号初始状态异常: %+v", acc)
		}

		const userA int64 = 1001
		res, err := appSvc.Claim(ctx, userA, &request.ClaimIotDeviceReq{Token: st.Token})
		if err != nil {
			t.Fatalf("认领失败: %v", err)
		}
		if res.Created || res.Role != model.IotAccountRoleShared || res.Device.ID != dev.ID {
			t.Errorf("认领结果异常: %+v", res)
		}
		if res.Account.PlatformUserID == nil || *res.Account.PlatformUserID != userA {
			t.Errorf("platform_user_id 未写入: %+v", res.Account)
		}

		var claimed model.IotDeviceAccount
		if err := tx.First(&claimed, st.AccountID).Error; err != nil {
			t.Fatalf("读取认领后账号失败: %v", err)
		}
		if claimed.PlatformUserID == nil || *claimed.PlatformUserID != userA {
			t.Errorf("库中 platform_user_id 未写入")
		}
		if claimed.ShareToken != nil {
			t.Errorf("认领后 share_token 必须清空，got %v", *claimed.ShareToken)
		}

		// 二次认领（同一用户或他人）都必须失败
		if _, err := appSvc.Claim(ctx, 1002, &request.ClaimIotDeviceReq{Token: st.Token}); !errors.Is(err, svciot.ErrShareTokenNotFound) {
			t.Errorf("二次认领应返回 ErrShareTokenNotFound，got %v", err)
		}
		if _, err := appSvc.Claim(ctx, userA, &request.ClaimIotDeviceReq{Token: st.Token}); !errors.Is(err, svciot.ErrShareTokenNotFound) {
			t.Errorf("同一用户二次认领也应失败，got %v", err)
		}
		// 不存在的令牌
		if _, err := appSvc.Claim(ctx, userA, &request.ClaimIotDeviceReq{Token: "share_not_exists"}); !errors.Is(err, svciot.ErrShareTokenNotFound) {
			t.Errorf("不存在令牌应返回 ErrShareTokenNotFound，got %v", err)
		}

		// 撤销：新令牌可被撤销，撤销后无法认领
		st2, err := shareSvc.CreateShareToken(ctx, dev.ID, "127.0.0.1")
		if err != nil {
			t.Fatalf("第二次生成令牌失败: %v", err)
		}
		n, err := shareSvc.RevokeShareToken(ctx, dev.ID)
		if err != nil {
			t.Fatalf("撤销令牌失败: %v", err)
		}
		if n != 1 {
			t.Errorf("应撤销 1 条未认领令牌，got %d", n)
		}
		if _, err := appSvc.Claim(ctx, 1003, &request.ClaimIotDeviceReq{Token: st2.Token}); !errors.Is(err, svciot.ErrShareTokenNotFound) {
			t.Errorf("撤销后认领应失败，got %v", err)
		}
	})
}

// TestAppBindOwnerIdempotentShared 绑定：owner/shared 选择、幂等、错误 secret。
func TestAppBindOwnerIdempotentShared(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		dev := mustDevice(t, ctx, uniq("BIND_D"))
		secret := "0123456789abcdef0123456789abcdef0123456789abcdef"
		if err := repository.NewIotDeviceRepo().SetSecret(ctx, dev.ID, secret); err != nil {
			t.Fatalf("设置设备密钥失败: %v", err)
		}
		appSvc := svciot.NewAppDeviceService()

		// 错误 secret
		if _, err := appSvc.Bind(ctx, 2001, &request.BindIotDeviceReq{DeviceCode: dev.DeviceCode, DeviceSecret: "wrong"}, "127.0.0.1"); !errors.Is(err, svciot.ErrDeviceSecretInvalid) {
			t.Errorf("错误密钥应返回 ErrDeviceSecretInvalid，got %v", err)
		}
		// 不存在的设备
		if _, err := appSvc.Bind(ctx, 2001, &request.BindIotDeviceReq{DeviceCode: uniq("NO_DEV"), DeviceSecret: secret}, "127.0.0.1"); !errors.Is(err, svciot.ErrDeviceSecretInvalid) {
			t.Errorf("不存在设备应返回 ErrDeviceSecretInvalid，got %v", err)
		}

		// 首次绑定：创建 owner
		r1, err := appSvc.Bind(ctx, 2001, &request.BindIotDeviceReq{
			DeviceCode: dev.DeviceCode, DeviceSecret: secret, Nickname: "客厅",
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("首次绑定失败: %v", err)
		}
		if !r1.Created || r1.Role != model.IotAccountRoleOwner || r1.Account.Nickname != "客厅" {
			t.Errorf("首次绑定应创建 owner，got %+v", r1)
		}
		if r1.Account.ActivatedAt == nil {
			t.Errorf("owner 应写入 activated_at")
		}
		var d model.Device
		if err := tx.First(&d, dev.ID).Error; err != nil {
			t.Fatalf("读取设备失败: %v", err)
		}
		if d.ActivatedAt == nil {
			t.Errorf("设备 activated_at 未写入")
		}

		// 幂等：同一用户再次绑定返回同一账号
		r2, err := appSvc.Bind(ctx, 2001, &request.BindIotDeviceReq{
			DeviceCode: dev.DeviceCode, DeviceSecret: secret,
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("重复绑定失败: %v", err)
		}
		if r2.Created || r2.Account.ID != r1.Account.ID || r2.Role != model.IotAccountRoleOwner {
			t.Errorf("重复绑定应幂等返回原账号，got %+v", r2)
		}

		// 第二个用户：已有 owner → shared
		r3, err := appSvc.Bind(ctx, 2002, &request.BindIotDeviceReq{
			DeviceCode: dev.DeviceCode, DeviceSecret: secret,
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("第二用户绑定失败: %v", err)
		}
		if !r3.Created || r3.Role != model.IotAccountRoleShared || r3.Account.ID == r1.Account.ID {
			t.Errorf("已有 owner 时应创建 shared，got %+v", r3)
		}

		// 账号数量：owner 1 + shared 1
		var ownerCount, sharedCount int64
		tx.Model(&model.IotDeviceAccount{}).Where("device_id = ? AND role = ?", dev.ID, model.IotAccountRoleOwner).Count(&ownerCount)
		tx.Model(&model.IotDeviceAccount{}).Where("device_id = ? AND role = ?", dev.ID, model.IotAccountRoleShared).Count(&sharedCount)
		if ownerCount != 1 || sharedCount != 1 {
			t.Errorf("账号数量异常 owner=%d shared=%d", ownerCount, sharedCount)
		}

		// 事件留痕
		var eventCount int64
		tx.Model(&model.IotDeviceEvent{}).
			Where("device_id = ? AND event = ?", dev.ID, model.IotEventAccountCreate).
			Count(&eventCount)
		if eventCount != 2 {
			t.Errorf("应有 2 条 account-create 事件，got %d", eventCount)
		}
	})
}

// TestAppBindSecretNotConfigured 设备未配置密钥时给出明确提示。
func TestAppBindSecretNotConfigured(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		dev := mustDevice(t, ctx, uniq("NOSEC_D"))
		_, err := svciot.NewAppDeviceService().Bind(ctx, 3001, &request.BindIotDeviceReq{
			DeviceCode: dev.DeviceCode, DeviceSecret: "whatever",
		}, "127.0.0.1")
		if !errors.Is(err, svciot.ErrDeviceSecretNotSet) {
			t.Errorf("未配置密钥应返回 ErrDeviceSecretNotSet，got %v", err)
		}
	})
}

// TestAppDeviceListAndScope 设备列表只返回当前用户，详情/服务组拒绝未绑定用户。
func TestAppDeviceListAndScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		p, g, _ := setupProductWithMQTT(t, ctx)

		devA := mustDevice(t, ctx, uniq("LIST_A"))
		devB := mustDevice(t, ctx, uniq("LIST_B"))
		devSvc := svciot.NewDeviceService()
		for _, dev := range []*model.Device{devA, devB} {
			if err := devSvc.SetProduct(ctx, dev.ID, &p.ID); err != nil {
				t.Fatalf("设置产品失败: %v", err)
			}
			if err := repository.NewIotDeviceRepo().SetSecret(ctx, dev.ID, "secret-"+dev.DeviceCode); err != nil {
				t.Fatalf("设置密钥失败: %v", err)
			}
		}

		appSvc := svciot.NewAppDeviceService()
		if _, err := appSvc.Bind(ctx, 4001, &request.BindIotDeviceReq{DeviceCode: devA.DeviceCode, DeviceSecret: "secret-" + devA.DeviceCode}, "127.0.0.1"); err != nil {
			t.Fatalf("用户1绑定 A 失败: %v", err)
		}
		if _, err := appSvc.Bind(ctx, 4002, &request.BindIotDeviceReq{DeviceCode: devB.DeviceCode, DeviceSecret: "secret-" + devB.DeviceCode}, "127.0.0.1"); err != nil {
			t.Fatalf("用户2绑定 B 失败: %v", err)
		}

		list1, err := appSvc.List(ctx, 4001)
		if err != nil {
			t.Fatalf("用户1列表失败: %v", err)
		}
		if len(list1) != 1 || list1[0].ID != devA.ID {
			t.Fatalf("用户1只应看到设备 A，got %+v", list1)
		}
		if list1[0].Product == nil || list1[0].Product.Code != p.Code {
			t.Errorf("列表应带产品摘要: %+v", list1[0].Product)
		}

		list3, err := appSvc.List(ctx, 4003)
		if err != nil {
			t.Fatalf("用户3列表失败: %v", err)
		}
		if len(list3) != 0 {
			t.Errorf("未绑定用户应看到空列表，got %+v", list3)
		}

		// 用户1不能查看用户2的设备
		if _, err := appSvc.Get(ctx, 4001, devB.ID); !errors.Is(err, svciot.ErrDeviceNotBound) {
			t.Errorf("跨用户详情应拒绝，got %v", err)
		}
		if _, err := appSvc.Services(ctx, 4001, devB.ID); !errors.Is(err, svciot.ErrDeviceNotBound) {
			t.Errorf("跨用户服务组应拒绝，got %v", err)
		}

		detailed, err := appSvc.Get(ctx, 4001, devA.ID)
		if err != nil {
			t.Fatalf("设备详情失败: %v", err)
		}
		if detailed.Device.Role != model.IotAccountRoleOwner || detailed.Product == nil || detailed.Product.Code != p.Code {
			t.Errorf("详情字段异常: %+v", detailed)
		}
		if len(detailed.ServiceGroups) != 1 || detailed.ServiceGroups[0].Code != g.Code {
			t.Errorf("服务组摘要异常: %+v", detailed.ServiceGroups)
		}

		svc, err := appSvc.Services(ctx, 4001, devA.ID)
		if err != nil {
			t.Fatalf("服务组凭据失败: %v", err)
		}
		if len(svc.Groups) != 1 || len(svc.Groups[0].Services) != 2 {
			t.Fatalf("App 服务组/服务数量异常: %+v", svc.Groups)
		}
		for _, item := range svc.Groups[0].Services {
			if item.Username == "" || item.Password == "****" || item.Password == "" {
				t.Errorf("App 服务应返回明文凭据: %+v", item)
			}
		}
		if svc.MQTT == nil || svc.MQTT.TopicPrefix != "iot/"+g.Code {
			t.Errorf("App MQTT 信息异常: %+v", svc.MQTT)
		}
		if svc.Turn != nil {
			t.Errorf("App services 不应下发 TURN 凭据")
		}
	})
}

// TestAppDeviceWithoutProductServices 未绑定产品的设备返回空服务组而不是报错。
func TestAppDeviceWithoutProductServices(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		dev := mustDevice(t, ctx, uniq("NOPROD_D"))
		if err := repository.NewIotDeviceRepo().SetSecret(ctx, dev.ID, "noprod-secret"); err != nil {
			t.Fatalf("设置密钥失败: %v", err)
		}
		appSvc := svciot.NewAppDeviceService()
		if _, err := appSvc.Bind(ctx, 5001, &request.BindIotDeviceReq{
			DeviceCode: dev.DeviceCode, DeviceSecret: "noprod-secret",
		}, "127.0.0.1"); err != nil {
			t.Fatalf("绑定失败: %v", err)
		}
		svc, err := appSvc.Services(ctx, 5001, dev.ID)
		if err != nil {
			t.Fatalf("未绑定产品时应返回空服务组: %v", err)
		}
		if len(svc.Groups) != 0 || svc.MQTT != nil {
			t.Errorf("应返回空服务组与空 MQTT，got %+v", svc)
		}
	})
}

// TestIceServers 未配置 TURN 返回空；配置后签发给当前用户。
func TestIceServers(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		appSvc := svciot.NewAppDeviceService()

		t.Setenv("TURN_STATIC_AUTH_SECRET", "")
		empty, err := appSvc.IceServers(ctx, 6001)
		if err != nil {
			t.Fatalf("未配置 TURN 时应成功返回空列表: %v", err)
		}
		if empty == nil || len(empty) != 0 {
			t.Errorf("未配置 TURN 时应为空数组（非 nil），got %#v", empty)
		}

		t.Setenv("TURN_STATIC_AUTH_SECRET", "test-turn-secret")
		t.Setenv("TURN_URLS", "turn:turn.example.com:3478?transport=udp")
		t.Setenv("TURN_REALM", "camera-test")
		got, err := appSvc.IceServers(ctx, 6001)
		if err != nil {
			t.Fatalf("签发 ICE 失败: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("应返回 1 条 ICE 配置，got %d", len(got))
		}
		if len(got[0].URLs) != 1 || got[0].Realm != "camera-test" {
			t.Errorf("ICE 字段异常: %+v", got[0])
		}
		if !strings.Contains(got[0].Username, ":6001") {
			t.Errorf("TURN username 应包含过期时间与 user_id，got %q", got[0].Username)
		}
		if got[0].Credential == "" || got[0].ExpiresAt <= 0 {
			t.Errorf("凭据/过期时间异常: %+v", got[0])
		}
	})
}

// TestDeviceQRAndDetail 设备二维码包含产品编码 + 设备编码，详情直接携带。
func TestDeviceQRAndDetail(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		// 未绑定产品：不允许拼出空产品编码的二维码。
		unbound := mustDevice(t, ctx, uniq("QR_NO_P"))
		if _, err := svciot.NewShareService().DeviceQR(ctx, unbound.ID); !errors.Is(err, svciot.ErrDeviceProductRequired) {
			t.Errorf("未绑定产品应返回 ErrDeviceProductRequired，got %v", err)
		}

		p := mustProduct(t, ctx, uniq("QR_P"))
		dev := mustDevice(t, ctx, uniq("QR_D"))
		if err := svciot.NewDeviceService().SetProduct(ctx, dev.ID, &p.ID); err != nil {
			t.Fatalf("设置产品失败: %v", err)
		}

		qr, err := svciot.NewShareService().DeviceQR(ctx, dev.ID)
		if err != nil {
			t.Fatalf("生成设备二维码失败: %v", err)
		}
		if qr.ProductCode != p.Code || qr.DeviceCode != dev.DeviceCode {
			t.Errorf("二维码标识异常: %+v", qr)
		}
		// 与参考 App 对齐的 URL 格式：boticz.cn/qrDevice/{product_code}/{device_code}
		want := "https://boticz.cn/qrDevice/" + url.PathEscape(p.Code) + "/" + url.PathEscape(dev.DeviceCode)
		if qr.QRContent != want {
			t.Errorf("设备二维码格式异常: got %q want %q", qr.QRContent, want)
		}
		if strings.Contains(qr.QRContent, "secret") {
			t.Errorf("设备二维码不应包含密钥: %s", qr.QRContent)
		}

		detail, err := svciot.NewDeviceService().Get(ctx, dev.ID)
		if err != nil {
			t.Fatalf("设备详情失败: %v", err)
		}
		if detail.QRContent != qr.QRContent {
			t.Errorf("详情 qr_content 与独立接口不一致: %q vs %q", detail.QRContent, qr.QRContent)
		}
	})
}

// TestBindBySIP SIP 侧绑定：空编码写入、同编码幂等、异编码拒绝、产品不匹配拒绝。
func TestBindBySIP(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		p := mustProduct(t, ctx, uniq("SIP_P"))
		other := mustProduct(t, ctx, uniq("SIP_OTHER"))
		dev := mustDevice(t, ctx, uniq("SIP_D"))
		// 独立国标设备：没有产品编码/设备编码，不参与产品绑定。
		independent := mustDevice(t, ctx, uniq("SIP_INDEP"))
		if got, err := svciot.NewDeviceService().BindBySIP(ctx, "", "", "34020000001320000009"); got != nil || err != nil {
			t.Errorf("无产品/设备编码时不应绑定: got=%v err=%v", got, err)
		}
		if got, err := svciot.NewDeviceService().BindBySIP(ctx, p.Code, "", "34020000001320000009"); got != nil || err != nil {
			t.Errorf("只带产品编码时不应绑定: got=%v err=%v", got, err)
		}
		var untouched model.Device
		if err := tx.First(&untouched, independent.ID).Error; err != nil {
			t.Fatalf("读取独立设备失败: %v", err)
		}
		if untouched.GbDeviceID != "" || untouched.ProductID != nil {
			t.Errorf("独立设备不应被改动: gb=%q product=%v", untouched.GbDeviceID, untouched.ProductID)
		}
		svc := svciot.NewDeviceService()
		if err := svc.SetProduct(ctx, dev.ID, &p.ID); err != nil {
			t.Fatalf("设置产品失败: %v", err)
		}
		const gbID = "34020000001320000001"

		// 空编码写入 + 置 activated
		got, err := svc.BindBySIP(ctx, p.Code, dev.DeviceCode, gbID)
		if err != nil {
			t.Fatalf("SIP 绑定失败: %v", err)
		}
		if got.GbDeviceID != gbID || got.RegisterStatus != model.GbRegisterStatusActivated {
			t.Errorf("绑定后字段异常: gb=%q status=%q", got.GbDeviceID, got.RegisterStatus)
		}

		// 同编码幂等
		again, err := svc.BindBySIP(ctx, p.Code, dev.DeviceCode, gbID)
		if err != nil {
			t.Fatalf("同编码幂等失败: %v", err)
		}
		if again.GbDeviceID != gbID {
			t.Errorf("幂等后编码变化: %q", again.GbDeviceID)
		}

		// 不同编码拒绝
		if _, err := svc.BindBySIP(ctx, p.Code, dev.DeviceCode, "34020000001320000002"); !errors.Is(err, svciot.ErrGbDeviceIDConflict) {
			t.Errorf("异编码应返回 ErrGbDeviceIDConflict，got %v", err)
		}
		var unchanged model.Device
		if err := tx.First(&unchanged, dev.ID).Error; err != nil {
			t.Fatalf("读取设备失败: %v", err)
		}
		if unchanged.GbDeviceID != gbID {
			t.Errorf("冲突时不得改绑，库中为 %q", unchanged.GbDeviceID)
		}

		// 产品不匹配拒绝
		if _, err := svc.BindBySIP(ctx, other.Code, dev.DeviceCode, "34020000001320000003"); !errors.Is(err, svciot.ErrDeviceProductMismatch) {
			t.Errorf("产品不匹配应返回 ErrDeviceProductMismatch，got %v", err)
		}
		// 设备不存在
		if _, err := svc.BindBySIP(ctx, p.Code, uniq("SIP_NONE"), gbID); !errors.Is(err, svciot.ErrDeviceNotFound) {
			t.Errorf("设备不存在应返回 ErrDeviceNotFound，got %v", err)
		}
		// 空国标编码拒绝
		if _, err := svc.BindBySIP(ctx, p.Code, dev.DeviceCode, "  "); !errors.Is(err, svciot.ErrGbDeviceIDRequired) {
			t.Errorf("空国标编码应返回 ErrGbDeviceIDRequired，got %v", err)
		}
	})
}

// TestAppBindByQR 扫码绑定：产品匹配校验、owner/shared 选择、幂等。
func TestAppBindByQR(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		p := mustProduct(t, ctx, uniq("BQR_P"))
		other := mustProduct(t, ctx, uniq("BQR_OTHER"))
		dev := mustDevice(t, ctx, uniq("BQR_D"))
		if err := svciot.NewDeviceService().SetProduct(ctx, dev.ID, &p.ID); err != nil {
			t.Fatalf("设置产品失败: %v", err)
		}
		appSvc := svciot.NewAppDeviceService()

		// 产品不匹配
		if _, err := appSvc.BindByQR(ctx, 7001, &request.BindIotDeviceQRReq{
			ProductCode: other.Code, DeviceCode: dev.DeviceCode,
		}, "127.0.0.1"); !errors.Is(err, svciot.ErrBindQRInvalid) {
			t.Errorf("产品不匹配应返回 ErrBindQRInvalid，got %v", err)
		}
		// 设备不存在
		if _, err := appSvc.BindByQR(ctx, 7001, &request.BindIotDeviceQRReq{
			ProductCode: p.Code, DeviceCode: uniq("BQR_NONE"),
		}, "127.0.0.1"); !errors.Is(err, svciot.ErrBindQRInvalid) {
			t.Errorf("设备不存在应返回 ErrBindQRInvalid，got %v", err)
		}

		// 首次：owner
		r1, err := appSvc.BindByQR(ctx, 7001, &request.BindIotDeviceQRReq{
			ProductCode: p.Code, DeviceCode: dev.DeviceCode,
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("扫码绑定失败: %v", err)
		}
		if !r1.Created || r1.Role != model.IotAccountRoleOwner {
			t.Errorf("首次扫码应创建 owner，got %+v", r1)
		}

		// 幂等
		r2, err := appSvc.BindByQR(ctx, 7001, &request.BindIotDeviceQRReq{
			ProductCode: p.Code, DeviceCode: dev.DeviceCode,
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("重复扫码绑定失败: %v", err)
		}
		if r2.Created || r2.Account.ID != r1.Account.ID {
			t.Errorf("重复扫码应幂等，got %+v", r2)
		}

		// 第二个用户 → shared
		r3, err := appSvc.BindByQR(ctx, 7002, &request.BindIotDeviceQRReq{
			ProductCode: p.Code, DeviceCode: dev.DeviceCode,
		}, "127.0.0.1")
		if err != nil {
			t.Fatalf("第二用户扫码绑定失败: %v", err)
		}
		if !r3.Created || r3.Role != model.IotAccountRoleShared {
			t.Errorf("已有 owner 时应创建 shared，got %+v", r3)
		}
	})
}
