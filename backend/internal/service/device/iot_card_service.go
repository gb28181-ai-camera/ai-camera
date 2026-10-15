package device

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 资源管理 - 物联网卡 / 换卡记录
// ════════════════════════════════════════════════════════════════

// timeLayouts 请求中可能出现的时间字符串格式。
var timeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02 15:04",
	"2006-01-02",
}

// IotCardService 物联网卡业务逻辑。
type IotCardService struct {
	repo *repository.IotCardRepo
}

// NewIotCardService 构造物联网卡服务。
func NewIotCardService() *IotCardService {
	return &IotCardService{repo: repository.NewIotCardRepo()}
}

// Page 物联网卡分页列表。
func (s *IotCardService) Page(ctx context.Context, req *request.PageIotCardReq, ds *utils.DataScope) (*response.PageIotCardResp, error) {
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.IotCard{}
	}
	return &response.PageIotCardResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// Options 物联网卡下拉选项。Name 取卡名称，Code 取卡号。
func (s *IotCardService) Options(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	items, err := s.repo.Options(ctx, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.OptionItem, 0, len(items))
	for i := range items {
		out = append(out, response.OptionItem{
			ID:   items[i].ID,
			Name: items[i].Name,
			Code: items[i].CardNo,
		})
	}
	return out, nil
}

// Create 新增物联网卡。status 由到期时间与剩余流量自动推导。
func (s *IotCardService) Create(ctx context.Context, req *request.CreateIotCardReq) (*model.IotCard, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("物联网卡名称不能为空")
	}
	cardNo := strings.TrimSpace(req.CardNo)
	if cardNo == "" {
		return nil, errors.New("卡号不能为空")
	}

	exists, err := s.repo.ExistsByCardNo(ctx, cardNo, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("卡号已存在")
	}

	statDT, err := parseDateTime(req.StatDT)
	if err != nil {
		return nil, err
	}
	expireDT, err := parseDateTime(req.ExpireDT)
	if err != nil {
		return nil, err
	}

	m := &model.IotCard{
		Name:         name,
		CardNo:       cardNo,
		RegionID:     req.RegionID,
		DeviceID:     req.DeviceID,
		TrafficTotal: req.TrafficTotal,
		TrafficUsed:  req.TrafficUsed,
		TrafficLeft:  req.TrafficLeft,
		StatDT:       statDT,
		ExpireDT:     expireDT,
		Remark:       req.Remark,
	}
	m.Active = boolOr(req.Active, true)
	m.TrafficLeft = normalizeTrafficLeft(m.TrafficTotal, m.TrafficUsed, m.TrafficLeft)
	m.Status = deriveCardStatus(req.Status, m.ExpireDT, m.TrafficLeft)

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 更新物联网卡。
//
// 流量或到期时间变化后重新推导 status；文本非空才覆盖，数值大于 0 才覆盖，
// 指针非 nil 才覆盖（device_id / region_id 传 0 表示清空）。
func (s *IotCardService) Update(ctx context.Context, id int64, req *request.UpdateIotCardReq) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIotCardNotFound
		}
		return err
	}

	if v := strings.TrimSpace(req.Name); v != "" {
		m.Name = v
	}
	if v := strings.TrimSpace(req.CardNo); v != "" && v != m.CardNo {
		exists, err := s.repo.ExistsByCardNo(ctx, v, id)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("卡号已存在")
		}
		m.CardNo = v
	}
	if req.RegionID != nil {
		m.RegionID = nullableID(*req.RegionID)
	}
	if req.DeviceID != nil {
		m.DeviceID = nullableID(*req.DeviceID)
	}
	if req.TrafficTotal > 0 {
		m.TrafficTotal = req.TrafficTotal
	}
	if req.TrafficUsed > 0 {
		m.TrafficUsed = req.TrafficUsed
	}
	if req.TrafficLeft > 0 {
		m.TrafficLeft = req.TrafficLeft
	}
	if v := strings.TrimSpace(req.StatDT); v != "" {
		t, err := parseDateTime(v)
		if err != nil {
			return err
		}
		m.StatDT = t
	}
	if v := strings.TrimSpace(req.ExpireDT); v != "" {
		t, err := parseDateTime(v)
		if err != nil {
			return err
		}
		m.ExpireDT = t
	}
	if v := strings.TrimSpace(req.Remark); v != "" {
		m.Remark = v
	}
	if req.Active != nil {
		m.Active = req.Active
	}

	// 流量总额/已用变化时同步剩余流量，再按最新数据推导状态
	m.TrafficLeft = normalizeTrafficLeft(m.TrafficTotal, m.TrafficUsed, m.TrafficLeft)
	reqStatus := strings.TrimSpace(req.Status)
	if reqStatus == "" {
		reqStatus = m.Status
	}
	m.Status = deriveCardStatus(reqStatus, m.ExpireDT, m.TrafficLeft)

	return s.repo.Update(ctx, m)
}

// Delete 删除物联网卡（软删除）。
func (s *IotCardService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIotCardNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}

// PageReplaceLogs 换卡记录分页列表。
func (s *IotCardService) PageReplaceLogs(ctx context.Context, req *request.PageIotCardReplaceLogReq, ds *utils.DataScope) (*response.PageIotCardReplaceLogResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageReplaceLogs(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.IotCardReplaceLog{}
	}
	return &response.PageIotCardReplaceLogResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// ReplaceCard 设备换卡。
//
// 单事务内完成四步：解绑旧卡 → 绑定新卡 → 更新设备当前卡 → 写入换卡记录。
// OldIotCardID 缺省时取设备当前绑定卡；操作人与冗余字段（设备编号/位置/区域）一并落库。
func (s *IotCardService) ReplaceCard(ctx context.Context, req *request.ReplaceIotCardReq, operator string) (*model.IotCardReplaceLog, error) {
	if req.DeviceID <= 0 {
		return nil, errors.New("设备不能为空")
	}
	if req.NewIotCardID <= 0 {
		return nil, errors.New("新物联网卡不能为空")
	}
	if req.OldIotCardID != nil && *req.OldIotCardID == req.NewIotCardID {
		return nil, errors.New("新物联网卡与原卡相同")
	}

	if operator == "" {
		operator = model.UsernameFromCtx(ctx)
	}

	var logRow *model.IotCardReplaceLog
	err := bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dev model.Device
		if err := tx.First(&dev, req.DeviceID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDeviceNotFound
			}
			return err
		}

		var newCard model.IotCard
		if err := tx.First(&newCard, req.NewIotCardID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIotCardNotFound
			}
			return err
		}
		if newCard.DeviceID != nil && *newCard.DeviceID != dev.ID {
			return ErrIotCardInUse
		}

		// 旧卡：优先取入参，缺省取设备当前卡
		oldCardID := req.OldIotCardID
		if oldCardID == nil {
			oldCardID = dev.IotCardID
		}
		oldCardNo := ""
		if oldCardID != nil && *oldCardID > 0 && *oldCardID != newCard.ID {
			var oldCard model.IotCard
			if err := tx.First(&oldCard, *oldCardID).Error; err == nil {
				oldCardNo = oldCard.CardNo
				if err := tx.Model(&model.IotCard{}).
					Where("id = ?", oldCard.ID).
					Update("device_id", nil).Error; err != nil {
					return err
				}
			}
		} else {
			oldCardID = nil
		}

		if err := tx.Model(&model.IotCard{}).
			Where("id = ?", newCard.ID).
			Update("device_id", dev.ID).Error; err != nil {
			return err
		}

		newCardID := newCard.ID
		if err := tx.Model(&model.Device{}).
			Where("id = ?", dev.ID).
			Update("iot_card_id", newCardID).Error; err != nil {
			return err
		}

		row := &model.IotCardReplaceLog{
			DeviceID:     dev.ID,
			DeviceCode:   dev.DeviceCode,
			RegionID:     dev.RegionID,
			Location:     dev.Location,
			OldIotCardID: oldCardID,
			NewIotCardID: &newCardID,
			OldCardNo:    oldCardNo,
			NewCardNo:    newCard.CardNo,
			ReplaceDT:    time.Now(),
			Operator:     operator,
			Remark:       req.Remark,
			DeptID:       dev.DeptID,
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		logRow = row
		return nil
	})
	if err != nil {
		return nil, err
	}

	utils.Info(fmt.Sprintf("设备换卡 device=%s(%d) %s -> %s operator=%s",
		logRow.DeviceCode, logRow.DeviceID, logRow.OldCardNo, logRow.NewCardNo, operator))
	return logRow, nil
}

// ── 内部工具 ──────────────────────────────────────────────────

// parseDateTime 解析可选的时间字符串，空串返回 nil。
func parseDateTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("时间格式错误: %s", s)
}

// nullableID 把 0 或负数视为「清空」，返回 nil。
func nullableID(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	v := id
	return &v
}

// normalizeTrafficLeft 在未显式给出剩余流量时，按总额减已用补算。
func normalizeTrafficLeft(total, used, left int64) int64 {
	if left > 0 || total <= 0 {
		return left
	}
	v := total - used
	if v < 0 {
		v = 0
	}
	return v
}

// deriveCardStatus 依据到期时间与剩余流量推导物联网卡状态。
//
//	已到期      -> expired
//	剩余流量<=0 -> arrears
//	手动停用    -> disabled
//	其余        -> normal
func deriveCardStatus(reqStatus string, expireDT *time.Time, trafficLeft int64) string {
	if expireDT != nil && expireDT.Before(time.Now()) {
		return model.IotCardStatusExpired
	}
	if trafficLeft <= 0 {
		return model.IotCardStatusArrears
	}
	if strings.TrimSpace(reqStatus) == model.IotCardStatusDisabled {
		return model.IotCardStatusDisabled
	}
	return model.IotCardStatusNormal
}
