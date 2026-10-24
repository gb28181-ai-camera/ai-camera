// Package mqttrpc 实现 mochi-mqtt（内部 fork）的 gRPC 鉴权服务。
//
// 为什么是 gRPC 而不是 HTTP：
// 内部 fork 的 `hooks/auth/rpc.go` 已经写死了 gRPC 客户端
// （`IotMqttAuthService.MqttAuth`），broker 配置里只有一个 `rpc.address`。
// 要去掉「再改一遍 broker」的成本，后端就按同一个 proto 提供 gRPC 服务。
//
// 协议来自 fork 的 `hooks/auth/utils/rpc/IotMqttAuth.proto`：
//
//	service IotMqttAuthService {
//	  rpc MqttAuth(MqttAuthRequest) returns (MqttAuthResponse);
//	}
//	message MqttAuthRequest  { string username = 1; string password = 2; }
//	message MqttAuthResponse { int32 code = 1; string msg = 2; bool data = 3; }
//
// 约定：code=200 且 data=true 表示放行；其余一律视为拒绝。
//
// ⚠️ fork 的 `OnACLCheck` 目前直接返回 true，即**只鉴权、不做话题隔离**。
// 若需要按服务组隔离话题，必须在 fork 里把 ACL 也改成 RPC 调用；后端已
// 提供 `MqttAuthService.CheckACL` 可直接复用（见本包的报告/文档说明）。
package mqttrpc

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	proto "github.com/boticz/ai-camera-backend/internal/mqttrpc/proto"
	"github.com/boticz/ai-camera-backend/internal/service/iot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server 是 gRPC 鉴权服务。
type Server struct {
	proto.UnimplementedIotMqttAuthServiceServer

	auth   *iot.MqttAuthService
	logger *slog.Logger
	grpc   *grpc.Server
	lis    net.Listener
}

// New 构造服务（不监听）。
func New(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		auth:   iot.NewMqttAuthService(),
		logger: logger,
	}
}

// MqttAuth 是 fork 调用的唯一方法。
func (s *Server) MqttAuth(ctx context.Context, req *proto.MqttAuthRequest) (*proto.MqttAuthResponse, error) {
	if req == nil {
		return &proto.MqttAuthResponse{Code: 400, Msg: "请求为空", Data: false}, nil
	}
	resp, err := s.auth.Authenticate(ctx, &request.MqttAuthReq{
		Username: req.GetUsername(),
		Password: req.GetPassword(),
	})
	if err != nil {
		// 服务层理论上不返回 error；真出现异常也按拒绝处理。
		s.logger.Error("MQTT gRPC 鉴权内部错误", "error", err)
		return &proto.MqttAuthResponse{Code: 500, Msg: "内部错误", Data: false}, nil
	}
	if !resp.Allow {
		// 400 而不是 401：fork 只看 Code 是否 200，这里保持「非 200 即拒绝」。
		return &proto.MqttAuthResponse{Code: 401, Msg: resp.Reason, Data: false}, nil
	}
	return &proto.MqttAuthResponse{Code: 200, Msg: "ok", Data: true}, nil
}

// Start 监听并服务。空地址时返回错误（不做隐式默认，避免误开端口）。
func (s *Server) Start(addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return status.Error(codes.InvalidArgument, "MQTT gRPC 监听地址为空")
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.lis = lis
	s.grpc = grpc.NewServer()

	// 内部网络使用明文；若 broker 与后端跨主机，应在两者之间加 mTLS 或隧道。
	proto.RegisterIotMqttAuthServiceServer(s.grpc, s)
	s.logger.Info("MQTT gRPC 鉴权服务已启动", "addr", addr)
	go func() {
		if err := s.grpc.Serve(lis); err != nil {
			s.logger.Error("MQTT gRPC 服务退出", "error", err)
		}
	}()
	return nil
}

// Stop 优雅停止。
func (s *Server) Stop() {
	if s.grpc == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		s.grpc.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		s.grpc.Stop()
	}
	if s.lis != nil {
		_ = s.lis.Close()
	}
}
