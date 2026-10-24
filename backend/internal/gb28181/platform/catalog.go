package platform

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// maxCatalogBodyBytes 是单条 Catalog MESSAGE 的正文上限。
//
// ★ 为什么必须按**字节**分片：Catalog 走 UDP（GB28181 默认），一个 IP 分片
// 丢失就会导致整条报文丢失，而 SumNum 语义要求上级收齐所有分片才认为目录完整。
// 以太网 MTU 1500 字节，减去 IP(20) + UDP(8) + SIP 头(≈250–350) 后，
// **整包**安全上限约 1200–1300 字节；sipgo 自身在总长 >1300（1500-200）时
// 直接返回 ErrUDPMTUCongestion。因此正文取 900 字节，给请求行/Via/From/To/
// Call-ID/CSeq/Contact/Content-Type 留足余量。中文在 GB2312 下占 2 字节，
// 若按「条数」分片，长名称通道很容易把单包撑到 IP 分片或触发上述错误。
const maxCatalogBodyBytes = 900

// CatalogEntry 是目录中的一条通道（已完成定制字段覆盖与可见性过滤）。
type CatalogEntry struct {
	DeviceID     string
	Name         string
	Manufacturer string
	Model        string
	Owner        string
	CivilCode    string
	Address      string
	Parental     int
	ParentID     string
	Status       string
	ChannelType  int
	PTZType      int
	Longitude    float64
	Latitude     float64

	// App/Stream 是本地 ZLM 中该通道的流标识，上级 INVITE 时用于 StartSendRTP。
	App    string
	Stream string

	// 本地关联 ID，排障与日志用。
	ChannelID int64
}

// Source 是级联服务的数据来源。
//
// 抽成接口的目的：单测用内存实现，不需要数据库；生产实现读写 ac_gb_platform
// 系列表。StatusSink 是可选能力（状态落库）。
type Source interface {
	ListPlatforms(ctx context.Context) ([]model.GbPlatform, error)
	GetPlatform(ctx context.Context, id int64) (*model.GbPlatform, error)
	Catalog(ctx context.Context, p *model.GbPlatform) ([]CatalogEntry, error)
}

// DBPlatformSource 是 Source 的生产实现。
type DBPlatformSource struct{}

// NewDBSource 构造数据库数据源。
func NewDBSource() *DBPlatformSource { return &DBPlatformSource{} }

// ListPlatforms 返回启用的上级平台。
func (s *DBPlatformSource) ListPlatforms(ctx context.Context) ([]model.GbPlatform, error) {
	var out []model.GbPlatform
	err := bootstrap.DB().WithContext(ctx).
		Where("(enable IS NULL OR enable = true) AND (active IS NULL OR active = true)").
		Order("id ASC").Find(&out).Error
	return out, err
}

// GetPlatform 按主键读取平台。
func (s *DBPlatformSource) GetPlatform(ctx context.Context, id int64) (*model.GbPlatform, error) {
	return repository.NewGbPlatformRepo().GetByID(ctx, id)
}

// SetPlatformStatus 持久化注册状态（StatusSink）。
func (s *DBPlatformSource) SetPlatformStatus(ctx context.Context, id int64, status string) error {
	return repository.NewGbPlatformRepo().UpdateFields(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// Catalog 组装某个上级平台可见的目录。
//
// 可见范围 = GbPlatformChannel ∩（按需）GbPlatformGroup / GbPlatformRegion：
//   - 始终以 ac_gb_platform_channel 为准（只有显式加入的本地通道才会推给上级）；
//   - catalog_with_group > 0 且配置了分组时，只保留属于这些分组（含子分组）的通道；
//   - catalog_with_region > 0 且配置了区域时，只保留这些区域内的通道；
//   - 映射表为空时不做过滤（否则「刚开启开关还没配范围」会得到空目录，
//     管理员会误以为级联坏了）。
func (s *DBPlatformSource) Catalog(ctx context.Context, p *model.GbPlatform) ([]CatalogEntry, error) {
	if p == nil || p.ID <= 0 {
		return nil, fmt.Errorf("上级平台为空")
	}

	var rows []model.GbPlatformChannel
	if err := bootstrap.DB().WithContext(ctx).
		Preload("Channel").
		Where("platform_id = ?", p.ID).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	groupScope, err := groupScopeFor(ctx, p)
	if err != nil {
		return nil, err
	}
	regionScope, err := regionScopeFor(ctx, p)
	if err != nil {
		return nil, err
	}

	entries := make([]CatalogEntry, 0, len(rows))
	for _, row := range rows {
		ch := row.Channel
		if ch == nil {
			// 本地通道已删除：映射行可能因软删而失去 Preload 结果，直接跳过。
			continue
		}
		if groupScope != nil {
			if ch.GroupID == nil || !groupScope[*ch.GroupID] {
				continue
			}
		}
		if regionScope != nil {
			if ch.RegionID == nil || !regionScope[*ch.RegionID] {
				continue
			}
		}

		app, stream := localStream(ch)
		entries = append(entries, CatalogEntry{
			DeviceID:     row.CustomDeviceID,
			Name:         firstNonEmpty(row.CustomName, ch.Name, row.CustomDeviceID),
			Manufacturer: firstNonEmpty(row.CustomManufacturer, ch.Manufacturer),
			Model:        firstNonEmpty(row.CustomModel, ch.Model),
			Owner:        firstNonEmpty(row.CustomOwner, ch.Owner),
			CivilCode:    firstNonEmpty(row.CustomCivilCode, ch.CivilCode),
			Address:      firstNonEmpty(row.CustomAddress, ch.Address),
			Parental:     parentalOf(row.CustomParental, ch.Parental),
			ParentID:     firstNonEmpty(row.CustomParentID, ch.ParentID),
			Status:       firstNonEmpty(row.CustomStatus, ch.Status, model.GbStatusOff),
			ChannelType:  intOf(row.CustomChannelType, ch.ChannelType),
			PTZType:      intOf(row.CustomPTZType, ch.PtzType),
			Longitude:    ch.Longitude,
			Latitude:     ch.Latitude,
			App:          app,
			Stream:       stream,
			ChannelID:    ch.ID,
		})
	}
	return entries, nil
}

// localStream 解析本地通道在 ZLM 中的 app/stream。
//
// 优先用正在进行的本地点播会话（最可靠：流一定存在，且 stream 是实际流名）；
// 其次用 ac_gb_channel.stream_id（支持 "app/stream" 或裸 stream）。
// 两者都没有时返回空，INVITE 会明确回 480，而不是让 StartSendRTP
// 对着一个不存在的流报一个难以定位的错误。
func localStream(ch *model.GbChannel) (app, stream string) {
	if ch == nil {
		return "", ""
	}
	if raw := strings.TrimSpace(ch.StreamID); raw != "" {
		if a, st, ok := strings.Cut(raw, "/"); ok && a != "" && st != "" {
			return a, st
		}
		return "rtp", raw
	}
	if st, ok := play.DefaultRegistry().GetByChannel(ch.ID); ok && st != nil {
		return st.App, st.Stream
	}
	return "", ""
}

// groupScopeFor 计算按分组过滤的可见集合（含子分组）。
// 未启用（catalog_with_group<=0）或未配置分组时返回 nil，表示不过滤。
func groupScopeFor(ctx context.Context, p *model.GbPlatform) (map[int64]bool, error) {
	if p.CatalogWithGroup <= 0 {
		return nil, nil
	}
	var rels []model.GbPlatformGroup
	if err := bootstrap.DB().WithContext(ctx).
		Where("platform_id = ?", p.ID).Find(&rels).Error; err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return nil, nil
	}
	scope := make(map[int64]bool, len(rels))
	for _, r := range rels {
		scope[r.GroupID] = true
	}

	// 分组是树形（ac_gb_group.parent_id），只配父分组时应包含其子分组下的通道。
	var groups []model.GbGroup
	if err := bootstrap.DB().WithContext(ctx).
		Select("id", "parent_id").Find(&groups).Error; err != nil {
		return nil, err
	}
	children := make(map[int64][]int64, len(groups))
	for _, g := range groups {
		if g.ParentID != nil {
			children[*g.ParentID] = append(children[*g.ParentID], g.ID)
		}
	}
	queue := make([]int64, 0, len(scope))
	for id := range scope {
		queue = append(queue, id)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, child := range children[id] {
			if !scope[child] {
				scope[child] = true
				queue = append(queue, child)
			}
		}
	}
	return scope, nil
}

// regionScopeFor 计算按区域过滤的可见集合。区域表无层级，直接取映射。
func regionScopeFor(ctx context.Context, p *model.GbPlatform) (map[int64]bool, error) {
	if p.CatalogWithRegion <= 0 {
		return nil, nil
	}
	var rels []model.GbPlatformRegion
	if err := bootstrap.DB().WithContext(ctx).
		Where("platform_id = ?", p.ID).Find(&rels).Error; err != nil {
		return nil, err
	}
	if len(rels) == 0 {
		return nil, nil
	}
	scope := make(map[int64]bool, len(rels))
	for _, r := range rels {
		scope[r.RegionID] = true
	}
	return scope, nil
}

func parentalOf(custom int64, local int64) int {
	if custom > 0 {
		return int(custom)
	}
	return int(local)
}

func intOf(custom int64, local int64) int {
	if custom > 0 {
		return int(custom)
	}
	return int(local)
}

// ────────────────────────────────────────────────────────────────
// Catalog 报文组装与分片
// ────────────────────────────────────────────────────────────────

// catalogWire 是 Response/Notify 两种根元素共用的线格式。
type catalogWire struct {
	XMLName    xml.Name
	CmdType    string            `xml:"CmdType"`
	SN         int               `xml:"SN"`
	DeviceID   string            `xml:"DeviceID"`
	SumNum     int               `xml:"SumNum"`
	DeviceList catalogDeviceList `xml:"DeviceList"`
}

type catalogDeviceList struct {
	Num   int                   `xml:"Num,attr"`
	Items []manscdp.CatalogItem `xml:"Item"`
}

// buildCatalogChunks 把目录按字节上限切分并编码。
func buildCatalogChunks(root, cs string, sn int, deviceID string, secrecy int, entries []CatalogEntry) ([][]byte, error) {
	total := len(entries)
	var out [][]byte

	marshal := func(items []manscdp.CatalogItem) ([]byte, error) {
		wire := catalogWire{
			XMLName:  xml.Name{Local: root},
			CmdType:  manscdp.CmdCatalog,
			SN:       sn,
			DeviceID: deviceID,
			SumNum:   total,
			DeviceList: catalogDeviceList{
				Num:   len(items),
				Items: items,
			},
		}
		return manscdp.Marshal(wire, cs)
	}

	var cur []manscdp.CatalogItem
	flush := func() error {
		body, err := marshal(cur)
		if err != nil {
			return err
		}
		out = append(out, body)
		cur = nil
		return nil
	}

	for _, e := range entries {
		item := toCatalogItem(e, secrecy)
		next := append(append([]manscdp.CatalogItem{}, cur...), item)
		body, err := marshal(next)
		if err != nil {
			return nil, err
		}
		if len(body) > maxCatalogBodyBytes && len(cur) > 0 {
			if err := flush(); err != nil {
				return nil, err
			}
			cur = []manscdp.CatalogItem{item}
			continue
		}
		cur = next
	}
	if len(cur) > 0 || total == 0 {
		if err := flush(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// toCatalogItem 把内部条目映射为国标字段。
//
// 文本字段按「单条 Item 必须能装进一个 UDP 包」裁剪：DB 允许 255 字符的
// 名称/地址，如果全部字段都写满，单条 Item 就可能超过 900 字节，分片算法
// 再正确也无法发出（sipgo 会直接返回 MTU 错误）。目录展示不需要完整长文本，
// 因此这里取远小于列宽的展示上限。
func toCatalogItem(e CatalogEntry, secrecy int) manscdp.CatalogItem {
	status := strings.TrimSpace(e.Status)
	if status == "" {
		status = model.GbStatusOff
	}
	return manscdp.CatalogItem{
		DeviceID:     clipRunes(e.DeviceID, 20),
		Name:         clipRunes(e.Name, 64),
		Manufacturer: clipRunes(e.Manufacturer, 32),
		Model:        clipRunes(e.Model, 32),
		Owner:        clipRunes(e.Owner, 32),
		CivilCode:    clipRunes(e.CivilCode, 20),
		Address:      clipRunes(e.Address, 64),
		Parental:     e.Parental,
		ParentID:     clipRunes(e.ParentID, 20),
		RegisterWay:  1, // 1 = 符合 GB/T 28181 的注册方式
		Secrecy:      secrecy,
		Status:       status,
		Longitude:    e.Longitude,
		Latitude:     e.Latitude,
		ChannelType:  e.ChannelType,
		PTZType:      e.PTZType,
	}
}

// clipRunes 按字符（而非字节）截断，避免把 UTF-8/GB 转换后的多字节字符切断。
func clipRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max])
}

// ────────────────────────────────────────────────────────────────
// 上级查询应答
// ────────────────────────────────────────────────────────────────

// answerCatalogQuery 应答 Catalog 查询。
func (s *Service) answerCatalogQuery(ctx context.Context, p *peer, destination string, sn int) {
	plat := p.snapshot()
	entries, err := s.source.Catalog(ctx, &plat)
	if err != nil {
		s.log.Warn("组装级联目录失败", "server_gb_id", plat.ServerGBID, "error", err)
		return
	}
	if err := s.pushCatalog(ctx, p, entries, "Response", sn, &destination); err != nil {
		s.log.Warn("发送级联目录应答失败", "server_gb_id", plat.ServerGBID, "error", err)
		return
	}
	s.log.Info("已应答上级 Catalog 查询", "server_gb_id", plat.ServerGBID,
		"count", len(entries), "destination", destination)
}

// answerDeviceInfoQuery 用本地平台配置应答 DeviceInfo。
func (s *Service) answerDeviceInfoQuery(ctx context.Context, p *peer, destination string, sn int) {
	plat := p.snapshot()
	cs := charset.Normalize(plat.CharacterSet)

	channelCount := 0
	if entries, err := s.source.Catalog(ctx, &plat); err == nil {
		channelCount = len(entries)
	}

	body, err := manscdp.Marshal(&manscdp.DeviceInfoResponse{
		CmdType:      manscdp.CmdDeviceInfo,
		SN:           sn,
		DeviceID:     firstNonEmpty(plat.DeviceGBID, s.cfg.ServerID),
		Result:       "OK",
		DeviceName:   plat.Name,
		Manufacturer: firstNonEmpty(plat.Manufacturer, "AI Camera"),
		Model:        firstNonEmpty(plat.Model, "AI Camera Cascade"),
		Channel:      channelCount,
	}, cs)
	if err != nil {
		s.log.Warn("编码 DeviceInfo 应答失败", "error", err)
		return
	}
	if err := s.sendToSuperior(ctx, p, body, ContentTypeMANSCDP, &destination); err != nil {
		s.log.Warn("发送 DeviceInfo 应答失败", "server_gb_id", plat.ServerGBID, "error", err)
	}
}

// answerDeviceStatusQuery 用本地注册状态应答 DeviceStatus。
func (s *Service) answerDeviceStatusQuery(ctx context.Context, p *peer, destination string, sn int) {
	plat := p.snapshot()
	cs := charset.Normalize(plat.CharacterSet)

	online := "OFFLINE"
	status := "ERROR"
	if p.status().Online {
		online = "ONLINE"
		status = "OK"
	}
	body, err := manscdp.Marshal(&manscdp.DeviceStatusResponse{
		CmdType:    manscdp.CmdDeviceStatus,
		SN:         sn,
		DeviceID:   firstNonEmpty(plat.DeviceGBID, s.cfg.ServerID),
		Result:     "OK",
		Online:     online,
		Status:     status,
		Encode:     "ON",
		Record:     "OFF",
		DeviceTime: time.Now().Format("2006-01-02T15:04:05"),
	}, cs)
	if err != nil {
		s.log.Warn("编码 DeviceStatus 应答失败", "error", err)
		return
	}
	if err := s.sendToSuperior(ctx, p, body, ContentTypeMANSCDP, &destination); err != nil {
		s.log.Warn("发送 DeviceStatus 应答失败", "server_gb_id", plat.ServerGBID, "error", err)
	}
}

// parseUint32 解析十进制 SSRC。
func parseUint32(s string) (uint32, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}
