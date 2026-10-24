package iot

import (
	"context"
	"crypto/subtle"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// MqttAuthService 处理 MQTT broker 的 RPC 鉴权与 ACL。
//
// 为什么由 IOT 模块承担：MQTT 是「WebRTC 服务组」下的一个服务项，
// 客户端用的就是服务组的组级用户名/密码。放在 IOT 模块里可以复用现有的
// 凭据生成、脱敏与授权模型，不必再维护第二套账号。
//
// 安全原则：**任何异常都返回 allow=false**。broker 侧同样 fail-closed，
// 两层都拒绝，避免「后端 500 → broker 放行」这种最危险的组合。
type MqttAuthService struct {
	groupRepo *repository.IotServiceGroupRepo
}

// NewMqttAuthService 构造服务。
func NewMqttAuthService() *MqttAuthService {
	return &MqttAuthService{groupRepo: repository.NewIotServiceGroupRepo()}
}

// mqttGroup 返回可用的 MQTT 服务组；不可用时返回 nil 与原因。
//
// 判定顺序：存在 → 启用 → 类型为 webrtc → 含启用的 MQTT 服务项。
// 顺序有意从「便宜」到「昂贵」，且原因要能直接写进 broker 日志。
func (s *MqttAuthService) mqttGroup(ctx context.Context, username string) (*model.IotServiceGroup, string) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, "用户名为空"
	}
	g, err := s.groupRepo.FindByUsername(ctx, username)
	if err != nil {
		// 不区分「不存在」与「查询失败」对外文案，避免枚举账号；
		// 但 broker 日志能看到具体原因。
		return nil, "用户名或密码错误"
	}
	if g.Active != nil && !*g.Active {
		return nil, "服务组已停用"
	}
	if g.Type != model.IotServiceGroupWebRTC {
		return nil, "该服务组不提供 MQTT 服务"
	}
	items, err := s.groupRepo.ListItems(ctx, g.ID)
	if err != nil {
		return nil, "服务项查询失败"
	}
	for _, it := range items {
		if it.Protocol != model.IotProtocolMQTT {
			continue
		}
		if it.Active != nil && !*it.Active {
			continue
		}
		return g, ""
	}
	return nil, "服务组未配置可用的 MQTT 服务项"
}

// Authenticate 校验 MQTT 客户端凭据。
func (s *MqttAuthService) Authenticate(ctx context.Context, req *request.MqttAuthReq) (*response.MqttAuthResp, error) {
	if req == nil {
		return &response.MqttAuthResp{Allow: false, Reason: "请求体为空"}, nil
	}
	g, reason := s.mqttGroup(ctx, req.Username)
	if g == nil {
		return &response.MqttAuthResp{Allow: false, Reason: reason}, nil
	}
	// 固定时间比较：普通 == 会因提前返回泄露前缀匹配长度。
	if subtle.ConstantTimeCompare([]byte(g.Password), []byte(req.Password)) != 1 {
		return &response.MqttAuthResp{Allow: false, Reason: "用户名或密码错误"}, nil
	}
	return &response.MqttAuthResp{Allow: true, GroupCode: g.Code}, nil
}

// CheckACL 判定话题读写权限。
//
// 约定：每个 MQTT 服务组只能访问自己命名空间下的话题，
//
//	iot/<group_code>/**   publish / subscribe 均允许
//
// 这样即使多个设备共用一个组凭据，也越不出本组的边界；后续要做设备级隔离时，
// 把 group_code 换成 device_code 即可，broker 侧无需改动。
func (s *MqttAuthService) CheckACL(ctx context.Context, req *request.MqttACLReq) (*response.MqttACLResp, error) {
	if req == nil {
		return &response.MqttACLResp{Allow: false, Reason: "请求体为空"}, nil
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "publish" && action != "subscribe" {
		return &response.MqttACLResp{Allow: false, Reason: "未知的 ACL 动作"}, nil
	}
	g, reason := s.mqttGroup(ctx, req.Username)
	if g == nil {
		return &response.MqttACLResp{Allow: false, Reason: reason}, nil
	}

	topic := strings.TrimSpace(req.Topic)
	prefix := "iot/" + g.Code
	if topic == prefix || strings.HasPrefix(topic, prefix+"/") {
		return &response.MqttACLResp{Allow: true}, nil
	}
	return &response.MqttACLResp{
		Allow:  false,
		Reason: "话题不在本服务组命名空间内（允许 " + prefix + "/**）",
	}, nil
}
