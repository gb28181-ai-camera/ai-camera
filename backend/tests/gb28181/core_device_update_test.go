package gb28181_test

import (
	"context"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标设备档案编辑（WVP 对齐 PUT /web/gb/devices/:id）
// ════════════════════════════════════════════════════════════════

func strPtr(v string) *string { return &v }

func TestCoreGbDeviceUpdateFieldsAndValidation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CORE_UPD_001", "旧设备名", "34020000001320001001")
		ch := createChannel(t, ctx, d, "34020000001320001001", "旧通道名", "")

		// 媒体节点：更新时校验 server_id 必须存在。
		ms := &model.GbMediaServer{Name: "核心测试节点", ServerID: "zlm-core-upd", IP: "127.0.0.1", HTTPPort: 80}
		if err := tx.Create(ms).Error; err != nil {
			t.Fatalf("创建媒体节点失败: %v", err)
		}

		svc := gbsvc.NewGbDeviceService()
		req := &request.UpdateGbDeviceReq{
			Name:          strPtr("东门球机"),
			Location:      strPtr("园区东门"),
			Remark:        strPtr("核心测试"),
			SipRealm:      strPtr("3402000000"),
			SipTransport:  strPtr("tcp"),
			StreamMode:    strPtr("TCP-PASSIVE"),
			Charset:       strPtr("utf8"),
			GbVersion:     strPtr("2022"),
			MediaServerID: strPtr("zlm-core-upd"),
			LocalIP:       strPtr("192.168.1.10"),
			SDPIP:         strPtr("10.0.0.5"),
			Latitude:      strPtr("39.916527"),
			Longitude:     strPtr("116.397128"),
			Active:        boolPtr(true),
		}
		if err := svc.Update(ctx, d.ID, req); err != nil {
			t.Fatalf("更新设备失败: %v", err)
		}

		var saved model.Device
		if err := tx.First(&saved, d.ID).Error; err != nil {
			t.Fatalf("读取设备失败: %v", err)
		}
		if saved.Name != "东门球机" || saved.Location != "园区东门" || saved.Remark != "核心测试" {
			t.Fatalf("基础字段未更新: %+v", saved)
		}
		if saved.SipRealm != "3402000000" || saved.SipTransport != model.GbSipTransportTCP ||
			saved.StreamMode != model.GbStreamModeTCPPassive || saved.Charset != model.GbCharsetUTF8 ||
			saved.GbVersion != model.GbVersion2022 {
			t.Fatalf("国标接入字段未更新: %+v", saved)
		}
		if saved.MediaServerID != "zlm-core-upd" || saved.LocalIP != "192.168.1.10" || saved.SDPIP != "10.0.0.5" {
			t.Fatalf("媒体/网络字段未更新: %+v", saved)
		}
		if saved.Latitude != "39.916527" || saved.Longitude != "116.397128" {
			t.Fatalf("经纬度未更新: lat=%q lon=%q", saved.Latitude, saved.Longitude)
		}
		if saved.UpdatedBy == "" || saved.UpdatedDT.IsZero() {
			t.Fatalf("审计字段未写入: updated_by=%q updated_dt=%v", saved.UpdatedBy, saved.UpdatedDT)
		}

		// 详情与列表都必须带 gb_version（common.go 的映射不含该字段，由 service 补齐）。
		detail, err := svc.Get(ctx, d.ID)
		if err != nil {
			t.Fatalf("查询详情失败: %v", err)
		}
		if detail.GbVersion != model.GbVersion2022 {
			t.Fatalf("详情未返回 gb_version: %+v", detail.GbDeviceItem)
		}
		page, err := svc.Page(ctx, &request.PageGbDeviceReq{Keyword: "东门球机"}, allScope())
		if err != nil {
			t.Fatalf("分页查询失败: %v", err)
		}
		if page.Total != 1 || page.Items[0].GbVersion != model.GbVersion2022 {
			t.Fatalf("列表未返回 gb_version: %+v", page.Items)
		}

		// ── 校验：非法值必须拒绝，且不得半更新 ──
		cases := []struct {
			name string
			req  *request.UpdateGbDeviceReq
			want string
		}{
			{"空名称", &request.UpdateGbDeviceReq{Name: strPtr("  ")}, "设备名称"},
			{"非法版本", &request.UpdateGbDeviceReq{GbVersion: strPtr("2020")}, "协议版本"},
			{"非法传输", &request.UpdateGbDeviceReq{SipTransport: strPtr("BOTH")}, "传输方式"},
			{"非法流模式", &request.UpdateGbDeviceReq{StreamMode: strPtr("RTP")}, "媒体传输"},
			{"非法字符集", &request.UpdateGbDeviceReq{Charset: strPtr("BIG5")}, "字符集"},
			{"非法realm", &request.UpdateGbDeviceReq{SipRealm: strPtr("123")}, "SIP 域"},
			{"非法IP", &request.UpdateGbDeviceReq{LocalIP: strPtr("not-an-ip")}, "local_ip"},
			{"非法纬度", &request.UpdateGbDeviceReq{Latitude: strPtr("91")}, "纬度"},
			{"非法经度", &request.UpdateGbDeviceReq{Longitude: strPtr("181")}, "经度"},
			{"不存在的媒体节点", &request.UpdateGbDeviceReq{MediaServerID: strPtr("no-such-node")}, "媒体节点"},
		}
		for _, tc := range cases {
			err := svc.Update(ctx, d.ID, tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s：应返回包含 %q 的错误，实际 %v", tc.name, tc.want, err)
			}
		}

		// 在线设备禁止修改国标编码。
		online := &request.UpdateGbDeviceReq{GbDeviceID: strPtr("34020000001320001009")}
		if err := svc.Update(ctx, d.ID, online); err == nil || !strings.Contains(err.Error(), "在线") {
			t.Fatalf("在线设备修改国标编码应被拒绝，实际 %v", err)
		}

		// 离线后允许修改，并同步通道冗余列。
		if err := tx.Model(&model.Device{}).Where("id = ?", d.ID).Update("status", model.DeviceStatusOffline).Error; err != nil {
			t.Fatalf("置离线失败: %v", err)
		}
		if err := svc.Update(ctx, d.ID, &request.UpdateGbDeviceReq{
			GbDeviceID: strPtr("34020000001320001009"),
		}); err != nil {
			t.Fatalf("离线设备修改国标编码失败: %v", err)
		}
		if err := tx.First(&saved, d.ID).Error; err != nil {
			t.Fatalf("读取设备失败: %v", err)
		}
		if saved.GbDeviceID != "34020000001320001009" {
			t.Fatalf("国标编码未更新: %q", saved.GbDeviceID)
		}
		var savedCh model.GbChannel
		if err := tx.First(&savedCh, ch.ID).Error; err != nil {
			t.Fatalf("读取通道失败: %v", err)
		}
		if savedCh.GbDeviceID != "34020000001320001009" {
			t.Fatalf("通道冗余国标编码未同步: %q", savedCh.GbDeviceID)
		}
	})
}

func TestCoreGbDeviceUpdateNotFound(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		err := gbsvc.NewGbDeviceService().Update(ctx, 99999999, &request.UpdateGbDeviceReq{Name: strPtr("x")})
		if err != gbsvc.ErrGbDeviceNotFound {
			t.Fatalf("不存在的设备应返回 ErrGbDeviceNotFound，实际 %v", err)
		}
	})
}
