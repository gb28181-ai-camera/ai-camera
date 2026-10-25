package iot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/internal/service/alarm"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
)

// DeviceReportService 处理 MQTT 设备上报（告警/电量/位置/心跳）。
//
// 话题：iot/<group_code>/device/<device_code>/report
//
// 身份校验不走 JWT/API Key：设备用「产品 × MQTT 服务」凭据连接 broker，
// broker 已按 ACL 限制到本服务组命名空间；本服务再校验：
//   - device_code 存在且启用；
//   - 设备产品启用，且产品绑定到消息所在服务组；
//   - 服务组启用、类型为 webrtc，且组内有启用的 MQTT 服务。
//
// 这样即使同组内另一台设备伪造 device_code，也只能影响同组设备；
// 真正的设备级隔离需要把 ACL 从 group 收窄到 device（后续演进）。
type DeviceReportService struct {
	devices   *repository.DeviceRepo
	products  *repository.IotProductRepo
	groups    *repository.IotServiceGroupRepo
	positions *repository.IotDevicePositionRepo
	alarms    *alarm.AlarmService
}

// NewDeviceReportService 构造服务。
func NewDeviceReportService() *DeviceReportService {
	return &DeviceReportService{
		devices:   repository.NewDeviceRepo(),
		products:  repository.NewIotProductRepo(),
		groups:    repository.NewIotServiceGroupRepo(),
		positions: repository.NewIotDevicePositionRepo(),
		alarms:    alarm.NewAlarmService(),
	}
}

// Handle 解析并落库一条上报。错误只用于日志，不向设备回包。
func (s *DeviceReportService) Handle(ctx context.Context, groupCode string, payload []byte) error {
	if len(payload) == 0 {
		return errors.New("空上报消息")
	}
	var msg request.DeviceReportMsg
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("解析上报消息失败: %w", err)
	}
	msg.Type = strings.ToLower(strings.TrimSpace(msg.Type))
	msg.DeviceCode = strings.TrimSpace(msg.DeviceCode)
	if msg.DeviceCode == "" {
		return errors.New("device_code 不能为空")
	}

	dev, err := s.resolveDevice(ctx, groupCode, msg.DeviceCode)
	if err != nil {
		return err
	}

	now := time.Now()
	if msg.Timestamp > 0 {
		// 兼容毫秒/秒时间戳。
		ts := msg.Timestamp
		if ts > 1e12 {
			ts /= 1000
		}
		now = time.Unix(ts, 0)
	}
	fields := map[string]interface{}{
		"last_beat_dt": now,
		"status":       "online",
	}

	switch msg.Type {
	case "heartbeat":
		return s.devices.UpdateFields(ctx, dev.ID, fields)

	case "battery":
		if msg.Battery != nil {
			if *msg.Battery < 0 || *msg.Battery > 100 {
				return errors.New("battery 必须在 0-100")
			}
			fields["battery"] = *msg.Battery
		}
		if v := strings.TrimSpace(msg.Voltage); v != "" {
			fields["voltage"] = v
		}
		if msg.StorageFree != nil {
			fields["storage_free"] = *msg.StorageFree
		}
		if err := s.devices.UpdateFields(ctx, dev.ID, fields); err != nil {
			return err
		}
		// 电量/存储变化写入历史，供「设备历史记录」页趋势图使用。
		return repository.NewDeviceRepo().CreateHistory(ctx, &model.DeviceHistory{
			DeviceID: dev.ID, DeviceCode: dev.DeviceCode, RegionID: dev.RegionID,
			Location: dev.Location, Status: "online",
			Battery:     intValue(msg.Battery, dev.Battery),
			Voltage:     stringOr(msg.Voltage, dev.Voltage),
			StorageFree: int64Value(msg.StorageFree, dev.StorageFree),
			SampledDT:   now, DeptID: dev.DeptID,
		})

	case "position":
		if msg.Longitude == nil || msg.Latitude == nil {
			return errors.New("position 缺少 longitude/latitude")
		}
		if *msg.Longitude < -180 || *msg.Longitude > 180 || *msg.Latitude < -90 || *msg.Latitude > 90 {
			return errors.New("经纬度超出合法范围")
		}
		fields["longitude"] = strconv.FormatFloat(*msg.Longitude, 'f', -1, 64)
		fields["latitude"] = strconv.FormatFloat(*msg.Latitude, 'f', -1, 64)
		if err := s.devices.UpdateFields(ctx, dev.ID, fields); err != nil {
			return err
		}
		return s.positions.Create(ctx, &model.IotDevicePosition{
			DeviceID: dev.ID, DeviceCode: dev.DeviceCode,
			Longitude: *msg.Longitude, Latitude: *msg.Latitude,
			Speed: floatValue(msg.Speed), Direction: floatValue(msg.Direction),
			Altitude: floatValue(msg.Altitude), ReportedDT: now, DeptID: dev.DeptID,
		})

	case "alarm":
		code := strings.TrimSpace(msg.AlarmTypeCode)
		if code == "" {
			return errors.New("alarm 缺少 alarm_type_code")
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			content = "设备上报告警：" + code
		}
		occurred := strings.TrimSpace(msg.OccurredDT)
		if occurred == "" {
			occurred = now.Format("2006-01-02 15:04:05")
		}
		_, err := s.alarms.Report(ctx, &request.ReportAlarmReq{
			DeviceCode: dev.DeviceCode, AlarmTypeCode: code,
			Content: content, AlarmLevel: strings.TrimSpace(msg.AlarmLevel),
			OccurredDT: occurred,
		})
		if err != nil {
			return fmt.Errorf("告警落库失败: %w", err)
		}
		// 告警同时刷新心跳状态。
		return s.devices.UpdateFields(ctx, dev.ID, fields)

	default:
		return fmt.Errorf("未知上报类型 %q（支持 heartbeat/battery/position/alarm）", msg.Type)
	}
}

// resolveDevice 校验 device_code 与服务组归属。
func (s *DeviceReportService) resolveDevice(ctx context.Context, groupCode, deviceCode string) (*model.Device, error) {
	group, err := s.groups.GetByCode(ctx, groupCode)
	if err != nil {
		return nil, errors.New("上报话题中的服务组不存在")
	}
	if group.Active != nil && !*group.Active {
		return nil, errors.New("服务组已停用")
	}
	if group.Type != model.IotServiceGroupWebRTC {
		return nil, errors.New("该服务组不支持 MQTT 上报")
	}
	services, err := s.groups.ListServices(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	if !hasActiveMQTT(services) {
		return nil, errors.New("服务组未启用 MQTT 服务")
	}

	dev, err := s.devices.GetByCode(ctx, deviceCode)
	if err != nil {
		return nil, errors.New("设备不存在")
	}
	if dev.Active != nil && !*dev.Active {
		return nil, errors.New("设备已停用")
	}
	if dev.ProductID == nil || *dev.ProductID <= 0 {
		return nil, errors.New("设备未绑定 IOT 产品")
	}
	product, err := s.products.GetByID(ctx, *dev.ProductID)
	if err != nil {
		return nil, errors.New("设备所属产品不存在")
	}
	if product.Active != nil && !*product.Active {
		return nil, errors.New("设备所属产品已停用")
	}
	groups, err := s.products.ListServiceGroups(ctx, *dev.ProductID)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g.ID == group.ID {
			return dev, nil
		}
	}
	utils.Warn("MQTT 上报：设备与服务组不匹配",
		zap.String("device_code", deviceCode), zap.String("group", groupCode))
	return nil, errors.New("设备未绑定到该服务组")
}

func hasActiveMQTT(services []model.IotService) bool {
	for _, svc := range services {
		if strings.EqualFold(svc.Protocol, model.IotProtocolMQTT) && (svc.Active == nil || *svc.Active) {
			return true
		}
	}
	return false
}

func intValue(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

func int64Value(v *int64, def int64) int64 {
	if v == nil {
		return def
	}
	return *v
}

func floatValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func stringOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
