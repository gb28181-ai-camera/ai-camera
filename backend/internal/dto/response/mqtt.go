package response

// MqttAuthResp 是 broker 连接鉴权的 RPC 响应。
//
// Allow=false 时 broker 会拒绝连接；Reason 用于 broker 日志与现场排查。
// GroupCode 供 ACL 推导（话题前缀 iot/<group_code>/），broker 不解释其含义。
type MqttAuthResp struct {
	Allow     bool   `json:"allow"`
	GroupCode string `json:"group_code,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// MqttACLResp 是 broker ACL 判定的 RPC 响应。
type MqttACLResp struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}
