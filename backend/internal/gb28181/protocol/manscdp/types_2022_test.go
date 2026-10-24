package manscdp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
)

// f64 构造可选的 double 字段。
func f64(v float64) *float64 { return &v }

// 2022 的各条查询命令：encode → decode 往返，CmdType/SN/DeviceID 与专有字段都必须保持。
func Test2022QueryRoundTrip(t *testing.T) {
	const dev = "34020000001320000001"

	cases := []struct {
		name  string
		in    any
		out   any
		check func(t *testing.T, env *Envelope, out any)
	}{
		{
			name: "PTZPosition",
			in:   NewPTZPositionQuery(dev, 101),
			out:  &PTZPositionQuery{},
			check: func(t *testing.T, env *Envelope, out any) {
				q := out.(*PTZPositionQuery)
				if env.Root() != "Query" || env.CmdType != CmdPTZPosition || q.DeviceID != dev {
					t.Fatalf("信封错误：%+v / %+v", env, q)
				}
			},
		},
		{
			name: "CruiseTrackListQuery",
			in:   NewCruiseTrackListQuery(dev, 102),
			out:  &CruiseTrackListQuery{},
			check: func(t *testing.T, env *Envelope, out any) {
				if env.CmdType != CmdCruiseTrackList || out.(*CruiseTrackListQuery).DeviceID != dev {
					t.Fatalf("解码错误：%+v / %+v", env, out)
				}
			},
		},
		{
			name: "CruiseTrackQuery",
			in:   NewCruiseTrackQuery(dev, 103, 2),
			out:  &CruiseTrackQuery{},
			check: func(t *testing.T, env *Envelope, out any) {
				q := out.(*CruiseTrackQuery)
				if env.CmdType != CmdCruiseTrack || q.Number != 2 {
					t.Fatalf("轨迹编号错误：%+v / %+v", env, q)
				}
			},
		},
		{
			name: "HomePositionQuery",
			in:   NewHomePositionQuery(dev, 104),
			out:  &HomePositionQuery{},
			check: func(t *testing.T, env *Envelope, out any) {
				if env.CmdType != CmdHomePosition || env.SN != 104 {
					t.Fatalf("解码错误：%+v", env)
				}
			},
		},
		{
			name: "SDCardStatusQuery",
			in:   NewSDCardStatusQuery(dev, 105),
			out:  &SDCardStatusQuery{},
			check: func(t *testing.T, env *Envelope, out any) {
				if env.CmdType != CmdSDCardStatus || out.(*SDCardStatusQuery).SN != 105 {
					t.Fatalf("解码错误：%+v / %+v", env, out)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body, err := Marshal(c.in, charset.UTF8)
			if err != nil {
				t.Fatalf("Marshal 失败：%v", err)
			}
			env, err := DecodeInto(body, charset.UTF8, c.out)
			if err != nil {
				t.Fatalf("DecodeInto 失败：%v", err)
			}
			c.check(t, env, c.out)
		})
	}
}

// PTZ 精准位置应答：0 是合法值，必须与「未上报」区分。
func Test2022PTZPositionResponseRoundTrip(t *testing.T) {
	in := &PTZPositionResponse{
		CmdType:  CmdPTZPosition,
		SN:       201,
		DeviceID: "34020000001320000001",
		PTZPosition: PTZPosition{
			Pan:                  f64(0), // 回正，不能因 omitempty 丢失
			Tilt:                 f64(-12.5),
			Zoom:                 f64(3.25),
			HorizontalFieldAngle: f64(60.5),
			VerticalFieldAngle:   f64(35),
			MaxViewDistance:      f64(120),
		},
	}
	body, err := Marshal(in, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	if !bytes.Contains(body, []byte("<Pan>0</Pan>")) {
		t.Fatalf("Pan=0 未编码：%s", body)
	}

	env, out, err := DecodePTZPosition(body, charset.UTF8)
	if err != nil {
		t.Fatalf("DecodePTZPosition 失败：%v", err)
	}
	if env.CmdType != CmdPTZPosition || out.SN != 201 {
		t.Fatalf("信封错误：%+v", env)
	}
	if out.Pan == nil || *out.Pan != 0 {
		t.Fatalf("Pan = %v，期望 0", out.Pan)
	}
	if out.Tilt == nil || *out.Tilt != -12.5 {
		t.Fatalf("Tilt = %v，期望 -12.5", out.Tilt)
	}
	if out.Zoom == nil || *out.Zoom != 3.25 {
		t.Fatalf("Zoom = %v，期望 3.25", out.Zoom)
	}
	if out.HorizontalFieldAngle == nil || *out.HorizontalFieldAngle != 60.5 {
		t.Fatalf("HorizontalFieldAngle = %v", out.HorizontalFieldAngle)
	}
	if out.MaxViewDistance == nil || *out.MaxViewDistance != 120 {
		t.Fatalf("MaxViewDistance = %v", out.MaxViewDistance)
	}
}

// 未上报的选项必须是 nil（而不是 0），否则业务会把「不知道」当成「正北」。
func Test2022PTZPositionOptionalAbsent(t *testing.T) {
	in := &PTZPositionResponse{CmdType: CmdPTZPosition, SN: 1, DeviceID: "1"}
	body, err := Marshal(in, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	_, out, err := DecodePTZPosition(body, charset.UTF8)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if out.Pan != nil || out.Tilt != nil || out.Zoom != nil || out.MaxViewDistance != nil {
		t.Fatalf("未上报的可选字段应为 nil：%+v", out.PTZPosition)
	}
}

// PTZ 位置变化通知（订阅类型 ptz_position）。
func Test2022PTZPositionNotify(t *testing.T) {
	in := &PTZPositionNotify{
		CmdType: CmdPTZPosition, SN: 202, DeviceID: "34020000001320000001",
		PTZPosition: PTZPosition{Pan: f64(88.8), Tilt: f64(10.1), Zoom: f64(2)},
	}
	body, err := Marshal(in, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	env, out, err := DecodePTZPositionNotify(body, charset.UTF8)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if env.Root() != "Notify" || env.CmdType != CmdPTZPosition {
		t.Fatalf("根元素/CmdType 错误：%+v", env)
	}
	if out.Pan == nil || *out.Pan != 88.8 || out.Zoom == nil || *out.Zoom != 2 {
		t.Fatalf("字段错误：%+v", out.PTZPosition)
	}
}

// 巡航轨迹列表：Num 是属性，SumNum 是总数；中文名称走 GB2312 往返。
func Test2022CruiseTrackListGB2312(t *testing.T) {
	in := &CruiseTrackListResponse{
		CmdType: CmdCruiseTrackList, SN: 301, DeviceID: "34020000001320000001",
		SumNum: 2,
		CruiseTrackList: &CruiseTrackList{
			Num: 2,
			Tracks: []CruiseTrack{
				{Number: 0, Name: "东门巡航"},
				{Number: 1, Name: "仓库巡航"},
			},
		},
	}
	body, err := Marshal(in, charset.GB2312)
	if err != nil {
		t.Fatalf("Marshal(GB2312) 失败：%v", err)
	}
	if !bytes.HasPrefix(body, []byte(`<?xml version="1.0" encoding="GB2312"?>`)) {
		t.Fatalf("XML 声明不正确：%q", firstLine(body))
	}
	if bytes.Contains(body, []byte("东门巡航")) {
		t.Fatalf("报文中仍含 UTF-8 中文，说明没有转码")
	}
	if bytes.Contains(body, []byte(`Num="2"`)) == false {
		t.Fatalf("CruiseTrackList Num 属性缺失：%s", body)
	}

	env, out, err := DecodeCruiseTrackList(body, charset.GB2312)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if env.CmdType != CmdCruiseTrackList || out.SumNum != 2 {
		t.Fatalf("信封/SumNum 错误：%+v", env)
	}
	if out.CruiseTrackList == nil || out.CruiseTrackList.Num != 2 || len(out.CruiseTrackList.Tracks) != 2 {
		t.Fatalf("CruiseTrackList 解析错误：%+v", out.CruiseTrackList)
	}
	if got := out.CruiseTrackList.Tracks[0].Name; got != "东门巡航" {
		t.Fatalf("轨迹名 = %q（GB2312 往返失败）", got)
	}
	if out.CruiseTrackList.Tracks[1].Number != 1 {
		t.Fatalf("第二条轨迹编号 = %d", out.CruiseTrackList.Tracks[1].Number)
	}
}

// 巡航轨迹详情：PresetIndex / StayTime / Speed 与 Num 属性。
func Test2022CruiseTrackResponse(t *testing.T) {
	in := &CruiseTrackResponse{
		CmdType: CmdCruiseTrack, SN: 302, DeviceID: "34020000001320000001",
		Number: 1, Name: "仓库巡航",
		CruisePointList: &CruisePointList{
			Num: 2,
			Points: []CruisePoint{
				{PresetIndex: 1, StayTime: 30, Speed: 5},
				{PresetIndex: 2, StayTime: 45, Speed: 15},
			},
		},
	}
	body, err := Marshal(in, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	_, out, err := DecodeCruiseTrack(body, charset.UTF8)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if out.Number != 1 || out.Name != "仓库巡航" {
		t.Fatalf("轨迹字段错误：%+v", out)
	}
	if out.CruisePointList == nil || out.CruisePointList.Num != 2 {
		t.Fatalf("CruisePointList 错误：%+v", out.CruisePointList)
	}
	p := out.CruisePointList.Points[1]
	if p.PresetIndex != 2 || p.StayTime != 45 || p.Speed != 15 {
		t.Fatalf("轨迹点错误：%+v", p)
	}
}

// 看守位：Enabled/ResetTime/PresetIndex。
func Test2022HomePositionResponse(t *testing.T) {
	in := &HomePositionResponse{
		CmdType: CmdHomePosition, SN: 303, DeviceID: "34020000001320000001",
		HomePosition: &HomePosition{Enabled: 1, ResetTime: 300, PresetIndex: 7},
	}
	body, err := Marshal(in, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	_, out, err := DecodeHomePosition(body, charset.UTF8)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if out.HomePosition == nil {
		t.Fatalf("HomePosition 块缺失")
	}
	if *out.HomePosition != (HomePosition{Enabled: 1, ResetTime: 300, PresetIndex: 7}) {
		t.Fatalf("HomePosition = %+v", *out.HomePosition)
	}
}

// 存储卡状态（尽力实现）：容量/剩余空间已确认，其余字段待真机校正。
func Test2022SDCardStatusResponse(t *testing.T) {
	in := &SDCardStatusResponse{
		CmdType: CmdSDCardStatus, SN: 304, DeviceID: "34020000001320000001",
		SDCardStatusInfo: &SDCardStatusInfo{
			Num: 1, Status: 1, FormatProgress: 100,
			Cards: []SDCardInfo{{Capacity: 32768, FreeSpace: 8192, Status: 1}},
		},
	}
	body, err := Marshal(in, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	_, out, err := DecodeSDCardStatus(body, charset.UTF8)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	info := out.SDCardStatusInfo
	if info == nil || info.Num != 1 || info.FormatProgress != 100 || len(info.Cards) != 1 {
		t.Fatalf("SDCardStatusInfo 解析错误：%+v", info)
	}
	if info.Cards[0].Capacity != 32768 || info.Cards[0].FreeSpace != 8192 {
		t.Fatalf("容量字段错误：%+v", info.Cards[0])
	}
}

// PTZ 精准控制：CmdType 仍是 DeviceControl，Pan/Tilt/Zoom 在子元素里。
func Test2022PTZPreciseControl(t *testing.T) {
	ctrl := NewPTZPreciseControl("34020000001320000001", 305, 120.5, -15, 4)
	body, err := Marshal(ctrl, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	s := string(body)
	for _, want := range []string{
		"<CmdType>DeviceControl</CmdType>",
		"<PTZPreciseCtrl>",
		"<Pan>120.5</Pan>",
		"<Tilt>-15</Tilt>",
		"<Zoom>4</Zoom>",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("报文缺少 %q：%s", want, s)
		}
	}

	_, out, err := DecodeControl(body, charset.UTF8)
	if err != nil {
		t.Fatalf("DecodeControl 失败：%v", err)
	}
	if out.PTZPrecise == nil {
		t.Fatalf("PTZPreciseCtrl 未解析")
	}
	if out.PTZPrecise.Pan == nil || *out.PTZPrecise.Pan != 120.5 {
		t.Fatalf("Pan = %v", out.PTZPrecise.Pan)
	}
	if out.PTZPrecise.Tilt == nil || *out.PTZPrecise.Tilt != -15 {
		t.Fatalf("Tilt = %v", out.PTZPrecise.Tilt)
	}
	if out.PTZPrecise.Zoom == nil || *out.PTZPrecise.Zoom != 4 {
		t.Fatalf("Zoom = %v", out.PTZPrecise.Zoom)
	}
}

// Catalog Event 往返 + 语义映射。
func Test2022CatalogEventRoundTripAndMapping(t *testing.T) {
	in := &CatalogResponse{
		CmdType: CmdCatalog, SN: 401, DeviceID: "34020000001320000001", SumNum: 3,
		DeviceList: DeviceList{Num: 3, Items: []CatalogItem{
			{DeviceID: "34020000001320000001", Name: "新增通道", Event: CatalogEventAdd, Status: StatusOn},
			{DeviceID: "34020000001320000002", Name: "丢失通道", Event: CatalogEventVLost, Status: StatusOff},
			{DeviceID: "34020000001320000003", Name: "删除通道", Event: CatalogEventDel, Status: StatusOff},
		}},
	}
	body, err := Marshal(in, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	_, out, err := DecodeCatalog(body, charset.UTF8)
	if err != nil {
		t.Fatalf("DecodeCatalog 失败：%v", err)
	}
	if got := out.DeviceList.Items[1].Event; got != CatalogEventVLost {
		t.Fatalf("Event = %q，期望 VLOST", got)
	}

	// 订阅推送使用 Notify 根元素，不能复用 Response 类型。
	notify := &CatalogNotify{
		CmdType: CmdCatalog, SN: 402, DeviceID: "34020000001320000001", SumNum: 1,
		DeviceList: DeviceList{Num: 1, Items: []CatalogItem{
			{DeviceID: "34020000001320000001", Name: "上线通道", Event: CatalogEventOn, Status: StatusOn},
		}},
	}
	notifyBody, err := Marshal(notify, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal(CatalogNotify) 失败：%v", err)
	}
	notifyEnv, notifyOut, err := DecodeCatalogNotify(notifyBody, charset.UTF8)
	if err != nil {
		t.Fatalf("DecodeCatalogNotify 失败：%v", err)
	}
	if notifyEnv.Root() != "Notify" || notifyOut.DeviceList.Items[0].Event != CatalogEventOn {
		t.Fatalf("Notify 目录解析错误：%+v", notifyEnv)
	}

	cases := []struct {
		event  string
		known  bool
		online *bool
		delete bool
	}{
		{CatalogEventAdd, true, boolPtr(true), false},
		{CatalogEventUpdate, true, boolPtr(true), false},
		{CatalogEventOn, true, boolPtr(true), false},
		{CatalogEventOff, true, boolPtr(false), false},
		{CatalogEventVLost, true, boolPtr(false), false},
		{CatalogEventDefect, true, boolPtr(false), false},
		{CatalogEventDel, true, boolPtr(false), true},
		{" add ", true, boolPtr(true), false}, // 大小写/空白归一
		{"", false, nil, false},
		{"UNKNOWN", false, nil, false},
	}
	for _, c := range cases {
		eff := ParseCatalogEvent(c.event)
		if eff.Known != c.known || eff.Delete != c.delete {
			t.Errorf("ParseCatalogEvent(%q) = %+v，期望 known=%v delete=%v", c.event, eff, c.known, c.delete)
		}
		if c.online == nil {
			if eff.Online != nil {
				t.Errorf("ParseCatalogEvent(%q).Online = %v，期望 nil", c.event, *eff.Online)
			}
			continue
		}
		if eff.Online == nil || *eff.Online != *c.online {
			t.Errorf("ParseCatalogEvent(%q).Online = %v，期望 %v", c.event, eff.Online, *c.online)
		}
	}
}

// 全量目录没有 Event，应退回 Status 判定。
func Test2022CatalogEventFallsBackToStatus(t *testing.T) {
	on := ResolveCatalogItemState(CatalogItem{Status: StatusOn})
	if !on.Known || on.Online == nil || !*on.Online {
		t.Fatalf("Status=ON 应解析为在线：%+v", on)
	}
	off := ResolveCatalogItemState(CatalogItem{Status: StatusOff})
	if !off.Known || off.Online == nil || *off.Online {
		t.Fatalf("Status=OFF 应解析为离线：%+v", off)
	}
	unknown := ResolveCatalogItemState(CatalogItem{})
	if unknown.Known {
		t.Fatalf("没有 Event/Status 时不应臆断：%+v", unknown)
	}
}
