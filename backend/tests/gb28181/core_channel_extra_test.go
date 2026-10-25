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
// 国标通道设备参数扩展（WVP 对齐 PUT /web/gb/channels/:id）
// ════════════════════════════════════════════════════════════════

func f64Ptr(v float64) *float64 { return &v }

func TestCoreGbChannelUpdateExtraFields(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CORE_CH_001", "通道扩展设备", "34020000001320002001")
		ch := createChannel(t, ctx, d, "34020000001320002001", "原通道", "")

		svc := gbsvc.NewGbChannelService()
		req := &request.UpdateGbChannelReq{
			Longitude:       f64Ptr(116.397128),
			Latitude:        f64Ptr(39.916527),
			DirectionType:   int64PtrLocal(2),
			PositionType:    int64PtrLocal(1),
			RoomType:        int64PtrLocal(3),
			UseType:         int64PtrLocal(4),
			SupplyLightType: int64PtrLocal(5),
			Secrecy:         int64PtrLocal(1),
		}
		if err := svc.Update(ctx, ch.ID, req); err != nil {
			t.Fatalf("更新通道扩展字段失败: %v", err)
		}

		var saved model.GbChannel
		if err := tx.First(&saved, ch.ID).Error; err != nil {
			t.Fatalf("读取通道失败: %v", err)
		}
		if saved.Longitude != 116.397128 || saved.Latitude != 39.916527 {
			t.Fatalf("经纬度未保存: %f,%f", saved.Longitude, saved.Latitude)
		}
		if saved.DirectionType != 2 || saved.PositionType != 1 || saved.RoomType != 3 ||
			saved.UseType != 4 || saved.SupplyLightType != 5 || saved.Secrecy != 1 {
			t.Fatalf("扩展枚举未保存: %+v", saved)
		}

		// 详情必须原样返回，供前端设备参数页展示。
		detail, err := svc.Get(ctx, ch.ID)
		if err != nil {
			t.Fatalf("查询详情失败: %v", err)
		}
		if detail.Longitude != 116.397128 || detail.Latitude != 39.916527 ||
			detail.DirectionType != 2 || detail.PositionType != 1 || detail.RoomType != 3 ||
			detail.UseType != 4 || detail.SupplyLightType != 5 || detail.Secrecy != 1 {
			t.Fatalf("详情扩展字段缺失: %+v", detail.GbChannelItem)
		}

		// 分页列表同样补齐。
		page, err := svc.Page(ctx, &request.PageGbChannelReq{Keyword: "原通道"}, allScope())
		if err != nil {
			t.Fatalf("通道分页失败: %v", err)
		}
		if page.Total != 1 || page.Items[0].Longitude != 116.397128 || page.Items[0].DirectionType != 2 {
			t.Fatalf("列表扩展字段缺失: %+v", page.Items)
		}

		// 非法经纬度/方位必须拒绝。
		bad := []struct {
			req  *request.UpdateGbChannelReq
			want string
		}{
			{&request.UpdateGbChannelReq{Longitude: f64Ptr(181)}, "经度"},
			{&request.UpdateGbChannelReq{Latitude: f64Ptr(-91)}, "纬度"},
			{&request.UpdateGbChannelReq{DirectionType: int64PtrLocal(9)}, "监视方位"},
		}
		for _, tc := range bad {
			err := svc.Update(ctx, ch.ID, tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("应返回包含 %q 的错误，实际 %v", tc.want, err)
			}
		}
	})
}

func int64PtrLocal(v int64) *int64 { return &v }
