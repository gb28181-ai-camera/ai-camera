package gb28181_test

// ════════════════════════════════════════════════════════════════
// 运维增强：主机指标采集与进程内 ring buffer
//
// 运行前清空测试库并指向 ai_camera_test：
//   cd backend && go run ./tests/testutil/resetdb/main.go
//   set -a; source ../.env; set +a
//   export TEST_DB_HOST=localhost TEST_DB_PORT=5432 TEST_DB_USER=aicamera \
//          TEST_DB_PASSWORD="$POSTGRES_PASSWORD" TEST_DB_NAME=ai_camera_test
//   go test ./tests/gb28181/... -run 'TestOps' -count=1 -p 1 -timeout 300s
// ════════════════════════════════════════════════════════════════

import (
	"context"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// 关闭后台采样器：避免 goroutine 在事务回滚后继续访问测试库。
func init() { gbsvc.SetMetricsSamplerEnabled(false) }

func TestOpsMetricsRingBuffer(t *testing.T) {
	gbsvc.ClearMetricsBuffers()
	svc := gbsvc.NewGbMediaMetricsService()
	now := time.Now()

	// 1) 写入一个 25 小时前的点，再写一个当前点：旧点应被保留期淘汰。
	svc.RecordSample("ops-ring", response.GbMediaMetricPoint{
		TS: now.Add(-25 * time.Hour), CPUPercent: 99, StreamCount: 9, Online: true,
	})
	svc.RecordSample("ops-ring", response.GbMediaMetricPoint{
		TS: now, CPUPercent: 12.5, MemPercent: 30, StreamCount: 3, Online: true,
	})

	all := svc.HistoryPoints("ops-ring", now.Add(-26*time.Hour))
	if len(all) != 1 {
		t.Fatalf("超过 24h 的采样点应被淘汰，实际 %d 个点: %+v", len(all), all)
	}
	if all[0].CPUPercent != 12.5 || all[0].StreamCount != 3 {
		t.Fatalf("保留的点不正确: %+v", all[0])
	}

	// 2) since 过滤：只返回范围之后的点。
	svc.RecordSample("ops-ring", response.GbMediaMetricPoint{TS: now.Add(-30 * time.Minute), CPUPercent: 5})
	if got := svc.HistoryPoints("ops-ring", now.Add(-time.Hour)); len(got) != 2 {
		t.Fatalf("1h 范围内应有 2 个点，实际 %d", len(got))
	}

	// 3) 不同节点互不干扰。
	if got := svc.HistoryPoints("other-node", now.Add(-time.Hour)); len(got) != 0 {
		t.Fatalf("另一节点不应有采样点，实际 %d", len(got))
	}
}

func TestOpsCollectHostMetrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	host, err := gbsvc.CollectHostMetrics(ctx)
	if err != nil {
		t.Fatalf("采集主机指标失败: %v", err)
	}
	if host.MemTotal == 0 {
		t.Fatalf("内存总量不应为 0: %+v", host)
	}
	if host.CPUCores <= 0 {
		t.Fatalf("CPU 核心数不应为 0: %+v", host)
	}
	if host.OSInfo == "" {
		t.Fatalf("OSInfo 不应为空")
	}
	if host.CPUPercent < 0 || host.CPUPercent > 100 {
		t.Fatalf("CPU 百分比超出范围: %v", host.CPUPercent)
	}
	if host.MemPercent < 0 || host.MemPercent > 100 {
		t.Fatalf("内存百分比超出范围: %v", host.MemPercent)
	}
}

func TestOpsMetricsHistoryEndpoint(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		gbsvc.ClearMetricsBuffers()
		ms := &model.GbMediaServer{
			Name:          "指标节点",
			ServerID:      "ops-metrics-node",
			IP:            "127.0.0.1",
			HTTPPort:      8082,
			DefaultServer: pbool(true),
		}
		if err := tx.Create(ms).Error; err != nil {
			t.Fatalf("创建媒体节点失败: %v", err)
		}

		svc := gbsvc.NewGbMediaMetricsService()
		resp, err := svc.History(ctx, ms.ID, "1h")
		if err != nil {
			t.Fatalf("查询指标历史失败: %v", err)
		}
		if resp.MediaServerID != ms.ServerID || resp.Range != "1h" {
			t.Fatalf("历史响应元信息不正确: %+v", resp)
		}
		if resp.IntervalSec != 5 {
			t.Fatalf("采样间隔应为 5 秒，实际 %d", resp.IntervalSec)
		}
		if len(resp.Items) == 0 {
			t.Fatalf("没有历史时应补一个当前采样点")
		}
		if _, err := svc.History(ctx, ms.ID, "7d"); err == nil {
			t.Fatalf("非法 range 应返回错误")
		}
		if _, err := svc.History(ctx, 99999999, "1h"); err != gbsvc.ErrGbMediaServerNotFound {
			t.Fatalf("不存在的节点应返回 ErrGbMediaServerNotFound，实际 %v", err)
		}
	})
}
