package iot

import (
	"context"
	"crypto/subtle"
	"os"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// 平台内部账号。
//
// 后端信令 worker 需要以 MQTT 客户端身份订阅 App 的 offer 并发布 answer。
// 它不属于任何 IOT 服务组，因此使用固定用户名 + 共享 token：
//   - 用户名：__platform__（普通服务组不可能与之重名，因为 username 来自 ac_iot_service_group）
//   - 密码：环境变量 MQTT_RPC_TOKEN（与 broker 的 X-RPC-Token 同一个值）
//
// 权限被刻意收窄到两条信令话题，避免这个高权限账号被误用为通用数据通道。
const (
	PlatformUsername  = "__platform__"
	PlatformGroupCode = "__platform__"

	platformSubscribeTopic = "iot/+/signal/backend"
	platformPublishTopic   = "iot/+/app/+/signal"
)

// MqttAuthService 处理 MQTT broker 的 RPC 鉴权与 ACL。
//
// MQTT 是「WebRTC 服务组」下的一个服务；客户端使用的是
// **产品 × MQTT 服务** 维度的用户名/密码（ac_iot_product_service），
// 因此同一服务组下不同产品各有独立账号。
//
// 安全原则：**任何异常都返回 allow=false**。broker 侧同样 fail-closed，
// 两层都拒绝，避免「后端 500 → broker 放行」这种最危险的组合。
type MqttAuthService struct {
	groupRepo      *repository.IotServiceGroupRepo
	productRepo    *repository.IotProductRepo
	credentialRepo *repository.IotProductServiceRepo
}

// NewMqttAuthService 构造服务。
func NewMqttAuthService() *MqttAuthService {
	return &MqttAuthService{
		groupRepo:      repository.NewIotServiceGroupRepo(),
		productRepo:    repository.NewIotProductRepo(),
		credentialRepo: repository.NewIotProductServiceRepo(),
	}
}

// mqttContext 返回可用的 MQTT 产品服务；不可用时返回 nil 与原因。
//
// 判定顺序：产品服务凭据存在 → 启用 → 服务为 MQTT 且启用 →
// 服务组启用且类型为 webrtc → 产品启用。
func (s *MqttAuthService) mqttContext(ctx context.Context, username string) (*model.IotProductService, *model.IotService, *model.IotServiceGroup, string) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, nil, nil, "用户名为空"
	}
	row, err := s.credentialRepo.FindByUsername(ctx, username)
	if err != nil {
		// 不区分「不存在」与「查询失败」对外文案，避免枚举账号。
		return nil, nil, nil, "用户名或密码错误"
	}
	if row.Active != nil && !*row.Active {
		return nil, nil, nil, "产品服务凭据已停用"
	}
	svc, err := s.groupRepo.GetService(ctx, row.ServiceID)
	if err != nil {
		return nil, nil, nil, "服务不存在"
	}
	if svc.Protocol != model.IotProtocolMQTT || (svc.Active != nil && !*svc.Active) {
		return nil, nil, nil, "该服务不提供 MQTT 接入"
	}
	group, err := s.groupRepo.GetByID(ctx, svc.GroupID)
	if err != nil {
		return nil, nil, nil, "服务组不存在"
	}
	if group.Active != nil && !*group.Active {
		return nil, nil, nil, "服务组已停用"
	}
	if group.Type != model.IotServiceGroupWebRTC {
		return nil, nil, nil, "该服务组不提供 MQTT 服务"
	}
	product, err := s.productRepo.GetByID(ctx, row.ProductID)
	if err != nil {
		return nil, nil, nil, "产品不存在"
	}
	if product.Active != nil && !*product.Active {
		return nil, nil, nil, "产品已停用"
	}
	return row, svc, group, ""
}

// Authenticate 校验 MQTT 客户端凭据。
func (s *MqttAuthService) Authenticate(ctx context.Context, req *request.MqttAuthReq) (*response.MqttAuthResp, error) {
	if req == nil {
		return &response.MqttAuthResp{Allow: false, Reason: "请求体为空"}, nil
	}
	// 内部账号必须在查库之前识别。
	if strings.TrimSpace(req.Username) == PlatformUsername {
		return authenticatePlatform(req), nil
	}
	row, _, group, reason := s.mqttContext(ctx, req.Username)
	if row == nil {
		return &response.MqttAuthResp{Allow: false, Reason: reason}, nil
	}
	// 固定时间比较：普通 == 会因提前返回泄露前缀匹配长度。
	if subtle.ConstantTimeCompare([]byte(row.Password), []byte(req.Password)) != 1 {
		return &response.MqttAuthResp{Allow: false, Reason: "用户名或密码错误"}, nil
	}
	return &response.MqttAuthResp{Allow: true, GroupCode: group.Code}, nil
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
	if strings.TrimSpace(req.Username) == PlatformUsername {
		return checkPlatformACL(action, req.Topic), nil
	}
	_, _, group, reason := s.mqttContext(ctx, req.Username)
	if group == nil {
		return &response.MqttACLResp{Allow: false, Reason: reason}, nil
	}

	topic := strings.TrimSpace(req.Topic)
	prefix := "iot/" + group.Code
	if topic == prefix || strings.HasPrefix(topic, prefix+"/") {
		return &response.MqttACLResp{Allow: true}, nil
	}
	return &response.MqttACLResp{
		Allow:  false,
		Reason: "话题不在本服务组命名空间内（允许 " + prefix + "/**）",
	}, nil
}

// authenticatePlatform 校验内部账号 __platform__。
//
// 密码来自 MQTT_RPC_TOKEN，必须 constant-time 比较。token 未配置时一律拒绝
// （fail-closed）：否则一个空的 MQTT_RPC_TOKEN 会让空密码通过。
func authenticatePlatform(req *request.MqttAuthReq) *response.MqttAuthResp {
	token := strings.TrimSpace(os.Getenv("MQTT_RPC_TOKEN"))
	if token == "" {
		return &response.MqttAuthResp{Allow: false, Reason: "内部账号未配置 MQTT_RPC_TOKEN"}
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(req.Password)) != 1 {
		return &response.MqttAuthResp{Allow: false, Reason: "用户名或密码错误"}
	}
	return &response.MqttAuthResp{Allow: true, GroupCode: PlatformGroupCode}
}

// checkPlatformACL 校验内部账号的话题权限。
//
// 只允许两条信令话题：
//   - subscribe iot/+/signal/backend
//   - publish   iot/+/app/+/signal
//
// 订阅侧允许「精确的通配符过滤器」与「具体组的话题」；发布侧禁止任何通配符，
// 防止内部账号向未指定客户端广播。
func checkPlatformACL(action, topic string) *response.MqttACLResp {
	if strings.TrimSpace(os.Getenv("MQTT_RPC_TOKEN")) == "" {
		return &response.MqttACLResp{Allow: false, Reason: "内部账号未配置 MQTT_RPC_TOKEN"}
	}

	topic = strings.TrimSpace(topic)
	switch action {
	case "subscribe":
		if topic == platformSubscribeTopic {
			return &response.MqttACLResp{Allow: true}
		}
		if strings.ContainsAny(topic, "+#") {
			return &response.MqttACLResp{Allow: false, Reason: "内部账号只允许订阅 " + platformSubscribeTopic}
		}
		if matchMQTTTopic(platformSubscribeTopic, topic) {
			return &response.MqttACLResp{Allow: true}
		}
	case "publish":
		if !strings.ContainsAny(topic, "+#") && matchMQTTTopic(platformPublishTopic, topic) {
			return &response.MqttACLResp{Allow: true}
		}
		return &response.MqttACLResp{Allow: false, Reason: "内部账号只允许发布 " + platformPublishTopic}
	}
	return &response.MqttACLResp{Allow: false, Reason: "内部账号无权访问该话题"}
}

// matchMQTTTopic 判断具体话题是否匹配只含 + 单层通配符的过滤器。
//
// 只实现 +（本项目的话题契约不需要 #）：层级数必须相同，+ 匹配任意一层。
func matchMQTTTopic(filter, topic string) bool {
	f := strings.Split(filter, "/")
	t := strings.Split(topic, "/")
	if len(f) != len(t) {
		return false
	}
	for i := range f {
		if f[i] == "+" {
			continue
		}
		if f[i] != t[i] {
			return false
		}
	}
	return true
}
