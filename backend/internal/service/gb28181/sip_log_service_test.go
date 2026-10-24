package gb28181

import (
	"context"
	"errors"
	"testing"
)

// TestDeleteSipLogsInBatches_StopsWhenPartialBatch 一批不足 batchSize 即结束。
func TestDeleteSipLogsInBatches_StopsWhenPartialBatch(t *testing.T) {
	var calls int
	got, err := deleteSipLogsInBatches(context.Background(), 100, func(ctx context.Context, limit int) (int64, error) {
		calls++
		if limit != 100 {
			t.Fatalf("limit 应为 100，实际 %d", limit)
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if got != 42 || calls != 1 {
		t.Fatalf("期望删除 42 行、调用 1 次，实际 %d 行、%d 次", got, calls)
	}
}

// TestDeleteSipLogsInBatches_ExactMultipleDeletesUntilEmpty
// 恰好删满一批时必须继续，直到某一批返回 0 才停 —— 否则会漏掉最后一批。
func TestDeleteSipLogsInBatches_ExactMultipleDeletesUntilEmpty(t *testing.T) {
	remaining := int64(10)
	var calls int
	got, err := deleteSipLogsInBatches(context.Background(), 5, func(ctx context.Context, limit int) (int64, error) {
		calls++
		n := int64(limit)
		if remaining < n {
			n = remaining
		}
		remaining -= n
		return n, nil
	})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if got != 10 {
		t.Fatalf("期望删除 10 行，实际 %d", got)
	}
	// 5 + 5 + 0：第三批为 0 才能确认已清空。
	if calls != 3 {
		t.Fatalf("期望调用 3 次，实际 %d 次", calls)
	}
}

// TestDeleteSipLogsInBatches_PropagatesError 中途失败时返回已删除数量与错误。
func TestDeleteSipLogsInBatches_PropagatesError(t *testing.T) {
	sentinel := errors.New("db unavailable")
	calls := 0
	got, err := deleteSipLogsInBatches(context.Background(), 10, func(ctx context.Context, limit int) (int64, error) {
		calls++
		if calls == 2 {
			return 3, sentinel
		}
		return 10, nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("应返回原始错误，实际 %v", err)
	}
	if got != 13 {
		t.Fatalf("应返回已删除的 13 行，实际 %d", got)
	}
	if calls != 2 {
		t.Fatalf("失败后不应继续，实际调用 %d 次", calls)
	}
}

// TestDeleteSipLogsInBatches_ContextCanceled 取消后立即停止。
func TestDeleteSipLogsInBatches_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	got, err := deleteSipLogsInBatches(ctx, 10, func(ctx context.Context, limit int) (int64, error) {
		calls++
		return 10, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回 context.Canceled，实际 %v", err)
	}
	if got != 0 || calls != 0 {
		t.Fatalf("取消后不应执行删除，实际 %d 行、%d 次", got, calls)
	}
}

// TestDeleteSipLogsInBatches_DefaultBatchSize batchSize<=0 时回退到默认值。
func TestDeleteSipLogsInBatches_DefaultBatchSize(t *testing.T) {
	var gotLimit int
	_, err := deleteSipLogsInBatches(context.Background(), 0, func(ctx context.Context, limit int) (int64, error) {
		gotLimit = limit
		return 0, nil
	})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if gotLimit != sipLogCleanupBatchSize {
		t.Fatalf("期望回退到 %d，实际 %d", sipLogCleanupBatchSize, gotLimit)
	}
}
