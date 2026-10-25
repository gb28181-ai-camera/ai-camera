package iot

import (
	"context"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svciot "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// TestPageIotDevices_ServiceGroupTypeFilter 按产品绑定的服务组类型过滤设备。
//
// 关键场景：
//   - 同一产品绑定多个服务组（gb28181 + aichat）时，同一台设备必须在两个
//     类型的查询里都出现；
//   - 列表项要带 service_groups 摘要，前端才能显示「属于哪些组」；
//   - 停用的服务组不再命中（与 OTA 下发语义一致）。
func TestPageIotDevices_ServiceGroupTypeFilter(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()

		gbGroup := mustGroup(t, ctx, uniq("GB_G"), model.IotServiceGroupGb28181)
		webGroup := mustGroup(t, ctx, uniq("WEB_G"), model.IotServiceGroupWebRTC)
		chatGroup := mustGroup(t, ctx, uniq("CHAT_G"), model.IotServiceGroupAIChat)

		webProduct := mustProduct(t, ctx, uniq("WEB_P"))
		chatProduct := mustProduct(t, ctx, uniq("CHAT_P"))
		multiProduct := mustProduct(t, ctx, uniq("MULTI_P"))
		plainProduct := mustProduct(t, ctx, uniq("PLAIN_P"))
		disabledProduct := mustProduct(t, ctx, uniq("DISABLED_P"))

		productSvc := svciot.NewProductService()
		for _, pair := range []struct {
			product *model.IotProduct
			groups  []int64
		}{
			{webProduct, []int64{webGroup.ID}},
			{chatProduct, []int64{chatGroup.ID}},
			{multiProduct, []int64{gbGroup.ID, chatGroup.ID}},
			{disabledProduct, []int64{chatGroup.ID}},
		} {
			if _, err := productSvc.BindServiceGroups(ctx, pair.product.ID, pair.groups); err != nil {
				t.Fatalf("绑定服务组失败 product=%d: %v", pair.product.ID, err)
			}
		}

		// 停用 chatGroup 只影响 disabledProduct 的查询语义：为了不影响其它
		// 用例，这里单独建一个停用组，而不是把共享的 chatGroup 停用。
		disabledGroup := mustGroup(t, ctx, uniq("DISABLED_G"), model.IotServiceGroupAIChat)
		if err := tx.Model(&model.IotServiceGroup{}).Where("id = ?", disabledGroup.ID).
			Update("active", false).Error; err != nil {
			t.Fatalf("停用服务组失败: %v", err)
		}
		if _, err := productSvc.BindServiceGroups(ctx, disabledProduct.ID, []int64{disabledGroup.ID}); err != nil {
			t.Fatalf("绑定停用服务组失败: %v", err)
		}

		deviceSvc := svciot.NewDeviceService()
		webDevice := mustDevice(t, ctx, uniq("FILTER_WEB"))
		chatDevice := mustDevice(t, ctx, uniq("FILTER_CHAT"))
		multiDevice := mustDevice(t, ctx, uniq("FILTER_MULTI"))
		plainDevice := mustDevice(t, ctx, uniq("FILTER_PLAIN"))
		disabledDevice := mustDevice(t, ctx, uniq("FILTER_DISABLED"))
		noProductDevice := mustDevice(t, ctx, uniq("FILTER_NONE"))
		for _, pair := range []struct {
			device  *model.Device
			product *model.IotProduct
		}{
			{webDevice, webProduct},
			{chatDevice, chatProduct},
			{multiDevice, multiProduct},
			{plainDevice, plainProduct},
			{disabledDevice, disabledProduct},
		} {
			if err := deviceSvc.SetProduct(ctx, pair.device.ID, &pair.product.ID); err != nil {
				t.Fatalf("设置设备产品失败: %v", err)
			}
		}

		ids := func(items []model.Device) map[int64]bool {
			out := make(map[int64]bool, len(items))
			for _, d := range items {
				out[d.ID] = true
			}
			return out
		}
		req := func(groupType string) *request.PageIotDeviceReq {
			return &request.PageIotDeviceReq{
				PageReq:          request.PageReq{Page: 1, PageSize: 50},
				ServiceGroupType: groupType,
			}
		}

		// WebRTC：只有 webDevice。
		webResp, err := deviceSvc.Page(ctx, req(model.IotServiceGroupWebRTC))
		if err != nil {
			t.Fatalf("WebRTC 分页失败: %v", err)
		}
		if webResp.Total != 1 || !ids(webResp.Items)[webDevice.ID] {
			t.Fatalf("WebRTC 过滤结果异常: total=%d items=%+v", webResp.Total, webResp.Items)
		}

		// AIChat：chatDevice + multiDevice；停用组设备不出现。
		chatResp, err := deviceSvc.Page(ctx, req(model.IotServiceGroupAIChat))
		if err != nil {
			t.Fatalf("AIChat 分页失败: %v", err)
		}
		chatIDs := ids(chatResp.Items)
		if chatResp.Total != 2 || !chatIDs[chatDevice.ID] || !chatIDs[multiDevice.ID] {
			t.Fatalf("AIChat 过滤结果异常: total=%d items=%+v", chatResp.Total, chatResp.Items)
		}
		if chatIDs[disabledDevice.ID] {
			t.Errorf("停用服务组的设备不应出现在 AIChat 过滤结果中")
		}

		// GB28181：multiDevice（同一产品同时绑 GB + AIChat）。
		gbResp, err := deviceSvc.Page(ctx, req(model.IotServiceGroupGb28181))
		if err != nil {
			t.Fatalf("GB28181 分页失败: %v", err)
		}
		if gbResp.Total != 1 || !ids(gbResp.Items)[multiDevice.ID] {
			t.Fatalf("GB28181 过滤结果异常: total=%d items=%+v", gbResp.Total, gbResp.Items)
		}

		// 列表项带服务组摘要。
		for _, d := range chatResp.Items {
			if d.ID == multiDevice.ID {
				if len(d.ServiceGroups) != 2 {
					t.Errorf("多组设备 service_groups 数量 = %d, 期望 2: %+v", len(d.ServiceGroups), d.ServiceGroups)
				}
			}
		}

		// 非法类型返回错误。
		if _, err := deviceSvc.Page(ctx, req("unknown")); err == nil {
			t.Errorf("非法 service_group_type 应返回错误")
		}

		// 未指定类型时返回全部设备（含未绑定产品的设备）。
		allResp, err := deviceSvc.Page(ctx, &request.PageIotDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 50},
		})
		if err != nil {
			t.Fatalf("全部设备分页失败: %v", err)
		}
		allIDs := ids(allResp.Items)
		for _, d := range []*model.Device{webDevice, chatDevice, multiDevice, plainDevice, disabledDevice, noProductDevice} {
			if !allIDs[d.ID] {
				t.Errorf("未指定类型时应包含设备 %d", d.ID)
			}
		}
	})
}
