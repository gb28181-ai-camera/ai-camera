package voiceagent

import (
	"context"
	"errors"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// AdminService Web 端记忆与 MCP 管理。
type AdminService struct {
	memories *repository.VoiceMemoryRepo
	tools    *repository.VoiceMcpToolRepo
	logs     *repository.VoiceMcpCallLogRepo
}

// NewAdminService 构造管理服务。
func NewAdminService() *AdminService {
	return &AdminService{
		memories: repository.NewVoiceMemoryRepo(),
		tools:    repository.NewVoiceMcpToolRepo(),
		logs:     repository.NewVoiceMcpCallLogRepo(),
	}
}

// EnsureBuiltins 把内置工具写入数据库（幂等）。
func (s *AdminService) EnsureBuiltins(ctx context.Context) error {
	return s.tools.EnsureBuiltin(ctx, builtinTools())
}

// PageMemories 记忆分页。
func (s *AdminService) PageMemories(ctx context.Context, req *request.PageVoiceMemoryReq, ds *utils.DataScope) (*response.PageVoiceMemoryResp, error) {
	if req == nil {
		req = &request.PageVoiceMemoryReq{}
	}
	items, total, err := s.memories.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.VoiceMemoryItem, 0, len(items))
	for _, m := range items {
		out = append(out, memoryRespItem(m))
	}
	return &response.PageVoiceMemoryResp{
		Total: total, Page: req.Page, PageSize: req.PageSize, Items: out,
	}, nil
}

// DeleteMemory 删除记忆。
func (s *AdminService) DeleteMemory(ctx context.Context, id int64, ds *utils.DataScope) error {
	if id <= 0 {
		return errors.New("记忆ID无效")
	}
	rows, err := s.memories.Delete(ctx, id, ds)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("记忆不存在或无权访问")
	}
	return nil
}

// ExportMemories 导出记忆（最多 10000 条）。
func (s *AdminService) ExportMemories(ctx context.Context, req *request.PageVoiceMemoryReq, ds *utils.DataScope) ([]response.VoiceMemoryBrief, error) {
	if req == nil {
		req = &request.PageVoiceMemoryReq{}
	}
	items, err := s.memories.Export(ctx, req, ds, 10000)
	if err != nil {
		return nil, err
	}
	out := make([]response.VoiceMemoryBrief, 0, len(items))
	for _, m := range items {
		out = append(out, response.VoiceMemoryBrief{
			ID:         m.ID,
			OwnerKey:   m.OwnerKey,
			Kind:       m.Kind,
			Content:    m.Content,
			Importance: m.Importance,
			CreatedDT:  m.CreatedDT,
		})
	}
	return out, nil
}

// PageTools MCP 工具分页。
func (s *AdminService) PageTools(ctx context.Context, req *request.PageVoiceMcpToolReq) (*response.PageVoiceMcpToolResp, error) {
	if req == nil {
		req = &request.PageVoiceMcpToolReq{}
	}
	_ = s.tools.EnsureBuiltin(ctx, builtinTools())
	items, total, err := s.tools.List(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make([]response.VoiceMcpToolItem, 0, len(items))
	for _, t := range items {
		out = append(out, response.VoiceMcpToolItem{
			ID:             t.ID,
			ServerID:       t.ServerID,
			Source:         t.Source,
			Name:           t.Name,
			WireName:       t.WireName,
			DisplayName:    t.DisplayName,
			Description:    t.Description,
			InputSchema:    t.InputSchema,
			RiskLevel:      t.RiskLevel,
			RequireConfirm: t.RequireConfirm,
			Enabled:        t.Enabled,
			Active:         t.Active,
			CreatedDT:      t.CreatedDT,
		})
	}
	return &response.PageVoiceMcpToolResp{
		Total: total, Page: req.Page, PageSize: req.PageSize, Items: out,
	}, nil
}

// UpdateTool 更新工具的启用/风险/确认策略。
func (s *AdminService) UpdateTool(ctx context.Context, id int64, req *request.UpdateVoiceMcpToolReq) error {
	if id <= 0 {
		return errors.New("工具ID无效")
	}
	if _, err := s.tools.GetByID(ctx, id); err != nil {
		return errors.New("工具不存在")
	}
	fields := map[string]interface{}{}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.RequireConfirm != nil {
		fields["require_confirm"] = *req.RequireConfirm
	}
	if req.RiskLevel != nil {
		risk := strings.ToLower(strings.TrimSpace(*req.RiskLevel))
		switch risk {
		case model.VoiceRiskLow, model.VoiceRiskMedium, model.VoiceRiskHigh:
			fields["risk_level"] = risk
		default:
			return errors.New("risk_level 仅支持 low/medium/high")
		}
	}
	if len(fields) == 0 {
		return errors.New("没有需要更新的字段")
	}
	return s.tools.UpdateFields(ctx, id, fields)
}

// PageLogs MCP 审计分页。
func (s *AdminService) PageLogs(ctx context.Context, req *request.PageVoiceMcpLogReq, ds *utils.DataScope) (*response.PageVoiceMcpLogResp, error) {
	if req == nil {
		req = &request.PageVoiceMcpLogReq{}
	}
	items, total, err := s.logs.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.VoiceMcpLogItem, 0, len(items))
	for _, l := range items {
		out = append(out, response.VoiceMcpLogItem{
			ID:         l.ID,
			SessionID:  l.SessionID,
			UserID:     l.UserID,
			DeviceID:   l.DeviceID,
			ServerCode: l.ServerCode,
			ToolName:   l.ToolName,
			RiskLevel:  l.RiskLevel,
			Confirmed:  l.Confirmed,
			Status:     l.Status,
			Args:       l.Args,
			Result:     l.Result,
			ErrorMsg:   l.ErrorMsg,
			ElapsedMS:  l.ElapsedMS,
			IP:         l.IP,
			CreatedBy:  l.CreatedBy,
			CreatedDT:  l.CreatedDT,
		})
	}
	return &response.PageVoiceMcpLogResp{
		Total: total, Page: req.Page, PageSize: req.PageSize, Items: out,
	}, nil
}

func memoryRespItem(m model.VoiceMemory) response.VoiceMemoryItem {
	return response.VoiceMemoryItem{
		ID:              m.ID,
		OwnerType:       m.OwnerType,
		OwnerID:         m.OwnerID,
		OwnerKey:        m.OwnerKey,
		Kind:            m.Kind,
		Content:         m.Content,
		ContentHash:     m.ContentHash,
		Importance:      m.Importance,
		EmbeddingStatus: m.EmbeddingStatus,
		SourceSessionID: m.SourceSessionID,
		TurnID:          m.TurnID,
		LastAccessDT:    m.LastAccessDT,
		ExpiresDT:       m.ExpiresDT,
		UserID:          m.UserID,
		DeptID:          m.DeptID,
		CreatedBy:       m.CreatedBy,
		CreatedDT:       m.CreatedDT,
	}
}
