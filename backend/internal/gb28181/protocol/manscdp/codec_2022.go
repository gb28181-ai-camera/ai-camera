package manscdp

// 本文件是 GB/T 28181-2022 新增报文的解码便捷入口。
//
// 所有实现都复用 codec.go 的 DecodeInto，因此字符集规则完全一致：
//   - 报文自带声明时按声明解码（GB2312 / UTF-8 / GBK / GB18030）；
//   - 无声明且非法 UTF-8 直接报错，绝不静默解成乱码。
//
// 平台侧的查询/控制是下行，通常只需要 Marshal；这里提供的解码入口主要给
// 联调、信令日志回放与测试使用，Wave 2 的 sipserver 可以直接复用。

// DecodeCatalogNotify 解码目录订阅通知（Notify/Catalog）。
//
// 与 DecodeCatalog（Response/Catalog）配对使用：全量目录查询走 Response，
// 订阅变更推送走 Notify。两者的 Item 都可能带 Event，语义映射见
// ParseCatalogEvent / ResolveCatalogItemState。
func DecodeCatalogNotify(body []byte, cs string) (*Envelope, *CatalogNotify, error) {
	var msg CatalogNotify
	env, err := DecodeInto(body, cs, &msg)
	if err != nil {
		return nil, nil, err
	}
	return env, &msg, nil
}

// DecodePTZPosition 解码 PTZ 精准状态查询应答（Response/PTZPosition）。
func DecodePTZPosition(body []byte, cs string) (*Envelope, *PTZPositionResponse, error) {
	var resp PTZPositionResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// DecodePTZPositionNotify 解码 PTZ 精准位置变化通知（Notify/PTZPosition）。
func DecodePTZPositionNotify(body []byte, cs string) (*Envelope, *PTZPositionNotify, error) {
	var msg PTZPositionNotify
	env, err := DecodeInto(body, cs, &msg)
	if err != nil {
		return nil, nil, err
	}
	return env, &msg, nil
}

// DecodeCruiseTrackList 解码巡航轨迹列表应答。
func DecodeCruiseTrackList(body []byte, cs string) (*Envelope, *CruiseTrackListResponse, error) {
	var resp CruiseTrackListResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// DecodeCruiseTrack 解码单条巡航轨迹应答。
func DecodeCruiseTrack(body []byte, cs string) (*Envelope, *CruiseTrackResponse, error) {
	var resp CruiseTrackResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// DecodeHomePosition 解码看守位查询应答。
func DecodeHomePosition(body []byte, cs string) (*Envelope, *HomePositionResponse, error) {
	var resp HomePositionResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// DecodeSDCardStatus 解码存储卡状态应答。
//
// 结构中的待确认字段见 SDCardStatusInfo / SDCardInfo 注释；未知元素被忽略。
func DecodeSDCardStatus(body []byte, cs string) (*Envelope, *SDCardStatusResponse, error) {
	var resp SDCardStatusResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// DecodeControl 解码 Control 报文（2016 PTZCmd / 2022 PTZPreciseCtrl 共用）。
//
// ⚠️ PTZCmd 的 8 字节二进制不是合法 UTF-8，本函数只适用于不含 PTZCmd 的
// 控制报文（例如 PTZPreciseCtrl、RecordCmd）。含二进制 PTZCmd 的报文按设计
// 属于平台下行，正常不会进入解码路径。
func DecodeControl(body []byte, cs string) (*Envelope, *Control, error) {
	var ctrl Control
	env, err := DecodeInto(body, cs, &ctrl)
	if err != nil {
		return nil, nil, err
	}
	return env, &ctrl, nil
}
