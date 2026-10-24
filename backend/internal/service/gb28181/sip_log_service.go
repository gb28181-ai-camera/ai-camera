package gb28181

import (
	"context"
	"errors"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/metrics"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标信令日志（ac_gb_sip_log）
//
// ★ 全模块唯一不走 DataScope 的表：模型未嵌 BaseModel，没有 dept_id / deleted_at。
//   可见性由接口级权限码 gb:sip-log:list 控制（契约 §10.0-3 / §15.5）。
// ════════════════════════════════════════════════════════════════

// 信令日志保留策略。
//
// ac_gb_sip_log 是**追加写、无软删除**的大表：每台设备每分钟一条心跳 + 点播/云台
// 期间的 INVITE/ACK/BYE/INFO，几百台设备一天就能写出百万行。不清理会拖慢分页
// 查询并持续吃磁盘，因此必须有 janitor。
const (
	// DefaultSipLogRetentionDays 默认保留 30 天。
	// 与「风险记录保留 3 年」不同：信令日志只用于排障，30 天足够覆盖
	// 「上周还好好的，这周点播黑屏」这类现场问题。
	DefaultSipLogRetentionDays = 30

	// sipLogCleanupBatchSize 单批删除行数。
	// ★ 必须分批：一条 DELETE 删几百万行会长时间持锁，期间信令写入被阻塞，
	//   表现为设备注册/心跳超时。5000 行是「单事务足够短」与「总批次不太多」
	//   之间的折中（100 万行约 200 批）。
	sipLogCleanupBatchSize = 5000
)

// GbSipLogService 信令日志业务逻辑。
type GbSipLogService struct{}

// NewGbSipLogService 构造信令日志服务。
func NewGbSipLogService() *GbSipLogService { return &GbSipLogService{} }

// Cleanup 物理删除 created_dt 早于 retentionDays 天的信令日志，返回删除行数。
//
// 为什么用物理删除：GbSipLog 是日志表，没有 deleted_at 列，GORM 的 Delete
// 本身就是 DELETE（不会走软删）。保留软删既无意义，也会让表越清越大。
//
// 分批策略：每批独立事务，上一批删满 batchSize 才继续下一批；一批不足
// batchSize 说明已无更多过期行，立即结束。这样单事务时长可控，不会长时间
// 阻塞信令写入。
func (s *GbSipLogService) Cleanup(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = DefaultSipLogRetentionDays
	}
	db := bootstrap.DB()
	if db == nil {
		return 0, errors.New("数据库未初始化，无法清理信令日志")
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deleted, err := s.cleanupBefore(ctx, db, cutoff, sipLogCleanupBatchSize)
	metrics.AddSipLogDeleted(deleted)
	return deleted, err
}

// cleanupBefore 按截止时间分批删除，batchSize 单独传入以便单测覆盖分批边界。
func (s *GbSipLogService) cleanupBefore(ctx context.Context, db *gorm.DB, cutoff time.Time, batchSize int) (int64, error) {
	return deleteSipLogsInBatches(ctx, batchSize, func(ctx context.Context, limit int) (int64, error) {
		// 用子查询限定主键而不是直接 DELETE ... LIMIT：
		// PostgreSQL 的 DELETE 不支持 LIMIT，子查询方式既标准又能命中 created_dt 索引。
		// ORDER BY created_dt, id 保证每批都从最旧的数据开始。
		res := db.WithContext(ctx).Exec(`
			DELETE FROM ac_gb_sip_log
			 WHERE id IN (
			   SELECT id FROM ac_gb_sip_log
			    WHERE created_dt < ?
			    ORDER BY created_dt ASC, id ASC
			    LIMIT ?
			 )`, cutoff, limit)
		if res.Error != nil {
			return 0, res.Error
		}
		return res.RowsAffected, nil
	})
}

// deleteSipLogsInBatches 是分批删除的纯逻辑部分，独立出来便于无数据库单测。
//
// 终止条件：本批删除数 < batchSize。若恰好等于 batchSize，说明可能还有，
// 继续下一批；ctx 取消时立即返回已删除数量与错误，让调用方知道清理是否完整。
func deleteSipLogsInBatches(
	ctx context.Context,
	batchSize int,
	del func(context.Context, int) (int64, error),
) (int64, error) {
	if batchSize <= 0 {
		batchSize = sipLogCleanupBatchSize
	}

	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := del(ctx, batchSize)
		total += n
		if err != nil {
			return total, err
		}
		if n < int64(batchSize) {
			return total, nil
		}
	}
}

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
