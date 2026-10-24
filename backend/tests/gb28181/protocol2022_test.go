package gb28181_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/version"
)

// 本文件是 GB/T 28181-2022 协议扩展的跨包验收用例：
// version 能力判定 + manscdp 编解码 + GB2312 严格性 + Catalog Event 语义。
//
// 不需要数据库夹具：全部是纯协议层用例，`go test ./tests/gb28181 -run 2022`
// 即可运行（包的 bootstrap init 需要本机 PostgreSQL 可达，这是本仓库测试的既有前提）。

func gb2022F64(v float64) *float64 { return &v }

// 2022 能力判定：只有 2022 版本才允许下发新命令。
func TestGB2022VersionCapability(t *testing.T) {
	if version.Normalize("GBT28181-2022") != version.V2022 {
		t.Fatalf("版本归一失败")
	}
	if version.Supports(version.V2016, version.FeaturePTZPosition) {
		t.Fatalf("2016 不应支持 PTZPosition")
	}
	if !version.Supports(version.V2022, version.FeaturePTZPosition) {
		t.Fatalf("2022 应支持 PTZPosition")
	}
	if version.UpgradeOnCommand(version.V2016, manscdp.CmdCruiseTrack) != version.V2022 {
		t.Fatalf("收到 2022 独有命令应升级版本")
	}
	if version.UpgradeOnCommand(version.V2016, manscdp.CmdCatalog) != version.V2016 {
		t.Fatalf("2016 命令不应升级版本")
	}
}

// 巡航轨迹列表：GB2312 声明与内容一致，decode 后中文正确。
func TestGB2022CruiseTrackListGB2312(t *testing.T) {
	in := &manscdp.CruiseTrackListResponse{
		CmdType:  manscdp.CmdCruiseTrackList,
		SN:       9001,
		DeviceID: "34020000001320000001",
		SumNum:   1,
		CruiseTrackList: &manscdp.CruiseTrackList{
			Num: 1,
			Tracks: []manscdp.CruiseTrack{
				{Number: 0, Name: "厂区巡航"},
			},
		},
	}
	body, err := manscdp.Marshal(in, charset.GB2312)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	if !bytes.HasPrefix(body, []byte(`<?xml version="1.0" encoding="GB2312"?>`)) {
		t.Fatalf("声明不是 GB2312")
	}
	if bytes.Contains(body, []byte("厂区巡航")) {
		t.Fatalf("内容未转成 GB2312")
	}

	_, out, err := manscdp.DecodeCruiseTrackList(body, charset.GB2312)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if out.CruiseTrackList == nil || out.CruiseTrackList.Tracks[0].Name != "厂区巡航" {
		t.Fatalf("中文往返失败：%+v", out.CruiseTrackList)
	}
}

// PTZ 精准位置应答：Pan=0 与缺失字段必须区分。
func TestGB2022PTZPositionResponse(t *testing.T) {
	in := &manscdp.PTZPositionResponse{
		CmdType:  manscdp.CmdPTZPosition,
		SN:       9002,
		DeviceID: "34020000001320000001",
		PTZPosition: manscdp.PTZPosition{
			Pan:  gb2022F64(0),
			Tilt: gb2022F64(45.5),
		},
	}
	body, err := manscdp.Marshal(in, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	_, out, err := manscdp.DecodePTZPosition(body, charset.UTF8)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if out.Pan == nil || *out.Pan != 0 {
		t.Fatalf("Pan=0 丢失：%v", out.Pan)
	}
	if out.Zoom != nil {
		t.Fatalf("未上报的 Zoom 应为 nil：%v", out.Zoom)
	}
}

// PTZ 精准控制：CmdType 仍为 DeviceControl，2022 参数在子元素中。
func TestGB2022PTZPreciseControl(t *testing.T) {
	ctrl := manscdp.NewPTZPreciseControl("34020000001320000001", 9003, 10, 20, 3)
	body, err := manscdp.Marshal(ctrl, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	s := string(body)
	if !strings.Contains(s, "<CmdType>DeviceControl</CmdType>") ||
		!strings.Contains(s, "<PTZPreciseCtrl>") ||
		!strings.Contains(s, "<Pan>10</Pan>") {
		t.Fatalf("报文不正确：%s", s)
	}
}

// Catalog 事件：解析 + 映射到通道状态。
func TestGB2022CatalogEvent(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?><Notify><CmdType>Catalog</CmdType><SN>9004</SN><DeviceID>34020000001320000001</DeviceID><DeviceList Num="2"><Item><DeviceID>34020000001320000001</DeviceID><Name>东门球机</Name><Event>ON</Event><Status>ON</Status></Item><Item><DeviceID>34020000001320000002</DeviceID><Name>仓库枪机</Name><Event>DEL</Event><Status>OFF</Status></Item></DeviceList></Notify>`
	_, out, err := manscdp.DecodeCatalogNotify([]byte(xmlBody), charset.UTF8)
	if err != nil {
		t.Fatalf("DecodeCatalog 失败：%v", err)
	}
	if out.DeviceList.Items[0].Event != manscdp.CatalogEventOn {
		t.Fatalf("Event 未解析：%+v", out.DeviceList.Items[0])
	}
	if eff := manscdp.ParseCatalogEvent(out.DeviceList.Items[1].Event); !eff.Known || !eff.Delete {
		t.Fatalf("DEL 应映射为删除：%+v", eff)
	}
}
