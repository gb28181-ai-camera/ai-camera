package voiceagent

import (
	"sync"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
)

// 进程级单例。
//
// ticket 的一次性语义依赖共享的 used map，因此 TicketService 必须是单例；
// Gateway/MCP/Memory/Admin 无状态，但复用可避免每次请求重复构造。
// 单副本部署足够；多副本时 ticket 的 used 集合需迁移到 Redis。
var (
	providerOnce sync.Once
	provider     ConfigProvider

	ticketOnce sync.Once
	tickets    *TicketService

	gatewayOnce sync.Once
	gateway     *Gateway

	mcpOnce sync.Once
	mcpSvc  *MCPService

	memoryOnce sync.Once
	memorySvc  *MemoryService

	adminOnce sync.Once
	adminSvc  *AdminService
)

// DefaultConfigProvider 返回配置提供者。
func DefaultConfigProvider() ConfigProvider {
	providerOnce.Do(func() { provider = NewConfigProvider() })
	return provider
}

// DefaultTicketService 返回 ticket 单例。
func DefaultTicketService() *TicketService {
	ticketOnce.Do(func() {
		fallback := ""
		if cfg := bootstrap.Config(); cfg != nil {
			fallback = cfg.JWT.Secret
		}
		tickets = NewTicketService(TicketSecret(fallback), TicketTTL)
	})
	return tickets
}

// DefaultGateway 返回 WS 网关单例。
func DefaultGateway() *Gateway {
	gatewayOnce.Do(func() {
		gateway = NewGateway(DefaultConfigProvider(), DefaultTicketService())
	})
	return gateway
}

// DefaultMCPService 返回 MCP 服务单例。
func DefaultMCPService() *MCPService {
	mcpOnce.Do(func() { mcpSvc = NewMCPService(DefaultConfigProvider()) })
	return mcpSvc
}

// DefaultMemoryService 返回记忆服务单例。
func DefaultMemoryService() *MemoryService {
	memoryOnce.Do(func() { memorySvc = NewMemoryService(DefaultConfigProvider()) })
	return memorySvc
}

// DefaultAdminService 返回管理服务单例。
func DefaultAdminService() *AdminService {
	adminOnce.Do(func() { adminSvc = NewAdminService() })
	return adminSvc
}
