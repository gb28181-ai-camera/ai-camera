package voiceagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// 记忆错误。
var (
	ErrMemoryOwnerRequired = errors.New("owner_keys / owner_type 不能为空")
	ErrMemoryContentEmpty  = errors.New("content 不能为空")
	ErrMemoryOwnerType     = errors.New("owner_type 仅支持 user/device/dept/global")
)

// MemoryService 长期记忆读写。
type MemoryService struct {
	provider ConfigProvider
	repo     *repository.VoiceMemoryRepo
	now      func() time.Time
}

// NewMemoryService 构造记忆服务。
func NewMemoryService(provider ConfigProvider) *MemoryService {
	return &MemoryService{
		provider: provider,
		repo:     repository.NewVoiceMemoryRepo(),
		now:      time.Now,
	}
}

// Retrieve 记忆检索。
//
// 有 embedding：pgvector 余弦相似度排序；
// 无 embedding（未配置或调用失败）：降级为关键词 + 重要度 + 新近度。
func (s *MemoryService) Retrieve(ctx context.Context, req *request.MemoryRetrieveReq) ([]request.MemoryItem, error) {
	if req == nil {
		return nil, ErrMemoryOwnerRequired
	}
	keys := normalizeOwnerKeys(req.OwnerKeys)
	if len(keys) == 0 {
		return nil, ErrMemoryOwnerRequired
	}
	cfg := s.runtime(ctx)
	if !cfg.MemoryEnabled {
		return []request.MemoryItem{}, nil
	}
	topK := req.TopK
	if topK <= 0 {
		topK = cfg.MemoryTopK
	}
	if topK <= 0 {
		topK = defaultTopK
	}
	if topK > 50 {
		topK = 50
	}

	vec := req.Embedding
	if len(vec) == 0 && strings.TrimSpace(req.Query) != "" {
		client := NewEmbeddingClient(cfg.EmbeddingEndpoint, cfg.EmbeddingModel, cfg.EmbeddingAPIKey)
		if v, err := client.Embed(ctx, req.Query); err == nil {
			vec = v
		}
	}

	var hits []repository.VoiceMemoryHit
	if len(vec) > 0 {
		if h, err := s.repo.RetrieveSimilar(ctx, keys, vectorLiteral(vec), topK); err == nil {
			hits = h
		}
	}
	if len(hits) == 0 {
		h, err := s.repo.RetrieveFallback(ctx, keys, req.Query, topK)
		if err != nil {
			return nil, err
		}
		hits = h
	}

	ids := make([]int64, 0, len(hits))
	out := make([]request.MemoryItem, 0, len(hits))
	for i := range hits {
		ids = append(ids, hits[i].ID)
		out = append(out, memoryItem(hits[i].VoiceMemory))
	}
	// 召回时间用于「新近度」排序与 Web 管理展示；失败不影响检索结果。
	_ = s.repo.TouchAccessed(ctx, ids)
	return out, nil
}

// Write 记忆写入。
//
// 去重顺序：
//  1. owner_key + content_hash 完全相同；
//  2. 有 embedding 时，同 owner_key 下余弦相似度 > 0.90。
//
// 命中任一条件即返回已有记录（deduplicated=true），不重复写入。
func (s *MemoryService) Write(ctx context.Context, req *request.MemoryWriteReq) (*request.MemoryItem, bool, error) {
	if req == nil {
		return nil, false, ErrMemoryOwnerRequired
	}
	ownerType := strings.ToLower(strings.TrimSpace(req.OwnerType))
	switch ownerType {
	case model.VoiceOwnerUser, model.VoiceOwnerDevice, model.VoiceOwnerDept, model.VoiceOwnerGlobal:
	default:
		return nil, false, ErrMemoryOwnerType
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, false, ErrMemoryContentEmpty
	}
	cfg := s.runtime(ctx)
	if !cfg.MemoryEnabled {
		return nil, false, errors.New("长期记忆未启用")
	}
	ownerKey := ownerKey(ownerType, req.OwnerID)
	hash := contentHash(content)

	// 1) 完全重复
	if existing, err := s.repo.FindByHash(ctx, ownerKey, hash); err == nil && existing != nil {
		return ptrMemoryItem(*existing), true, nil
	}

	// 2) 语义近似重复
	vec := req.Embedding
	if len(vec) == 0 {
		client := NewEmbeddingClient(cfg.EmbeddingEndpoint, cfg.EmbeddingModel, cfg.EmbeddingAPIKey)
		if v, err := client.Embed(ctx, content); err == nil {
			vec = v
		}
	}
	if len(vec) > 0 {
		if hit, err := s.repo.FindSimilar(ctx, ownerKey, vectorLiteral(vec), 0.90); err == nil && hit != nil {
			return ptrMemoryItem(hit.VoiceMemory), true, nil
		}
	}

	importance := req.Importance
	if importance < 1 || importance > 5 {
		importance = 3
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "fact"
	}
	now := s.now()
	ttlDays := cfg.MemoryTTLDays
	if ttlDays <= 0 {
		ttlDays = defaultMemoryTTLDays
	}
	expires := now.Add(time.Duration(ttlDays) * 24 * time.Hour)
	status := model.VoiceEmbeddingFailed
	if len(vec) > 0 {
		status = model.VoiceEmbeddingPending
	}

	m := &model.VoiceMemory{
		OwnerType:       ownerType,
		OwnerID:         req.OwnerID,
		OwnerKey:        ownerKey,
		Kind:            kind,
		Content:         content,
		ContentHash:     hash,
		Importance:      importance,
		EmbeddingStatus: status,
		SourceSessionID: strings.TrimSpace(req.SourceSessionID),
		TurnID:          strings.TrimSpace(req.TurnID),
		ExpiresDT:       &expires,
	}
	m.UserID, m.DeptID = s.scopeIDs(ctx, ownerType, req.OwnerID)
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, false, err
	}
	if len(vec) > 0 {
		if err := s.repo.UpdateEmbedding(ctx, m.ID, vectorLiteral(vec), model.VoiceEmbeddingOK); err != nil {
			// 向量写失败不影响记忆本身：标记 failed，让后台可重试。
			_ = s.repo.UpdateEmbedding(ctx, m.ID, "", model.VoiceEmbeddingFailed)
			m.EmbeddingStatus = model.VoiceEmbeddingFailed
		} else {
			m.EmbeddingStatus = model.VoiceEmbeddingOK
		}
	}
	return ptrMemoryItem(*m), false, nil
}

// runtime 读取配置；数据库不可用时返回默认值，保证内部接口仍可工作。
func (s *MemoryService) runtime(ctx context.Context) *RuntimeConfig {
	if s.provider == nil {
		return BuildRuntimeConfig(nil)
	}
	cfg, err := s.provider.Runtime(ctx)
	if err != nil || cfg == nil {
		return BuildRuntimeConfig(nil)
	}
	return cfg
}

// scopeIDs 为记忆推导 user_id / dept_id，供 Web 端数据可见范围使用。
func (s *MemoryService) scopeIDs(ctx context.Context, ownerType string, ownerID int64) (*int64, *int64) {
	if ownerID <= 0 {
		return nil, nil
	}
	if ownerType == model.VoiceOwnerUser {
		uid := ownerID
		var deptID *int64
		var row struct {
			DeptID *int64 `gorm:"column:dept_id"`
		}
		if bootstrap.DB() != nil {
			_ = bootstrap.DB().WithContext(ctx).Raw("SELECT dept_id FROM ac_user WHERE id = ?", ownerID).Scan(&row).Error
			deptID = row.DeptID
		}
		return &uid, deptID
	}
	if ownerType == model.VoiceOwnerDept {
		did := ownerID
		return nil, &did
	}
	if ownerType == model.VoiceOwnerDevice {
		var deptID *int64
		var row struct {
			DeptID *int64 `gorm:"column:dept_id"`
		}
		if bootstrap.DB() != nil {
			_ = bootstrap.DB().WithContext(ctx).Raw("SELECT dept_id FROM ac_device WHERE id = ?", ownerID).Scan(&row).Error
			deptID = row.DeptID
		}
		// device 归属设备不写 user_id；有部门则写 dept，便于 Web 侧按部门可见。
		return nil, deptID
	}
	return nil, nil
}

// ── 工具函数 ──────────────────────────────────────────────────

func ownerKey(ownerType string, ownerID int64) string {
	if ownerType == model.VoiceOwnerGlobal {
		return "global"
	}
	return ownerType + ":" + strconv.FormatInt(ownerID, 10)
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func normalizeOwnerKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func memoryItem(m model.VoiceMemory) request.MemoryItem {
	return request.MemoryItem{
		ID:              m.ID,
		OwnerType:       m.OwnerType,
		OwnerID:         m.OwnerID,
		OwnerKey:        m.OwnerKey,
		Kind:            m.Kind,
		Content:         m.Content,
		Importance:      m.Importance,
		EmbeddingStatus: m.EmbeddingStatus,
		CreatedDT:       m.CreatedDT.Format(time.RFC3339),
	}
}

func ptrMemoryItem(m model.VoiceMemory) *request.MemoryItem {
	item := memoryItem(m)
	return &item
}
