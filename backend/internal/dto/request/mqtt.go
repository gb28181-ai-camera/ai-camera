package request

// MqttAuthReq 是 broker 连接鉴权的 RPC 请求体。
//
// 字段与 deploy/mochi-mqtt/hook/rpc_auth.go 的 AuthRequest 一一对应；
// 两端改字段必须同步，否则 MQTT 连接会因 JSON 解析失败被 fail-closed。
type MqttAuthReq struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	ClientID   string `json:"client_id"`
	RemoteAddr string `json:"remote_addr"`
}

// MqttACLReq 是 broker 发布/订阅 ACL 的 RPC 请求体。
type MqttACLReq struct {
	Username string `json:"username"`
	ClientID string `json:"client_id"`
	Topic    string `json:"topic"`
	Action   string `json:"action"` // publish / subscribe
}
