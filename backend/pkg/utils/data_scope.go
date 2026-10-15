package utils

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DataScope 描述当前请求的行级数据权限。
//
//	data_perm 取值：
//	  all          — 全部数据（超级管理员）
//	  cross_dept   — 本部门 + 指定跨部门数据
//	  dept         — 仅本部门数据
//	  personal     — 仅本人创建的数据
type DataScope struct {
	UserID       int64
	UserName     string
	DataPerm     string
	DeptID       *int64
	CrossDeptIDs string
}

// NewDataScope 从 gin.Context 构造 DataScope（Auth 中间件已写入相关键）。
func NewDataScope(c *gin.Context) *DataScope {
	ds := &DataScope{
		UserID:       c.GetInt64("user_id"),
		UserName:     c.GetString("user_name"),
		DataPerm:     c.GetString("data_perm"),
		CrossDeptIDs: c.GetString("cross_dept_ids"),
	}
	if deptID := c.GetInt64("dept_id"); deptID > 0 {
		ds.DeptID = &deptID
	}
	if ds.DataPerm == "" {
		ds.DataPerm = "personal"
	}
	return ds
}

// GetDeptID 返回当前请求的部门 ID（无则 nil）。
func GetDeptID(c *gin.Context) *int64 {
	if deptID := c.GetInt64("dept_id"); deptID > 0 {
		return &deptID
	}
	return nil
}

// ApplyToQuery 依据数据权限为查询附加过滤条件。
// deptCol 必须是带表别名的限定列名，例如 "d.dept_id"；
// 对应的 created_by 列由前缀推导（"d.dept_id" -> "d.created_by"）。
func (ds *DataScope) ApplyToQuery(q *gorm.DB, deptCol string) *gorm.DB {
	createdByCol := "created_by"
	if idx := strings.LastIndex(deptCol, "."); idx >= 0 {
		createdByCol = deptCol[:idx+1] + "created_by"
	}

	switch ds.DataPerm {
	case "all":
		return q

	case "cross_dept":
		ids := ParseIDs(ds.CrossDeptIDs)
		if ds.DeptID != nil {
			ids = append(ids, *ds.DeptID)
		}
		if len(ids) == 0 {
			return q.Where(createdByCol+" = ?", ds.UserName)
		}
		return q.Where(deptCol+" IN ?", dedup(ids))

	case "dept":
		if ds.DeptID == nil {
			return q.Where(createdByCol+" = ?", ds.UserName)
		}
		return q.Where(deptCol+" = ?", *ds.DeptID)

	default: // personal
		if ds.DeptID == nil {
			return q.Where(createdByCol+" = ?", ds.UserName)
		}
		return q.Where(deptCol+" = ? AND "+createdByCol+" = ?", *ds.DeptID, ds.UserName)
	}
}

// ParseIDs 解析逗号分隔的 ID 列表，忽略非法项。
func ParseIDs(s string) []int64 {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int64
		var ok = true
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				ok = false
				break
			}
			n = n*10 + int64(ch-'0')
		}
		if ok {
			out = append(out, n)
		}
	}
	return out
}

func dedup(ids []int64) []int64 {
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
