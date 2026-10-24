package request

// TurnCredentialReq 是内部 RPC 签发 TURN 凭据的请求体。
type TurnCredentialReq struct {
	// Subject 是凭据主体（用户 ID / 设备编码 / 服务组编码），
	// 会出现在 coturn 的 username 与日志里。
	Subject string `json:"subject"`
	// TTL 是有效期（秒），可选；服务端会限制在 60–86400。
	TTL int `json:"ttl,omitempty"`
}
