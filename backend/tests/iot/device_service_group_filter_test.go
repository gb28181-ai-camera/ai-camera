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
// 「实时音视频」「智能语音聊天」两个页面复用 GET /web/iot/devices，
// 通过 service_group_type 只展示对应服务组下的产品设备；未绑定产品的设备
// 不属于任何服务组，两个页面都不应出现。
func TestPageIotDevices_ServiceGroupTypeFilter(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()

		webGroup := mustGroup(t, ctx, uniq("WEB_G"), model.IotServiceGroupWebRTC)
		chatGroup := mustGroup(t, ctx, uniq("CHAT_G"), model.IotServiceGroupAIChat)

		webProduct := mustProduct(t, ctx, uniq("WEB_P"))
		chatProduct := mustProduct(t, ctx, uniq("CHAT_P"))
		plainProduct := mustProduct(t, ctx, uniq("PLAIN_P"))

		productSvc := svciot.NewProductService()
		if _, err := productSvc.BindServiceGroups(ctx, webProduct.ID, []int64{webGroup.ID}); err != nil {
			t.Fatalf("绑定 WebRTC 服务组失败: %v", err)
		}
		if _, err := productSvc.BindServiceGroups(ctx, chatProduct.ID, []int64{chatGroup.ID}); err != nil {
			t.Fatalf("绑定 AIChat 服务组失败: %v", err)
		}

		deviceSvc := svciot.NewDeviceService()
		webDevice := mustDevice(t, ctx, uniq("FILTER_WEB"))
		chatDevice := mustDevice(t, ctx, uniq("FILTER_CHAT"))
		plainDevice := mustDevice(t, ctx, uniq("FILTER_PLAIN"))
		noProductDevice := mustDevice(t, ctx, uniq("FILTER_NONE"))
		for _, pair := range []struct {
			device  *model.Device
			product *model.IotProduct
		}{
			{webDevice, webProduct},
			{chatDevice, chatProduct},
			{plainDevice, plainProduct},
		} {
			if err := deviceSvc.SetProduct(ctx, pair.device.ID, &pair.product.ID); err != nil {
				t.Fatalf("设置设备产品失败: %v", err)
			}
		}

		webPage, err := deviceSvc.Page(ctx, &request.PageIotDeviceReq{
			PageReq:          request.PageReq{Page: 1, PageSize: 50},
			ServiceGroupType: model.IotServiceGroupWebRTC,
		})
		if err != nil {
			t.Fatalf("WebRTC 设备分页失败: %v", err)
		}
		if webPage.Total != 1 || len(webPage.Items) != 1 || webPage.Items[0].ID != webDevice.ID {
			t.Fatalf("WebRTC 过滤结果异常: total=%d items=%+v", webPage.Total, webPage.Items)
		}

		chatPage, err := deviceSvc.Page(ctx, &request.PageIotDeviceReq{
			PageReq:          request.PageReq{Page: 1, PageSize: 50},
			ServiceGroupType: model.IotServiceGroupAIChat,
		})
		if err != nil {
			t.Fatalf("AIChat 设备分页失败: %v", err)
		}
		if chatPage.Total != 1 || len(chatPage.Items) != 1 || chatPage.Items[0].ID != chatDevice.ID {
			t.Fatalf("AIChat 过滤结果异常: total=%d items=%+v", chatPage.Total, chatPage.Items)
		}

		// 未指定类型时返回全部设备（含未绑定产品的设备）。
		allPage, err := deviceSvc.Page(ctx, &request.PageIotDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 50},
		})
		if err != nil {
			t.Fatalf("全部设备分页失败: %v", err)
		}
		ids := make(map[int64]bool, len(allPage.Items))
		for _, d := range allPage.Items {
			ids[d.ID] = true
		}
		for _, d := range []*model.Device{webDevice, chatDevice, plainDevice, noProductDevice} {
			if !ids[d.ID] {
				t.Errorf("未指定类型时应包含设备 %d", d.ID)
			}
		}
	})
}
