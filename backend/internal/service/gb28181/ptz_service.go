package gb28181

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// ════════════════════════════════════════════════════════════════
// 云台控制（GB/T 28181 设计 §2.5）
//
// ★ 发送路径：云台走**带外 SIP MESSAGE**（MANSCDP <Control><PTZCmd>），
// 回放的暂停/恢复/拖动/倍速才走**带内 SIP INFO**（MANSRTSP）。
// 两者搞反是典型的一周级 bug —— 表现为「按了没反应但也不报错」。
// ════════════════════════════════════════════════════════════════

var (
	// ErrGbPtzSpeedInvalid 速度越界（契约钉死 1–8）。
	ErrGbPtzSpeedInvalid = errors.New("云台速度必须在 1-8 之间")
	// ErrGbPtzDirectionConflict direction 与 action 互斥。
	ErrGbPtzDirectionConflict = errors.New("direction 与 action 互斥，只能传一个")
	// ErrGbPtzCommandMissing 既没传方向也没传动作。
	ErrGbPtzCommandMissing = errors.New("direction 与 action 至少传一个")
	// ErrGbPtzPresetIDInvalid 预置位编号越界。
	ErrGbPtzPresetIDInvalid = errors.New("预置位编号必须在 1-255 之间")
	// ErrGbPtzGroupIDInvalid 巡航/扫描组号越界。
	ErrGbPtzGroupIDInvalid = errors.New("巡航/扫描组号必须在 1-255 之间")
	// ErrGbPtzStartStopInvalid action 取值非法。
	ErrGbPtzStartStopInvalid = errors.New("action 仅支持 start 或 stop")
)

// PTZ 扩展指令码（预置位/巡航/扫描/雨刷）。
//
// 这组码不在标准方向位的取值范围里，国内设备基本沿用 WVP/GB28181 附录 A 的约定：
//
//	0x81 设置预置位   0x82 调用预置位   0x83 删除预置位
//	0x84 设置巡航组   0x85 调用巡航组   0x86 停止巡航   0x87 删除巡航组
//	0x88 设置扫描     0x89 调用扫描     0x8A 停止扫描
//	0x8B 设置雨刷     0x8C 打开雨刷     0x8D 关闭雨刷
//
// 组号/预置位号放在第 7 字节（下标 6，即方向指令里的「变倍速度」字节），
// 方向/速度字节全部置 0。不同厂商的个别实现可能有差异，真机联调时以此为准。
const (
	ptzCmdPresetSet    byte = 0x81
	ptzCmdPresetCall   byte = 0x82
	ptzCmdPresetDelete byte = 0x83
	ptzCmdCruiseStart  byte = 0x85
	ptzCmdCruiseStop   byte = 0x86
	ptzCmdScanStart    byte = 0x89
	ptzCmdScanStop     byte = 0x8A
	ptzCmdWiperOn      byte = 0x8C
	ptzCmdWiperOff     byte = 0x8D
)

// ptzSN 是 MANSCDP 的流水号。同一进程内单调递增即可；设备只用来关联请求/应答。
var ptzSN atomic.Int32

// presetStore 是进程内的预置位缓存。
//
// 为什么没有落库：数据库设计里没有「平台侧预置位」表，设备的预置位以设备为准。
// 本期 ListPresets 先发一次 PresetQuery 让设备刷新，再返回本进程缓存的条目
// （AddPreset/DeletePreset 维护）。重启后端后缓存为空，这是已知限制。
var presetStore = newPresetCache()

// GbPtzService 云台业务逻辑。
type GbPtzService struct {
	channelRepo *repository.GbChannelRepo
}

// NewGbPtzService 构造云台服务。
func NewGbPtzService() *GbPtzService {
	return &GbPtzService{channelRepo: repository.NewGbChannelRepo()}
}

// Control 云台方向/变焦/光圈/聚焦控制。
func (s *GbPtzService) Control(ctx context.Context, channelID int64, req *request.GbPtzReq) error {
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return err
	}

	// 先做纯参数校验，再检查信令服务：参数错误应在 400 直接暴露，
	// 不该因为「sipsvc 没启动」而变成 503，干扰前端排障。
	cmd, err := BuildPtzCommand(req)
	if err != nil {
		return err
	}

	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}
	return sendPtzCommand(ctx, ctl, dev, ch, cmd)
}

// BuildPtzCommand 把契约 §15.2 的请求转换为 8 字节 PTZCmd。
//
// 抽成导出函数是为了让单元测试能在不依赖 SIP/数据库的情况下验证
// 「方向与动作互斥」「速度 1–8」这两条最容易回归的约束。
func BuildPtzCommand(req *request.GbPtzReq) ([]byte, error) {
	if req == nil {
		return nil, ErrGbPtzCommandMissing
	}
	direction := strings.ToLower(trim(req.Direction))
	action := strings.ToLower(trim(req.Action))

	if direction != "" && action != "" {
		return nil, ErrGbPtzDirectionConflict
	}
	if direction == "" && action == "" {
		return nil, ErrGbPtzCommandMissing
	}

	speed := req.Speed
	if speed == 0 {
		// 契约：缺省 5。manscdp 内部还会夹到 1–8，但这里先拒绝非法值，
		// 避免前端传 99 被静默改成 8（用户看到「速度不对但不报错」）。
		speed = 5
	}
	if speed < 1 || speed > 8 {
		return nil, ErrGbPtzSpeedInvalid
	}

	if direction != "" {
		return manscdp.BuildDirectionCmd(direction, speed)
	}
	return manscdp.BuildActionCmd(action, speed)
}

// ListPresets 查询预置位列表。
//
// 设备的 PresetQuery 响应是异步 MESSAGE，本期 sipserver 没有把该响应回传给
// 业务层（SetRecordInfoHandler 只覆盖了录像查询），因此这里返回本地缓存。
// 返回空切片而不是 nil，前端可以直接 length 判断。
func (s *GbPtzService) ListPresets(ctx context.Context, channelID int64) ([]response.GbPresetItem, error) {
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return nil, err
	}
	ctl := control.Default()
	if ctl == nil {
		return nil, ErrSipServiceUnavailable
	}

	// 尽力发一次查询，让设备刷新预置位；结果由设备异步返回，本期不消费。
	if q, ok := ctl.(interface {
		QueryPreset(context.Context, *model.Device, string) error
	}); ok {
		_ = q.QueryPreset(ctx, dev, ch.GbChannelID)
	}
	return presetStore.list(channelID), nil
}

// AddPreset 设置预置位（PTZCmd 0x81）。
func (s *GbPtzService) AddPreset(ctx context.Context, channelID int64, presetID int) error {
	if presetID < 1 || presetID > 255 {
		return ErrGbPtzPresetIDInvalid
	}
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}

	if err := sendPtzCommand(ctx, ctl, dev, ch, buildValuePTZCmd(ptzCmdPresetSet, presetID)); err != nil {
		return err
	}
	presetStore.set(channelID, presetID, "")
	return nil
}

// DeletePreset 删除预置位（PTZCmd 0x83）。
func (s *GbPtzService) DeletePreset(ctx context.Context, channelID int64, presetID int) error {
	if presetID < 1 || presetID > 255 {
		return ErrGbPtzPresetIDInvalid
	}
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}

	if err := sendPtzCommand(ctx, ctl, dev, ch, buildValuePTZCmd(ptzCmdPresetDelete, presetID)); err != nil {
		return err
	}
	presetStore.del(channelID, presetID)
	return nil
}

// Cruise 巡航控制（0x85 启动 / 0x86 停止）。
func (s *GbPtzService) Cruise(ctx context.Context, channelID int64, req *request.GbPtzCruiseReq) error {
	if req == nil || req.GroupID < 1 || req.GroupID > 255 {
		return ErrGbPtzGroupIDInvalid
	}
	start, err := parseStartStop(req.Action)
	if err != nil {
		return err
	}
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}

	code := ptzCmdCruiseStop
	if start {
		code = ptzCmdCruiseStart
	}
	return sendPtzCommand(ctx, ctl, dev, ch, buildValuePTZCmd(code, req.GroupID))
}

// Scan 自动扫描控制（0x89 启动 / 0x8A 停止）。
func (s *GbPtzService) Scan(ctx context.Context, channelID int64, req *request.GbPtzScanReq) error {
	groupID := 1
	if req != nil && req.GroupID > 0 {
		groupID = req.GroupID
	}
	if groupID < 1 || groupID > 255 {
		return ErrGbPtzGroupIDInvalid
	}
	action := "start"
	if req != nil && strings.TrimSpace(req.Action) != "" {
		action = req.Action
	}
	start, err := parseStartStop(action)
	if err != nil {
		return err
	}
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}

	code := ptzCmdScanStop
	if start {
		code = ptzCmdScanStart
	}
	return sendPtzCommand(ctx, ctl, dev, ch, buildValuePTZCmd(code, groupID))
}

// Wiper 雨刷控制（0x8C 开 / 0x8D 关）。
func (s *GbPtzService) Wiper(ctx context.Context, channelID int64, on bool) error {
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}

	code := ptzCmdWiperOff
	if on {
		code = ptzCmdWiperOn
	}
	return sendPtzCommand(ctx, ctl, dev, ch, buildValuePTZCmd(code, 0))
}

// sendPtzCommand 把 8 字节指令封装成 MANSCDP 控制报文并经 MESSAGE 发出。
//
// 为什么 MarshalString 用 UTF-8 而 SendMessage 不会再转一次：
// PTZCmd 是二进制（含 0xA5，非法 UTF-8），SendMessage 检测到「非法 UTF-8 +
// 有 XML 声明」会原样透传，避免 GB18030 转码破坏指令。
func sendPtzCommand(ctx context.Context, ctl control.SipController, dev *model.Device, ch *model.GbChannel, cmd []byte) error {
	sn := int(ptzSN.Add(1))
	ctrl := manscdp.NewPTZControl(ch.GbChannelID, sn, cmd)
	body, err := manscdp.MarshalString(ctrl, charset.UTF8)
	if err != nil {
		return fmt.Errorf("云台指令编码失败：%w", err)
	}
	if err := ctl.SendMessage(ctx, dev, body); err != nil {
		return fmt.Errorf("云台指令下发失败：%w", err)
	}
	return nil
}

// buildValuePTZCmd 构造带一个数值参数（预置位号/组号）的 8 字节指令。
func buildValuePTZCmd(code byte, value int) []byte {
	out := []byte{0xA5, 0x0F, 0x01, code, 0x00, 0x00, byte(value), 0x00}
	out[7] = ptzChecksum(out[:7])
	return out
}

// ptzChecksum 计算前 7 字节的算术和（模 256），设备用它做完整性校验。
func ptzChecksum(head []byte) byte {
	var sum byte
	for _, b := range head {
		sum += b
	}
	return sum
}

// parseStartStop 解析巡航/扫描的 action。
func parseStartStop(action string) (bool, error) {
	switch strings.ToLower(trim(action)) {
	case "start":
		return true, nil
	case "stop":
		return false, nil
	default:
		return false, ErrGbPtzStartStopInvalid
	}
}

// ── 预置位缓存 ────────────────────────────────────────────────

// presetCache 按通道保存预置位编号与名称。
type presetCache struct {
	mu    sync.RWMutex
	items map[int64]map[int]string
}

// newPresetCache 构造空缓存。
func newPresetCache() *presetCache {
	return &presetCache{items: map[int64]map[int]string{}}
}

// set 新增/更新一个预置位。
func (c *presetCache) set(channelID int64, presetID int, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items[channelID] == nil {
		c.items[channelID] = map[int]string{}
	}
	c.items[channelID][presetID] = name
}

// del 删除一个预置位。
func (c *presetCache) del(channelID int64, presetID int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok := c.items[channelID]; ok {
		delete(m, presetID)
	}
}

// list 返回某通道的预置位快照（按编号升序）。
func (c *presetCache) list(channelID int64) []response.GbPresetItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := c.items[channelID]
	out := make([]response.GbPresetItem, 0, len(m))
	for id, name := range m {
		out = append(out, response.GbPresetItem{PresetID: id, Name: name})
	}
	// 编号数量很少（<=255），插入排序足够，避免为一个小列表引入 sort 包的噪音。
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].PresetID > out[j].PresetID; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
