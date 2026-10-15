package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/gin-gonic/gin"
)

// bodyLogLimit 记录请求体参数的最大字节数，超出部分截断，避免日志表膨胀。
const bodyLogLimit = 4096

// OperationLog 记录增删改操作到 ac_operation_log。
//
// 仅记录 POST / PUT / DELETE / PATCH，GET 不记录。
// 模块名由路径第二段推导（/web/system/users -> system）。
func OperationLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method != http.MethodPost && method != http.MethodPut &&
			method != http.MethodDelete && method != http.MethodPatch {
			c.Next()
			return
		}

		start := time.Now()

		// 读取请求体用于审计后需重新写回，否则下游 handler 读到空 body
		var params string
		if c.Request.Body != nil {
			body, err := io.ReadAll(io.LimitReader(c.Request.Body, bodyLogLimit))
			if err == nil && len(body) > 0 {
				params = string(body)
				c.Request.Body = io.NopCloser(bytes.NewReader(body))
			} else {
				c.Request.Body = io.NopCloser(bytes.NewReader(nil))
			}
		}

		c.Next()

		status := 1
		errMsg := ""
		if c.Writer.Status() >= 400 {
			status = 0
			if len(c.Errors) > 0 {
				errMsg = c.Errors.String()
			} else {
				errMsg = http.StatusText(c.Writer.Status())
			}
		}

		log := model.OperationLog{
			Module:   moduleFromPath(c.Request.URL.Path),
			Action:   actionFromMethod(method),
			Method:   method,
			Path:     c.Request.URL.Path,
			Operator: c.GetString("user_name"),
			IP:       c.ClientIP(),
			Status:   status,
			Params:   params,
			ErrorMsg: errMsg,
			CostMs:   time.Since(start).Milliseconds(),
		}
		if log.Operator == "" {
			log.Operator = "anonymous"
		}

		// 审计写入失败不应影响业务响应
		_ = bootstrap.DB().Create(&log).Error
	}
}

// moduleFromPath 从 /web/system/users/1 中提取 system 作为模块名。
func moduleFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// 期望形态：<prefix>/<module>/... ，前缀为 web 或 app
	if len(parts) >= 3 && (parts[0] == "web" || parts[0] == "app") {
		return parts[1]
	}
	if len(parts) >= 2 {
		return parts[1]
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

// actionFromMethod 把 HTTP 方法映射为中文操作类型。
func actionFromMethod(method string) string {
	switch method {
	case http.MethodPost:
		return "新增"
	case http.MethodPut, http.MethodPatch:
		return "修改"
	case http.MethodDelete:
		return "删除"
	default:
		return method
	}
}
