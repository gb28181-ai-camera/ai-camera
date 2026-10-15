package sipua

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// Register 执行一次完整的 GB28181 注册握手：
//
//	设备 ── REGISTER（无凭证） ──▶ 平台
//	设备 ◀── 401 + WWW-Authenticate ── 平台
//	设备 ── REGISTER（带 Authorization，RFC 2617 MD5） ──▶ 平台
//	设备 ◀── 200 OK ── 平台
//
// 若平台未开启鉴权，首个 REGISTER 即得 200，直接返回；若平台 403（密码错 / 设备未建档，§4.2），
// 返回 ErrForbidden —— 这是需求 2「SIP 设备必须是设备管理里有的设备」的验收点。
func (u *UA) Register(ctx context.Context) error {
	req := u.buildRegister(u.cfg.Expires)
	res, err := u.client.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("发送 REGISTER 失败：%w", err)
	}
	u.log.Info("REGISTER 首包响应", "status", res.StatusCode, "reason", res.Reason,
		"call_id", callIDOf(req), "uri", u.recipient.Addr())

	switch res.StatusCode {
	case sip.StatusOK:
		u.markRegistered(res)
		u.log.Info("注册成功（平台未要求 Digest）", "expires_at", u.ExpiresAt().Format(time.RFC3339))
		return nil
	case sip.StatusForbidden:
		u.mu.Lock()
		u.lastResponse = res
		u.mu.Unlock()
		return ErrForbidden
	case sip.StatusUnauthorized:
		// 继续走 Digest 流程
	default:
		u.mu.Lock()
		u.lastResponse = res
		u.mu.Unlock()
		return fmt.Errorf("REGISTER 收到未预期响应 %d %s", res.StatusCode, res.Reason)
	}

	// —— 处理 401 挑战 ——
	wa := res.GetHeader("WWW-Authenticate")
	if wa == nil {
		return fmt.Errorf("401 响应缺少 WWW-Authenticate 头")
	}
	chal, err := ParseChallenge(wa.Value())
	if err != nil {
		return fmt.Errorf("解析 WWW-Authenticate 失败：%w", err)
	}

	auth := &Authorization{
		Username: u.cfg.DeviceID,
		Password: u.cfg.Password,
		Realm:    chal.Realm,
		Nonce:    chal.Nonce,
		Opaque:   chal.Opaque,
		Method:   sip.REGISTER.String(),
		// uri 必须与 Request-URI 完全一致，否则平台侧 HA2 对不上。
		URI: u.recipient.Addr(),
	}
	if chal.QopAuth() {
		auth.Qop = "auth"
		auth.NC = 1
		auth.CNonce = newCNonce()
	}

	// 带凭证重发：复用同一 Call-ID，CSeq +1，Via 必须重新生成（新事务）。
	retry := req.Clone()
	retry.RemoveHeader("Via")
	retry.AppendHeader(sip.NewHeader("Authorization", auth.HeaderValue()))
	res, err = u.client.Do(ctx, retry, sipgo.ClientRequestIncreaseCSEQ, sipgo.ClientRequestAddVia)
	if err != nil {
		return fmt.Errorf("带凭证重发 REGISTER 失败：%w", err)
	}
	u.log.Info("REGISTER 鉴权响应", "status", res.StatusCode, "reason", res.Reason,
		"realm", chal.Realm, "qop", auth.Qop, "call_id", callIDOf(retry))

	switch res.StatusCode {
	case sip.StatusOK:
		u.markRegistered(res)
		u.log.Info("注册成功", "expires_at", u.ExpiresAt().Format(time.RFC3339))
		return nil
	case sip.StatusForbidden:
		u.mu.Lock()
		u.lastResponse = res
		u.mu.Unlock()
		return ErrForbidden
	case sip.StatusUnauthorized:
		u.mu.Lock()
		u.lastResponse = res
		u.mu.Unlock()
		return ErrUnauthorized
	default:
		u.mu.Lock()
		u.lastResponse = res
		u.mu.Unlock()
		return fmt.Errorf("带凭证 REGISTER 收到未预期响应 %d %s", res.StatusCode, res.Reason)
	}
}

// Unregister 发送 Expires: 0 的 REGISTER 注销，让平台立刻把设备置为离线。
// 模拟器收到 SIGINT 时调用；失败不影响退出。
func (u *UA) Unregister(ctx context.Context) error {
	req := u.buildRegister(0)
	res, err := u.client.Do(ctx, req)
	if err != nil {
		return err
	}
	if res.StatusCode == sip.StatusUnauthorized {
		wa := res.GetHeader("WWW-Authenticate")
		if wa == nil {
			return fmt.Errorf("注销 401 缺少 WWW-Authenticate")
		}
		chal, err := ParseChallenge(wa.Value())
		if err != nil {
			return err
		}
		auth := &Authorization{
			Username: u.cfg.DeviceID, Password: u.cfg.Password,
			Realm: chal.Realm, Nonce: chal.Nonce, Opaque: chal.Opaque,
			Method: sip.REGISTER.String(), URI: u.recipient.Addr(),
		}
		if chal.QopAuth() {
			auth.Qop, auth.NC, auth.CNonce = "auth", 1, newCNonce()
		}
		retry := req.Clone()
		retry.RemoveHeader("Via")
		retry.AppendHeader(sip.NewHeader("Authorization", auth.HeaderValue()))
		res, err = u.client.Do(ctx, retry, sipgo.ClientRequestIncreaseCSEQ, sipgo.ClientRequestAddVia)
		if err != nil {
			return err
		}
	}
	u.mu.Lock()
	u.registered = false
	u.mu.Unlock()
	u.log.Info("已发送注销 REGISTER", "status", res.StatusCode)
	return nil
}

// ReRegisterLoop 按有效期的一半周期性地重注册，直到 ctx 取消。
//
// 取值有效期的一半（而非等到过期）是设备侧通行做法：网络抖动或平台重启时留有重试余量。
func (u *UA) ReRegisterLoop(ctx context.Context) {
	period := time.Duration(u.cfg.Expires) * time.Second / 2
	if period <= 0 {
		period = 30 * time.Second
	}
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := u.Register(ctx); err != nil {
				u.log.Error("周期重注册失败", "error", err, "next_retry_in", period.String())
			}
		}
	}
}

// buildRegister 构造 REGISTER 请求。expires 为 0 表示注销。
func (u *UA) buildRegister(expires int) *sip.Request {
	req := u.newRequest(sip.REGISTER, u.recipient)
	// REGISTER 的 To 头是设备自己的 AoR，而非平台域 —— 覆盖 newRequest 的默认值。
	req.ReplaceHeader(&sip.ToHeader{Address: u.aor})

	exp := sip.ExpiresHeader(expires)
	req.AppendHeader(&exp)
	// Contact 上同时带 expires 参数：部分平台只读 Contact 的这一项。
	contact := &sip.ContactHeader{Address: u.contactURI}
	cparams := sip.NewParams()
	contact.Params = cparams.Add("expires", strconv.Itoa(expires))
	req.ReplaceHeader(contact)
	return req
}

// markRegistered 记录注册成功状态与过期时刻。
func (u *UA) markRegistered(res *sip.Response) {
	expires := u.cfg.Expires
	if h := res.GetHeader("Expires"); h != nil {
		if v, err := strconv.Atoi(h.Value()); err == nil && v > 0 {
			expires = v
		}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.registered = true
	u.expiresAt = time.Now().Add(time.Duration(expires) * time.Second)
	u.lastResponse = res
}

// newCNonce 生成客户端随机数（RFC 2617 的 cnonce）。
func newCNonce() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失败极罕见；退化为纳秒时间戳仍然可用。
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
