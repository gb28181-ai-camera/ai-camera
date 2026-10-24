package iot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 产品
// ════════════════════════════════════════════════════════════════

// ProductService IOT 产品业务逻辑。
type ProductService struct {
	repo  *repository.IotProductRepo
	group *repository.IotServiceGroupRepo
}

// NewProductService 构造产品服务。
func NewProductService() *ProductService {
	return &ProductService{
		repo:  repository.NewIotProductRepo(),
		group: repository.NewIotServiceGroupRepo(),
	}
}

// Page 产品分页列表。
func (s *ProductService) Page(ctx context.Context, req *request.PageIotProductReq) (*response.PageIotProductResp, error) {
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.IotProduct{}
	}
	return &response.PageIotProductResp{
		Total: total, Page: req.Page, PageSize: req.PageSize, Items: items,
	}, nil
}

// Get 产品详情。
func (s *ProductService) Get(ctx context.Context, id int64) (*model.IotProduct, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	return m, nil
}

// Create 新增产品。
//
// 配置约束在这里**严格**执行：产品是设备接入配置的聚合根，一旦写入错误组合，
// 后续 OTA 会把设备根本用不了的配网方式/服务组下发下去。历史数据的兼容
// 只在 Update 中通过 warning 放行，新建没有这个包袱。
func (s *ProductService) Create(ctx context.Context, req *request.CreateIotProductReq) (*model.IotProduct, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("产品名称不能为空")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, errors.New("产品编码不能为空")
	}

	exists, err := s.repo.ExistsByCode(ctx, code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrProductCodeExists
	}

	// 配网方式与分享策略都由产品自己配置：
	// 安防相机通常填 qrcode（无需配网），适老化相机填 bluetooth。
	// 平台不再按产品类型做联动约束，只校验取值本身合法。
	mode := strings.TrimSpace(req.NetConfigMode)
	if mode == "" {
		mode = model.IotNetConfigBluetooth
	}
	if err := validateNetConfigMode(mode); err != nil {
		return nil, err
	}

	m := &model.IotProduct{
		Name:          name,
		Code:          code,
		Category:      strings.TrimSpace(req.Category),
		Description:   req.Description,
		AutoNetwork:   req.AutoNetwork != nil && *req.AutoNetwork,
		NetConfigMode: mode,
		AllowShare:    req.AllowShare != nil && *req.AllowShare,
		Sequence:      req.Sequence,
		Active:        boolOr(req.Active, true),
	}
	if m.Sequence <= 0 {
		m.Sequence = 10
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 更新产品。文本非空才覆盖，指针非 nil 才覆盖。
//
// 配网方式与服务组都是产品自身配置，只校验取值合法，不做跨字段联动。
// 返回值保留 warning 以兼容旧响应结构，正常为空。
func (s *ProductService) Update(ctx context.Context, id int64, req *request.UpdateIotProductReq) (string, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrProductNotFound
		}
		return "", err
	}

	if v := strings.TrimSpace(req.Name); v != "" {
		m.Name = v
	}
	if v := strings.TrimSpace(req.Code); v != "" && v != m.Code {
		exists, err := s.repo.ExistsByCode(ctx, v, id)
		if err != nil {
			return "", err
		}
		if exists {
			return "", ErrProductCodeExists
		}
		m.Code = v
	}
	if v := strings.TrimSpace(req.Category); v != "" {
		m.Category = v
	}
	if req.Description != "" {
		m.Description = req.Description
	}
	if req.AutoNetwork != nil {
		m.AutoNetwork = *req.AutoNetwork
	}
	if req.Sequence > 0 {
		m.Sequence = req.Sequence
	}
	if req.Active != nil {
		m.Active = req.Active
	}

	// 配网方式与分享策略都是产品自身的配置，不做跨字段联动约束。
	if v := strings.TrimSpace(req.NetConfigMode); v != "" {
		if err := validateNetConfigMode(v); err != nil {
			return "", err
		}
		m.NetConfigMode = v
	}
	if req.AllowShare != nil {
		m.AllowShare = *req.AllowShare
	}

	if err := s.repo.Update(ctx, m); err != nil {
		return "", err
	}
	return "", nil
}

// Delete 删除产品（软删除）。
func (s *ProductService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProductNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}

// GenerateShareQR 生成/刷新产品分享二维码内容。
//
// 内容存成 JSON 字符串而不是纯 URL：二维码本身只是载体，
// 手机扫码后需要知道「这是哪个产品的分享、是否允许分享」，
// 由客户端解析并决定跳转与鉴权，平台后续扩展字段也不会破坏兼容性。
func (s *ProductService) GenerateShareQR(ctx context.Context, id int64) (*response.IotShareQRResp, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	payload := map[string]interface{}{
		"type":         "iot_product_share",
		"product_id":   m.ID,
		"product_code": m.Code,
		"product_name": m.Name,
		"allow_share":  m.AllowShare,
		"generated_at": time.Now().Format(time.RFC3339),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("生成分享二维码内容失败: %w", err)
	}
	content := string(raw)
	if err := s.repo.UpdateShareQR(ctx, id, content); err != nil {
		return nil, err
	}
	return &response.IotShareQRResp{
		ProductID:   m.ID,
		ProductCode: m.Code,
		Content:     content,
	}, nil
}

// GetServiceGroups 查询产品已绑定的服务组（密码脱敏）。
func (s *ProductService) GetServiceGroups(ctx context.Context, id int64) ([]model.IotServiceGroup, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	items, err := s.repo.ListServiceGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	return maskGroups(items), nil
}

// BindServiceGroups 整体替换产品绑定的服务组。
//
// 服务组是设备 OTA 配置的来源；配网方式与协议由管理员在产品上配置，
// 平台只校验引用的服务组真实存在。
func (s *ProductService) BindServiceGroups(ctx context.Context, id int64, ids []int64) ([]model.IotServiceGroup, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}

	// 去重后校验存在性：不存在的 ID 静默忽略会掩盖前端传错 ID 的问题。
	uniq := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, gid := range ids {
		if gid <= 0 {
			return nil, fmt.Errorf("无效的服务组ID: %d", gid)
		}
		if _, ok := seen[gid]; ok {
			continue
		}
		seen[gid] = struct{}{}
		uniq = append(uniq, gid)
	}
	if len(uniq) > 0 {
		found, err := s.group.ListByIDs(ctx, uniq)
		if err != nil {
			return nil, err
		}
		if len(found) != len(uniq) {
			return nil, errors.New("存在未找到的服务组，请刷新后重试")
		}
		// 服务组与产品的匹配由管理员在产品配置里决定（配网方式与协议都在产品上），
		// 平台只校验引用的服务组真实存在。
	}
	if err := s.repo.ReplaceServiceGroups(ctx, id, uniq); err != nil {
		return nil, err
	}
	return s.GetServiceGroups(ctx, id)
}

// netConfigLabel 配网方式的中文说明，用于错误信息。
func netConfigLabel(mode string) string {
	switch mode {
	case model.IotNetConfigBluetooth:
		return "蓝牙配网(bluetooth)"
	case model.IotNetConfigQRCode:
		return "二维码配网(qrcode)"
	case model.IotNetConfigAP:
		return "AP 配网(ap)"
	default:
		return fmt.Sprintf("配网方式 %q", mode)
	}
}

// boolOr 返回指针值，nil 时取默认值。
func boolOr(v *bool, def bool) *bool {
	if v == nil {
		return &def
	}
	return v
}
