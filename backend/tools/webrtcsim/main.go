// webrtcsim 模拟一台「WebRTC 服务组」设备：通过 MQTT 收房间信令，
// 用 Pion 生成真实 H.264 视频，供管理后台「实时音视频 → 实时预览」验证。
//
// 这不是生产代码，也不替代真实固件；它的价值是把
// 「浏览器 ↔ Go ↔ MQTT ↔ 设备」整条链路跑通，并能看到真实画面。
//
// 用法：
//
//	cd backend
//	go run ./tools/webrtcsim \
//	  --broker tcp://127.0.0.1:1883 \
//	  --username svc_xxx --password '***' \
//	  --group-code webrtc --device-code CAM_001 \
//	  --stream-file tools/gbsim/testdata/sample.h264
//
// 也可以把凭据放在环境变量 MQTT_DEVICE_USERNAME / MQTT_DEVICE_PASSWORD。
//
// 信令约定（与 internal/signal/room 一致）：
//
//	设备订阅 iot/<group>/device/<device_code>/signal
//	设备发布 iot/<group>/signal/backend（带 room_id）
//	查看方 joinRoom → 设备回 joinRoom；查看方再 joinRoom → 设备发 offer
//	answer / iceCandidate / leaveRoom 双向转发
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

// room.Message 的最小兼容结构：模拟器不依赖后端内部包（tools 保持独立）。
type signalMessage struct {
	Type       string        `json:"type"`
	RoomID     string        `json:"room_id,omitempty"`
	DeviceCode string        `json:"device_code,omitempty"`
	ClientID   string        `json:"client_id,omitempty"`
	To         string        `json:"to,omitempty"`
	Role       string        `json:"role,omitempty"`
	SDP        string        `json:"sdp,omitempty"`
	SDPType    string        `json:"sdp_type,omitempty"`
	Candidate  *iceCandidate `json:"candidate,omitempty"`
	Reason     string        `json:"reason,omitempty"`
}

type iceCandidate struct {
	Candidate        string  `json:"candidate"`
	SDPMid           *string `json:"sdpMid,omitempty"`
	SDPMLineIndex    *uint16 `json:"sdpMLineIndex,omitempty"`
	UsernameFragment *string `json:"usernameFragment,omitempty"`
}

type peer struct {
	viewerID  string
	pc        *webrtc.PeerConnection
	track     *webrtc.TrackLocalStaticSample
	joins     int
	closeOnce sync.Once
}

type simulator struct {
	broker     string
	username   string
	password   string
	groupCode  string
	deviceCode string
	streamFile string
	fps        int
	stunURLs   string
	turnURL    string
	turnUser   string
	turnPass   string
	verbose    bool

	client   paho.Client
	pubTopic string
	subTopic string
	media    *mediaSource
	api      *webrtc.API

	mu    sync.Mutex
	peers map[string]*peer
}

func main() {
	sim := &simulator{}
	flag.StringVar(&sim.broker, "broker", envOr("MQTT_BROKER_URL", "tcp://127.0.0.1:1883"), "MQTT broker，例如 tcp://127.0.0.1:1883")
	flag.StringVar(&sim.username, "username", os.Getenv("MQTT_DEVICE_USERNAME"), "产品 × MQTT 服务用户名")
	flag.StringVar(&sim.password, "password", os.Getenv("MQTT_DEVICE_PASSWORD"), "产品 × MQTT 服务密码")
	flag.StringVar(&sim.groupCode, "group-code", "webrtc", "服务组编码")
	flag.StringVar(&sim.deviceCode, "device-code", "CAM_001", "设备编号（房间 ID）")
	flag.StringVar(&sim.streamFile, "stream-file", "tools/gbsim/testdata/sample.h264", "H.264 Annex-B 文件")
	flag.IntVar(&sim.fps, "fps", 25, "推流帧率")
	flag.StringVar(&sim.stunURLs, "stun", "stun:127.0.0.1:3478", "STUN 地址，逗号分隔")
	flag.StringVar(&sim.turnURL, "turn-url", "", "TURN 地址，例如 turn:127.0.0.1:3478?transport=udp")
	flag.StringVar(&sim.turnUser, "turn-user", "", "TURN 用户名（可选）")
	flag.StringVar(&sim.turnPass, "turn-pass", "", "TURN 密码（可选）")
	flag.BoolVar(&sim.verbose, "verbose", false, "打印信令细节")
	flag.Parse()

	if sim.username == "" || sim.password == "" {
		log.Fatal("缺少 MQTT 凭据：用 --username/--password 或 MQTT_DEVICE_USERNAME/MQTT_DEVICE_PASSWORD")
	}
	if sim.groupCode == "" || sim.deviceCode == "" {
		log.Fatal("--group-code / --device-code 不能为空")
	}

	src, err := loadMediaSource(sim.streamFile)
	if err != nil {
		log.Fatalf("加载 H.264 文件失败: %v", err)
	}
	sim.media = src
	sim.pubTopic = fmt.Sprintf("iot/%s/signal/backend", sim.groupCode)
	sim.subTopic = fmt.Sprintf("iot/%s/device/%s/signal", sim.groupCode, sim.deviceCode)
	sim.peers = make(map[string]*peer)

	if err := sim.initWebRTC(); err != nil {
		log.Fatalf("初始化 WebRTC 失败: %v", err)
	}

	opts := paho.NewClientOptions().
		AddBroker(sim.broker).
		SetClientID("webrtc-sim-" + sim.deviceCode).
		SetUsername(sim.username).
		SetPassword(sim.password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(3 * time.Second).
		SetCleanSession(true)

	sim.client = paho.NewClient(opts)
	if tok := sim.client.Connect(); !tok.WaitTimeout(8*time.Second) || tok.Error() != nil {
		log.Fatalf("连接 MQTT 失败: %v", tok.Error())
	}
	if tok := sim.client.Subscribe(sim.subTopic, 1, sim.onCommand); !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		log.Fatalf("订阅 %s 失败: %v", sim.subTopic, tok.Error())
	}
	log.Printf("设备模拟器已连接 broker=%s user=%s room=%s", sim.broker, sim.username, sim.deviceCode)
	log.Printf("  订阅 %s", sim.subTopic)
	log.Printf("  发布 %s", sim.pubTopic)
	log.Printf("  媒体 %s（%dx%d, %d fps）", sim.streamFile, src.width, src.height, sim.fps)

	// 设备进入房间：让后台 ticket 的 device_online 立即为 true。
	sim.publish(&signalMessage{
		Type: "joinRoom", RoomID: sim.deviceCode, DeviceCode: sim.deviceCode,
		ClientID: "device-" + sim.deviceCode, Role: "device",
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在退出…")
	sim.closeAll()
	sim.client.Disconnect(250)
}

func (s *simulator) initWebRTC() error {
	// 使用 Pion 默认媒体引擎（已包含 H.264/VP8/Opus）；
	// 具体 profile-level-id 由 Track 的 RTPCodecCapability 写进 SDP。
	s.api = webrtc.NewAPI()
	return nil
}

func (s *simulator) iceServers() []webrtc.ICEServer {
	out := []webrtc.ICEServer{}
	for _, u := range strings.Split(s.stunURLs, ",") {
		if u = strings.TrimSpace(u); u != "" {
			out = append(out, webrtc.ICEServer{URLs: []string{u}})
		}
	}
	if s.turnURL != "" {
		out = append(out, webrtc.ICEServer{
			URLs:       []string{s.turnURL},
			Username:   s.turnUser,
			Credential: s.turnPass,
		})
	}
	return out
}

func (s *simulator) onCommand(_ paho.Client, msg paho.Message) {
	var m signalMessage
	if err := json.Unmarshal(msg.Payload(), &m); err != nil {
		log.Printf("信令解析失败: %v", err)
		return
	}
	if s.verbose {
		raw, _ := json.Marshal(m)
		log.Printf("← %s", raw)
	}
	switch m.Type {
	case "joinRoom":
		s.handleJoin(m)
	case "answer":
		s.handleAnswer(m)
	case "iceCandidate":
		s.handleCandidate(m)
	case "leaveRoom":
		s.handleLeave(m)
	case "offer":
		// 当前角色固定为 offer 方，收到 offer 说明对端角色配置反了。
		log.Printf("忽略来自查看方的 offer（设备应为 offer 方）")
	}
}

func (s *simulator) handleJoin(m signalMessage) {
	if m.ClientID == "" || m.Role == "device" {
		return
	}
	s.mu.Lock()
	p := s.peers[m.ClientID]
	if p == nil {
		p = &peer{viewerID: m.ClientID}
		s.peers[m.ClientID] = p
	}
	p.joins++
	joins := p.joins
	s.mu.Unlock()

	// 第一次 joinRoom：回设备 joinRoom；查看方收到后会再发一次 joinRoom。
	if joins == 1 {
		s.publish(&signalMessage{
			Type: "joinRoom", RoomID: s.deviceCode, DeviceCode: s.deviceCode,
			ClientID: "device-" + s.deviceCode, To: m.ClientID, Role: "device",
		})
		return
	}
	// 第二次及以后：创建/复用 PeerConnection 并发 offer（支持重新协商）。
	if err := s.makeOffer(p); err != nil {
		log.Printf("创建 offer 失败 viewer=%s: %v", m.ClientID, err)
	}
}

func (s *simulator) makeOffer(p *peer) error {
	s.mu.Lock()
	if p.pc == nil {
		if err := s.newPeerLocked(p); err != nil {
			s.mu.Unlock()
			return err
		}
	}
	pc := p.pc
	s.mu.Unlock()

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		return err
	}
	log.Printf("→ offer viewer=%s", p.viewerID)
	s.publish(&signalMessage{
		Type: "offer", RoomID: s.deviceCode, DeviceCode: s.deviceCode,
		ClientID: "device-" + s.deviceCode, To: p.viewerID, Role: "device",
		SDP: pc.LocalDescription().SDP, SDPType: "offer",
	})
	return nil
}

// newPeerLocked 调用方必须持有 s.mu。
func (s *simulator) newPeerLocked(p *peer) error {
	pc, err := s.api.NewPeerConnection(webrtc.Configuration{ICEServers: s.iceServers()})
	if err != nil {
		return err
	}
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: fmt.Sprintf("level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=%s", s.media.profileLevelID),
		},
		"video", "webrtc-sim-"+p.viewerID,
	)
	if err != nil {
		_ = pc.Close()
		return err
	}
	if _, err := pc.AddTrack(track); err != nil {
		_ = pc.Close()
		return err
	}
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		init := c.ToJSON()
		s.publish(&signalMessage{
			Type: "iceCandidate", RoomID: s.deviceCode, DeviceCode: s.deviceCode,
			ClientID: "device-" + s.deviceCode, To: p.viewerID, Role: "device",
			Candidate: &iceCandidate{
				Candidate: init.Candidate, SDPMid: init.SDPMid,
				SDPMLineIndex: init.SDPMLineIndex, UsernameFragment: init.UsernameFragment,
			},
		})
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("viewer=%s connection=%s", p.viewerID, state)
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			p.closeOnce.Do(func() { _ = pc.Close() })
		}
	})
	p.pc = pc
	p.track = track
	go s.streamLoop(p)
	return nil
}

func (s *simulator) handleAnswer(m signalMessage) {
	s.mu.Lock()
	p := s.peers[m.ClientID]
	s.mu.Unlock()
	if p == nil || p.pc == nil {
		return
	}
	if err := p.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer, SDP: m.SDP,
	}); err != nil {
		log.Printf("设置 answer 失败 viewer=%s: %v", m.ClientID, err)
		return
	}
	log.Printf("← answer viewer=%s", m.ClientID)
}

func (s *simulator) handleCandidate(m signalMessage) {
	s.mu.Lock()
	p := s.peers[m.ClientID]
	s.mu.Unlock()
	if p == nil || p.pc == nil || m.Candidate == nil {
		return
	}
	err := p.pc.AddICECandidate(webrtc.ICECandidateInit{
		Candidate:        m.Candidate.Candidate,
		SDPMid:           m.Candidate.SDPMid,
		SDPMLineIndex:    m.Candidate.SDPMLineIndex,
		UsernameFragment: m.Candidate.UsernameFragment,
	})
	if err != nil {
		log.Printf("添加 candidate 失败 viewer=%s: %v", m.ClientID, err)
	}
}

func (s *simulator) handleLeave(m signalMessage) {
	s.mu.Lock()
	p := s.peers[m.ClientID]
	delete(s.peers, m.ClientID)
	s.mu.Unlock()
	if p != nil && p.pc != nil {
		_ = p.pc.Close()
		log.Printf("查看方离开 viewer=%s reason=%s", m.ClientID, m.Reason)
	}
}

func (s *simulator) streamLoop(p *peer) {
	interval := time.Second / time.Duration(s.fps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	index := 0
	for range ticker.C {
		if p.track == nil {
			return
		}
		au := s.media.accessUnits[index%len(s.media.accessUnits)]
		index++
		if err := p.track.WriteSample(media.Sample{
			Data:     au,
			Duration: interval,
		}); err != nil {
			// 连接关闭后 WriteSample 会报错，正常退出即可。
			return
		}
	}
}

func (s *simulator) publish(m *signalMessage) {
	payload, err := json.Marshal(m)
	if err != nil {
		return
	}
	if s.verbose {
		log.Printf("→ %s", payload)
	}
	tok := s.client.Publish(s.pubTopic, 1, false, payload)
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		log.Printf("发布失败: %v", tok.Error())
	}
}

func (s *simulator) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, p := range s.peers {
		if p.pc != nil {
			_ = p.pc.Close()
		}
		delete(s.peers, id)
	}
}

// ── H.264 Annex-B 解析 ─────────────────────────────────────────

type mediaSource struct {
	width          int
	height         int
	profileLevelID string
	accessUnits    [][]byte
}

func loadMediaSource(path string) (*mediaSource, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	nalus := splitNALUs(raw)
	if len(nalus) == 0 {
		return nil, fmt.Errorf("文件中没有 NAL unit")
	}
	src := &mediaSource{}
	var au []byte
	for _, nalu := range nalus {
		typ := nalu[0] & 0x1f
		switch typ {
		case 7: // SPS
			if len(nalu) >= 4 && src.profileLevelID == "" {
				src.profileLevelID = fmt.Sprintf("%02x%02x%02x", nalu[1], nalu[2], nalu[3])
			}
			src.width, src.height = parseSPSSize(nalu)
		case 9: // AUD：一帧的开始，跳过
			if len(au) > 0 {
				src.accessUnits = append(src.accessUnits, au)
				au = nil
			}
			continue
		}
		// 统一还原为 Annex-B（emitNalus 认得起始码）。
		au = append(au, 0, 0, 0, 1)
		au = append(au, nalu...)
		if typ == 1 || typ == 5 { // VCL：一个访问单元结束
			src.accessUnits = append(src.accessUnits, au)
			au = nil
		}
	}
	if len(au) > 0 {
		src.accessUnits = append(src.accessUnits, au)
	}
	if len(src.accessUnits) == 0 {
		return nil, fmt.Errorf("没有可用的视频帧")
	}
	if src.profileLevelID == "" {
		src.profileLevelID = "42e01f"
	}
	return src, nil
}

func splitNALUs(data []byte) [][]byte {
	var out [][]byte
	start := -1
	for i := 0; i+3 <= len(data); {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			if start >= 0 {
				out = append(out, trimTrailingZeros(data[start:i]))
			}
			i += 3
			start = i
			continue
		}
		if i+4 <= len(data) && data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1 {
			if start >= 0 {
				out = append(out, trimTrailingZeros(data[start:i]))
			}
			i += 4
			start = i
			continue
		}
		i++
	}
	if start >= 0 && start < len(data) {
		out = append(out, trimTrailingZeros(data[start:]))
	}
	filtered := out[:0]
	for _, n := range out {
		if len(n) > 0 {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

func trimTrailingZeros(b []byte) []byte {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return b
}

// parseSPSSize 解析 SPS 的宽高（含裁剪窗口）。失败时返回 0,0。
func parseSPSSize(sps []byte) (int, int) {
	if len(sps) < 4 {
		return 0, 0
	}
	r := &bitReader{data: sps[1:]}
	profile := r.u(8)
	r.u(8) // constraint flags
	r.u(8) // level_idc
	r.ue() // seq_parameter_set_id
	chromaFormat := uint(1)
	if profile == 100 || profile == 110 || profile == 122 || profile == 244 ||
		profile == 44 || profile == 83 || profile == 86 || profile == 118 || profile == 128 {
		chromaFormat = r.ue()
		if chromaFormat == 3 {
			r.u(1)
		}
		r.ue()
		r.ue()
		r.u(1)
		if r.u(1) == 1 {
			n := 8
			if chromaFormat == 3 {
				n = 12
			}
			for i := 0; i < n; i++ {
				if r.u(1) == 1 {
					size := 16
					if i < 6 {
						size = 16
					} else {
						size = 64
					}
					last, next := 8, 8
					for j := 0; j < size; j++ {
						if next != 0 {
							next = (last + r.se() + 256) % 256
						}
						last = next
						if next != 0 {
							last = next
						}
					}
				}
			}
		}
	}
	r.ue() // log2_max_frame_num_minus4
	pocType := r.ue()
	if pocType == 0 {
		r.ue()
	} else if pocType == 1 {
		r.u(1)
		r.se()
		r.se()
		n := r.ue()
		for i := uint(0); i < n; i++ {
			r.se()
		}
	}
	r.ue() // max_num_ref_frames
	r.u(1) // gaps_in_frame_num_value_allowed_flag
	widthMB := r.ue() + 1
	heightMapUnits := r.ue() + 1
	frameMbsOnly := r.u(1)
	if frameMbsOnly == 0 {
		r.u(1)
	}
	r.u(1) // direct_8x8_inference_flag
	width := int(widthMB) * 16
	height := int(heightMapUnits) * 16
	if r.u(1) == 1 { // frame_cropping_flag
		left, right := r.ue(), r.ue()
		top, bottom := r.ue(), r.ue()
		cropX, cropY := uint(1), uint(1)
		switch chromaFormat {
		case 1:
			cropX, cropY = 2, 2
		case 2:
			cropX, cropY = 2, 2
		case 3:
			cropX, cropY = 1, 1
		}
		width -= int((left + right) * cropX)
		height -= int((top + bottom) * cropY)
		if frameMbsOnly == 0 {
			height *= 2
		}
	}
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	return width, height
}

type bitReader struct {
	data []byte
	pos  int
}

func (r *bitReader) u(n int) uint {
	var v uint
	for i := 0; i < n; i++ {
		byteIdx := r.pos / 8
		if byteIdx >= len(r.data) {
			return v
		}
		bit := (r.data[byteIdx] >> (7 - uint(r.pos%8))) & 1
		v = (v << 1) | uint(bit)
		r.pos++
	}
	return v
}

func (r *bitReader) ue() uint {
	zeros := 0
	for r.u(1) == 0 {
		zeros++
		if zeros > 31 {
			return 0
		}
	}
	if zeros == 0 {
		return 0
	}
	return (1 << uint(zeros)) - 1 + r.u(zeros)
}

func (r *bitReader) se() int {
	v := r.ue()
	if v&1 == 1 {
		return int((v + 1) / 2)
	}
	return -int(v / 2)
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
