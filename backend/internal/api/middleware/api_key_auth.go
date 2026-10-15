package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ApiKeyAuth 校验 X-Api-Key 请求头。
//
// 适用场景：设备回调、CI 触发等无用户会话的接口。
// 该分组**不经过** Casbin 校验，因此仅应挂载到明确需要的端点上。
func ApiKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-Api-Key")
		if apiKey == "" {
			utils.Unauthorized(c, "缺少API Key")
			c.Abort()
			return
		}

		sum := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(sum[:])

		var key model.ApiKey
		err := bootstrap.DB().
			Where("key_hash = ? AND active = true", keyHash).
			First(&key).Error
		if err != nil {
			utils.Unauthorized(c, "无效的API Key")
			c.Abort()
			return
		}

		// 更新最后使用时间（失败不影响请求）。
		// 注意：必须用 gorm.Expr 包装，直接传字符串 "NOW()" 会被当作字面量写入，
		// PostgreSQL 无法把该字符串转成 timestamp，更新会失败。
		_ = bootstrap.DB().Model(&model.ApiKey{}).
			Where("id = ?", key.ID).
			Update("last_used_dt", gorm.Expr("NOW()")).Error

		c.Set("user_id", key.UserID)
		c.Set("api_key_id", key.ID)
		c.Next()
	}
}

// HashApiKey 计算 API Key 的 SHA256 十六进制摘要。
func HashApiKey(apiKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(apiKey)))
	return hex.EncodeToString(sum[:])
}
