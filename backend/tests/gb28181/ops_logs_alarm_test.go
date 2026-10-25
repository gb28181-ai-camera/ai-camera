package gb28181_test

// ════════════════════════════════════════════════════════════════
// 运维增强：内存日志 ring buffer / SSE 事件格式 / 报警增量轮询
// ════════════════════════════════════════════════════════════════

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/config"
	web "github.com/boticz/ai-camera-backend/internal/api/handler/web"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestOpsLogRingSearchAndPaging(t *testing.T) {
	utils.InitLogger(&config.LogConfig{Level: "debug", Format: "console", Output: "console"})
	defer utils.InitLogger(&config.LogConfig{Level: "fatal", Format: "console", Output: "console"})
	utils.ResetLogRing()

	utils.Info("hello-ops-log", zap.String("module", "gb-ops"))
	utils.Warn("danger-ops-log", zap.Int("code", 42))

	svc := gbsvc.NewGbLogStreamService()

	// 级别过滤：warn 只命中 warn 及以上。
	page, err := svc.Page(&request.GbLogQueryReq{Level: "warn"})
	if err != nil {
		t.Fatalf("检索日志失败: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Message != "danger-ops-log" {
		t.Fatalf("级别过滤不正确: %+v", page)
	}
	if page.Items[0].Fields["code"] != float64(42) {
		t.Fatalf("结构化字段应被保留: %+v", page.Items[0].Fields)
	}

	// 关键字命中消息。
	page, err = svc.Page(&request.GbLogQueryReq{Keyword: "hello-ops"})
	if err != nil || page.Total != 1 || page.Items[0].Message != "hello-ops-log" {
		t.Fatalf("关键字检索不正确: %+v err=%v", page, err)
	}

	// 关键字命中结构化字段。
	page, err = svc.Page(&request.GbLogQueryReq{Keyword: "gb-ops"})
	if err != nil || page.Total != 1 {
		t.Fatalf("字段关键字检索不正确: %+v err=%v", page, err)
	}

	// 最新在前 + 分页。
	page, err = svc.Page(&request.GbLogQueryReq{PageReq: request.PageReq{Page: 1, PageSize: 1}})
	if err != nil || page.Total != 2 || len(page.Items) != 1 {
		t.Fatalf("分页总数不正确: %+v err=%v", page, err)
	}
	if page.Items[0].Message != "danger-ops-log" {
		t.Fatalf("分页应最新在前，实际 %q", page.Items[0].Message)
	}
	page, err = svc.Page(&request.GbLogQueryReq{PageReq: request.PageReq{Page: 2, PageSize: 1}})
	if err != nil || len(page.Items) != 1 || page.Items[0].Message != "hello-ops-log" {
		t.Fatalf("第二页不正确: %+v err=%v", page, err)
	}
}

func TestOpsLogsStreamSSEFormat(t *testing.T) {
	utils.InitLogger(&config.LogConfig{Level: "debug", Format: "console", Output: "console"})
	defer utils.InitLogger(&config.LogConfig{Level: "fatal", Format: "console", Output: "console"})
	utils.ResetLogRing()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Request = httptest.NewRequest(http.MethodGet, "/web/gb/logs/stream", nil).WithContext(ctx)

	h := web.NewGbOpsExtraHandler()
	go func() {
		// 建连后写入一条日志，确保走实时通道而不是历史回放。
		time.Sleep(40 * time.Millisecond)
		utils.Info("sse-format-check", zap.String("stream", "cam-1"))
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		h.LogsStream(c)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("SSE 处理器未在客户端断开后退出")
	}

	body := w.Body.String()
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type 应为 text/event-stream，实际 %q", ct)
	}
	if !strings.Contains(body, "event:log") && !strings.Contains(body, "event: log") {
		t.Fatalf("SSE 应包含 event:log 帧，实际:\n%s", body)
	}
	if !strings.Contains(body, "data:") {
		t.Fatalf("SSE 应包含 data: 帧，实际:\n%s", body)
	}
	if !strings.Contains(body, "sse-format-check") || !strings.Contains(body, "\"stream\":\"cam-1\"") {
		t.Fatalf("SSE 数据应包含消息与结构化字段，实际:\n%s", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Fatalf("SSE 帧应以空行结束，实际尾部 %q", tail(body, 8))
	}
}

func TestOpsAlarmPollOnce(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := createDevice(t, ctx, "CAM_ALARM_001", "报警设备", "34020000001320007001")
		ch := createChannel(t, ctx, dev, "34020000001320007001", "报警通道", "")

		alarmTime := time.Now()
		alarm := &model.GbAlarm{
			DeviceID:         &dev.ID,
			ChannelID:        &ch.ID,
			GbDeviceID:       dev.GbDeviceID,
			GbChannelID:      ch.GbChannelID,
			AlarmPriority:    "1",
			AlarmMethod:      4,
			AlarmType:        "video-loss",
			AlarmTime:        &alarmTime,
			AlarmDescription: "视频丢失",
			Handled:          pbool(false),
		}
		if err := tx.Create(alarm).Error; err != nil {
			t.Fatalf("创建报警失败: %v", err)
		}

		svc := gbsvc.NewGbAlarmStreamServiceWithInterval(20 * time.Millisecond)
		items, cursor, err := svc.PollOnce(ctx, 0)
		if err != nil {
			t.Fatalf("轮询报警失败: %v", err)
		}
		if len(items) != 1 || cursor != alarm.ID {
			t.Fatalf("轮询结果不正确: items=%d cursor=%d", len(items), cursor)
		}
		if items[0].DeviceName != "报警设备" || items[0].ChannelName != "报警通道" {
			t.Fatalf("应返回关联设备/通道名称: %+v", items[0])
		}
		if items[0].AlarmType != "video-loss" || items[0].AlarmDescription != "视频丢失" {
			t.Fatalf("报警字段不正确: %+v", items[0])
		}

		// 游标之后没有新报警。
		items, cursor2, err := svc.PollOnce(ctx, cursor)
		if err != nil {
			t.Fatalf("二次轮询失败: %v", err)
		}
		if len(items) != 0 || cursor2 != cursor {
			t.Fatalf("游标之后不应有新报警: items=%d cursor=%d", len(items), cursor2)
		}

		if latest := svc.LatestID(ctx); latest != alarm.ID {
			t.Fatalf("LatestID 应为 %d，实际 %d", alarm.ID, latest)
		}
	})
}

// tail 返回字符串末尾 n 个字符（安全截断）。
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
