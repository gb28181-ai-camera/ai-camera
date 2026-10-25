package gb28181

// 内存日志检索与实时订阅。
//
// 数据源是 pkg/utils/logger.go 的 ring core —— 历史检索和 SSE 共用同一份
// 内存数据，没有新增日志表。进程重启后历史清空是预期行为（文件日志仍在）。

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// GbLogStreamService 日志检索 / 订阅服务。
type GbLogStreamService struct{}

// NewGbLogStreamService 构造日志服务。
func NewGbLogStreamService() *GbLogStreamService { return &GbLogStreamService{} }

// Page 分页检索内存日志（最新在前）。
func (s *GbLogStreamService) Page(req *request.GbLogQueryReq) (*response.PageGbLogResp, error) {
	if req == nil {
		req = &request.GbLogQueryReq{}
	}
	req.Normalize()
	entries, total := utils.LogHistory(req.Level, req.Keyword, req.Page, req.PageSize)
	items := make([]response.GbLogItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, ToGbLogItem(e))
	}
	return &response.PageGbLogResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Subscribe 订阅实时日志；调用方必须 defer 取消函数。
func (s *GbLogStreamService) Subscribe(buffer int) (<-chan utils.LogEntry, func()) {
	return utils.SubscribeLogs(buffer)
}

// Recent 返回最近 n 条日志（SSE 建连回放）。
func (s *GbLogStreamService) Recent(n int) []response.GbLogItem {
	entries := utils.RecentLogs(n)
	items := make([]response.GbLogItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, ToGbLogItem(e))
	}
	return items
}

// Match 判断日志是否满足当前 SSE 过滤条件。
func (s *GbLogStreamService) Match(e utils.LogEntry, level, keyword string) bool {
	return utils.LogEntryMatches(e, level, keyword)
}

// ToGbLogItem 把内部日志条目转为响应 DTO。
func ToGbLogItem(e utils.LogEntry) response.GbLogItem {
	return response.GbLogItem{
		ID:      e.ID,
		Time:    e.Time,
		Level:   e.Level,
		Logger:  e.Logger,
		Message: e.Message,
		Caller:  e.Caller,
		Fields:  e.Fields,
	}
}
