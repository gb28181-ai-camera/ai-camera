package gb28181_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/version"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
)

// ════════════════════════════════════════════════════════════════
// GB/T 28181-2022 云台扩展的服务层约束
//
// 这些用例不依赖 SIP 与数据库，只覆盖两条最容易回归的规则：
//   1. 2016 设备必须在**发报文之前**被拦截（否则用户只会看到 5 秒超时）；
//   2. 精准控制必须至少带一个轴，空请求不能返回成功。
// ════════════════════════════════════════════════════════════════

func f64(v float64) *float64 { return &v }

// TestEnsurePTZ2022VersionGate 版本能力判定。
//
// 2016 设备收到 PTZPosition / CruiseTrackQuery 等未知 CmdType 时会静默丢弃，
// 平台只能等到 5 秒超时。提前拦截后错误立即返回且语义明确。
func TestEnsurePTZ2022VersionGate(t *testing.T) {
	features := []version.Feature{
		version.FeaturePTZPosition,
		version.FeaturePTZPreciseCtrl,
		version.FeatureCruiseTrackList,
		version.FeatureCruiseTrack,
		version.FeatureHomePosition,
		version.FeatureSDCardStatus,
	}

	for _, feature := range features {
		t.Run(string(feature), func(t *testing.T) {
			old := &model.Device{GbDeviceID: "34020000001320000001", GbVersion: model.GbVersion2016}
			err := gbsvc.EnsurePTZ2022(old, feature)
			if !errors.Is(err, gbsvc.ErrGb2022Unsupported) {
				t.Fatalf("2016 设备应返回版本不支持，实际：%v", err)
			}
			if !strings.Contains(err.Error(), "GB28181-2016") {
				t.Fatalf("错误信息应明确写出 2016，实际：%v", err)
			}

			newer := &model.Device{GbDeviceID: "34020000001320000001", GbVersion: model.GbVersion2022}
			if err := gbsvc.EnsurePTZ2022(newer, feature); err != nil {
				t.Fatalf("2022 设备不应被拦截：%v", err)
			}
		})
	}
}

// TestEnsurePTZ2022DefaultsTo2016 空版本按 2016 处理（安全默认值）。
//
// 宁可拒绝，也不要把 2022 独有 CmdType 发给一台版本未知的设备。
func TestEnsurePTZ2022DefaultsTo2016(t *testing.T) {
	dev := &model.Device{GbDeviceID: "34020000001320000002"}
	err := gbsvc.EnsurePTZ2022(dev, version.FeaturePTZPosition)
	if !errors.Is(err, gbsvc.ErrGb2022Unsupported) {
		t.Fatalf("空版本应按 2016 拦截，实际：%v", err)
	}
	if !errors.Is(gbsvc.EnsurePTZ2022(nil, version.FeaturePTZPosition), gbsvc.ErrGbDeviceNotFound) {
		t.Fatalf("nil 设备应返回设备不存在")
	}
}

// TestValidatePTZPreciseReq 精准控制参数校验。
func TestValidatePTZPreciseReq(t *testing.T) {
	cases := []struct {
		name string
		req  *request.GbPtzPreciseReq
		want error
	}{
		{name: "nil", req: nil, want: gbsvc.ErrGbPtzPreciseEmpty},
		{name: "全空", req: &request.GbPtzPreciseReq{}, want: gbsvc.ErrGbPtzPreciseEmpty},
		{name: "仅 pan", req: &request.GbPtzPreciseReq{Pan: f64(0)}, want: nil},
		{name: "仅 tilt", req: &request.GbPtzPreciseReq{Tilt: f64(0)}, want: nil},
		{name: "仅 zoom", req: &request.GbPtzPreciseReq{Zoom: f64(1)}, want: nil},
		{
			name: "NaN",
			req:  &request.GbPtzPreciseReq{Pan: f64(math.NaN())},
			want: gbsvc.ErrGbPtzPreciseInvalid,
		},
		{
			name: "Inf",
			req:  &request.GbPtzPreciseReq{Zoom: f64(math.Inf(1))},
			want: gbsvc.ErrGbPtzPreciseInvalid,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := gbsvc.ValidatePTZPreciseReq(tc.req)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("期望通过，实际：%v", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("期望 %v，实际：%v", tc.want, err)
			}
		})
	}
}
