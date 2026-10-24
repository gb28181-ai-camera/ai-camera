package gb28181_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/device"
	"github.com/boticz/ai-camera-backend/internal/gb28181/diagnostics"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 设备诊断：ok / warn / failed / skipped 四条分支
//
// 全部依赖通过 Options 注入，测试不会真的发 SIP 报文或请求 ZLM。
// 这样既能稳定覆盖分支，也不会在 CI 里产生外部网络依赖。
// ════════════════════════════════════════════════════════════════

func boolPtr(v bool) *bool { return &v }

// newDiagDevice 建一台「已注册、在线、心跳新鲜」的国标设备。
func newDiagDevice(t *testing.T, tx *gorm.DB, code string) *model.Device {
	t.Helper()
	now := time.Now()
	expires := now.Add(time.Hour)
	dev := &model.Device{
		Name:           "诊断测试设备",
		DeviceCode:     code,
		GbDeviceID:     "34020000001320000001",
		RegisterStatus: model.GbRegisterStatusActivated,
		Status:         model.DeviceStatusOnline,
		InfoInterval:   60,
		RegisterDT:     &now,
		LastBeatDT:     &now,
		SipExpires:     &expires,
	}
	if err := tx.Create(dev).Error; err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
	return dev
}

func itemByName(t *testing.T, rep *diagnostics.Report, name string) diagnostics.Item {
	t.Helper()
	for _, it := range rep.Items {
		if it.Name == name {
			return it
		}
	}
	t.Fatalf("诊断结果缺少检查项 %q", name)
	return diagnostics.Item{}
}

// TestDiagnose_SkippedWithoutSIPRuntime 没有内嵌 SIP 服务时，端点与主动探测应为 skipped，
// 而不是 failed —— 否则独立 API 进程的每次诊断都会误报。
func TestDiagnose_SkippedWithoutSIPRuntime(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := newDiagDevice(t, tx, "DIAG_SKIP_001")

		svc := diagnostics.NewServiceWithOptions(diagnostics.Options{
			DB:            tx,
			MediaPublicIP: "127.0.0.1",
			// Endpoint / QueryDeviceInfo 显式留空：测试进程没有 SIP 服务，
			// runtimeHooks() 取不到 control.Default()，两者保持 nil。
			ProbeZLM: func(ctx context.Context, server *model.GbMediaServer) (bool, string) {
				return true, "注入的探测成功"
			},
		})

		rep, err := svc.Diagnose(ctx, dev.ID)
		if err != nil {
			t.Fatalf("诊断失败: %v", err)
		}

		if it := itemByName(t, rep, "SIP 信令地址"); it.Status != diagnostics.StatusSkipped {
			t.Errorf("无 SIP 运行时时端点应为 skipped，实际 %s（%s）", it.Status, it.Detail)
		}
		if it := itemByName(t, rep, "设备可达性探测"); it.Status != diagnostics.StatusSkipped {
			t.Errorf("无 SIP 运行时时探测应为 skipped，实际 %s（%s）", it.Status, it.Detail)
		}
		// 回环地址必须判失败，并给出「真机不可达」的中文提示。
		sdp := itemByName(t, rep, "SDP 通告地址")
		if sdp.Status != diagnostics.StatusFailed || !strings.Contains(sdp.Detail, "回环") {
			t.Errorf("回环 SDP 地址应 failed 且提示回环，实际 %s（%s）", sdp.Status, sdp.Detail)
		}
		if !strings.Contains(rep.Summary, "失败") {
			t.Errorf("summary 应包含失败计数，实际 %q", rep.Summary)
		}
	})
}

// TestDiagnose_AllOK 注入正常依赖时所有项通过，summary 为「全部检查通过」。
func TestDiagnose_AllOK(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := newDiagDevice(t, tx, "DIAG_OK_001")

		// 种子数据里有一条默认节点（online=false）。若不清掉它的 default 标记，
		// 「选默认节点」可能选中种子行，导致本用例断言到错误的节点。
		if err := tx.Model(&model.GbMediaServer{}).
			Where("default_server = ?", true).
			Update("default_server", false).Error; err != nil {
			t.Fatalf("清理默认节点标记失败: %v", err)
		}

		now := time.Now()
		ms := &model.GbMediaServer{
			Name:            "zlm-test",
			ServerID:        "zlm-test-01",
			Active:          boolPtr(true),
			DefaultServer:   boolPtr(true),
			Online:          boolPtr(true),
			LastKeepaliveDT: &now,
			IP:              "10.0.0.10",
			HTTPPort:        80,
		}
		if err := tx.Create(ms).Error; err != nil {
			t.Fatalf("创建媒体节点失败: %v", err)
		}

		svc := diagnostics.NewServiceWithOptions(diagnostics.Options{
			DB:            tx,
			MediaPublicIP: "203.0.113.10", // TEST-NET-3，公网语义
			Endpoint: func(gbID string) (device.Endpoint, bool) {
				return device.Endpoint{Addr: "192.168.1.50:5060", Transport: "UDP", UpdatedAt: now}, true
			},
			QueryDeviceInfo: func(ctx context.Context, d *model.Device) error { return nil },
			ProbeZLM: func(ctx context.Context, server *model.GbMediaServer) (bool, string) {
				return true, "HTTP API 可达（1ms）"
			},
		})

		rep, err := svc.Diagnose(ctx, dev.ID)
		if err != nil {
			t.Fatalf("诊断失败: %v", err)
		}
		for _, it := range rep.Items {
			if it.Status == diagnostics.StatusFailed || it.Status == diagnostics.StatusWarning {
				t.Errorf("期望全部通过，但 %q 为 %s（%s）", it.Name, it.Status, it.Detail)
			}
		}
		if !strings.Contains(rep.Summary, "全部检查通过") {
			t.Errorf("summary 应表示全部通过，实际 %q", rep.Summary)
		}
	})
}

// TestDiagnose_UnknownEndpointFailed 设备有国标编码但内存里没有源地址 → failed；
// 探测无法进行 → skipped（两者语义不同，不能混为一谈）。
func TestDiagnose_UnknownEndpointFailed(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := newDiagDevice(t, tx, "DIAG_EP_001")

		svc := diagnostics.NewServiceWithOptions(diagnostics.Options{
			DB:            tx,
			MediaPublicIP: "203.0.113.10",
			Endpoint: func(gbID string) (device.Endpoint, bool) {
				return device.Endpoint{}, false
			},
			ProbeZLM: func(ctx context.Context, server *model.GbMediaServer) (bool, string) {
				return false, "未启动"
			},
		})

		rep, err := svc.Diagnose(ctx, dev.ID)
		if err != nil {
			t.Fatalf("诊断失败: %v", err)
		}
		if it := itemByName(t, rep, "SIP 信令地址"); it.Status != diagnostics.StatusFailed {
			t.Errorf("未知端点应为 failed，实际 %s（%s）", it.Status, it.Detail)
		}
		if it := itemByName(t, rep, "设备可达性探测"); it.Status != diagnostics.StatusSkipped {
			t.Errorf("端点未知时探测应为 skipped，实际 %s（%s）", it.Status, it.Detail)
		}
	})
}

// TestDiagnose_PrivateSDPWarn 私网 SDP 地址是「可工作但有风险」，应为 warn 而非 failed。
func TestDiagnose_PrivateSDPWarn(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := newDiagDevice(t, tx, "DIAG_SDP_001")

		svc := diagnostics.NewServiceWithOptions(diagnostics.Options{
			DB:            tx,
			MediaPublicIP: "192.168.1.10",
			Endpoint: func(gbID string) (device.Endpoint, bool) {
				return device.Endpoint{Addr: "192.168.1.50:5060", Transport: "UDP", UpdatedAt: time.Now()}, true
			},
			QueryDeviceInfo: func(ctx context.Context, d *model.Device) error { return nil },
			ProbeZLM: func(ctx context.Context, server *model.GbMediaServer) (bool, string) {
				return true, "OK"
			},
		})

		rep, err := svc.Diagnose(ctx, dev.ID)
		if err != nil {
			t.Fatalf("诊断失败: %v", err)
		}
		sdp := itemByName(t, rep, "SDP 通告地址")
		if sdp.Status != diagnostics.StatusWarning || !sdp.OK {
			t.Errorf("私网地址应为 warn 且 ok=true，实际 %s ok=%v（%s）", sdp.Status, sdp.OK, sdp.Detail)
		}
	})
}

// TestDiagnose_ProbeAndZLMFailed 设备不响应 + ZLM 不可达都应判 failed，
// 并让 summary 明确提示「存在 N 项失败」。
func TestDiagnose_ProbeAndZLMFailed(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := newDiagDevice(t, tx, "DIAG_FAIL_001")

		online := true
		ms := &model.GbMediaServer{
			Name:          "zlm-down",
			ServerID:      "zlm-down-01",
			Active:        boolPtr(true),
			DefaultServer: boolPtr(true),
			Online:        &online,
			IP:            "10.0.0.11",
			HTTPPort:      80,
		}
		if err := tx.Create(ms).Error; err != nil {
			t.Fatalf("创建媒体节点失败: %v", err)
		}

		svc := diagnostics.NewServiceWithOptions(diagnostics.Options{
			DB:            tx,
			MediaPublicIP: "203.0.113.10",
			Endpoint: func(gbID string) (device.Endpoint, bool) {
				return device.Endpoint{Addr: "192.168.1.50:5060", Transport: "UDP", UpdatedAt: time.Now()}, true
			},
			QueryDeviceInfo: func(ctx context.Context, d *model.Device) error {
				return errors.New("等待响应超时")
			},
			ProbeZLM: func(ctx context.Context, server *model.GbMediaServer) (bool, string) {
				return false, "HTTP 请求失败：connection refused"
			},
		})

		rep, err := svc.Diagnose(ctx, dev.ID)
		if err != nil {
			t.Fatalf("诊断失败: %v", err)
		}
		if it := itemByName(t, rep, "设备可达性探测"); it.Status != diagnostics.StatusFailed {
			t.Errorf("设备不响应应为 failed，实际 %s（%s）", it.Status, it.Detail)
		}
		if it := itemByName(t, rep, "ZLM 节点可达性"); it.Status != diagnostics.StatusFailed {
			t.Errorf("ZLM 不可达应为 failed，实际 %s（%s）", it.Status, it.Detail)
		}
		if !strings.Contains(rep.Summary, "存在 2 项失败") {
			t.Errorf("summary 应统计 2 项失败，实际 %q", rep.Summary)
		}
	})
}

// TestDiagnose_DeviceNotFound 不存在的设备返回 ErrDeviceNotFound，由 handler 映射 404。
func TestDiagnose_DeviceNotFound(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := diagnostics.NewServiceWithOptions(diagnostics.Options{DB: tx})
		_, err := svc.Diagnose(ctx, 999999999)
		if !errors.Is(err, diagnostics.ErrDeviceNotFound) {
			t.Fatalf("期望 ErrDeviceNotFound，实际 %v", err)
		}
	})
}
