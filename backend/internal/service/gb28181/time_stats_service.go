package gb28181

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 设备心跳 / 注册间隔统计（WVP 对齐）
//
// 数据来源：ac_gb_sip_log 的入向报文，不建时序表。
//   - keepalive → method=MESSAGE, cmd_type=Keepalive
//   - register  → method=REGISTER
//
// 只计算「相邻两条报文的间隔」：这样既能看出设备心跳是否稳定，
// 也能发现注册重试（401 → 200）造成的异常短间隔。
// ════════════════════════════════════════════════════════════════

// ErrGbTimeStatTypeInvalid 时间统计类型非法。
var ErrGbTimeStatTypeInvalid = errors.New("type 仅支持 keepalive 或 register")

// GbTimeStatisticsService 国标设备时间统计业务逻辑。
type GbTimeStatisticsService struct {
	deviceRepo *repository.GbDeviceExtraRepo
}

// NewGbTimeStatisticsService 构造时间统计服务。
func NewGbTimeStatisticsService() *GbTimeStatisticsService {
	return &GbTimeStatisticsService{deviceRepo: repository.NewGbDeviceExtraRepo()}
}

// TimeStatistics 聚合设备最近 N 条报文的相邻间隔。
func (s *GbTimeStatisticsService) TimeStatistics(ctx context.Context, deviceID int64, statType string, count int) (*response.GbTimeStatisticsResp, error) {
	typ := strings.ToLower(trim(statType))
	if typ == "" {
		typ = "keepalive"
	}

	method, cmdType := "", ""
	switch typ {
	case "keepalive":
		method = "MESSAGE"
		cmdType = "Keepalive"
	case "register":
		method = "REGISTER"
	default:
		return nil, ErrGbTimeStatTypeInvalid
	}

	if count <= 0 {
		count = 50
	}
	if count < 2 {
		count = 2
	}
	if count > 500 {
		count = 500
	}

	d, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbDeviceNotFound
		}
		return nil, err
	}
	if trim(d.GbDeviceID) == "" {
		return nil, ErrGbDeviceNotConfigured
	}

	// 多取一条才能得到 count 个间隔。
	logs, err := s.deviceRepo.ListInboundLogsAsc(ctx, d.GbDeviceID, method, cmdType, count+1)
	if err != nil {
		return nil, err
	}

	resp := &response.GbTimeStatisticsResp{
		Type:   typ,
		Points: make([]response.GbTimeStatisticsPoint, 0),
	}
	if len(logs) < 2 {
		return resp, nil
	}

	var sum int64
	for i := 1; i < len(logs); i++ {
		prev, cur := logs[i-1], logs[i]
		interval := int64(cur.CreatedDT.Sub(prev.CreatedDT).Seconds())
		if interval < 0 {
			// 时钟回拨/异常数据：跳过该点而不是产生负值。
			continue
		}
		resp.Points = append(resp.Points, response.GbTimeStatisticsPoint{
			Time:            cur.CreatedDT.Local().Format("2006-01-02 15:04:05"),
			IntervalSeconds: interval,
		})
		sum += interval
	}

	resp.Count = len(resp.Points)
	if resp.Count == 0 {
		return resp, nil
	}
	resp.AverageSeconds = math.Round(float64(sum)/float64(resp.Count)*100) / 100
	resp.MaxSeconds = resp.Points[0].IntervalSeconds
	resp.MinSeconds = resp.Points[0].IntervalSeconds
	for _, p := range resp.Points[1:] {
		if p.IntervalSeconds > resp.MaxSeconds {
			resp.MaxSeconds = p.IntervalSeconds
		}
		if p.IntervalSeconds < resp.MinSeconds {
			resp.MinSeconds = p.IntervalSeconds
		}
	}
	return resp, nil
}
