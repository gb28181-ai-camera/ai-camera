package gb28181_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
)

// ════════════════════════════════════════════════════════════════
// 设备控制 / 巡航 / 扫描报文的字节与 XML 构造
//
// 这些用例不依赖数据库与 SIP，是「按了没反应」类回归的第一道防线。
// ════════════════════════════════════════════════════════════════

func assertPTZBytes(t *testing.T, got []byte, code, d1, d2, d3 byte) {
	t.Helper()
	want := []byte{0xA5, 0x0F, 0x01, code, d1, d2, d3 & 0xF0, 0}
	var sum byte
	for _, b := range want[:7] {
		sum += b
	}
	want[7] = sum
	if !bytes.Equal(got, want) {
		t.Fatalf("PTZ 字节不正确\n got=% X\nwant=% X", got, want)
	}
}

func TestCoreBuildCruiseAndScanCommands(t *testing.T) {
	// 加入巡航点：组 1，预置位 5。
	cmd, err := manscdp.BuildCruisePointCmd(1, 5)
	if err != nil {
		t.Fatalf("加入巡航点构造失败: %v", err)
	}
	assertPTZBytes(t, cmd, manscdp.PtzCmdCruisePointAdd, 1, 5, 0)

	// 删除巡航点：组 2，预置位 0 表示删除整条巡航。
	cmd, err = manscdp.BuildDeleteCruisePointCmd(2, 0)
	if err != nil {
		t.Fatalf("删除巡航点构造失败: %v", err)
	}
	assertPTZBytes(t, cmd, manscdp.PtzCmdCruisePointDelete, 2, 0, 0)

	// 巡航速度 300 = 0x12C：低 8 位 0x2C，高 4 位 1 放在索引 6 高半字节。
	cmd, err = manscdp.BuildCruiseSpeedCmd(1, 300)
	if err != nil {
		t.Fatalf("巡航速度构造失败: %v", err)
	}
	assertPTZBytes(t, cmd, manscdp.PtzCmdCruiseSpeed, 1, 0x2C, 0x10)

	// 停留时间 10 秒。
	cmd, err = manscdp.BuildCruiseStayTimeCmd(1, 10)
	if err != nil {
		t.Fatalf("停留时间构造失败: %v", err)
	}
	assertPTZBytes(t, cmd, manscdp.PtzCmdCruiseStayTime, 1, 10, 0)

	// 扫描左边界 / 右边界 / 速度。
	cmd, err = manscdp.BuildScanBoundCmd(3, true)
	if err != nil {
		t.Fatalf("左边界构造失败: %v", err)
	}
	assertPTZBytes(t, cmd, manscdp.PtzCmdScan, 3, manscdp.ScanActionLeftBound, 0)

	cmd, err = manscdp.BuildScanBoundCmd(3, false)
	if err != nil {
		t.Fatalf("右边界构造失败: %v", err)
	}
	assertPTZBytes(t, cmd, manscdp.PtzCmdScan, 3, manscdp.ScanActionRightBound, 0)

	cmd, err = manscdp.BuildScanSpeedCmd(3, 5)
	if err != nil {
		t.Fatalf("扫描速度构造失败: %v", err)
	}
	assertPTZBytes(t, cmd, manscdp.PtzCmdScanSpeed, 3, 5, 0)

	// 停止指令：指令码全零。
	stop := manscdp.BuildPTZStopCmd()
	assertPTZBytes(t, stop, 0, 0, 0, 0)
}

func TestCoreBuildCruiseCommandValidation(t *testing.T) {
	if _, err := manscdp.BuildCruisePointCmd(0, 1); err == nil {
		t.Fatalf("巡航组号 0 应被拒绝")
	}
	if _, err := manscdp.BuildCruisePointCmd(1, 0); err == nil {
		t.Fatalf("加入巡航点时预置位 0 应被拒绝")
	}
	if _, err := manscdp.BuildCruiseSpeedCmd(1, 4096); err == nil {
		t.Fatalf("速度 4096 超出 12 bit 应被拒绝")
	}
	if _, err := manscdp.BuildCruiseStayTimeCmd(1, 0); err == nil {
		t.Fatalf("停留时间 0 应被拒绝")
	}
	if _, err := manscdp.BuildScanBoundCmd(256, true); err == nil {
		t.Fatalf("扫描组号 256 应被拒绝")
	}
}

func TestCoreDeviceControlXML(t *testing.T) {
	cases := []struct {
		name  string
		ctrl  *manscdp.Control
		parts []string
	}{
		{
			name:  "重启",
			ctrl:  manscdp.NewDeviceControl("34020000001320000001", 10, manscdp.TeleBoot, "", ""),
			parts: []string{"<TeleBoot>Boot</TeleBoot>", "<CmdType>DeviceControl</CmdType>"},
		},
		{
			name:  "布防",
			ctrl:  manscdp.NewDeviceControl("34020000001320000001", 11, "", "", manscdp.GuardCmdSet),
			parts: []string{"<GuardCmd>SetGuard</GuardCmd>"},
		},
		{
			name:  "撤防",
			ctrl:  manscdp.NewDeviceControl("34020000001320000001", 12, "", "", manscdp.GuardCmdReset),
			parts: []string{"<GuardCmd>ResetGuard</GuardCmd>"},
		},
		{
			name:  "报警复位",
			ctrl:  manscdp.NewDeviceControl("34020000001320000002", 13, "", manscdp.AlarmCmdReset, ""),
			parts: []string{"<AlarmCmd>ResetAlarm</AlarmCmd>", "<DeviceID>34020000001320000002</DeviceID>"},
		},
		{
			name:  "开始录像",
			ctrl:  manscdp.NewRecordControl("34020000001320000003", 14, true),
			parts: []string{"<RecordCmd>Record</RecordCmd>"},
		},
		{
			name:  "停止录像",
			ctrl:  manscdp.NewRecordControl("34020000001320000003", 15, false),
			parts: []string{"<RecordCmd>StopRecord</RecordCmd>"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := manscdp.MarshalString(tc.ctrl, charset.UTF8)
			if err != nil {
				t.Fatalf("编码失败: %v", err)
			}
			for _, p := range tc.parts {
				if !strings.Contains(body, p) {
					t.Fatalf("报文缺少 %q:\n%s", p, body)
				}
			}
		})
	}
}
