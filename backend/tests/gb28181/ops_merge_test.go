package gb28181_test

// ════════════════════════════════════════════════════════════════
// 运维增强：云端录像多段合并任务
//
// 覆盖：创建任务、无 ffmpeg 的失败分支、非法输入、分页/详情/下载前提。
// 测试通过注入 ffmpeg 查找函数强制「未安装」分支，避免依赖 CI 环境。
// ════════════════════════════════════════════════════════════════

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

func TestOpsMergeTaskNoFFmpeg(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		// 两段云端录像（不要求文件存在：无 ffmpeg 时在探测阶段即失败）。
		r1 := createCloudRecord(t, tx, "cam-1", "part1.mp4", "2026-10-25T08:00:00")
		r2 := createCloudRecord(t, tx, "cam-1", "part2.mp4", "2026-10-25T09:00:00")

		svc := gbsvc.NewGbCloudMergeServiceWithFFmpegLookup(func() (string, error) {
			return "", errors.New("exec: \"ffmpeg\": executable file not found in $PATH")
		})

		task, err := svc.Create(ctx, &request.GbMergeTaskCreateReq{
			IDs:  []int64{r2.ID, r1.ID}, // 顺序故意颠倒：服务端按开始时间排序
			Name: "合并测试",
		})
		if err != nil {
			t.Fatalf("创建合并任务失败: %v", err)
		}
		if task.Status != model.GbMergeStatusFailed {
			t.Fatalf("缺少 ffmpeg 时任务应为 failed，实际 %s", task.Status)
		}
		if !strings.Contains(task.ErrorMsg, "ffmpeg") {
			t.Fatalf("失败原因应明确提示 ffmpeg，实际 %q", task.ErrorMsg)
		}
		if task.FinishedDT == nil {
			t.Fatalf("失败任务应记录结束时间")
		}
		if task.RecordCount != 2 || task.DurationSec != 120 {
			t.Fatalf("任务统计不正确: count=%d duration=%d", task.RecordCount, task.DurationSec)
		}
		if !strings.HasPrefix(task.RecordIDs, itoa64(r1.ID)+",") {
			t.Fatalf("录像应按开始时间升序保存，实际 %q", task.RecordIDs)
		}

		// 详情与分页。
		got, err := svc.Get(ctx, task.ID)
		if err != nil {
			t.Fatalf("查询任务详情失败: %v", err)
		}
		if got.Status != model.GbMergeStatusFailed || got.Name != "合并测试" {
			t.Fatalf("任务详情不正确: %+v", got)
		}
		page, err := svc.Page(ctx, &request.PageGbMergeTaskReq{Status: model.GbMergeStatusFailed}, allScope())
		if err != nil {
			t.Fatalf("分页查询任务失败: %v", err)
		}
		if page.Total != 1 {
			t.Fatalf("应查到 1 个失败任务，实际 %d", page.Total)
		}

		// 未完成的任务不允许下载。
		if _, _, err := svc.DownloadPath(ctx, task.ID); !errors.Is(err, gbsvc.ErrGbMergeTaskNotReady) {
			t.Fatalf("未完成任务下载应返回 ErrGbMergeTaskNotReady，实际 %v", err)
		}
	})
}

func TestOpsMergeTaskValidation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		r1 := createCloudRecord(t, tx, "cam-2", "only.mp4", "2026-10-25T08:00:00")
		svc := gbsvc.NewGbCloudMergeServiceWithFFmpegLookup(func() (string, error) {
			return "", errors.New("missing")
		})

		if _, err := svc.Create(ctx, &request.GbMergeTaskCreateReq{IDs: []int64{r1.ID}}); err == nil {
			t.Fatalf("少于两段录像应被拒绝")
		}
		if _, err := svc.Create(ctx, &request.GbMergeTaskCreateReq{IDs: []int64{r1.ID, 99999999}}); !errors.Is(err, gbsvc.ErrGbMergeRecordNotFound) {
			t.Fatalf("不存在的录像应返回 ErrGbMergeRecordNotFound，实际 %v", err)
		}
		if _, err := svc.Get(ctx, 99999999); !errors.Is(err, gbsvc.ErrGbMergeTaskNotFound) {
			t.Fatalf("不存在的任务应返回 ErrGbMergeTaskNotFound，实际 %v", err)
		}
	})
}

// createCloudRecord 写入一条云端录像索引。
func createCloudRecord(t *testing.T, tx *gorm.DB, stream, fileName, start string) *model.GbCloudRecord {
	t.Helper()
	startDT, err := time.ParseInLocation("2006-01-02T15:04:05", start, time.Local)
	if err != nil {
		t.Fatalf("解析录像开始时间失败: %v", err)
	}
	endDT := startDT.Add(60 * time.Second)
	r := &model.GbCloudRecord{
		App:       "record",
		Stream:    stream,
		FileName:  fileName,
		Folder:    "/data/record",
		FilePath:  "/data/record/" + fileName,
		StartTime: &startDT,
		EndTime:   &endDT,
		TimeLen:   60,
		FileSize:  1024,
	}
	if err := tx.Create(r).Error; err != nil {
		t.Fatalf("创建云端录像失败: %v", err)
	}
	return r
}

// itoa64 无依赖的 int64 转字符串（测试断言用）。
func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
