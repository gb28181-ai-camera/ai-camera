package gb28181_test

// 国标级联（下级平台侧）的端到端信令测试。
//
// 为什么用裸 UDP 假上级而不是再起一套 sipgo：
//   - 级联的「上级」行为（401 挑战、Catalog 查询、INVITE）在真实环境里各家
//     实现差异很大，用原始报文能精确控制每个字段，验证的是**本平台发出的字节**，
//     而不是两个 sipgo 实例互相迁就；
//   - 与 tools/gbsim/sipua/invite_test.go 的 rawPlatform 思路一致，便于对照。
//
// 本用例不依赖数据库与 ZLM：数据源和媒体发送都是假实现。

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/auth"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/platform"
	"github.com/boticz/ai-camera-backend/internal/model"
)

const (
	cascadeDomain     = "3402000000"
	cascadeServerGBID = "34020000002000000001"
	cascadeDeviceGBID = "34020000002000000099"
	cascadeUser       = "admin"
	cascadePassword   = "secret123"
	cascadeNonce      = "5f2c9d3b7a1e4c60"
)

// ────────────────────────────────────────────────────────────────
// 假数据源 / 假媒体
// ────────────────────────────────────────────────────────────────

type fakeSource struct {
	platforms []model.GbPlatform
	catalog   []platform.CatalogEntry
}

func (f *fakeSource) ListPlatforms(context.Context) ([]model.GbPlatform, error) {
	return f.platforms, nil
}

func (f *fakeSource) GetPlatform(_ context.Context, id int64) (*model.GbPlatform, error) {
	for i := range f.platforms {
		if f.platforms[i].ID == id {
			return &f.platforms[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (f *fakeSource) Catalog(context.Context, *model.GbPlatform) ([]platform.CatalogEntry, error) {
	return f.catalog, nil
}

type fakeMedia struct {
	mu      sync.Mutex
	started []mediaserver.SendRTPParam
	stopped []uint32
}

func (f *fakeMedia) StartSendRTP(_ context.Context, p mediaserver.SendRTPParam) (*mediaserver.SendRTPResult, error) {
	f.mu.Lock()
	f.started = append(f.started, p)
	f.mu.Unlock()
	return &mediaserver.SendRTPResult{LocalPort: 30001}, nil
}

func (f *fakeMedia) StopSendRTP(_ context.Context, _ string, _ string, ssrc uint32) error {
	f.mu.Lock()
	f.stopped = append(f.stopped, ssrc)
	f.mu.Unlock()
	return nil
}

func (f *fakeMedia) lastStart() (mediaserver.SendRTPParam, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.started) == 0 {
		return mediaserver.SendRTPParam{}, false
	}
	return f.started[len(f.started)-1], true
}

func (f *fakeMedia) stopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.stopped)
}

// ────────────────────────────────────────────────────────────────
// 假上级（裸 UDP）
// ────────────────────────────────────────────────────────────────

type fakeSuperior struct {
	t    *testing.T
	conn *net.UDPConn
	addr string

	branchNo int

	registered   chan struct{}
	registerOnce sync.Once
	registerErr  chan error

	catalogCh chan []catalogItem
	infoCh    chan string
	inviteCh  chan rawSIP

	mu              sync.Mutex
	inviteOK        *rawSIP
	byeOK           bool
	lastCatalogRoot string
	lastCatalogSum  int
}

type rawSIP struct {
	status int
	reason string
	header map[string]string
	body   string
}

type catalogItem struct {
	DeviceID string `xml:"DeviceID"`
	Name     string `xml:"Name"`
}

type catalogMessage struct {
	// 不写根元素名：查询应答是 <Response>，主动推送/订阅通知是 <Notify>。
	XMLName    xml.Name
	CmdType    string `xml:"CmdType"`
	SN         int    `xml:"SN"`
	SumNum     int    `xml:"SumNum"`
	DeviceList struct {
		Num   int           `xml:"Num,attr"`
		Items []catalogItem `xml:"Item"`
	} `xml:"DeviceList"`
}

func startFakeSuperior(t *testing.T) *fakeSuperior {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("假上级监听失败：%v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	fs := &fakeSuperior{
		t:           t,
		conn:        conn,
		addr:        conn.LocalAddr().String(),
		registered:  make(chan struct{}),
		registerErr: make(chan error, 1),
		catalogCh:   make(chan []catalogItem, 4),
		infoCh:      make(chan string, 4),
		inviteCh:    make(chan rawSIP, 4),
	}
	go fs.loop()
	return fs
}

func (f *fakeSuperior) nextBranch() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.branchNo++
	return fmt.Sprintf("z9hG4bKcascade%d", f.branchNo)
}

func (f *fakeSuperior) send(target, msg string) {
	f.t.Helper()
	addr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		f.t.Fatalf("解析目标地址失败：%v", err)
	}
	if _, err := f.conn.WriteToUDP([]byte(msg), addr); err != nil {
		f.t.Fatalf("假上级发送失败：%v", err)
	}
}

// loop 持续读取：既处理本平台发来的请求（REGISTER / MESSAGE / ACK），
// 也处理本平台对假上级查询的响应（200 OK）。
func (f *fakeSuperior) loop() {
	buf := make([]byte, 64*1024)
	pendingCatalog := make(map[int][]catalogItem)
	for {
		n, src, err := f.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		msg := string(buf[:n])
		switch {
		case strings.HasPrefix(msg, "REGISTER "):
			f.handleRegister(msg, src.String())
		case strings.HasPrefix(msg, "MESSAGE "):
			f.handleInboundMessage(msg, src.String(), pendingCatalog)
		case strings.HasPrefix(msg, "SIP/2.0 "):
			f.handleResponse(msg)
		case strings.HasPrefix(msg, "ACK "), strings.HasPrefix(msg, "BYE "):
			// 本平台不主动发 BYE；ACK 由本平台作为 UAS 接收，不会走到这里。
		}
	}
}

func (f *fakeSuperior) handleRegister(msg, replyTo string) {
	req := parseRawSIP(msg)
	authz := req.header["authorization"]
	if authz == "" {
		// 第一步：401 挑战。必须回给报文的源地址（本平台的级联端口），
		// 不能回 f.addr —— 那是假上级自己的地址。
		f.send(replyTo, buildResponse(req, 401, "Unauthorized", nil, map[string]string{
			"WWW-Authenticate": `Digest realm="` + cascadeDomain + `", nonce="` + cascadeNonce + `", algorithm=MD5`,
		}))
		return
	}

	params, err := auth.ParseAuthorization(authz)
	if err != nil {
		f.registerErr <- fmt.Errorf("解析 Authorization 失败：%w", err)
		return
	}
	qop := params["qop"]
	want := auth.DigestResponse(cascadeUser, cascadeDomain, cascadePassword,
		"REGISTER", params["uri"], cascadeNonce, params["cnonce"], qop, params["nc"])
	if !strings.EqualFold(want, params["response"]) {
		f.registerErr <- fmt.Errorf("Digest 校验失败：want=%s got=%s", want, params["response"])
		f.send(replyTo, buildResponse(req, 403, "Forbidden", nil, nil))
		return
	}
	f.send(replyTo, buildResponse(req, 200, "OK", nil, map[string]string{
		"Expires": "3600",
	}))
	f.registerOnce.Do(func() { close(f.registered) })
}

func (f *fakeSuperior) handleInboundMessage(msg, replyTo string, pending map[int][]catalogItem) {
	req := parseRawSIP(msg)
	// 每条 MESSAGE 都必须先回 200：本平台用事务化 client.Do 发送，
	// 收不到 200 会按 SIP 定时器重传，表现为「目录只有第一片」。
	f.send(replyTo, buildResponse(req, 200, "OK", nil, nil))
	if strings.Contains(req.body, "<CmdType>DeviceInfo</CmdType>") ||
		strings.Contains(req.body, "<CmdType>DeviceStatus</CmdType>") {
		select {
		case f.infoCh <- req.body:
		default:
		}
		return
	}
	if !strings.Contains(req.body, "<CmdType>Catalog</CmdType>") {
		if testing.Verbose() {
			f.t.Logf("非 Catalog MESSAGE body=%q", truncateForLog(req.body))
		}
		return // Keepalive 等
	}
	var cm catalogMessage
	if err := xml.Unmarshal([]byte(req.body), &cm); err != nil {
		f.t.Logf("解析 Catalog 失败：%v body=%s", err, truncateForLog(req.body))
		return
	}
	f.mu.Lock()
	f.lastCatalogRoot = cm.XMLName.Local
	f.lastCatalogSum = cm.SumNum
	f.mu.Unlock()
	if testing.Verbose() {
		f.t.Logf("Catalog 分片 root=%s SN=%d SumNum=%d Num=%d",
			cm.XMLName.Local, cm.SN, cm.SumNum, len(cm.DeviceList.Items))
	}
	items := pending[cm.SN]
	items = append(items, cm.DeviceList.Items...)
	pending[cm.SN] = items
	if len(items) >= cm.SumNum {
		delete(pending, cm.SN)
		f.catalogCh <- items
	}
}

func (f *fakeSuperior) handleResponse(msg string) {
	res := parseRawSIP(msg)
	cseq := res.header["cseq"]
	switch {
	case strings.Contains(cseq, "INVITE"):
		if res.status == 200 {
			f.mu.Lock()
			cp := res
			f.inviteOK = &cp
			f.mu.Unlock()
			f.inviteCh <- res
		}
	case strings.Contains(cseq, "BYE"):
		if res.status == 200 {
			f.mu.Lock()
			f.byeOK = true
			f.mu.Unlock()
			f.inviteCh <- res
		}
	}
}

// queryCatalog 发一条 Catalog 查询并等待完整目录。
func (f *fakeSuperior) queryCatalog(target string, sn int) []catalogItem {
	f.t.Helper()
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<Query><CmdType>Catalog</CmdType><SN>%d</SN><DeviceID>%s</DeviceID></Query>`,
		sn, cascadeDeviceGBID)
	f.send(target, f.buildRequest("MESSAGE", target, cascadeDeviceGBID, sn, "Application/MANSCDP+xml", body))
	return f.waitCatalog()
}

// waitCatalog 等待一次完整目录（可能跨多条 MESSAGE）。
func (f *fakeSuperior) waitCatalog() []catalogItem {
	f.t.Helper()
	select {
	case items := <-f.catalogCh:
		return items
	case <-time.After(5 * time.Second):
		f.t.Fatalf("等待 Catalog 超时")
		return nil
	}
}

func (f *fakeSuperior) catalogMeta() (root string, sum int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastCatalogRoot, f.lastCatalogSum
}

// queryTyped 发送 DeviceInfo / DeviceStatus 查询并等待应答正文。
func (f *fakeSuperior) queryTyped(target, cmdType string, sn int) string {
	f.t.Helper()
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<Query><CmdType>%s</CmdType><SN>%d</SN><DeviceID>%s</DeviceID></Query>`,
		cmdType, sn, cascadeDeviceGBID)
	f.send(target, f.buildRequest("MESSAGE", target, cascadeDeviceGBID, sn, "Application/MANSCDP+xml", body))
	select {
	case got := <-f.infoCh:
		return got
	case <-time.After(5 * time.Second):
		f.t.Fatalf("等待 %s 应答超时", cmdType)
		return ""
	}
}

func (f *fakeSuperior) invite(target, channelID, sdp string, cseq int) rawSIP {
	f.t.Helper()
	f.send(target, f.buildRequest("INVITE", target, channelID, cseq, "application/sdp", sdp))
	return f.waitResponse("INVITE")
}

// waitResponse 等待指定方法的最终响应。
//
// 不能直接读第一个响应：UDP 事务在 ACK 到达前会重传 200 OK，
// 读包顺序里可能先出现上一条 INVITE 的重传，误判成本次 BYE 的结果。
func (f *fakeSuperior) waitResponse(method string) rawSIP {
	f.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case res := <-f.inviteCh:
			if strings.Contains(res.header["cseq"], method) {
				return res
			}
		case <-deadline:
			f.t.Fatalf("等待 %s 最终响应超时", method)
			return rawSIP{}
		}
	}
}

func (f *fakeSuperior) bye(target, channelID, toTag string, cseq int) {
	f.t.Helper()
	msg := strings.Join([]string{
		"BYE sip:" + channelID + "@" + target + " SIP/2.0",
		"Via: SIP/2.0/UDP " + f.addr + ";rport;branch=" + f.nextBranch(),
		"Max-Forwards: 70",
		"From: <sip:" + cascadeServerGBID + "@" + cascadeDomain + ">;tag=fake-superior",
		"To: <sip:" + channelID + "@" + cascadeDomain + ">;tag=" + toTag,
		"Call-ID: cascade-invite-1",
		"CSeq: " + strconv.Itoa(cseq) + " BYE",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	f.send(target, msg)
	res := f.waitResponse("BYE")
	if res.status != 200 {
		f.t.Fatalf("BYE 期望 200，实际 %d %s", res.status, res.reason)
	}
}

// buildRequest 构造一条假上级发往本平台的请求。
func (f *fakeSuperior) buildRequest(method, target, channelID string, cseq int, contentType, body string) string {
	return strings.Join([]string{
		method + " sip:" + channelID + "@" + target + " SIP/2.0",
		"Via: SIP/2.0/UDP " + f.addr + ";rport;branch=" + f.nextBranch(),
		"Max-Forwards: 70",
		"From: <sip:" + cascadeServerGBID + "@" + cascadeDomain + ">;tag=fake-superior",
		"To: <sip:" + channelID + "@" + cascadeDomain + ">",
		"Call-ID: cascade-invite-1",
		"CSeq: " + strconv.Itoa(cseq) + " " + method,
		"Contact: <sip:" + cascadeServerGBID + "@" + f.addr + ">",
		"Content-Type: " + contentType,
		"Content-Length: " + strconv.Itoa(len(body)),
		"",
		body,
	}, "\r\n")
}

// buildResponse 按请求的 Via/From/To/Call-ID/CSeq 构造响应。
func buildResponse(req rawSIP, code int, reason string, body []byte, extra map[string]string) string {
	to := req.header["to"]
	if !strings.Contains(to, "tag=") {
		to += ";tag=fake-superior"
	}
	lines := []string{
		fmt.Sprintf("SIP/2.0 %d %s", code, reason),
		"Via: " + req.header["via"],
		"From: " + req.header["from"],
		"To: " + to,
		"Call-ID: " + req.header["call-id"],
		"CSeq: " + req.header["cseq"],
	}
	for k, v := range extra {
		lines = append(lines, k+": "+v)
	}
	// 空行是 SIP 头/体分隔符：必须写两个（header 结束 + 正文起始）。
	lines = append(lines, "Content-Length: "+strconv.Itoa(len(body)), "", string(body))
	return strings.Join(lines, "\r\n")
}

func parseRawSIP(raw string) rawSIP {
	head, body, _ := strings.Cut(raw, "\r\n\r\n")
	lines := strings.Split(head, "\r\n")
	out := rawSIP{header: map[string]string{}, body: body}
	if len(lines) > 0 {
		var ver string
		if _, err := fmt.Sscanf(lines[0], "%s %d %s", &ver, &out.status, &out.reason); err != nil {
			// 请求行（METHOD URI SIP/2.0）：status 保持 0。
			out.status = 0
		}
	}
	for _, line := range lines[1:] {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out.header[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}
	return out
}

func truncateForLog(s string) string {
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

func (r rawSIP) toTag() string {
	to := r.header["to"]
	_, params, ok := strings.Cut(to, ";")
	if !ok {
		return ""
	}
	for _, p := range strings.Split(params, ";") {
		name, value, found := strings.Cut(strings.TrimSpace(p), "=")
		if found && name == "tag" {
			return value
		}
	}
	return ""
}

// ────────────────────────────────────────────────────────────────
// 用例
// ────────────────────────────────────────────────────────────────

func TestCascadeRegisterCatalogInvite(t *testing.T) {
	superior := startFakeSuperior(t)

	enabled := true
	plat := model.GbPlatform{
		BaseModel:       model.BaseModel{ID: 1},
		Active:          &enabled,
		Enable:          &enabled,
		Name:            "测试上级",
		ServerGBID:      cascadeServerGBID,
		ServerGBDomain:  cascadeDomain,
		ServerIP:        "127.0.0.1",
		ServerPort:      int64(mustPort(t, superior.addr)),
		DeviceGBID:      cascadeDeviceGBID,
		Username:        cascadeUser,
		Password:        cascadePassword,
		Expires:         3600,
		KeepTimeout:     60,
		Transport:       "UDP",
		CharacterSet:    "UTF-8",
		Status:          model.GbStatusOff,
		AutoPushChannel: &enabled,
	}

	// 30 条长名称通道：验证按字节分片（单条 MESSAGE 不可能装下）。
	entries := make([]platform.CatalogEntry, 0, 30)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("3402000000132000%04d", i+1)
		entries = append(entries, platform.CatalogEntry{
			DeviceID:    id,
			Name:        fmt.Sprintf("测试通道-%02d-名称长度用于触发目录分片", i+1),
			Status:      model.GbStatusOn,
			ChannelType: 0,
			PTZType:     1,
			ChannelID:   int64(i + 1),
			App:         "rtp",
			Stream:      fmt.Sprintf("STREAM%04d", i+1),
		})
	}
	src := &fakeSource{platforms: []model.GbPlatform{plat}, catalog: entries}
	media := &fakeMedia{}

	svc, err := platform.New(platform.Config{
		Enabled:     true,
		ListenIP:    "127.0.0.1",
		Port:        0, // 测试用临时端口
		AdvertiseIP: "127.0.0.1",
		Domain:      cascadeDomain,
		ServerName:  "cascade-test",
		Charset:     "UTF-8",
	}, src, platform.WithMediaFactory(func(context.Context, *model.GbPlatform) (platform.MediaSender, error) {
		return media, nil
	}))
	if err != nil {
		t.Fatalf("构造级联服务失败：%v", err)
	}
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("启动级联服务失败：%v", err)
	}
	t.Cleanup(svc.Stop)

	// 1) REGISTER 401 → Digest → 200
	select {
	case <-superior.registered:
	case err := <-superior.registerErr:
		t.Fatalf("注册 Digest 校验失败：%v", err)
	case <-time.After(8 * time.Second):
		t.Fatalf("等待级联 REGISTER 超时")
	}
	waitOnline(t, svc)

	// 2) Catalog 查询：应分多条 MESSAGE，且通道名称使用定制值。
	items := superior.queryCatalog(svc.Addr(), 100)
	if len(items) != len(entries) {
		t.Fatalf("Catalog 条数不符：want=%d got=%d", len(entries), len(items))
	}
	if items[0].DeviceID != entries[0].DeviceID {
		t.Fatalf("Catalog 首条编码不符：want=%s got=%s", entries[0].DeviceID, items[0].DeviceID)
	}
	if !strings.Contains(items[0].Name, "测试通道-01") {
		t.Fatalf("Catalog 名称未按定制值返回：%q", items[0].Name)
	}
	if root, sum := superior.catalogMeta(); root != "Response" || sum != len(entries) {
		t.Fatalf("查询应答根元素/SumNum 不符：root=%s sum=%d", root, sum)
	}

	// 2b) 主动推送：映射变更后走 MESSAGE Notify/Catalog（本项目的推送策略）。
	if err := svc.NotifyCatalog(context.Background(), 1, true); err != nil {
		t.Fatalf("主动推送目录失败：%v", err)
	}
	pushed := superior.waitCatalog()
	if len(pushed) != len(entries) {
		t.Fatalf("主动推送条数不符：want=%d got=%d", len(entries), len(pushed))
	}
	if root, _ := superior.catalogMeta(); root != "Notify" {
		t.Fatalf("主动推送根元素应为 Notify，实际 %s", root)
	}

	// 2c) DeviceInfo / DeviceStatus 用本地数据应答。
	info := superior.queryTyped(svc.Addr(), "DeviceInfo", 101)
	if !strings.Contains(info, "<Result>OK</Result>") ||
		!strings.Contains(info, "<DeviceName>测试上级</DeviceName>") {
		t.Fatalf("DeviceInfo 应答不符：%s", info)
	}
	status := superior.queryTyped(svc.Addr(), "DeviceStatus", 102)
	if !strings.Contains(status, "<Online>ONLINE</Online>") || !strings.Contains(status, "<Status>OK</Status>") {
		t.Fatalf("DeviceStatus 应答不符：%s", status)
	}

	// 3) INVITE：接受并把本地流推到上级 SDP 指定的地址/端口/SSRC。
	sdp := strings.Join([]string{
		"v=0",
		"o=" + cascadeServerGBID + " 0 0 IN IP4 127.0.0.1",
		"s=Play",
		"c=IN IP4 127.0.0.1",
		"t=0 0",
		"m=video 30000 RTP/AVP 96",
		"a=recvonly",
		"a=rtpmap:96 PS/90000",
		"y=0123456789",
		"",
	}, "\r\n")
	res := superior.invite(svc.Addr(), entries[0].DeviceID, sdp, 1)
	if res.status != 200 {
		t.Fatalf("INVITE 期望 200，实际 %d %s", res.status, res.reason)
	}
	if !strings.Contains(res.body, "a=sendonly") || !strings.Contains(res.body, "y=0123456789") {
		t.Fatalf("INVITE 应答 SDP 不符合 sendonly/y= 约定：%s", res.body)
	}

	started, ok := media.lastStart()
	if !ok {
		t.Fatalf("未调用 StartSendRTP")
	}
	if started.DstURL != "127.0.0.1" || started.DstPort != 30000 {
		t.Fatalf("推流目标不符：%s:%d", started.DstURL, started.DstPort)
	}
	if started.App != "rtp" || started.Stream != "STREAM0001" {
		t.Fatalf("推流源流不符：%s/%s", started.App, started.Stream)
	}
	if started.SSRC != 123456789 {
		t.Fatalf("推流 SSRC 应为上级 SDP 的 y=，实际 %d", started.SSRC)
	}

	// 4) ACK + BYE：BYE 后必须停止推流。
	fakeSuperiorSendAck(t, superior, svc.Addr(), entries[0].DeviceID, res.toTag())
	superior.bye(svc.Addr(), entries[0].DeviceID, res.toTag(), 2)
	if media.stopCount() == 0 {
		t.Fatalf("BYE 后未调用 StopSendRTP")
	}
}

func waitOnline(t *testing.T, svc *platform.Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap := svc.Snapshot()
		for _, s := range snap.Platforms {
			if s.Online {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("级联平台未在超时时间内变为在线：%+v", svc.Snapshot())
}

func mustPort(t *testing.T, addr string) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("拆分地址失败：%v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("解析端口失败：%v", err)
	}
	return port
}

// fakeSuperiorSendAck 发送对话框内 ACK（全新 branch，不等待响应）。
func fakeSuperiorSendAck(t *testing.T, f *fakeSuperior, target, channelID, toTag string) {
	t.Helper()
	msg := strings.Join([]string{
		"ACK sip:" + channelID + "@" + target + " SIP/2.0",
		"Via: SIP/2.0/UDP " + f.addr + ";rport;branch=" + f.nextBranch(),
		"Max-Forwards: 70",
		"From: <sip:" + cascadeServerGBID + "@" + cascadeDomain + ">;tag=fake-superior",
		"To: <sip:" + channelID + "@" + cascadeDomain + ">;tag=" + toTag,
		"Call-ID: cascade-invite-1",
		"CSeq: 1 ACK",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	f.send(target, msg)
}
