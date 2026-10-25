package gb28181

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	// ErrGbPtzPresetNameInvalid 预置位名称非法。
	ErrGbPtzPresetNameInvalid = errors.New("预置位名称不能为空且不超过 255 个字符")
	// ErrGbPtzCruisePointInvalid 巡航点/巡航参数非法。
	ErrGbPtzCruisePointInvalid = errors.New("巡航速度与停留时间至少传一个")
	// ErrGbPtzScanBoundInvalid 扫描参数非法。
	ErrGbPtzScanBoundInvalid = errors.New("扫描边界与速度至少传一个")
)

// PTZ 扩展指令码（预置位/雨刷）。
//
// 预置位沿用国内设备兼容性最好的 WVP/GB28181 附录 A 约定：
//
//	0x81 设置预置位   0x82 调用预置位   0x83 删除预置位
//	0x8C 打开雨刷     0x8D 关闭雨刷
//
// 巡航/扫描的指令码与 8 字节布局在 protocol/manscdp/ptz_ext.go 中按标准
// A.3.5/A.3.6 实现（0x84 加巡航点 / 0x85 删巡航点 / 0x86 速度 /
// 0x87 停留时间 / 0x88 开始巡航 / 0x89 扫描 / 0x8A 扫描速度），
// 停止巡航与停止扫描都复用「指令码全零」的 PTZ 停止指令。
const (
	ptzCmdPresetSet    byte = 0x81
	ptzCmdPresetCall   byte = 0x82
	ptzCmdPresetDelete byte = 0x83
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
	presetRepo  *repository.GbPtzPresetRepo
}

// NewGbPtzService 构造云台服务。
func NewGbPtzService() *GbPtzService {
	return &GbPtzService{
		channelRepo: repository.NewGbChannelRepo(),
		presetRepo:  repository.NewGbPtzPresetRepo(),
	}
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
// 名称来源优先级：DB（ac_gb_ptz_preset）→ 进程内缓存。
// 为什么 DB 优先：进程缓存重启即丢，而 WVP 对齐要求预置位名称可持久化。
//
// 设备 PresetQuery 的应答仍是异步 MESSAGE，sipserver 没有回传给业务层；
// 因此只在信令可用时「尽力发一次」让设备刷新，不等待结果。
// 该查询失败/信令未启动都不影响返回平台侧已保存的名称。
func (s *GbPtzService) ListPresets(ctx context.Context, channelID int64) ([]response.GbPresetItem, error) {
	ch, err := s.channelRepo.GetByID(ctx, channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbChannelNotFound
		}
		return nil, err
	}

	if ctl := control.Default(); ctl != nil {
		dev := ch.Device
		if dev == nil || dev.ID == 0 {
			if d, err := repository.NewGbDeviceRepo().FindByGbDeviceID(ctx, ch.GbDeviceID); err == nil {
				dev = d
			}
		}
		if dev != nil {
			if q, ok := ctl.(interface {
				QueryPreset(context.Context, *model.Device, string) error
			}); ok {
				_ = q.QueryPreset(ctx, dev, ch.GbChannelID)
			}
		}
	}

	if rows, err := s.presetRepo.ListByChannel(ctx, channelID); err == nil && len(rows) > 0 {
		out := make([]response.GbPresetItem, 0, len(rows))
		for _, row := range rows {
			out = append(out, response.GbPresetItem{PresetID: row.PresetID, Name: row.Name})
		}
		return out, nil
	}

	return presetStore.list(channelID), nil
}

// AddPreset 设置预置位（PTZCmd 0x81）。
//
// name 为可选的第 4 个参数，保持旧调用签名兼容：
//   - 传了名称 → 同时写入 ac_gb_ptz_preset；
//   - 未传 → 仍记录编号（名称为空），列表照常可见。
func (s *GbPtzService) AddPreset(ctx context.Context, channelID int64, presetID int, names ...string) error {
	if presetID < 1 || presetID > 255 {
		return ErrGbPtzPresetIDInvalid
	}
	name := ""
	if len(names) > 0 {
		name = trim(names[0])
	}
	if len([]rune(name)) > 255 {
		return ErrGbPtzPresetNameInvalid
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

	// 设备指令成功后写平台侧名称。写库失败不回滚设备动作（预置位已经在设备上创建），
	// 但不能静默：记录告警，返回成功让前端知道「设备侧已生效」。
	//
	// name 为空时保留已有名称：前端「重新设置位置」通常不会带名称，
	// 直接 Upsert 空串会把用户起好的名字清掉。
	if name == "" {
		if existing, err := s.presetRepo.Get(ctx, channelID, presetID); err == nil {
			name = existing.Name
		}
	}
	presetStore.set(channelID, presetID, name)
	if err := s.presetRepo.Upsert(ctx, channelID, presetID, name); err != nil {
		utils.Warn("预置位名称落库失败", zap.String("channel_id", strconv.FormatInt(channelID, 10)),
			zap.Int("preset_id", presetID), zap.Error(err))
	}
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
	if err := s.presetRepo.Delete(ctx, channelID, presetID); err != nil {
		utils.Warn("预置位名称删除失败", zap.String("channel_id", strconv.FormatInt(channelID, 10)),
			zap.Int("preset_id", presetID), zap.Error(err))
	}
	return nil
}

// RenamePreset 重命名平台侧预置位名称（不下发设备指令）。
//
// 设备侧预置位名称为只读/不可靠，平台侧名称是展示层数据；重命名不需要
// 设备在线或信令服务，因此不经过 loadLiveChannelAndDevice。
func (s *GbPtzService) RenamePreset(ctx context.Context, channelID int64, presetID int, name string) error {
	if presetID < 1 || presetID > 255 {
		return ErrGbPtzPresetIDInvalid
	}
	name = trim(name)
	if name == "" || len([]rune(name)) > 255 {
		return ErrGbPtzPresetNameInvalid
	}
	if _, err := s.channelRepo.GetByID(ctx, channelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbChannelNotFound
		}
		return err
	}
	if err := s.presetRepo.Upsert(ctx, channelID, presetID, name); err != nil {
		return err
	}
	presetStore.set(channelID, presetID, name)
	return nil
}

// GotoPreset 调用预置位（PTZCmd 0x82）。
//
// 与 AddPreset 的区别只在命令码：0x81 是「把当前位置存为该预置位」，
// 0x82 是「转到该预置位」。缺了 0x82，预置位功能只有存没有用。
func (s *GbPtzService) GotoPreset(ctx context.Context, channelID int64, presetID int) error {
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
	return sendPtzCommand(ctx, ctl, dev, ch, buildValuePTZCmd(ptzCmdPresetCall, presetID))
}

// Cruise 巡航控制（标准 0x88 启动；停止复用全零 PTZ 停止指令）。
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

	var cmd []byte
	if start {
		cmd, err = manscdp.BuildCruiseStartCmd(req.GroupID)
		if err != nil {
			return err
		}
	} else {
		cmd = manscdp.BuildPTZStopCmd()
	}
	return sendPtzCommand(ctx, ctl, dev, ch, cmd)
}

// Scan 自动扫描控制（标准 0x89 数据2=0 启动；停止复用全零 PTZ 停止指令）。
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

	var cmd []byte
	if start {
		cmd, err = manscdp.BuildScanStartCmd(groupID)
		if err != nil {
			return err
		}
	} else {
		cmd = manscdp.BuildPTZStopCmd()
	}
	return sendPtzCommand(ctx, ctl, dev, ch, cmd)
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

// AddCruisePoint 加入巡航点（PTZCmd 0x84）。
//
// 标准布局：数据1=巡航组号，数据2=预置位号。预置位必须已经在设备上设置
// （AddPreset 0x81）；否则设备会接受指令但巡航时跳过该点。
func (s *GbPtzService) AddCruisePoint(ctx context.Context, channelID int64, groupID, presetID int) error {
	if groupID < 1 || groupID > 255 {
		return ErrGbPtzGroupIDInvalid
	}
	if presetID < 1 || presetID > 255 {
		return ErrGbPtzPresetIDInvalid
	}
	cmd, err := manscdp.BuildCruisePointCmd(groupID, presetID)
	if err != nil {
		return err
	}
	return s.sendRawPTZ(ctx, channelID, cmd)
}

// DeleteCruisePoint 删除巡航点（PTZCmd 0x85）。
// presetID 传 0 表示删除整条巡航（标准规定数据2=0）。
func (s *GbPtzService) DeleteCruisePoint(ctx context.Context, channelID int64, groupID, presetID int) error {
	if groupID < 1 || groupID > 255 {
		return ErrGbPtzGroupIDInvalid
	}
	if presetID < 0 || presetID > 255 {
		return ErrGbPtzPresetIDInvalid
	}
	cmd, err := manscdp.BuildDeleteCruisePointCmd(groupID, presetID)
	if err != nil {
		return err
	}
	return s.sendRawPTZ(ctx, channelID, cmd)
}

// SetCruisePoint 设置巡航速度（0x86）与/或停留时间（0x87）。
// 两条指令可能都要发；顺序无关，但任一失败立即返回。
func (s *GbPtzService) SetCruisePoint(ctx context.Context, channelID int64, groupID, speed, stayTime int) error {
	if groupID < 1 || groupID > 255 {
		return ErrGbPtzGroupIDInvalid
	}
	if speed == 0 && stayTime == 0 {
		return ErrGbPtzCruisePointInvalid
	}
	if speed != 0 {
		cmd, err := manscdp.BuildCruiseSpeedCmd(groupID, speed)
		if err != nil {
			return err
		}
		if err := s.sendRawPTZ(ctx, channelID, cmd); err != nil {
			return err
		}
	}
	if stayTime != 0 {
		cmd, err := manscdp.BuildCruiseStayTimeCmd(groupID, stayTime)
		if err != nil {
			return err
		}
		if err := s.sendRawPTZ(ctx, channelID, cmd); err != nil {
			return err
		}
	}
	return nil
}

// SetScanBounds 设置扫描左右边界与速度（0x89 / 0x8A）。
//
// 标准中的边界指令是把云台**当前位置**记为左/右边界，不带角度值；
// 调用前需先把云台转到目标位置。设置完成后用
// POST /ptz/:channelId/scan {action:"start"} 启动扫描。
func (s *GbPtzService) SetScanBounds(ctx context.Context, channelID int64, req *request.GbScanBoundsReq) error {
	if req == nil {
		return ErrGbPtzScanBoundInvalid
	}
	if req.GroupID < 1 || req.GroupID > 255 {
		return ErrGbPtzGroupIDInvalid
	}
	setLeft := req.SetLeft != nil && *req.SetLeft
	setRight := req.SetRight != nil && *req.SetRight
	if !setLeft && !setRight && req.Speed == nil {
		return ErrGbPtzScanBoundInvalid
	}

	if setLeft {
		cmd, err := manscdp.BuildScanBoundCmd(req.GroupID, true)
		if err != nil {
			return err
		}
		if err := s.sendRawPTZ(ctx, channelID, cmd); err != nil {
			return err
		}
	}
	if setRight {
		cmd, err := manscdp.BuildScanBoundCmd(req.GroupID, false)
		if err != nil {
			return err
		}
		if err := s.sendRawPTZ(ctx, channelID, cmd); err != nil {
			return err
		}
	}
	if req.Speed != nil {
		cmd, err := manscdp.BuildScanSpeedCmd(req.GroupID, *req.Speed)
		if err != nil {
			return err
		}
		if err := s.sendRawPTZ(ctx, channelID, cmd); err != nil {
			return err
		}
	}
	return nil
}

// sendRawPTZ 把已构造好的 PTZCmd 发给通道归属设备。
func (s *GbPtzService) sendRawPTZ(ctx context.Context, channelID int64, cmd []byte) error {
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}
	return sendPtzCommand(ctx, ctl, dev, ch, cmd)
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
