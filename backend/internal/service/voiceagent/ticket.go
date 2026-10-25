package voiceagent

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

// Ticket 错误。
var (
	ErrTicketInvalid  = errors.New("ticket 无效")
	ErrTicketExpired  = errors.New("ticket 已过期")
	ErrTicketUsed     = errors.New("ticket 已被使用")
	ErrTicketKind     = errors.New("ticket 类型不匹配")
	ErrTicketNotReady = errors.New("ticket 服务未初始化")
)

// TicketClaims ticket 载荷（服务端签发/校验）。
type TicketClaims struct {
	UserID       int64  `json:"uid"`
	UserName     string `json:"un,omitempty"`
	RoleCode     string `json:"rc,omitempty"`
	DeptID       int64  `json:"did,omitempty"`
	DataPerm     string `json:"dp,omitempty"`
	CrossDeptIDs string `json:"cd,omitempty"`
	DeviceID     int64  `json:"dev,omitempty"`
	ClientID     string `json:"cid,omitempty"`
	Kind         string `json:"kind"`
	AgentCode    string `json:"agent,omitempty"`
	ExpiresAt    int64  `json:"exp"`
	JTI          string `json:"jti"`
}

type ticketPayload struct {
	UID   int64  `json:"uid"`
	UN    string `json:"un,omitempty"`
	RC    string `json:"rc,omitempty"`
	DID   int64  `json:"did,omitempty"`
	DP    string `json:"dp,omitempty"`
	CD    string `json:"cd,omitempty"`
	DevID int64  `json:"dev,omitempty"`
	CID   string `json:"cid,omitempty"`
	Kind  string `json:"kind"`
	Agent string `json:"agent,omitempty"`
	Exp   int64  `json:"exp"`
	JTI   string `json:"jti"`
}

// TicketService 签发与校验一次性 WS ticket。
//
// 一次性语义依赖进程内 map：
//   - 单副本部署（本项目默认）足够；
//   - 多副本部署时必须把 used JTI 放入 Redis（SETNX + TTL），
//     否则同一个 ticket 可在不同副本各用一次。
type TicketService struct {
	secret []byte
	ttl    time.Duration

	mu   sync.Mutex
	used map[string]int64 // jti → exp（Unix 秒）

	now func() time.Time
}

// NewTicketService 构造 ticket 服务。secret 为空时拒绝签发。
func NewTicketService(secret string, ttl time.Duration) *TicketService {
	if ttl <= 0 {
		ttl = TicketTTL
	}
	return &TicketService{
		secret: []byte(secret),
		ttl:    ttl,
		used:   make(map[string]int64),
		now:    time.Now,
	}
}

// TTL 返回 ticket 有效期。
func (s *TicketService) TTL() time.Duration { return s.ttl }

// Issue 签发 ticket。
func (s *TicketService) Issue(c TicketClaims) (string, *TicketClaims, error) {
	if len(s.secret) == 0 {
		return "", nil, errors.New("ticket 密钥未配置")
	}
	if strings.TrimSpace(c.Kind) == "" {
		return "", nil, errors.New("ticket kind 不能为空")
	}
	now := s.now()
	exp := c.ExpiresAt
	if exp <= 0 {
		exp = now.Add(s.ttl).Unix()
	}
	jti, err := randomHex(16)
	if err != nil {
		return "", nil, err
	}
	c.ExpiresAt = exp
	c.JTI = jti

	p := ticketPayload{
		UID: c.UserID, UN: c.UserName, RC: c.RoleCode, DID: c.DeptID,
		DP: c.DataPerm, CD: c.CrossDeptIDs, DevID: c.DeviceID, CID: c.ClientID,
		Kind: c.Kind, Agent: c.AgentCode, Exp: exp, JTI: jti,
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", nil, err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	token := body + "." + s.sign(body)
	return token, &c, nil
}

// Verify 校验 ticket 并原子消费（一次性）。
func (s *TicketService) Verify(kind, token string) (*TicketClaims, error) {
	if len(s.secret) == 0 {
		return nil, ErrTicketNotReady
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, ErrTicketInvalid
	}
	expected := s.sign(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return nil, ErrTicketInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrTicketInvalid
	}
	var p ticketPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, ErrTicketInvalid
	}
	now := s.now().Unix()
	if p.Exp <= 0 || now > p.Exp {
		return nil, ErrTicketExpired
	}
	if kind != "" && p.Kind != kind {
		return nil, ErrTicketKind
	}

	// 一次性：先查后删必须在同一把锁内。
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	if _, ok := s.used[p.JTI]; ok {
		return nil, ErrTicketUsed
	}
	s.used[p.JTI] = p.Exp

	return &TicketClaims{
		UserID: p.UID, UserName: p.UN, RoleCode: p.RC, DeptID: p.DID,
		DataPerm: p.DP, CrossDeptIDs: p.CD, DeviceID: p.DevID, ClientID: p.CID,
		Kind: p.Kind, AgentCode: p.Agent, ExpiresAt: p.Exp, JTI: p.JTI,
	}, nil
}

// UsedCount 返回当前进程内已消费的 ticket 数（观测/测试用）。
func (s *TicketService) UsedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.used)
}

// cleanupLocked 清理过期 JTI。ticket 生命周期只有 60s，map 不会无限增长。
func (s *TicketService) cleanupLocked(now int64) {
	for k, exp := range s.used {
		if exp+60 < now {
			delete(s.used, k)
		}
	}
}

func (s *TicketService) sign(body string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// randomHex 生成 n 字节随机数的十六进制串。
func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
