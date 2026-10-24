// Package session 管理平台发起的 SIP 会话（INVITE 建立的点播/回放/对讲会话）。
//
// 为什么需要这一层：sipgo 的 DialogClientSession 只负责 RFC 3261 的对话框状态机，
// 而业务侧需要把 Call-ID 与「设备/通道/SSRC/ZLM 流名」关联起来 —— 停止播放、
// 回放控制（带内 INFO）、语音对讲回推都要靠这些信息找回同一路会话。
//
// 会话状态只放内存：一期 SIP 固定单节点（设计 §2.4），跨进程共享会话状态
// 需要 Redis RPC，不属于本期范围。
package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// Dialog 是一次已建立的 SIP 会话。
type Dialog struct {
	CallID      string // 贯穿整条媒体链路的关联键
	FromTag     string
	ToTag       string
	DeviceID    string // 设备国标编码
	ChannelID   string // 通道国标编码（INVITE 的 Request-URI 用户部分）
	RemoteAddr  string // 设备信令地址 host:port
	SSRC        string // 平台分配的 10 位 SSRC
	App         string // ZLM app，恒为 rtp
	Stream      string // ZLM 流名：%08X(ssrc) 大写
	SessionName string // SDP s=：Play / Playback / Download
	// Transport 本次协商的媒体传输（udp / tcp-passive / tcp-active）。
	// 排障时用它区分「INVITE 成功但设备往错的方向推」：UDP 与 TCP 的
	// 收流路径完全不同，日志里不带这一项只能靠翻 SDP 正文猜。
	Transport string
	DeviceSDP string // 设备 200 OK 里的 SDP 原文

	sess *sipgo.DialogClientSession

	mu       sync.Mutex
	closed   bool
	onClose  func(*Dialog)
	createDT time.Time
}

// New 包装 sipgo 的客户端对话框。
func New(callID, fromTag, toTag, deviceID, channelID, remoteAddr, ssrc, stream, sessionName, transport, sdp string, sess *sipgo.DialogClientSession) *Dialog {
	return &Dialog{
		CallID:      callID,
		FromTag:     fromTag,
		ToTag:       toTag,
		DeviceID:    deviceID,
		ChannelID:   channelID,
		RemoteAddr:  remoteAddr,
		SSRC:        ssrc,
		App:         "rtp",
		Stream:      stream,
		SessionName: sessionName,
		Transport:   transport,
		DeviceSDP:   sdp,
		sess:        sess,
		createDT:    time.Now(),
	}
}

// CreatedAt 返回会话建立时间。
func (d *Dialog) CreatedAt() time.Time { return d.createDT }

// SetOnClose 注册关闭回调（由管理器用来清理索引）。
func (d *Dialog) SetOnClose(f func(*Dialog)) {
	d.mu.Lock()
	d.onClose = f
	d.mu.Unlock()
}

// Closed 返回会话是否已结束。
func (d *Dialog) Closed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}

// Bye 平台主动结束会话。设备回 200 后返回；超时返回 error。
func (d *Dialog) Bye(ctx context.Context) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	sess := d.sess
	d.mu.Unlock()

	if sess == nil {
		return fmt.Errorf("会话 %s 没有底层对话框", d.CallID)
	}
	err := sess.Bye(ctx)
	d.markClosed()
	return err
}

// AcceptBye 处理设备主动发起的 BYE：回 200 OK 并结束会话。
func (d *Dialog) AcceptBye(req *sip.Request, tx sip.ServerTransaction) error {
	d.mu.Lock()
	sess := d.sess
	d.mu.Unlock()
	var err error
	if sess != nil {
		err = sess.ReadBye(req, tx)
	} else {
		err = tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil))
	}
	d.markClosed()
	return err
}

// Info 在对话框内发送 INFO（回放暂停/恢复/拖动/倍速走这里，设计 §2.5）。
//
// 注意与云台的 MESSAGE 区分：云台用带外 MESSAGE，回放控制用带内 INFO，
// 两者走的是完全不同的发送路径，搞反会「按了没反应但也不报错」。
func (d *Dialog) Info(ctx context.Context, body, contentType string) error {
	_, err := d.Request(ctx, sip.INFO, []byte(body), contentType)
	return err
}

// Request 在对话框内发送任意请求（From/To/Call-ID/CSeq 由 sipgo 补齐）。
func (d *Dialog) Request(ctx context.Context, method sip.RequestMethod, body []byte, contentType string) (*sip.Response, error) {
	d.mu.Lock()
	sess := d.sess
	closed := d.closed
	d.mu.Unlock()
	if closed || sess == nil {
		return nil, fmt.Errorf("会话 %s 已结束", d.CallID)
	}

	recipient := d.recipient()
	req := sip.NewRequest(method, recipient)
	req.SetDestination(d.RemoteAddr)
	if len(body) > 0 {
		req.SetBody(body)
	}
	if contentType != "" {
		req.AppendHeader(sip.NewHeader("Content-Type", contentType))
	}
	return sess.Do(ctx, req)
}

// recipient 取设备在 200 OK 里的 Contact；缺失时回落到 INVITE 的 Request-URI。
func (d *Dialog) recipient() sip.Uri {
	d.mu.Lock()
	sess := d.sess
	d.mu.Unlock()
	if sess != nil && sess.InviteResponse != nil {
		if c := sess.InviteResponse.Contact(); c != nil && c.Address.Host != "" {
			return c.Address
		}
	}
	if sess != nil && sess.InviteRequest != nil {
		return sess.InviteRequest.Recipient
	}
	return sip.Uri{Scheme: "sip", Host: d.RemoteAddr}
}

// Close 本地释放会话资源（不发送 BYE）。
func (d *Dialog) Close() error {
	d.mu.Lock()
	sess := d.sess
	d.mu.Unlock()
	if sess != nil {
		return sess.Close()
	}
	return nil
}

// markClosed 标记结束并触发一次回调。
func (d *Dialog) markClosed() {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.closed = true
	f := d.onClose
	d.mu.Unlock()
	if f != nil {
		f(d)
	}
}

// ────────────────────────────────────────────────────────────────
// Manager：按 Call-ID / 通道索引会话
// ────────────────────────────────────────────────────────────────

// Manager 维护当前进程内的全部会话。
type Manager struct {
	mu        sync.RWMutex
	byCallID  map[string]*Dialog
	byChannel map[string]*Dialog
}

// NewManager 构造会话管理器。
func NewManager() *Manager {
	return &Manager{
		byCallID:  map[string]*Dialog{},
		byChannel: map[string]*Dialog{},
	}
}

// Add 登记会话；同通道的旧会话会被顶掉（点播重入）。
func (m *Manager) Add(d *Dialog) {
	if d == nil || d.CallID == "" {
		return
	}
	d.SetOnClose(func(x *Dialog) { m.Remove(x.CallID) })
	m.mu.Lock()
	m.byCallID[d.CallID] = d
	if d.ChannelID != "" {
		m.byChannel[d.ChannelID] = d
	}
	m.mu.Unlock()
}

// Remove 按 Call-ID 移除。
func (m *Manager) Remove(callID string) {
	m.mu.Lock()
	d := m.byCallID[callID]
	delete(m.byCallID, callID)
	if d != nil && d.ChannelID != "" {
		if cur := m.byChannel[d.ChannelID]; cur == d {
			delete(m.byChannel, d.ChannelID)
		}
	}
	m.mu.Unlock()
}

// Get 按 Call-ID 查找。
func (m *Manager) Get(callID string) (*Dialog, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.byCallID[callID]
	return d, ok
}

// GetByChannel 按通道编码查找当前会话（停止播放/回放控制用）。
func (m *Manager) GetByChannel(channelID string) (*Dialog, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.byChannel[channelID]
	return d, ok
}

// List 返回全部会话的快照。
func (m *Manager) List() []*Dialog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Dialog, 0, len(m.byCallID))
	for _, d := range m.byCallID {
		out = append(out, d)
	}
	return out
}

// StopAll 结束全部会话（进程退出时调用）。
func (m *Manager) StopAll(ctx context.Context) {
	for _, d := range m.List() {
		_ = d.Bye(ctx)
	}
}
