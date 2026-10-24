package gb28181_test

import (
	"context"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// TestSipLogCleanup_DeletesOnlyExpiredRows 验证按 created_dt 物理删除：
// 过期行消失、未过期行保留。分批逻辑本身由 service 包内的纯函数单测覆盖。
func TestSipLogCleanup_DeletesOnlyExpiredRows(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		now := time.Now()
		old1 := now.AddDate(0, 0, -40)
		old2 := now.AddDate(0, 0, -31)
		fresh := now.AddDate(0, 0, -1)

		rows := []*model.GbSipLog{
			{GbDeviceID: "34020000001320000001", Direction: "in", Method: "REGISTER", Result: "success", CreatedDT: old1},
			{GbDeviceID: "34020000001320000001", Direction: "out", Method: "MESSAGE", Result: "success", CreatedDT: old2},
			{GbDeviceID: "34020000001320000001", Direction: "in", Method: "Keepalive", Result: "success", CreatedDT: fresh},
		}
		for _, r := range rows {
			if err := tx.Create(r).Error; err != nil {
				t.Fatalf("插入信令日志失败: %v", err)
			}
		}

		svc := gbsvc.NewGbSipLogService()
		deleted, err := svc.Cleanup(ctx, 30)
		if err != nil {
			t.Fatalf("清理失败: %v", err)
		}
		if deleted < 2 {
			t.Fatalf("至少应删除 2 行过期日志，实际 %d", deleted)
		}

		var remain int64
		if err := tx.Model(&model.GbSipLog{}).Where("id IN ?", []int64{rows[0].ID, rows[1].ID}).Count(&remain).Error; err != nil {
			t.Fatalf("统计过期行失败: %v", err)
		}
		if remain != 0 {
			t.Errorf("过期行应被物理删除，仍剩 %d 行", remain)
		}

		var freshCount int64
		if err := tx.Model(&model.GbSipLog{}).Where("id = ?", rows[2].ID).Count(&freshCount).Error; err != nil {
			t.Fatalf("统计新行失败: %v", err)
		}
		if freshCount != 1 {
			t.Errorf("未过期行应保留，实际 %d 行", freshCount)
		}
	})
}

// TestSipLogCleanup_InvalidRetentionFallsBack 保留天数 <=0 时按默认 30 天处理，
// 不能因为配置写成 0 就删光整张表。
func TestSipLogCleanup_InvalidRetentionFallsBack(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		now := time.Now()
		r := &model.GbSipLog{
			GbDeviceID: "34020000001320000001",
			Direction:  "in",
			Method:     "REGISTER",
			Result:     "success",
			CreatedDT:  now.AddDate(0, 0, -10), // 10 天前，默认保留期内
		}
		if err := tx.Create(r).Error; err != nil {
			t.Fatalf("插入信令日志失败: %v", err)
		}

		svc := gbsvc.NewGbSipLogService()
		if _, err := svc.Cleanup(ctx, 0); err != nil {
			t.Fatalf("清理失败: %v", err)
		}

		var n int64
		if err := tx.Model(&model.GbSipLog{}).Where("id = ?", r.ID).Count(&n).Error; err != nil {
			t.Fatalf("统计失败: %v", err)
		}
		if n != 1 {
			t.Errorf("retentionDays=0 应回退到 30 天，10 天前的行不应被删，实际剩 %d 行", n)
		}
	})
}
