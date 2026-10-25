package handler

import (
	"errors"
	"net/http"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 通道免登录分享
//
// 管理端接口挂在 /web/gb/channels/:id/share-tokens（JWT + Casbin）；
// 匿名接口挂在 /public/gb/shares/:token（不挂 authMW / casbinMW），
// 令牌独立校验，绝不复用用户 JWT。
// ════════════════════════════════════════════════════════════════

// GbShareHandler 分享令牌处理器。
type GbShareHandler struct {
	svc *gbsvc.GbShareService
}

// NewGbShareHandler 构造分享处理器。
func NewGbShareHandler() *GbShareHandler {
	return &GbShareHandler{svc: gbsvc.NewGbShareService()}
}

// CreateShareToken 生成分享令牌
//
// @Summary      生成通道分享令牌
// @Description  生成高熵令牌，明文只在本次响应中返回；库中仅存 SHA-256。TTL 默认 3600 秒，最长 86400 秒
// @Tags         GbShare
// @Accept       json
// @Produce      json
// @Param        id    path      int                             true  "通道ID"
// @Param        body  body      request.GbShareTokenCreateReq   false "令牌参数"
// @Success      200  {object}  response.RGbShareTokenCreated
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id}/share-tokens [post]
// @ID           createGbShareToken
func (h *GbShareHandler) CreateShareToken(c *gin.Context) {
	channelID, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	var req request.GbShareTokenCreateReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	resp, err := h.svc.Create(c.Request.Context(), channelID, &req)
	if err != nil {
		respondShareAdminErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// ListShareTokens 分享令牌列表
//
// @Summary      分享令牌列表
// @Description  只返回未过期且未吊销的令牌（不含明文与哈希）
// @Tags         GbShare
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbShareTokenList
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id}/share-tokens [get]
// @ID           listGbShareTokens
func (h *GbShareHandler) ListShareTokens(c *gin.Context) {
	channelID, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	items, err := h.svc.List(c.Request.Context(), channelID, utils.NewDataScope(c))
	if err != nil {
		respondShareAdminErr(c, err)
		return
	}
	utils.Success(c, items)
}

// RevokeShareToken 吊销分享令牌
//
// @Summary      吊销分享令牌
// @Description  吊销后该令牌立即失效；重复吊销幂等成功
// @Tags         GbShare
// @Accept       json
// @Produce      json
// @Param        id       path      int  true  "通道ID"
// @Param        tokenId  path      int  true  "令牌ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id}/share-tokens/{tokenId} [delete]
// @ID           revokeGbShareToken
func (h *GbShareHandler) RevokeShareToken(c *gin.Context) {
	channelID, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	tokenID, ok := parsePathID(c, "tokenId", "分享令牌")
	if !ok {
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), channelID, tokenID); err != nil {
		respondShareAdminErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// PublicShareInfo 匿名分享信息
//
// @Summary      匿名分享信息
// @Description  不需要 JWT；按令牌 hash 校验，返回通道名称、过期时间与可用协议
// @Tags         GbShare
// @Accept       json
// @Produce      json
// @Param        token  path      string  true  "分享令牌"
// @Success      200  {object}  response.RGbShareInfo
// @Failure      401  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /public/gb/shares/{token} [get]
// @ID           getPublicGbShareInfo
func (h *GbShareHandler) PublicShareInfo(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	resp, err := h.svc.Info(c.Request.Context(), c.Param("token"))
	if err != nil {
		respondSharePublicErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// PublicSharePlay 匿名点播
//
// @Summary      匿名分享点播
// @Description  不需要 JWT；协议仅支持 webrtc/flv/hls，并对令牌+IP 与令牌双层限频
// @Tags         GbShare
// @Accept       json
// @Produce      json
// @Param        token  path      string                  true  "分享令牌"
// @Param        body   body      request.GbSharePlayReq  false "播放参数"
// @Success      200  {object}  response.RGbSharePlay
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      429  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /public/gb/shares/{token}/play [post]
// @ID           playPublicGbShare
func (h *GbShareHandler) PublicSharePlay(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	var req request.GbSharePlayReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	resp, err := h.svc.Play(c.Request.Context(), c.Param("token"), c.ClientIP(), req.Protocol)
	if err != nil {
		respondSharePublicErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// respondShareAdminErr 管理端错误映射。
func respondShareAdminErr(c *gin.Context, err error) {
	if errors.Is(err, gbsvc.ErrGbShareChannelNotFound) || errors.Is(err, gbsvc.ErrGbShareTokenInvalid) {
		utils.NotFound(c, err.Error())
		return
	}
	utils.BadRequest(c, err.Error())
}

// respondSharePublicErr 匿名端错误映射。
//
// 令牌无效/过期/吊销统一回 401 且消息相同：避免匿名访问者通过响应码
// 探测「令牌是否存在」「是否曾经有效」。
func respondSharePublicErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gbsvc.ErrGbShareTokenInvalid),
		errors.Is(err, gbsvc.ErrGbShareTokenExpired),
		errors.Is(err, gbsvc.ErrGbShareTokenRevoked):
		utils.Unauthorized(c, gbsvc.ErrGbShareTokenInvalid.Error())
	case errors.Is(err, gbsvc.ErrGbShareChannelNotFound):
		utils.NotFound(c, err.Error())
	case errors.Is(err, gbsvc.ErrGbShareProtocolInvalid):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, gbsvc.ErrGbShareRateLimited):
		c.JSON(http.StatusTooManyRequests, utils.Response{Code: http.StatusTooManyRequests, Message: err.Error()})
	case errors.Is(err, gbsvc.ErrSipServiceUnavailable):
		c.JSON(http.StatusServiceUnavailable, utils.Response{Code: http.StatusServiceUnavailable, Message: err.Error()})
	default:
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound)
	}
}
