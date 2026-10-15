package repository

import (
	"context"
	"errors"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// UpgradeTarget 一次升级下发的目标：固件与/或 AI 模型。
// 由 Service 依据入参解析出版本与名称后传入，Repo 不再回表。
type UpgradeTarget struct {
	FirmwareID   *int64
	FirmwareName string
	FirmwareVer  string
	AIModelID    *int64
	AIModelName  string
	AIModelVer   string
}

// FirmwareRepo 升级管理（固件类型 / 固件 / AI模型 / 升级计划 / 升级记录）数据访问层。
type FirmwareRepo struct{}

// NewFirmwareRepo 构造升级管理仓储。
func NewFirmwareRepo() *FirmwareRepo { return &FirmwareRepo{} }

func (r *FirmwareRepo) db(ctx context.Context) *gorm.DB {
	return bootstrap.DB().WithContext(ctx)
}

// ────────────────────────────────────────────────────────────────
// 固件类型
// ────────────────────────────────────────────────────────────────

// PageFirmwareTypes 固件类型分页查询。
func (r *FirmwareRepo) PageFirmwareTypes(ctx context.Context, req *request.PageFirmwareTypeReq, ds *utils.DataScope) ([]model.FirmwareType, int64, error) {
	build := func() *gorm.DB {
		q := r.db(ctx).Model(&model.FirmwareType{})
		q = ds.ApplyToQuery(q, "ac_firmware_type.dept_id")
		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			q = q.Where("(ac_firmware_type.name ILIKE ? OR ac_firmware_type.code ILIKE ?)", like, like)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.FirmwareType, 0)
	if err := build().
		Order("ac_firmware_type.sequence ASC").
		Order("ac_firmware_type.id ASC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FirmwareTypeOptions 固件类型下拉选项。
func (r *FirmwareRepo) FirmwareTypeOptions(ctx context.Context, ds *utils.DataScope) ([]model.FirmwareType, error) {
	items := make([]model.FirmwareType, 0)
	q := r.db(ctx).Model(&model.FirmwareType{})
	q = ds.ApplyToQuery(q, "ac_firmware_type.dept_id")
	if err := q.
		Order("ac_firmware_type.sequence ASC").
		Order("ac_firmware_type.id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetFirmwareType 按主键读取固件类型。
func (r *FirmwareRepo) GetFirmwareType(ctx context.Context, id int64) (*model.FirmwareType, error) {
	var m model.FirmwareType
	if err := r.db(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateFirmwareType 新增固件类型。
func (r *FirmwareRepo) CreateFirmwareType(ctx context.Context, m *model.FirmwareType) error {
	return r.db(ctx).Create(m).Error
}

// UpdateFirmwareType 全量更新固件类型可编辑字段。
func (r *FirmwareRepo) UpdateFirmwareType(ctx context.Context, m *model.FirmwareType) error {
	return r.db(ctx).
		Model(m).
		Select("name", "code", "sequence", "description", "active").
		Updates(m).Error
}

// DeleteFirmwareType 软删除固件类型。
func (r *FirmwareRepo) DeleteFirmwareType(ctx context.Context, id int64) error {
	return r.db(ctx).Delete(&model.FirmwareType{}, id).Error
}

// ────────────────────────────────────────────────────────────────
// 固件
// ────────────────────────────────────────────────────────────────

// PageFirmwares 固件分页查询（含固件类型名称）。
func (r *FirmwareRepo) PageFirmwares(ctx context.Context, req *request.PageFirmwareReq, ds *utils.DataScope) ([]model.Firmware, int64, error) {
	build := func() *gorm.DB {
		q := r.db(ctx).Model(&model.Firmware{})
		q = ds.ApplyToQuery(q, "ac_firmware.dept_id")
		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			q = q.Where("(ac_firmware.name ILIKE ? OR ac_firmware.version ILIKE ?)", like, like)
		}
		if req.FirmwareTypeID != nil && *req.FirmwareTypeID > 0 {
			q = q.Where("ac_firmware.firmware_type_id = ?", *req.FirmwareTypeID)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.Firmware, 0)
	if err := build().
		Preload("FirmwareType").
		Order("ac_firmware.id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FirmwareOptions 固件下拉选项（Code 返回版本号，便于前端展示「名称 版本」）。
func (r *FirmwareRepo) FirmwareOptions(ctx context.Context, ds *utils.DataScope) ([]model.Firmware, error) {
	items := make([]model.Firmware, 0)
	q := r.db(ctx).Model(&model.Firmware{})
	q = ds.ApplyToQuery(q, "ac_firmware.dept_id")
	if err := q.Order("ac_firmware.id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetFirmware 按主键读取固件。
func (r *FirmwareRepo) GetFirmware(ctx context.Context, id int64) (*model.Firmware, error) {
	var m model.Firmware
	if err := r.db(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateFirmware 新增固件。
func (r *FirmwareRepo) CreateFirmware(ctx context.Context, m *model.Firmware) error {
	return r.db(ctx).Create(m).Error
}

// UpdateFirmware 全量更新固件可编辑字段。
func (r *FirmwareRepo) UpdateFirmware(ctx context.Context, m *model.Firmware) error {
	return r.db(ctx).
		Model(m).
		Select("name", "version", "firmware_type_id", "file_url", "file_size", "md5", "release_dt", "description", "active").
		Updates(m).Error
}

// DeleteFirmware 软删除固件。
func (r *FirmwareRepo) DeleteFirmware(ctx context.Context, id int64) error {
	return r.db(ctx).Delete(&model.Firmware{}, id).Error
}

// ────────────────────────────────────────────────────────────────
// AI 模型
// ────────────────────────────────────────────────────────────────

// PageModels AI模型分页查询（含适用固件名称）。
func (r *FirmwareRepo) PageModels(ctx context.Context, req *request.PageAIModelReq, ds *utils.DataScope) ([]model.AIModel, int64, error) {
	build := func() *gorm.DB {
		q := r.db(ctx).Model(&model.AIModel{})
		q = ds.ApplyToQuery(q, "ac_ai_model.dept_id")
		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			q = q.Where("(ac_ai_model.name ILIKE ? OR ac_ai_model.version ILIKE ?)", like, like)
		}
		if req.FirmwareID != nil && *req.FirmwareID > 0 {
			q = q.Where("ac_ai_model.firmware_id = ?", *req.FirmwareID)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.AIModel, 0)
	if err := build().
		Preload("Firmware").
		Order("ac_ai_model.id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ModelOptions AI模型下拉选项（Code 返回版本号）。
func (r *FirmwareRepo) ModelOptions(ctx context.Context, ds *utils.DataScope) ([]model.AIModel, error) {
	items := make([]model.AIModel, 0)
	q := r.db(ctx).Model(&model.AIModel{})
	q = ds.ApplyToQuery(q, "ac_ai_model.dept_id")
	if err := q.Order("ac_ai_model.id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetModel 按主键读取AI模型。
func (r *FirmwareRepo) GetModel(ctx context.Context, id int64) (*model.AIModel, error) {
	var m model.AIModel
	if err := r.db(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateModel 新增AI模型。
func (r *FirmwareRepo) CreateModel(ctx context.Context, m *model.AIModel) error {
	return r.db(ctx).Create(m).Error
}

// UpdateModel 全量更新AI模型可编辑字段。
func (r *FirmwareRepo) UpdateModel(ctx context.Context, m *model.AIModel) error {
	return r.db(ctx).
		Model(m).
		Select("name", "version", "firmware_id", "file_url", "file_size", "md5", "release_dt", "description", "active").
		Updates(m).Error
}

// DeleteModel 软删除AI模型。
func (r *FirmwareRepo) DeleteModel(ctx context.Context, id int64) error {
	return r.db(ctx).Delete(&model.AIModel{}, id).Error
}

// ────────────────────────────────────────────────────────────────
// 升级下发
// ────────────────────────────────────────────────────────────────

// UpgradeNow 在事务内为每台设备写入一条升级记录（升级方式 now、状态 done），
// 并把设备当前固件/模型版本更新为目标版本。返回处理的设备台数。
func (r *FirmwareRepo) UpgradeNow(ctx context.Context, deviceIDs []int64, t *UpgradeTarget, operator string) (int64, error) {
	now := time.Now()
	var count int64

	err := r.db(ctx).Transaction(func(tx *gorm.DB) error {
		for _, deviceID := range deviceIDs {
			var d model.Device
			if err := tx.Where("id = ?", deviceID).First(&d).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}

			rec := model.DeviceUpgradeRecord{
				DeviceID:        d.ID,
				DeviceCode:      d.DeviceCode,
				DeviceName:      d.Name,
				RegionID:        d.RegionID,
				Location:        d.Location,
				IotCardNo:       r.iotCardNo(tx, d.IotCardID),
				PrevFirmwareID:  d.FirmwareID,
				PrevFirmwareVer: d.FirmwareVer,
				PrevAIModelID:   d.ModelID,
				PrevAIModelVer:  d.ModelVer,
				UpgradeType:     "now",
				UpgradeDT:       now,
				Status:          model.UpgradePlanDone,
				Operator:        operator,
			}
			// 未指定目标时沿用设备当前值，保证 prev_* 与目标版本可比对。
			if t.FirmwareID != nil {
				rec.FirmwareID, rec.FirmwareVer = t.FirmwareID, t.FirmwareVer
			} else {
				rec.FirmwareID, rec.FirmwareVer = d.FirmwareID, d.FirmwareVer
			}
			if t.AIModelID != nil {
				rec.AIModelID, rec.AIModelVer = t.AIModelID, t.AIModelVer
			} else {
				rec.AIModelID, rec.AIModelVer = d.ModelID, d.ModelVer
			}
			if err := tx.Create(&rec).Error; err != nil {
				return err
			}

			upd := &model.Device{}
			fields := make([]string, 0, 4)
			if t.FirmwareID != nil {
				upd.FirmwareID = t.FirmwareID
				upd.FirmwareVer = t.FirmwareVer
				fields = append(fields, "firmware_id", "firmware_ver")
			}
			if t.AIModelID != nil {
				upd.ModelID = t.AIModelID
				upd.ModelVer = t.AIModelVer
				fields = append(fields, "model_id", "model_ver")
			}
			if len(fields) > 0 {
				if err := tx.Model(&model.Device{}).Where("id = ?", d.ID).
					Select(fields).Updates(upd).Error; err != nil {
					return err
				}
			}
			count++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CreatePlans 在事务内为每台设备写入一条定时升级计划（状态 pending）。
// 计划冗余设备编号/名称/区域/位置与固件/模型名称，供列表直接展示。返回生成的计划条数。
func (r *FirmwareRepo) CreatePlans(ctx context.Context, deviceIDs []int64, t *UpgradeTarget, planDT time.Time) (int64, error) {
	var count int64

	err := r.db(ctx).Transaction(func(tx *gorm.DB) error {
		for _, deviceID := range deviceIDs {
			var d model.Device
			if err := tx.Where("id = ?", deviceID).First(&d).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}

			plan := model.DeviceUpgradePlan{
				DeviceID:     d.ID,
				DeviceCode:   d.DeviceCode,
				DeviceName:   d.Name,
				RegionID:     d.RegionID,
				Location:     d.Location,
				FirmwareID:   t.FirmwareID,
				AIModelID:    t.AIModelID,
				FirmwareName: t.FirmwareName,
				AIModelName:  t.AIModelName,
				PlanDT:       planDT,
				Status:       model.UpgradePlanPending,
			}
			if err := tx.Create(&plan).Error; err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// iotCardNo 读取设备流量卡号（冗余写入升级记录，缺失时返回空串）。
func (r *FirmwareRepo) iotCardNo(tx *gorm.DB, cardID *int64) string {
	if cardID == nil || *cardID <= 0 {
		return ""
	}
	var no string
	_ = tx.Table("ac_iot_card").Select("card_no").
		Where("id = ? AND deleted_at IS NULL", *cardID).Scan(&no).Error
	return no
}

// ────────────────────────────────────────────────────────────────
// 升级计划
// ────────────────────────────────────────────────────────────────

// PagePlans 升级计划分页查询（含设备信息）。
func (r *FirmwareRepo) PagePlans(ctx context.Context, req *request.PageUpgradePlanReq, ds *utils.DataScope) ([]model.DeviceUpgradePlan, int64, error) {
	build := func() *gorm.DB {
		q := r.db(ctx).Model(&model.DeviceUpgradePlan{})
		q = ds.ApplyToQuery(q, "ac_device_upgrade_plan.dept_id")
		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			q = q.Where("(ac_device_upgrade_plan.device_code ILIKE ? OR ac_device_upgrade_plan.device_name ILIKE ?)", like, like)
		}
		if req.RegionID != nil && *req.RegionID > 0 {
			q = q.Where("ac_device_upgrade_plan.region_id = ?", *req.RegionID)
		}
		if req.DeviceID != nil && *req.DeviceID > 0 {
			q = q.Where("ac_device_upgrade_plan.device_id = ?", *req.DeviceID)
		}
		if req.Status != "" {
			q = q.Where("ac_device_upgrade_plan.status = ?", req.Status)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.DeviceUpgradePlan, 0)
	if err := build().
		Preload("Device").
		Order("ac_device_upgrade_plan.plan_dt DESC").
		Order("ac_device_upgrade_plan.id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetPlan 按主键读取升级计划。
func (r *FirmwareRepo) GetPlan(ctx context.Context, id int64) (*model.DeviceUpgradePlan, error) {
	var m model.DeviceUpgradePlan
	if err := r.db(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CancelPlan 取消单条升级计划：仅 pending 状态可取消，否则返回业务错误。
func (r *FirmwareRepo) CancelPlan(ctx context.Context, id int64) error {
	res := r.db(ctx).
		Model(&model.DeviceUpgradePlan{}).
		Where("id = ? AND status = ?", id, model.UpgradePlanPending).
		Select("status").
		Updates(&model.DeviceUpgradePlan{Status: model.UpgradePlanCanceled})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// 区分「记录不存在」与「状态不可取消」，前者返回 ErrRecordNotFound 供上层转 404。
		var plan model.DeviceUpgradePlan
		if err := r.db(ctx).Where("id = ?", id).First(&plan).Error; err != nil {
			return err
		}
		return errors.New("仅待升级的计划可取消")
	}
	return nil
}

// BatchCancelPlans 批量取消升级计划，仅 pending 状态受影响。返回实际取消条数。
func (r *FirmwareRepo) BatchCancelPlans(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("请选择要取消的升级计划")
	}
	res := r.db(ctx).
		Model(&model.DeviceUpgradePlan{}).
		Where("id IN ? AND status = ?", ids, model.UpgradePlanPending).
		Select("status").
		Updates(&model.DeviceUpgradePlan{Status: model.UpgradePlanCanceled})
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, errors.New("没有可取消的升级计划")
	}
	return res.RowsAffected, nil
}

// ────────────────────────────────────────────────────────────────
// 升级记录
// ────────────────────────────────────────────────────────────────

// PageRecords 升级记录分页查询（含设备信息）。
func (r *FirmwareRepo) PageRecords(ctx context.Context, req *request.PageUpgradeRecordReq, ds *utils.DataScope) ([]model.DeviceUpgradeRecord, int64, error) {
	build := func() *gorm.DB {
		q := r.db(ctx).Model(&model.DeviceUpgradeRecord{})
		q = ds.ApplyToQuery(q, "ac_device_upgrade_record.dept_id")
		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			q = q.Where("(ac_device_upgrade_record.device_code ILIKE ? OR ac_device_upgrade_record.device_name ILIKE ?)", like, like)
		}
		if req.RegionID != nil && *req.RegionID > 0 {
			q = q.Where("ac_device_upgrade_record.region_id = ?", *req.RegionID)
		}
		if req.DeviceID != nil && *req.DeviceID > 0 {
			q = q.Where("ac_device_upgrade_record.device_id = ?", *req.DeviceID)
		}
		if req.StartDT != "" {
			q = q.Where("ac_device_upgrade_record.upgrade_dt >= ?", rangeStartDT(req.StartDT))
		}
		if req.EndDT != "" {
			q = q.Where("ac_device_upgrade_record.upgrade_dt <= ?", rangeEndDT(req.EndDT))
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.DeviceUpgradeRecord, 0)
	if err := build().
		Preload("Device").
		Order("ac_device_upgrade_record.upgrade_dt DESC").
		Order("ac_device_upgrade_record.id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
