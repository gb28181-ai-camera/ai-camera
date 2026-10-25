package gb28181

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标设备控制（WVP 对齐）
//
// 全部走带外 SIP MESSAGE / MANSCDP DeviceControl，与 PTZ 复用同一
// 发送通道，但**不复用** sendPtzCommand：DeviceControl 的语义是
// TeleBoot / GuardCmd / AlarmCmd，和 8 字节 PTZCmd 完全不同。
// ════════════════════════════════════════════════════════════════

var (
	// ErrGbGuardCmdInvalid 布防/撤防取值非法。
	ErrGbGuardCmdInvalid = errors.New("cmd 仅支持 SetGuard 或 ResetGuard")
	// ErrGbAlarmChannelMismatch 报警复位指定的通道不属于该设备。
	ErrGbAlarmChannelMismatch = errors.New("指定通道不属于该设备")
	// ErrGbRecordActionInvalid 设备侧录像控制动作非法。
	ErrGbRecordActionInvalid = errors.New("action 仅支持 start 或 stop")
)

// GbDeviceControlService 国标设备控制业务逻辑。
type GbDeviceControlService struct {
	deviceRepo  *repository.GbDeviceExtraRepo
	channelRepo *repository.GbChannelRepo
}

// NewGbDeviceControlService 构造设备控制服务。
func NewGbDeviceControlService() *GbDeviceControlService {
	return &GbDeviceControlService{
		deviceRepo:  repository.NewGbDeviceExtraRepo(),
		channelRepo: repository.NewGbChannelRepo(),
	}
}

// Reboot 重启设备（MANSCDP DeviceControl / TeleBoot=Boot）。
func (s *GbDeviceControlService) Reboot(ctx context.Context, deviceID int64) error {
	d, err := s.loadControllableDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}
	c := manscdp.NewDeviceControl(d.GbDeviceID, int(ptzSN.Add(1)), manscdp.TeleBoot, "", "")
	return sendDeviceControl(ctx, ctl, d, c, "设备重启")
}

// Guard 布防/撤防（GuardCmd=SetGuard / ResetGuard）。
func (s *GbDeviceControlService) Guard(ctx context.Context, deviceID int64, cmd string) error {
	guardCmd, err := normalizeGuardCmd(cmd)
	if err != nil {
		return err
	}
	d, err := s.loadControllableDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}
	c := manscdp.NewDeviceControl(d.GbDeviceID, int(ptzSN.Add(1)), "", "", guardCmd)
	return sendDeviceControl(ctx, ctl, d, c, "布防/撤防")
}

// AlarmReset 报警复位（AlarmCmd=ResetAlarm）。
//
// channelID 非空时用通道国标编码作为 DeviceID：GB28181 的报警复位既可针对
// 设备也可针对通道，用错对象设备会静默丢弃。这里先校验通道归属，避免
// 「拿 A 设备的通道复位 B 设备」。
func (s *GbDeviceControlService) AlarmReset(ctx context.Context, deviceID int64, channelID *int64) error {
	d, err := s.loadControllableDevice(ctx, deviceID)
	if err != nil {
		return err
	}

	target := d.GbDeviceID
	if channelID != nil && *channelID > 0 {
		ch, err := s.channelRepo.GetByID(ctx, *channelID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrGbChannelNotFound
			}
			return err
		}
		if ch.DeviceID != d.ID {
			return ErrGbAlarmChannelMismatch
		}
		if strings.TrimSpace(ch.GbChannelID) == "" {
			return ErrGbChannelNotFound
		}
		target = ch.GbChannelID
	}

	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}
	c := manscdp.NewDeviceControl(target, int(ptzSN.Add(1)), "", manscdp.AlarmCmdReset, "")
	return sendDeviceControl(ctx, ctl, d, c, "报警复位")
}

// ── 内部工具 ──────────────────────────────────────────────────

// loadControllableDevice 读取设备并校验信令前置条件。
func (s *GbDeviceControlService) loadControllableDevice(ctx context.Context, id int64) (*model.Device, error) {
	d, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbDeviceNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(d.GbDeviceID) == "" {
		return nil, ErrGbDeviceNotConfigured
	}
	return d, nil
}

// normalizeGuardCmd 归一化布防/撤防指令。
func normalizeGuardCmd(cmd string) (string, error) {
	switch strings.ToUpper(trim(cmd)) {
	case "SETGUARD":
		return manscdp.GuardCmdSet, nil
	case "RESETGUARD":
		return manscdp.GuardCmdReset, nil
	default:
		return "", ErrGbGuardCmdInvalid
	}
}

// sendDeviceControl 编码并发送一条 MANSCDP DeviceControl。
//
// 用 UTF-8 交给 SendMessage：实现会按设备 charset 转码；
// DeviceControl 不含二进制字节，走正常转码路径即可。
func sendDeviceControl(ctx context.Context, ctl control.SipController, d *model.Device, c *manscdp.Control, label string) error {
	body, err := manscdp.MarshalString(c, charset.UTF8)
	if err != nil {
		return fmt.Errorf("%s报文编码失败：%w", label, err)
	}
	if err := ctl.SendMessage(ctx, d, body); err != nil {
		return fmt.Errorf("%s下发失败：%w", label, err)
	}
	return nil
}
