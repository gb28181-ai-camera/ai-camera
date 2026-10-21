// Package auth 实现 GB28181 信令层的**白名单 + 服务端 Digest 鉴权**。
//
// 两条需求在这里交汇：
//  1. 需求 2「SIP 设备必须是设备管理里有的设备」：收到任何 SIP 请求，先按
//     From 头的用户部分查 ac_device.gb_device_id；查不到（或为空、或已软删）
//     一律 403 Forbidden，并写 ac_gb_sip_log。
//  2. 查得到且 sip_password 非空时走 RFC 2617 Digest：首次 401 + WWW-Authenticate，
//     带凭证则校验 response。
//
// 为什么服务端 Digest 要自研：sipgo 的 DigestAuth 只是 `{Username, Password}` 空结构体，
// 没有服务端能力（那是 emiago/diago 的 API）。约 150 行，比引入完整 VoIP 框架划算。
//
// nonce 是无状态的：`HMAC-SHA256(secret, deviceID|epoch)`。
// 不入库、不入缓存，因此不引入「节点本地缓存」问题（设计 §2.4/§6.2 #4）；
// 校验时接受当前与上一个 epoch，给跨窗口的注册往返留余量。
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo/sip"
	"gorm.io/gorm"
)

// ErrUnknownDevice 表示国标编码不在 ac_device 白名单内（或 gb_device_id 为空）。
var ErrUnknownDevice = errors.New("设备未在设备管理中建档")

// DeviceLookup 是白名单查询所需的最小接口，便于单测替换。
type DeviceLookup interface {
	FindByGbDeviceID(ctx context.Context, gbDeviceID string) (*model.Device, error)
}

// Authenticator 提供白名单查询与 Digest 挑战/校验。
type Authenticator struct {
	lookup   DeviceLookup
	realm    string
	secret   []byte
	nonceTTL time.Duration
	now      func() time.Time
}

// New 构造鉴权器。realm 是缺省鉴权域（设备有 sip_realm 时以设备为准）。
// secret 为空时退化为固定串 —— 仅开发环境可接受。
func New(lookup DeviceLookup, realm, secret string) *Authenticator {
	if strings.TrimSpace(secret) == "" {
		secret = "ai-camera-gb28181-nonce"
	}
	if strings.TrimSpace(realm) == "" {
		realm = "3402000000"
	}
	return &Authenticator{
		lookup:   lookup,
		realm:    realm,
		secret:   []byte(secret),
		nonceTTL: time.Hour,
		now:      time.Now,
	}
}

// SetNow 覆盖时钟（单测用）。
func (a *Authenticator) SetNow(f func() time.Time) {
	if f != nil {
		a.now = f
	}
}

// Lookup 按国标编码查设备（白名单）。
//
// 软删除由 GORM 的 deleted_at 条件自动排除；gb_device_id 为空时直接视为未命中，
// 绝不可退化用 device_code 匹配（两者是不同的 ID 命名空间）。
func (a *Authenticator) Lookup(ctx context.Context, gbDeviceID string) (*model.Device, error) {
	if strings.TrimSpace(gbDeviceID) == "" {
		return nil, ErrUnknownDevice
	}
	dev, err := a.lookup.FindByGbDeviceID(ctx, gbDeviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUnknownDevice
		}
		return nil, fmt.Errorf("查询设备白名单失败：%w", err)
	}
	if strings.TrimSpace(dev.GbDeviceID) == "" {
		return nil, ErrUnknownDevice
	}
	return dev, nil
}

// NeedAuth 判断该设备是否需要 Digest 校验（sip_password 非空）。
//
// 本项目不提供「清空全局公共密码」的开关（设计 §4.3）：密码是每设备独立的，
// 为空意味着该设备尚未配置密码，按「放行 + 记日志」处理。
func (a *Authenticator) NeedAuth(dev *model.Device) bool {
	return dev != nil && strings.TrimSpace(dev.SipPassword) != ""
}

// RealmFor 返回设备应使用的鉴权域。
func (a *Authenticator) RealmFor(dev *model.Device) string {
	if dev != nil && strings.TrimSpace(dev.SipRealm) != "" {
		return strings.TrimSpace(dev.SipRealm)
	}
	return a.realm
}

// Challenge 生成 401 响应用的 WWW-Authenticate 头值。
func (a *Authenticator) Challenge(deviceID, realm string) string {
	if realm == "" {
		realm = a.realm
	}
	return fmt.Sprintf(`Digest realm="%s", nonce="%s", algorithm=MD5, qop="auth"`,
		realm, a.nonceFor(deviceID, a.epoch()))
}

// epoch 当前 nonce 窗口编号。
func (a *Authenticator) epoch() int64 {
	ttl := int64(a.nonceTTL / time.Second)
	if ttl <= 0 {
		ttl = 3600
	}
	return a.now().Unix() / ttl
}

// nonceFor 计算某设备在某窗口的确定性 nonce。
func (a *Authenticator) nonceFor(deviceID string, epoch int64) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(deviceID))
	mac.Write([]byte("|"))
	mac.Write([]byte(strconv.FormatInt(epoch, 10)))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// validateNonce 校验 nonce 是否由本服务端为这台设备签发且未过期。
//
// 接受当前与上一个窗口：设备从收到 401 到带凭证重发可能跨越窗口边界。
// 不做真正的「重放」判定（那需要缓存已用 nc），与 WVP 的无状态实现对齐 ——
// 注册报文的重放没有额外收益，攻击者仍需持有正确密码才能构造摘要。
func (a *Authenticator) validateNonce(deviceID, nonce string) bool {
	if nonce == "" {
		return false
	}
	cur := a.epoch()
	for _, e := range []int64{cur, cur - 1} {
		want := a.nonceFor(deviceID, e)
		if secureEqual(want, nonce) {
			return true
		}
	}
	return false
}

// Verify 校验请求里的 Authorization 摘要。
//
// 支持的两种形式：
//   - qop=auth：response = MD5(HA1:nonce:nc:cnonce:auth:HA2)
//   - 无 qop（RFC 2069）：response = MD5(HA1:nonce:HA2)
//
// 失败时返回带原因的错误，调用方据此写 ac_gb_sip_log.reason。
func (a *Authenticator) Verify(req *sip.Request, dev *model.Device) error {
	if req == nil || dev == nil {
		return fmt.Errorf("鉴权输入为空")
	}
	h := req.GetHeader("Authorization")
	if h == nil {
		return fmt.Errorf("缺少 Authorization 头")
	}
	params, err := ParseAuthorization(h.Value())
	if err != nil {
		return err
	}

	username := params["username"]
	if username == "" {
		return fmt.Errorf("Authorization 缺少 username")
	}
	// 部分设备会带 display-name 或大小写差异，这里严格按国标编码比较。
	if username != dev.GbDeviceID {
		return fmt.Errorf("鉴权用户名 %q 与设备编码 %q 不匹配", username, dev.GbDeviceID)
	}

	realm := a.RealmFor(dev)
	if r := params["realm"]; r != "" && r != realm {
		return fmt.Errorf("鉴权域不匹配：报文 %q，期望 %q", r, realm)
	}

	nonce := params["nonce"]
	if nonce == "" {
		return fmt.Errorf("Authorization 缺少 nonce")
	}
	if !a.validateNonce(dev.GbDeviceID, nonce) {
		return fmt.Errorf("nonce 无效或已过期")
	}

	if alg := params["algorithm"]; alg != "" && !strings.EqualFold(alg, "MD5") {
		return fmt.Errorf("不支持的摘要算法 %q", alg)
	}

	uri := params["uri"]
	if uri == "" {
		uri = req.Recipient.Addr()
	}

	qop := params["qop"]
	nc, cnonce := "", ""
	if qop != "" {
		if !QopAuth(qop) || strings.Contains(strings.ToLower(qop), ",") {
			return fmt.Errorf("不支持的 qop %q（仅支持 auth）", qop)
		}
		nc = params["nc"]
		cnonce = params["cnonce"]
		if nc == "" || cnonce == "" {
			return fmt.Errorf("qop=auth 时缺少 nc 或 cnonce")
		}
		if _, err := strconv.ParseUint(nc, 16, 32); err != nil {
			return fmt.Errorf("nc %q 不是合法的十六进制", nc)
		}
	}

	want := DigestResponse(username, realm, dev.SipPassword, req.Method.String(), uri, nonce, cnonce, qop, nc)
	if !secureEqual(want, params["response"]) {
		return fmt.Errorf("摘要校验失败")
	}
	return nil
}
