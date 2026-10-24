package handler

import (
	"errors"
	"net/http"
	"strconv"

	iotsvc "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - Handler 公共辅助
// ════════════════════════════════════════════════════════════════

// respondIotErr 把 IOT 服务层错误映射为统一响应。
//
// 设备端鉴权错误 → 401；重复激活 → 409；notFound 集合 → 404；其余 → 400。
func respondIotErr(c *gin.Context, err error, notFound ...error) {
	for _, target := range notFound {
		if target != nil && errors.Is(err, target) {
			utils.NotFound(c, err.Error())
			return
		}
	}

	switch {
	case errors.Is(err, iotsvc.ErrDeviceCredentialMissing),
		errors.Is(err, iotsvc.ErrDeviceSecretNotSet),
		errors.Is(err, iotsvc.ErrDeviceSecretInvalid):
		utils.Unauthorized(c, err.Error())
	case errors.Is(err, iotsvc.ErrAlreadyActivated):
		c.JSON(http.StatusConflict, utils.Response{Code: http.StatusConflict, Message: err.Error()})
	default:
		utils.BadRequest(c, err.Error())
	}
}

// iotProductPathID 解析产品 ID。
func iotProductPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的产品ID")
		return 0, false
	}
	return id, true
}

// iotServiceGroupPathID 解析服务组 ID。
func iotServiceGroupPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的服务组ID")
		return 0, false
	}
	return id, true
}

// iotDevicePathID 解析设备 ID。
func iotDevicePathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的设备ID")
		return 0, false
	}
	return id, true
}

// iotAccountPathID 解析设备账号 ID。
func iotAccountPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("accountId"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的账号ID")
		return 0, false
	}
	return id, true
}
