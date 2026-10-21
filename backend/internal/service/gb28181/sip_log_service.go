package gb28181

import (
	"context"
	"errors"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标信令日志（ac_gb_sip_log）
//
// ★ 全模块唯一不走 DataScope 的表：模型未嵌 BaseModel，没有 dept_id / deleted_at。
//   可见性由接口级权限码 gb:sip-log:list 控制（契约 §10.0-3 / §15.5）。
// ════════════════════════════════════════════════════════════════

// GbSipLogService 信令日志业务逻辑。
type GbSipLogService struct{}

// NewGbSipLogService 构造信令日志服务。
func NewGbSipLogService() *GbSipLogService { return &GbSipLogService{} }

// Page 信令日志分页列表。
func (s *GbSipLogService) Page(ctx context.Context, req *request.PageGbSipLogReq) (*response.PageGbSipLogResp, error) {
	req.Normalize()

	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbSipLog{})
		if v := trim(req.GbDeviceID); v != "" {
			q = q.Where("gb_device_id = ?", v)
		}
		if v := trim(req.GbChannelID); v != "" {
			q = q.Where("gb_channel_id = ?", v)
		}
		if v := trim(req.Direction); v != "" {
			q = q.Where("direction = ?", v)
		}
		if v := trim(req.Method); v != "" {
			q = q.Where("method = ?", v)
		}
		if v := trim(req.CmdType); v != "" {
			q = q.Where("cmd_type = ?", v)
		}
		if v := trim(req.Result); v != "" {
			q = q.Where("result = ?", v)
		}
		if v := trim(req.StartDT); v != "" {
			q = q.Where("created_dt >= ?", v)
		}
		if v := trim(req.EndDT); v != "" {
			q = q.Where("created_dt <= ?", v)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, err
	}

	var items []model.GbSipLog
	if err := build().Order("id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.GbSipLog{}
	}

	return &response.PageGbSipLogResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Get 信令日志详情（含报文内容）。
func (s *GbSipLogService) Get(ctx context.Context, id int64) (*model.GbSipLog, error) {
	var m model.GbSipLog
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbSipLogNotFound
		}
		return nil, err
	}
	return &m, nil
}

// Stats 信令日志统计（总量 / 成功失败 / 方向 / 按方法 / 按命令类型）。
func (s *GbSipLogService) Stats(ctx context.Context, req *request.GbSipLogStatsReq) (*response.GbSipLogStatsResp, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbSipLog{})
		if v := trim(req.GbDeviceID); v != "" {
			q = q.Where("gb_device_id = ?", v)
		}
		if v := trim(req.Direction); v != "" {
			q = q.Where("direction = ?", v)
		}
		if v := trim(req.Method); v != "" {
			q = q.Where("method = ?", v)
		}
		if v := trim(req.CmdType); v != "" {
			q = q.Where("cmd_type = ?", v)
		}
		if v := trim(req.Result); v != "" {
			q = q.Where("result = ?", v)
		}
		if v := trim(req.StartDT); v != "" {
			q = q.Where("created_dt >= ?", v)
		}
		if v := trim(req.EndDT); v != "" {
			q = q.Where("created_dt <= ?", v)
		}
		return q
	}

	countBy := func(where string, args ...interface{}) (int64, error) {
		var n int64
		q := build()
		if where != "" {
			q = q.Where(where, args...)
		}
		err := q.Count(&n).Error
		return n, err
	}

	resp := &response.GbSipLogStatsResp{
		ByMethod: []response.GbSipLogStatItem{},
		ByCmd:    []response.GbSipLogStatItem{},
	}
	var err error
	if resp.Total, err = countBy(""); err != nil {
		return nil, err
	}
	if resp.Success, err = countBy("result = ?", "success"); err != nil {
		return nil, err
	}
	if resp.Failed, err = countBy("result = ?", "failed"); err != nil {
		return nil, err
	}
	if resp.Rejected, err = countBy("result = ?", "rejected"); err != nil {
		return nil, err
	}
	if resp.Inbound, err = countBy("direction = ?", "in"); err != nil {
		return nil, err
	}
	if resp.Outbound, err = countBy("direction = ?", "out"); err != nil {
		return nil, err
	}

	if err := build().Select("method AS key, COUNT(*) AS count").
		Group("method").Order("count DESC").
		Scan(&resp.ByMethod).Error; err != nil {
		return nil, err
	}
	if err := build().Select("cmd_type AS key, COUNT(*) AS count").
		Where("cmd_type IS NOT NULL AND cmd_type <> ''").
		Group("cmd_type").Order("count DESC").
		Scan(&resp.ByCmd).Error; err != nil {
		return nil, err
	}

	return resp, nil
}
