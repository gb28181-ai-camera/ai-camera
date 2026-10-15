package utils

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应体：{code, message, data}。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResult 通用分页载荷。
//
// 注意：业务代码请使用 internal/dto/response 中类型化的 PageXxxResp，
// 本类型仅用于通用工具场景（如导出、内部调用）。
type PageResult struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    interface{} `json:"items"`
}

// ParseJSON 解析 JSON 字符串为 map，空串或非法 JSON 返回 nil。
func ParseJSON(s string) map[string]interface{} {
	if s == "" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// Success 返回 200，body 为 {code:200, data}。
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 200, Data: data})
}

// Created 返回 201，body 的 code 仍为 200（前端以 code 判断成功）。
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{Code: 200, Data: data})
}

// BadRequest 返回 400。
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{Code: 400, Message: message})
}

// Unauthorized 返回 401。
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{Code: 401, Message: message})
}

// Forbidden 返回 403。
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, Response{Code: 403, Message: message})
}

// NotFound 返回 404。
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, Response{Code: 404, Message: message})
}

// ServerError 返回 500。
func ServerError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{Code: 500, Message: message})
}
