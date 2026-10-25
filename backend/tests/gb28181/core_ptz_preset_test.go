package gb28181_test

import (
	"context"
	"errors"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 预置位名称持久化（WVP 对齐）
// ════════════════════════════════════════════════════════════════

// coreCaptureController 记录业务层下发的 MESSAGE 正文。
type coreCaptureController struct {
	bodies []string
}

func (f *coreCaptureController) QueryCatalog(context.Context, *model.Device) error { return nil }
func (f *coreCaptureController) Subscribe(context.Context, *model.Device, string, int) error {
	return nil
}
func (f *coreCaptureController) Unsubscribe(context.Context, *model.Device, string) error { return nil }
func (f *coreCaptureController) SendMessage(_ context.Context, _ *model.Device, body string) error {
	f.bodies = append(f.bodies, body)
	return nil
}

func TestCorePtzPresetNamePersistence(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CORE_PTZ_001", "预置位设备", "34020000001320004001")
		ch := createChannel(t, ctx, d, "34020000001320004001", "预置位通道", "")

		fake := &coreCaptureController{}
		control.Set(fake)
		defer control.Set(nil)

		svc := gbsvc.NewGbPtzService()

		// 参数校验：非法编号在发报文前就拒绝。
		if err := svc.AddPreset(ctx, ch.ID, 0, "非法"); !errors.Is(err, gbsvc.ErrGbPtzPresetIDInvalid) {
			t.Fatalf("非法预置位编号应被拒绝，实际 %v", err)
		}

		// 设置预置位并携带名称。
		if err := svc.AddPreset(ctx, ch.ID, 3, "大门"); err != nil {
			t.Fatalf("设置预置位失败: %v", err)
		}
		if len(fake.bodies) != 1 {
			t.Fatalf("应下发 1 条 PTZ 指令，实际 %d", len(fake.bodies))
		}

		repo := repository.NewGbPtzPresetRepo()
		row, err := repo.Get(ctx, ch.ID, 3)
		if err != nil {
			t.Fatalf("预置位名称未落库: %v", err)
		}
		if row.Name != "大门" {
			t.Fatalf("预置位名称不正确: %q", row.Name)
		}

		// 列表 DB 优先，且无需信令服务。
		control.Set(nil)
		items, err := svc.ListPresets(ctx, ch.ID)
		if err != nil {
			t.Fatalf("查询预置位列表失败: %v", err)
		}
		if len(items) != 1 || items[0].PresetID != 3 || items[0].Name != "大门" {
			t.Fatalf("预置位列表不正确: %+v", items)
		}

		// 重命名只改平台侧名称，不下发设备指令。
		if err := svc.RenamePreset(ctx, ch.ID, 3, "东门"); err != nil {
			t.Fatalf("重命名失败: %v", err)
		}
		row, err = repo.Get(ctx, ch.ID, 3)
		if err != nil {
			t.Fatalf("重命名后读取失败: %v", err)
		}
		if row.Name != "东门" {
			t.Fatalf("重命名未生效: %q", row.Name)
		}
		if err := svc.RenamePreset(ctx, ch.ID, 3, "  "); !errors.Is(err, gbsvc.ErrGbPtzPresetNameInvalid) {
			t.Fatalf("空名称应被拒绝，实际 %v", err)
		}

		// 重新设置位置但不带名称时，已有名称必须保留。
		control.Set(fake)
		if err := svc.AddPreset(ctx, ch.ID, 3); err != nil {
			t.Fatalf("无名称重设预置位失败: %v", err)
		}
		row, err = repo.Get(ctx, ch.ID, 3)
		if err != nil {
			t.Fatalf("重设后读取失败: %v", err)
		}
		if row.Name != "东门" {
			t.Fatalf("无名称重设不应清空已有名称: %q", row.Name)
		}

		// 删除预置位：设备指令 + 名称记录一并清理。
		if err := svc.DeletePreset(ctx, ch.ID, 3); err != nil {
			t.Fatalf("删除预置位失败: %v", err)
		}
		if _, err := repo.Get(ctx, ch.ID, 3); err == nil {
			t.Fatalf("删除后名称记录仍存在")
		}
	})
}

// TestCorePtzPresetUpsertConflict 同一通道同一编号重复设置必须覆盖而不是报唯一键冲突。
func TestCorePtzPresetUpsertConflict(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CORE_PTZ_002", "预置位设备2", "34020000001320004002")
		ch := createChannel(t, ctx, d, "34020000001320004002", "预置位通道2", "")

		repo := repository.NewGbPtzPresetRepo()
		if err := repo.Upsert(ctx, ch.ID, 1, "旧名"); err != nil {
			t.Fatalf("首次写入失败: %v", err)
		}
		if err := repo.Upsert(ctx, ch.ID, 1, "新名"); err != nil {
			t.Fatalf("重复写入应覆盖，实际 %v", err)
		}
		items, err := repo.ListByChannel(ctx, ch.ID)
		if err != nil {
			t.Fatalf("列表失败: %v", err)
		}
		if len(items) != 1 || items[0].Name != "新名" {
			t.Fatalf("upsert 结果不正确: %+v", items)
		}

		// 删除后可以重新设置同一编号（物理删除，不受软删除唯一键影响）。
		if err := repo.Delete(ctx, ch.ID, 1); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if err := repo.Upsert(ctx, ch.ID, 1, "重设"); err != nil {
			t.Fatalf("删除后重设失败: %v", err)
		}
	})
}
