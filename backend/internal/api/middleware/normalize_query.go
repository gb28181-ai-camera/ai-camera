package middleware

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// NormalizeEmptyQuery 剔除查询串中「仅有空值」的参数。
//
// 背景：前端生成的 Controller 按 boticz-mes 的模板拼接查询串，对未填写的筛选项
// 一律发 `keyword=`、`active=`、`dept_id=` 这样的空参数（而非省略该键）。
// 后端把这些空串当作**有效过滤条件**时会出现两类错误：
//
//   - `*bool` 字段：`active=` 被绑定成「非 nil 且值为 false」，
//     于是 `WHERE active = false` —— 用户/角色/岗位/通知列表**恒为 0 条**；
//   - `*int64`/数值字段：`dept_id=` 解析为 0 后被当作有效 ID 过滤。
//
// 在绑定之前统一删除空值参数，一处修复覆盖全部列表接口，且对任何客户端都成立
// （前端即使继续发空参数也不会误过滤）。
//
// 注意：仅删除**值为空串**的参数；`?active=false` 这类显式传值不受影响。
func NormalizeEmptyQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Request.URL.RawQuery
		if raw == "" {
			c.Next()
			return
		}

		values, err := url.ParseQuery(raw)
		if err != nil {
			// 查询串本身无法解析时不动它，交给绑定期报错
			c.Next()
			return
		}

		changed := false
		for key, vals := range values {
			// 全部取值都是空白串 => 视为「未提供该筛选项」
			allEmpty := true
			for _, v := range vals {
				if strings.TrimSpace(v) != "" {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				delete(values, key)
				changed = true
			}
		}

		if changed {
			c.Request.URL.RawQuery = values.Encode()
		}

		c.Next()
	}
}
