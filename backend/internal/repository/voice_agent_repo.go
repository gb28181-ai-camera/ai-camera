package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// v0.6.0 智能语音聊天 - 数据访问
//
// 注意：ac_voice_memory.embedding 为 pgvector 类型，普通查询一律
// Omit 该列（database/sql 无法把 vector 扫描进 string）；向量读写
// 统一走原始 SQL 并显式 `?::vector`。
// ════════════════════════════════════════════════════════════════

// VoiceAgentConfigRepo 读取 voice_agent 配置（复用 ac_thirdparty_config）。
//
// 注意：不要命名为 VoiceConfigRepo —— 该名字已被 ac_voice_config 的
// CRUD 仓储占用（voice_config_repo.go）。
type VoiceAgentConfigRepo struct{}

// NewVoiceAgentConfigRepo 构造语音 Agent 配置仓储。
func NewVoiceAgentConfigRepo() *VoiceAgentConfigRepo { return &VoiceAgentConfigRepo{} }

// LoadMap 返回 category=voice_agent 的 code→value 映射。
func (r *VoiceAgentConfigRepo) LoadMap(ctx context.Context) (map[string]string, error) {
	var rows []model.ThirdPartyConfig
	if err := bootstrap.DB().WithContext(ctx).
		Where("category = ?", "voice_agent").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Code] = row.Value
	}
	return out, nil
}

// VoiceMcpToolRepo MCP 工具数据访问。
type VoiceMcpToolRepo struct{}

// NewVoiceMcpToolRepo 构造 MCP 工具仓储。
func NewVoiceMcpToolRepo() *VoiceMcpToolRepo { return &VoiceMcpToolRepo{} }

// EnsureBuiltin 把内置工具写入数据库（已存在则保留用户策略）。
func (r *VoiceMcpToolRepo) EnsureBuiltin(ctx context.Context, defs []model.VoiceMcpTool) error {
	for i := range defs {
		def := defs[i]
		var cnt int64
		if err := bootstrap.DB().WithContext(ctx).
			Model(&model.VoiceMcpTool{}).
			Where("name = ?", def.Name).
			Count(&cnt).Error; err != nil {
			return err
		}
		if cnt > 0 {
			continue
		}
		if def.Active == nil {
			active := true
			def.Active = &active
		}
		def.Enabled = true
		if err := bootstrap.DB().WithContext(ctx).Create(&def).Error; err != nil {
			return err
		}
	}
	return nil
}

// List 分页查询工具。
func (r *VoiceMcpToolRepo) List(ctx context.Context, req *request.PageVoiceMcpToolReq) ([]model.VoiceMcpTool, int64, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.VoiceMcpTool{})
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(name ILIKE ? OR display_name ILIKE ? OR description ILIKE ?)", like, like, like)
	}
	if src := strings.TrimSpace(req.Source); src != "" {
		q = q.Where("source = ?", src)
	}
	if risk := strings.TrimSpace(req.RiskLevel); risk != "" {
		q = q.Where("risk_level = ?", risk)
	}
	if req.Enabled != nil {
		q = q.Where("enabled = ?", *req.Enabled)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.VoiceMcpTool
	if err := q.Session(&gorm.Session{}).
		Order("id ASC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID 按主键查询工具。
func (r *VoiceMcpToolRepo) GetByID(ctx context.Context, id int64) (*model.VoiceMcpTool, error) {
	var m model.VoiceMcpTool
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByName 按工具名查询。
func (r *VoiceMcpToolRepo) GetByName(ctx context.Context, name string) (*model.VoiceMcpTool, error) {
	var m model.VoiceMcpTool
	if err := bootstrap.DB().WithContext(ctx).Where("name = ?", name).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateFields 更新工具的启用/风险/确认策略。
func (r *VoiceMcpToolRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.VoiceMcpTool{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// ListEnabled 返回启用中的工具。
func (r *VoiceMcpToolRepo) ListEnabled(ctx context.Context) ([]model.VoiceMcpTool, error) {
	var items []model.VoiceMcpTool
	err := bootstrap.DB().WithContext(ctx).
		Where("enabled = true").
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// VoiceToolPermissionRepo 工具权限数据访问。
type VoiceToolPermissionRepo struct{}

// NewVoiceToolPermissionRepo 构造工具权限仓储。
func NewVoiceToolPermissionRepo() *VoiceToolPermissionRepo { return &VoiceToolPermissionRepo{} }

// ListByTool 返回某工具的全部生效权限规则。
func (r *VoiceToolPermissionRepo) ListByTool(ctx context.Context, toolID int64) ([]model.VoiceToolPermission, error) {
	var items []model.VoiceToolPermission
	err := bootstrap.DB().WithContext(ctx).
		Where("tool_id = ? AND (active IS NULL OR active = true)", toolID).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

// VoiceMcpCallLogRepo MCP 调用审计。
type VoiceMcpCallLogRepo struct{}

// NewVoiceMcpCallLogRepo 构造审计仓储。
func NewVoiceMcpCallLogRepo() *VoiceMcpCallLogRepo { return &VoiceMcpCallLogRepo{} }

// Create 追加一条审计。
func (r *VoiceMcpCallLogRepo) Create(ctx context.Context, m *model.VoiceMcpCallLog) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Page 分页查询审计。
func (r *VoiceMcpCallLogRepo) Page(ctx context.Context, req *request.PageVoiceMcpLogReq, ds *utils.DataScope) ([]model.VoiceMcpCallLog, int64, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.VoiceMcpCallLog{})
	if v := strings.TrimSpace(req.ToolName); v != "" {
		q = q.Where("tool_name = ?", v)
	}
	if v := strings.TrimSpace(req.Status); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := strings.TrimSpace(req.SessionID); v != "" {
		q = q.Where("session_id = ?", v)
	}
	if req.UserID != nil {
		q = q.Where("user_id = ?", *req.UserID)
	}
	if req.DeviceID != nil {
		q = q.Where("device_id = ?", *req.DeviceID)
	}
	if t, ok := parseDT(req.StartDT); ok {
		q = q.Where("created_dt >= ?", t)
	}
	if t, ok := parseDT(req.EndDT); ok {
		q = q.Where("created_dt <= ?", t)
	}
	if ds != nil {
		q = ds.ApplyToQuery(q, "ac_voice_mcp_call_log.dept_id")
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.VoiceMcpCallLog
	if err := q.Session(&gorm.Session{}).
		Order("id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// VoiceMemoryHit 记忆检索命中（含相似度）。
type VoiceMemoryHit struct {
	model.VoiceMemory
	Score float64 `gorm:"column:score"`
}

// VoiceMemoryRepo 长期记忆数据访问。
type VoiceMemoryRepo struct{}

// NewVoiceMemoryRepo 构造记忆仓储。
func NewVoiceMemoryRepo() *VoiceMemoryRepo { return &VoiceMemoryRepo{} }

// Create 写入记忆；embedding 单独走 UpdateEmbedding。
func (r *VoiceMemoryRepo) Create(ctx context.Context, m *model.VoiceMemory) error {
	// 显式 Omit：即使调用方误填 embedding，也不走常规 INSERT（vector 需要 ::vector 转换）。
	return bootstrap.DB().WithContext(ctx).Omit("embedding").Create(m).Error
}

// FindByHash 按 owner_key + content_hash 查重。
func (r *VoiceMemoryRepo) FindByHash(ctx context.Context, ownerKey, hash string) (*model.VoiceMemory, error) {
	var m model.VoiceMemory
	err := bootstrap.DB().WithContext(ctx).
		Omit("embedding").
		Where("owner_key = ? AND content_hash = ? AND deleted_at IS NULL", ownerKey, hash).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindSimilar 查找同一归属下余弦相似度 > threshold 的记忆（无向量时返回 nil）。
func (r *VoiceMemoryRepo) FindSimilar(ctx context.Context, ownerKey, vector string, threshold float64) (*VoiceMemoryHit, error) {
	var hits []VoiceMemoryHit
	sql := `
		SELECT id, deleted_at, created_by, created_dt, updated_by, updated_dt,
		       owner_type, owner_id, owner_key, kind, content, content_hash, importance,
		       embedding_status, source_session_id, turn_id, last_access_dt, expires_dt,
		       user_id, dept_id,
		       1 - (embedding <=> ?::vector) AS score
		  FROM ac_voice_memory
		 WHERE deleted_at IS NULL
		   AND owner_key = ?
		   AND embedding IS NOT NULL
		   AND (expires_dt IS NULL OR expires_dt > now())
		 ORDER BY embedding <=> ?::vector
		 LIMIT 1`
	if err := bootstrap.DB().WithContext(ctx).Raw(sql, vector, ownerKey, vector).Scan(&hits).Error; err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, nil
	}
	if hits[0].Score < threshold {
		return nil, nil
	}
	return &hits[0], nil
}

// ListMissingEmbedding 列出尚无向量的记忆（按创建时间升序）。
//
// 用于「重建向量」：配置好 embedding 后，把历史记忆补成语义可检索。
// 只取正文与主键，避免把 embedding 列读进内存。
func (r *VoiceMemoryRepo) ListMissingEmbedding(ctx context.Context, limit int) ([]model.VoiceMemory, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var items []model.VoiceMemory
	err := bootstrap.DB().WithContext(ctx).
		Omit("embedding").
		Where("deleted_at IS NULL").
		Where("embedding IS NULL OR embedding_status <> ?", model.VoiceEmbeddingOK).
		Order("id ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// CountMissingEmbedding 统计尚无向量的记忆数。
func (r *VoiceMemoryRepo) CountMissingEmbedding(ctx context.Context) (int64, error) {
	var total int64
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.VoiceMemory{}).
		Where("deleted_at IS NULL").
		Where("embedding IS NULL OR embedding_status <> ?", model.VoiceEmbeddingOK).
		Count(&total).Error
	return total, err
}

// UpdateEmbedding 写入向量与状态。
func (r *VoiceMemoryRepo) UpdateEmbedding(ctx context.Context, id int64, vector string, status string) error {
	if vector == "" {
		return bootstrap.DB().WithContext(ctx).
			Model(&model.VoiceMemory{}).
			Where("id = ?", id).
			Update("embedding_status", status).Error
	}
	return bootstrap.DB().WithContext(ctx).Exec(`
		UPDATE ac_voice_memory
		   SET embedding = ?::vector, embedding_status = ?, updated_dt = now()
		 WHERE id = ?`, vector, status, id).Error
}

// RetrieveSimilar 按向量检索。ownerKeys 为归属键列表。
func (r *VoiceMemoryRepo) RetrieveSimilar(ctx context.Context, ownerKeys []string, vector string, topK int) ([]VoiceMemoryHit, error) {
	if len(ownerKeys) == 0 || vector == "" {
		return nil, nil
	}
	if topK <= 0 {
		topK = 6
	}
	var hits []VoiceMemoryHit
	sql := `
		SELECT id, deleted_at, created_by, created_dt, updated_by, updated_dt,
		       owner_type, owner_id, owner_key, kind, content, content_hash, importance,
		       embedding_status, source_session_id, turn_id, last_access_dt, expires_dt,
		       user_id, dept_id,
		       1 - (embedding <=> ?::vector) AS score
		  FROM ac_voice_memory
		 WHERE deleted_at IS NULL
		   AND owner_key IN ?
		   AND embedding IS NOT NULL
		   AND (expires_dt IS NULL OR expires_dt > now())
		 ORDER BY embedding <=> ?::vector
		 LIMIT ?`
	if err := bootstrap.DB().WithContext(ctx).Raw(sql, vector, ownerKeys, vector, topK).Scan(&hits).Error; err != nil {
		return nil, err
	}
	return hits, nil
}

// RetrieveFallback 无 embedding 时的降级检索：关键词 + 重要度 + 新近度。
func (r *VoiceMemoryRepo) RetrieveFallback(ctx context.Context, ownerKeys []string, query string, topK int) ([]VoiceMemoryHit, error) {
	if len(ownerKeys) == 0 {
		return nil, nil
	}
	if topK <= 0 {
		topK = 6
	}
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.VoiceMemory{}).
		Omit("embedding").
		Where("owner_key IN ? AND deleted_at IS NULL", ownerKeys).
		Where("(expires_dt IS NULL OR expires_dt > now())")
	if kw := strings.TrimSpace(query); kw != "" {
		q = q.Where("content ILIKE ?", "%"+kw+"%")
	}
	var items []model.VoiceMemory
	if err := q.Order("importance DESC, COALESCE(last_access_dt, created_dt) DESC").
		Limit(topK).
		Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]VoiceMemoryHit, 0, len(items))
	for _, m := range items {
		out = append(out, VoiceMemoryHit{VoiceMemory: m})
	}
	return out, nil
}

// TouchAccessed 更新最近召回时间。
func (r *VoiceMemoryRepo) TouchAccessed(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.VoiceMemory{}).
		Where("id IN ?", ids).
		Update("last_access_dt", time.Now()).Error
}

// Page 分页查询记忆（Web 管理）。
func (r *VoiceMemoryRepo) Page(ctx context.Context, req *request.PageVoiceMemoryReq, ds *utils.DataScope) ([]model.VoiceMemory, int64, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.VoiceMemory{}).Omit("embedding")
	if v := strings.TrimSpace(req.OwnerType); v != "" {
		q = q.Where("owner_type = ?", v)
	}
	if v := strings.TrimSpace(req.OwnerKey); v != "" {
		q = q.Where("owner_key = ?", v)
	}
	if v := strings.TrimSpace(req.Kind); v != "" {
		q = q.Where("kind = ?", v)
	}
	if v := strings.TrimSpace(req.EmbeddingStatus); v != "" {
		q = q.Where("embedding_status = ?", v)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		q = q.Where("content ILIKE ?", "%"+kw+"%")
	}
	q = applyMemoryScope(q, ds)

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.VoiceMemory
	if err := q.Session(&gorm.Session{}).
		Order("id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Export 导出记忆（不分页，上限由调用方控制）。
func (r *VoiceMemoryRepo) Export(ctx context.Context, req *request.PageVoiceMemoryReq, ds *utils.DataScope, limit int) ([]model.VoiceMemory, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.VoiceMemory{}).Omit("embedding")
	if v := strings.TrimSpace(req.OwnerType); v != "" {
		q = q.Where("owner_type = ?", v)
	}
	if v := strings.TrimSpace(req.OwnerKey); v != "" {
		q = q.Where("owner_key = ?", v)
	}
	if v := strings.TrimSpace(req.Kind); v != "" {
		q = q.Where("kind = ?", v)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		q = q.Where("content ILIKE ?", "%"+kw+"%")
	}
	q = applyMemoryScope(q, ds)
	if limit <= 0 {
		limit = 10000
	}
	var items []model.VoiceMemory
	err := q.Order("id DESC").Limit(limit).Find(&items).Error
	return items, err
}

// Delete 软删除记忆；scope 非空时同时校验数据可见范围。
func (r *VoiceMemoryRepo) Delete(ctx context.Context, id int64, ds *utils.DataScope) (int64, error) {
	var m model.VoiceMemory
	find := bootstrap.DB().WithContext(ctx).Omit("embedding").Where("id = ?", id)
	find = applyMemoryScope(find, ds)
	if err := find.First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	res := bootstrap.DB().WithContext(ctx).Delete(&model.VoiceMemory{}, id)
	return res.RowsAffected, res.Error
}

// applyMemoryScope 记忆的数据可见范围。
//
// 与通用 DataScope 的差异：记忆的 user_id/dept_id 可能为空
// （global 记忆、内部写入的会话记忆），dept 级过滤不能把它们全部隐藏。
// 规则：
//   - all：不过滤；
//   - 其他：本人 + 本部门/跨部门 + 无归属（global）。
func applyMemoryScope(q *gorm.DB, ds *utils.DataScope) *gorm.DB {
	if ds == nil || ds.DataPerm == "all" {
		return q
	}
	deptIDs := utils.ParseIDs(ds.CrossDeptIDs)
	if ds.DeptID != nil {
		deptIDs = append(deptIDs, *ds.DeptID)
	}
	if ds.UserID > 0 && len(deptIDs) > 0 {
		return q.Where(
			"(user_id = ? OR dept_id IN ? OR (user_id IS NULL AND dept_id IS NULL))",
			ds.UserID, dedupIDs(deptIDs),
		)
	}
	if ds.UserID > 0 {
		return q.Where("(user_id = ? OR (user_id IS NULL AND dept_id IS NULL))", ds.UserID)
	}
	return q.Where("user_id IS NULL AND dept_id IS NULL")
}

func dedupIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// DeleteExpired 物理清理已过期记忆（pipecat 侧定时任务/运维用）。
func (r *VoiceMemoryRepo) DeleteExpired(ctx context.Context) (int64, error) {
	res := bootstrap.DB().WithContext(ctx).
		Where("expires_dt IS NOT NULL AND expires_dt <= now()").
		Delete(&model.VoiceMemory{})
	return res.RowsAffected, res.Error
}

// ── 会话摘要 / 逐轮记录 ────────────────────────────────────────

// VoiceSessionMemoryRepo 摘要与逐轮记录数据访问。
type VoiceSessionMemoryRepo struct{}

// NewVoiceSessionMemoryRepo 构造会话记忆仓储。
func NewVoiceSessionMemoryRepo() *VoiceSessionMemoryRepo { return &VoiceSessionMemoryRepo{} }

// CreateSummary 写入会话摘要。
func (r *VoiceSessionMemoryRepo) CreateSummary(ctx context.Context, m *model.VoiceSessionSummary) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// CreateTurn 写入一轮对话。
func (r *VoiceSessionMemoryRepo) CreateTurn(ctx context.Context, m *model.VoiceTurn) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// ListSummaries 查询会话摘要。
func (r *VoiceSessionMemoryRepo) ListSummaries(ctx context.Context, sessionID string, limit int) ([]model.VoiceSessionSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []model.VoiceSessionSummary
	err := bootstrap.DB().WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("id DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// ── 语音会话扩展字段 ──────────────────────────────────────────

// VoiceSessionAgentRepo 语音 Agent 会话（复用 ac_voice_session）。
type VoiceSessionAgentRepo struct{}

// NewVoiceSessionAgentRepo 构造会话仓储。
func NewVoiceSessionAgentRepo() *VoiceSessionAgentRepo { return &VoiceSessionAgentRepo{} }

// Create 新建 WebSocket 语音会话。
func (r *VoiceSessionAgentRepo) Create(ctx context.Context, m *model.VoiceSession) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// UpdateFields 更新会话字段。
func (r *VoiceSessionAgentRepo) UpdateFields(ctx context.Context, sessionID string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.VoiceSession{}).
		Where("session_id = ?", sessionID).
		Updates(fields).Error
}

// ── 设备 Basic 凭据 ───────────────────────────────────────────

// FindProductServiceByUsername 按服务登录名查产品×服务凭据。
func FindProductServiceByUsername(ctx context.Context, username string) (*model.IotProductService, error) {
	var m model.IotProductService
	if err := bootstrap.DB().WithContext(ctx).
		Where("username = ? AND (active IS NULL OR active = true)", username).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ProductHasAIChatWebSocket 判断产品是否绑定 aichat 组且组内存在 WebSocket 服务。
func ProductHasAIChatWebSocket(ctx context.Context, productID int64) (bool, error) {
	var cnt int64
	err := bootstrap.DB().WithContext(ctx).Raw(`
		SELECT COUNT(1)
		  FROM ac_iot_product_service_group psg
		  JOIN ac_iot_service_group g ON g.id = psg.service_group_id
		  JOIN ac_iot_service s       ON s.group_id = g.id
		 WHERE psg.product_id = ?
		   AND psg.deleted_at IS NULL
		   AND g.type = ?
		   AND (g.active IS NULL OR g.active = true)
		   AND s.protocol = ?
		   AND (s.active IS NULL OR s.active = true)
		   AND s.deleted_at IS NULL`, productID, model.IotServiceGroupAIChat, model.IotProtocolWebSocket).
		Scan(&cnt).Error
	return cnt > 0, err
}

// FindDeviceByCode 按设备编号查询。
func FindDeviceByCode(ctx context.Context, code string) (*model.Device, error) {
	var m model.Device
	if err := bootstrap.DB().WithContext(ctx).Where("device_code = ?", code).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// parseDT 解析前端常见的时间字符串。
func parseDT(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
