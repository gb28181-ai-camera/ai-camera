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
	repo           *repository.IotProductRepo
	group          *repository.IotServiceGroupRepo
	productService *repository.IotProductServiceRepo
	turnUser       *TurnUserService
}

// NewProductService 构造产品服务。
func NewProductService() *ProductService {
	return &ProductService{
		repo:           repository.NewIotProductRepo(),
		group:          repository.NewIotServiceGroupRepo(),
		productService: repository.NewIotProductServiceRepo(),
		turnUser:       NewTurnUserService(),
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

// Create 新增产品，并同步绑定服务组、生成产品×服务凭据。
func (s *ProductService) Create(ctx context.Context, req *request.CreateIotProductReq) (*model.IotProduct, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("产品名称不能为空")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, errors.New("产品编码不能为空")
	}
	if len(req.ServiceGroupIDs) == 0 {
		return nil, errors.New("请至少选择一个服务组")
	}

	exists, err := s.repo.ExistsByCode(ctx, code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrProductCodeExists
	}

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
	if _, err := s.BindServiceGroups(ctx, m.ID, req.ServiceGroupIDs); err != nil {
		// 创建失败时回滚产品，避免留下一个没有服务配置的半成品。
		_ = s.repo.Delete(ctx, m.ID)
		return nil, err
	}
	return m, nil
}

// Update 更新产品。文本非空才覆盖，指针非 nil 才覆盖。
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
	// nil 表示不修改服务组；空数组表示清空；非空表示整体替换。
	if req.ServiceGroupIDs != nil {
		if _, err := s.BindServiceGroups(ctx, id, req.ServiceGroupIDs); err != nil {
			return "", err
		}
	}
	return "", nil
}

// Delete 删除产品：清理产品-服务凭据、服务组绑定，再软删除产品。
func (s *ProductService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProductNotFound
		}
		return err
	}
	rows, err := s.productService.ListByProduct(ctx, id)
	if err != nil {
		return err
	}
	for _, row := range rows {
		svc, err := s.group.GetService(ctx, row.ServiceID)
		if err == nil {
			s.turnUser.Delete(ctx, *svc, row)
		}
	}
	if err := s.productService.DeleteByProduct(ctx, id); err != nil {
		return err
	}
	if err := s.repo.ReplaceServiceGroups(ctx, id, nil); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// GenerateShareQR 生成/刷新产品分享二维码内容。
func (s *ProductService) GenerateShareQR(ctx context.Context, id int64) (*response.IotShareQRResp, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}
	if !m.AllowShare {
		return nil, ErrProductShareDisabled
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

// GetServiceGroups 查询产品已绑定的服务组。
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

// BindServiceGroups 整体替换产品绑定的服务组，并同步产品×服务凭据。
func (s *ProductService) BindServiceGroups(ctx context.Context, id int64, ids []int64) ([]model.IotServiceGroup, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}

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
	}
	if err := s.repo.ReplaceServiceGroups(ctx, id, uniq); err != nil {
		return nil, err
	}
	if err := s.syncProductServices(ctx, id, uniq); err != nil {
		return nil, err
	}
	return s.GetServiceGroups(ctx, id)
}

// syncProductServices 让产品×服务凭据与服务组下的服务保持一致：
// 新增服务生成凭据，移除服务删除凭据；已有服务保留原凭据（避免设备配置漂移）。
func (s *ProductService) syncProductServices(ctx context.Context, productID int64, groupIDs []int64) error {
	services, err := s.group.ListServicesByGroupIDs(ctx, groupIDs)
	if err != nil {
		return err
	}
	desired := make(map[int64]model.IotService, len(services))
	for _, svc := range services {
		desired[svc.ID] = svc
	}

	existing, err := s.productService.ListByProduct(ctx, productID)
	if err != nil {
		return err
	}
	existingByService := make(map[int64]model.IotProductService, len(existing))
	for _, row := range existing {
		existingByService[row.ServiceID] = row
	}

	// 新增缺失凭据。
	for serviceID, svc := range desired {
		if _, ok := existingByService[serviceID]; ok {
			continue
		}
		username, err := generateProductServiceUsername(ctx, s.productService, "svc_")
		if err != nil {
			return err
		}
		password, err := GenerateServicePassword()
		if err != nil {
			return err
		}
		row := &model.IotProductService{
			Active:    boolOr(nil, true),
			ProductID: productID,
			ServiceID: serviceID,
			Username:  username,
			Password:  password,
		}
		if err := s.productService.Create(ctx, row); err != nil {
			return err
		}
		if svc.Protocol == model.IotProtocolTURN {
			if err := s.turnUser.Upsert(ctx, svc, *row); err != nil {
				return err
			}
		}
	}

	// 删除已移除服务的凭据。
	for serviceID, row := range existingByService {
		if _, ok := desired[serviceID]; ok {
			continue
		}
		svc, err := s.group.GetService(ctx, serviceID)
		if err == nil {
			s.turnUser.Delete(ctx, *svc, row)
		}
		if err := s.productService.Delete(ctx, row.ID); err != nil {
			return err
		}
	}
	return nil
}

// GetProductServices 产品下的服务凭据（密码脱敏）。
func (s *ProductService) GetProductServices(ctx context.Context, id int64) ([]model.IotProductService, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.productService.ListByProduct(ctx, id)
	if err != nil {
		return nil, err
	}
	return maskProductServices(rows), nil
}

// ResetServiceCredentials 轮换产品在某个服务上的凭据，返回一次性明文。
//
// 这是产品×服务凭据的唯一轮换入口；TURN 服务会同步更新 coturn userdb。
func (s *ProductService) ResetServiceCredentials(ctx context.Context, productID, serviceID int64) (*response.IotProductServiceCredentials, error) {
	if _, err := s.Get(ctx, productID); err != nil {
		return nil, err
	}
	row, err := s.productService.Get(ctx, productID, serviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductServiceNotFound
		}
		return nil, err
	}
	username, err := generateProductServiceUsername(ctx, s.productService, "svc_")
	if err != nil {
		return nil, err
	}
	password, err := GenerateServicePassword()
	if err != nil {
		return nil, err
	}
	old := *row
	row.Username = username
	row.Password = password
	if err := s.productService.Update(ctx, row); err != nil {
		return nil, err
	}
	svc, err := s.group.GetService(ctx, serviceID)
	if err == nil && svc.Protocol == model.IotProtocolTURN {
		s.turnUser.Delete(ctx, *svc, old)
		if err := s.turnUser.Upsert(ctx, *svc, *row); err != nil {
			return nil, err
		}
	}
	return &response.IotProductServiceCredentials{
		ProductID: productID,
		ServiceID: serviceID,
		Username:  username,
		Password:  password,
	}, nil
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
