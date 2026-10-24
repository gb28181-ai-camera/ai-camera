package gb28181

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/audio"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/sdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/session"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 语音广播 / 对讲（设计 §8、契约 §15.4）
//
// 真实音频闭环（本期补齐）：
//
//	设备麦克风
//	  → INVITE 协商出的 ZLM 单端口 rtp/<设备流>（PCMA）
//	  → ZLM startSendRtp（上行）→ Pipecat UDP 接收
//	  → Pipecat: VAD → STT → LLM → TTS → PCMA
//	  → Pipecat WebRTC push → ZLM rtp/AI<设备流>
//	  → ZLM startSendRtp（下行）→ 设备喇叭
//
// 两条 startSendRtp 的源流不同：
//   - 上行源流 = dlg.Stream（设备流），目标是 Pipecat 的 UDP 端口；
//   - 下行源流 = audio.AgentStreamName(dlg.Stream)，目标是设备 200 OK
//     SDP 里的 m=audio 地址。
//
// ★ 默认半双工：全双工必然自激（设计 §8.1.1）—— 设备喇叭的声音会被
// 自己的麦克风收回，形成「agent 听到自己 → 判定被打断 → 再说话」的死循环。
// Pipecat 侧在输出 TTS 期间会静音上行（见 deploy/pipecat/transport.py）。
//
// ★ 降级：Pipecat 不可达 / 音频桥未完全连通时，只保留 SIP 对讲会话，
// 响应 agent_enabled=false，绝不把整个对讲判失败。
// ════════════════════════════════════════════════════════════════

var (
	// ErrGbVoiceSessionNotFound 语音会话不存在或已结束。
	ErrGbVoiceSessionNotFound = errors.New("语音会话不存在或已结束")
	// ErrGbVoiceSessionBusy 同一通道已有进行中的语音会话。
	ErrGbVoiceSessionBusy = errors.New("该通道已有进行中的语音会话，请先停止")
	// ErrGbBroadcastDisabled 通道未开启语音广播。
	ErrGbBroadcastDisabled = errors.New("该通道未开启语音广播")
	// ErrGbVoiceSessionTypeMismatch 停止接口与会的会话类型不匹配。
	ErrGbVoiceSessionTypeMismatch = errors.New("语音会话类型不匹配（广播/对讲）")
)

const (
	voiceStatusActive  = "active"
	voiceStatusStopped = "stopped"
	// voiceAudioCodec 一期固定 G.711A：GB28181 设备的默认音频编码。
	voiceAudioCodec = "G.711A"
	// voiceModeHalfDuplex 半双工（设计 §8.1.1 的默认决策）。
	voiceModeHalfDuplex = "half_duplex"

	// 语音 Agent 配置在 ac_thirdparty_config 中的分类与键名。
	// 与 backend/internal/service/setting/voice_agent_service.go 保持一致。
	pipecatConfigCategory = "voice_agent"
	pipecatEndpointKey    = "pipecat_endpoint"
	pipecatTokenKey       = "pipecat_token"

	// 环境变量兜底：容器编排里可以只用环境变量，不必先落库。
	envPipecatBaseURL = "PIPECAT_BASE_URL"
	envPipecatToken   = "PIPECAT_TOKEN"
	// PIPECAT_UDP_HOST 用于「控制面地址是域名、但 ZLM 需要 IP」的场景。
	// 未配置时由 PIPECAT_BASE_URL 的主机名解析出 IPv4。
	envPipecatUDPHost = "PIPECAT_UDP_HOST"
	// PIPECAT_ZLM_BASE_URL 覆盖传给 Pipecat 的 ZLM 地址。
	//
	// 为什么需要：ac_gb_media_server 的 ip 是**从 backend 视角**填的
	// （本地开发常见 127.0.0.1:8082）。Pipecat 是另一个容器，127.0.0.1
	// 指向它自己，WebRTC push 会连不上。compose 部署时把节点地址改成
	// zlm:80 即可；不想改库时用这个环境变量单独覆盖。
	envPipecatZLMBaseURL = "PIPECAT_ZLM_BASE_URL"

	// startSendRTPRetryDelay 是 startSendRtp 重试间隔。
	//
	// 为什么需要重试：设备收到 INVITE 200 OK 后还要回 ACK 才真正开始推流，
	// ZLM 的源流可能晚于本函数的第一次调用出现。强行失败会把一个正常的
	// 对讲判成「Agent 不可用」。
	startSendRTPRetryDelay = 300 * time.Millisecond
)

// GbVoiceService 语音广播/对讲业务逻辑。
type GbVoiceService struct {
	channelRepo *repository.GbChannelRepo
	sessionRepo *repository.VoiceSessionRepo
}

// NewGbVoiceService 构造语音服务。
func NewGbVoiceService() *GbVoiceService {
	return &GbVoiceService{
		channelRepo: repository.NewGbChannelRepo(),
		sessionRepo: repository.NewVoiceSessionRepo(),
	}
}

// StartBroadcast 开始语音广播。
//
// 广播必须由通道的 enable_broadcast 开启：这是业务侧的显式授权，
// 不能因为接口能调通就默认允许向现场喇叭喊话。
func (s *GbVoiceService) StartBroadcast(ctx context.Context, channelID int64, userID int64) (*response.GbVoiceSessionResp, error) {
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return nil, err
	}
	if !boolValue(ch.EnableBroadcast) {
		return nil, ErrGbBroadcastDisabled
	}
	return s.start(ctx, ch, dev, userID, model.VoiceSessionBroadcast, false)
}

// StartTalk 开始语音对讲。
//
// agentEnabled 只是「希望接入 Pipecat」的意愿；控制面未配置/不可达或
// 音频桥未完全连通时自动降级为纯对讲，响应里的 agent_enabled=false，
// 不把整个会话判失败。
func (s *GbVoiceService) StartTalk(ctx context.Context, channelID int64, userID int64, agentEnabled bool) (*response.GbVoiceSessionResp, error) {
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return nil, err
	}
	return s.start(ctx, ch, dev, userID, model.VoiceSessionTalk, agentEnabled)
}

// start 是广播/对讲的公共编排。
func (s *GbVoiceService) start(ctx context.Context, ch *model.GbChannel, dev *model.Device, userID int64, sessionType string, wantAgent bool) (*response.GbVoiceSessionResp, error) {
	// 同通道并发控制：设备同一时刻通常只接受一路语音会话。
	active, err := s.sessionRepo.ListActiveByChannel(ctx, ch.ID)
	if err != nil {
		return nil, err
	}
	if len(active) > 0 {
		return nil, ErrGbVoiceSessionBusy
	}

	svc, err := play.SIPService()
	if err != nil {
		return nil, mapLivePlayErr(err)
	}
	client, ms, err := play.ResolveMediaServer(ctx, dev)
	if err != nil {
		return nil, mapLivePlayErr(err)
	}

	ssrcStr, _, err := svc.AllocateSSRC()
	if err != nil {
		return nil, fmt.Errorf("分配语音 SSRC 失败：%w", err)
	}

	sessionName := play.SessionNameTalk
	if sessionType == model.VoiceSessionBroadcast {
		sessionName = play.SessionNameBroadcast
	}
	ip := svc.MediaPublicIP()
	if strings.TrimSpace(dev.SDPIP) != "" {
		ip = strings.TrimSpace(dev.SDPIP)
	}

	sdpBody := play.BuildAudioSDP(dev, ip, svc.RTPProxyPort(), ssrcStr, sessionName)
	target := sipserver.CallTarget{
		Device:      dev,
		ChannelID:   ch.GbChannelID,
		SessionName: sessionName,
		Subject:     play.FormatSubject(ch.GbChannelID, ssrcStr, svc.Domain()),
		ContentType: sipserver.ContentTypeSDP,
		SSRC:        ssrcStr,
	}

	dlg, err := svc.SendInvite(ctx, target, []byte(sdpBody), "")
	if err != nil {
		svc.ReleaseSSRC(ssrcStr)
		return nil, fmt.Errorf("建立语音通道失败：%w", err)
	}

	ssrcNum, err := play.SSRCToUint32(ssrcStr)
	if err != nil {
		cleanupVoiceSession(dlg, client, svc, ssrcStr)
		return nil, err
	}

	// ── 语音 Agent：只在对讲场景尝试，失败一律降级 ──────────────
	agent := false
	agentStream := ""
	uplinkPort := 0
	var pc *pipecatClient

	if wantAgent && sessionType == model.VoiceSessionTalk {
		baseURL, token, ok := resolvePipecatConfig(ctx)
		if !ok {
			utils.Warn("语音 Agent 降级为纯对讲：未配置 Pipecat 控制面地址/令牌")
		} else if control, cerr := newPipecatClient(baseURL, token); cerr != nil {
			utils.Warn("语音 Agent 降级为纯对讲：Pipecat 控制面地址非法", zap.Error(cerr))
		} else if !control.health(ctx) {
			utils.Warn("语音 Agent 降级为纯对讲：Pipecat 控制面不可达",
				zap.String("base_url", baseURL))
		} else {
			agentStream = audio.AgentStreamName(dlg.Stream)
			zlmBaseURL := strings.TrimSpace(os.Getenv(envPipecatZLMBaseURL))
			if zlmBaseURL == "" {
				zlmBaseURL = client.BaseURL
			}
			startResp, serr := control.startSession(ctx, pipecatStartParams{
				SessionID:      dlg.CallID,
				ChannelID:      ch.GbChannelID,
				Mode:           voiceModeHalfDuplex,
				UplinkPort:     0, // 0 = 由 Pipecat 分配空闲 UDP 端口
				UplinkSSRC:     ssrcNum,
				DownlinkStream: agentStream,
				DownlinkSSRC:   ssrcNum,
				ZLMBaseURL:     zlmBaseURL,
				ZLMSecret:      client.Secret,
				ZLMApp:         dlg.App,
			})
			switch {
			case serr != nil:
				utils.Warn("语音 Agent 降级为纯对讲：启动 Pipecat 会话失败",
					zap.String("session_id", dlg.CallID), zap.Error(serr))
			case !startResp.OK || !audio.DecideAgent(true, true, startResp.AgentReady) ||
				startResp.UplinkPort <= 0:
				utils.Warn("语音 Agent 降级为纯对讲：Pipecat 音频桥未完全连通",
					zap.String("session_id", dlg.CallID),
					zap.String("audio_bridge", startResp.AudioBridge),
					zap.Int("uplink_port", startResp.UplinkPort),
					zap.String("detail", startResp.Detail))
				// 不保留半连通会话，避免占着 UDP 端口和 provider 资源。
				_ = control.stopSession(ctx, dlg.CallID)
			default:
				pc = control
				agent = true
				uplinkPort = startResp.UplinkPort
			}
		}
	}

	now := time.Now()
	agentPtr := agent
	m := &model.VoiceSession{
		SessionID:     dlg.CallID,
		Type:          sessionType,
		DeviceID:      &dev.ID,
		ChannelID:     &ch.ID,
		GbChannelID:   ch.GbChannelID,
		MediaServerID: ms.ServerID,
		App:           dlg.App,
		// Stream 保存设备流名；若接了 Pipecat，再以 "|端口" 追加会话的
		// UDP 接收端口（ac_voice_session 没有端口列，见
		// audio.EncodeSessionStream 的说明）。Agent 下行流由
		// audio.AgentStreamName(设备流) 确定性推导。
		Stream:       audio.EncodeSessionStream(dlg.Stream, uplinkPort),
		SSRC:         ssrcStr,
		AudioCodec:   voiceAudioCodec,
		AgentEnabled: &agentPtr,
		StartDT:      &now,
		Status:       voiceStatusActive,
		DeptID:       ch.DeptID,
	}
	if userID > 0 {
		uid := userID
		m.UserID = &uid
	}

	if err := s.sessionRepo.Create(ctx, m); err != nil {
		// 落库失败必须回滚 SIP 与 Pipecat，否则设备侧会一直挂着一路无主音频。
		s.abortAgent(ctx, pc, dlg.CallID)
		cleanupVoiceSession(dlg, client, svc, ssrcStr)
		return nil, fmt.Errorf("保存语音会话失败：%w", err)
	}

	// ── 上行：ZLM 把设备音频推到 Pipecat UDP 接收端 ─────────────
	if agent && pc != nil {
		udpHost := pc.udpHost
		if udpHost == "" {
			udpHost = pc.resolveUDPHost()
		}
		err := s.startSendRTP(ctx, client, mediaserver.SendRTPParam{
			App:       dlg.App,
			Stream:    dlg.Stream,
			SSRC:      ssrcNum,
			DstURL:    udpHost,
			DstPort:   uplinkPort,
			IsUDP:     true,
			PT:        play.AudioPayloadTypePCMA,
			OnlyAudio: true,
		}, 10)
		if err != nil {
			utils.Warn("语音 Agent 上行 startSendRtp 失败，降级为纯对讲",
				zap.String("session_id", dlg.CallID),
				zap.String("dst", fmt.Sprintf("%s:%d", udpHost, uplinkPort)),
				zap.Error(err))
			s.abortAgent(ctx, pc, dlg.CallID)
			pc = nil
			agent = false
		}
	}

	// ── 下行：ZLM 把 Pipecat 的 TTS 流推到设备喇叭 ──────────────
	if agent && pc != nil {
		if err := s.startDownlinkRTP(ctx, client, dlg, agentStream, ssrcNum, sdp.ParseTransport(dev.StreamMode).IsTCP()); err != nil {
			utils.Warn("语音 Agent 下行 startSendRtp 失败，降级为纯对讲",
				zap.String("session_id", dlg.CallID), zap.Error(err))
			// 上行已经建立，也要一并撤掉，避免 Pipecat 对着没人听的流跑 STT。
			s.stopUplinkRTP(client, dlg.App, dlg.Stream, ssrcNum)
			s.abortAgent(ctx, pc, dlg.CallID)
			pc = nil
			agent = false
		}
	}

	// 广播没有音频源（Web/App 推流尚未接入），不伪造 startSendRtp 成功。
	if sessionType == model.VoiceSessionBroadcast {
		utils.Info("语音广播：仅建立 SIP 通道；音频源需由 Web/App 推流后另行接入",
			zap.String("session_id", dlg.CallID))
	}

	if !agent && boolValue(m.AgentEnabled) {
		// 理论上不会走到这里，防御性同步数据库与响应。
		agentPtr = false
		m.AgentEnabled = &agentPtr
	}
	if agent != boolValue(m.AgentEnabled) {
		if err := s.sessionRepo.UpdateFields(ctx, m.ID, map[string]interface{}{"agent_enabled": agent}); err != nil {
			utils.Warn("更新语音会话 agent_enabled 失败",
				zap.String("session_id", dlg.CallID), zap.Error(err))
		}
		agentPtr = agent
		m.AgentEnabled = &agentPtr
	}

	return toVoiceSessionResp(m), nil
}

// startDownlinkRTP 建立 Pipecat → 设备喇叭的回推。
//
// 重试是必要的：WebRTC push 的 answer 返回后，ZLM 内部流的注册与
// startSendRtp 之间存在毫秒级竞态；第一次调用可能报「流不存在」。
func (s *GbVoiceService) startDownlinkRTP(ctx context.Context, client *mediaserver.Client, dlg *session.Dialog, agentStream string, ssrc uint32, tcpDevice bool) error {
	dstIP, dstPort, ok := play.ParseAudioEndpoint(dlg.DeviceSDP)
	if !ok {
		return errors.New("设备 SDP 未给出音频接收地址（m=audio）")
	}
	if client == nil || agentStream == "" {
		return errors.New("回推参数不完整")
	}
	// 设备的传输模式决定 ZLM 用 UDP 还是 TCP 发包：
	// SDP 里 m=audio 是 TCP/RTP/AVP 时仍按 UDP 发，设备会完全收不到。
	return s.startSendRTP(ctx, client, mediaserver.SendRTPParam{
		App:       dlg.App,
		Stream:    agentStream,
		SSRC:      ssrc,
		DstURL:    dstIP,
		DstPort:   dstPort,
		IsUDP:     !tcpDevice,
		PT:        play.AudioPayloadTypePCMA,
		OnlyAudio: true,
	}, 5)
}

// startSendRTP 调用 ZLM startSendRtp，带有限次重试。
//
// 所有失败都返回给调用方决定是否降级；这里不吞错，避免「接口成功、
// 设备没声音」再次发生。
func (s *GbVoiceService) startSendRTP(ctx context.Context, client *mediaserver.Client, param mediaserver.SendRTPParam, attempts int) error {
	if client == nil {
		return errors.New("媒体节点客户端为空")
	}
	if attempts < 1 {
		attempts = 1
	}
	// 整体上限：单次重试 3s，但 10 次累计不能把 StartTalk 拖到 30s。
	// 6s 足够覆盖「设备刚 ACK、ZLM 源流尚未出现」的窗口。
	retryCtx, retryCancel := context.WithTimeout(ctx, 6*time.Second)
	defer retryCancel()

	var lastErr error
	for i := 0; i < attempts; i++ {
		cctx, cancel := context.WithTimeout(retryCtx, 3*time.Second)
		_, err := client.StartSendRTP(cctx, param)
		cancel()
		if err == nil {
			if i > 0 {
				utils.Info("startSendRtp 重试成功",
					zap.String("stream", param.Stream), zap.Int("attempt", i+1))
			}
			return nil
		}
		lastErr = err
		if i < attempts-1 {
			select {
			case <-retryCtx.Done():
				return lastErr
			case <-time.After(startSendRTPRetryDelay):
			}
		}
	}
	return lastErr
}

// stopUplinkRTP 尽力停止一路回推（停止路径不希望因为流已被回收而报错）。
func (s *GbVoiceService) stopUplinkRTP(client *mediaserver.Client, app, stream string, ssrc uint32) {
	if client == nil || stream == "" {
		return
	}
	cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.StopSendRTP(cctx, app, stream, ssrc); err != nil {
		utils.Warn("停止语音回推失败（流可能已被回收）",
			zap.String("app", app), zap.String("stream", stream), zap.Error(err))
	}
}

// abortAgent 关闭 Pipecat 会话，失败只告警。
func (s *GbVoiceService) abortAgent(ctx context.Context, pc *pipecatClient, sessionID string) {
	if pc == nil {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pc.stopSession(cctx, sessionID); err != nil {
		utils.Warn("关闭 Pipecat 会话失败", zap.String("session_id", sessionID), zap.Error(err))
	}
}

// StopBroadcast 停止广播会话。
func (s *GbVoiceService) StopBroadcast(ctx context.Context, sessionID string) error {
	return s.stop(ctx, sessionID, model.VoiceSessionBroadcast)
}

// StopTalk 停止对讲会话。
func (s *GbVoiceService) StopTalk(ctx context.Context, sessionID string) error {
	return s.stop(ctx, sessionID, model.VoiceSessionTalk)
}

// StopBroadcastByChannel 按通道停止进行中的广播（前端只传 channelId 的便捷入口）。
func (s *GbVoiceService) StopBroadcastByChannel(ctx context.Context, channelID int64) error {
	return s.stopByChannel(ctx, channelID, model.VoiceSessionBroadcast)
}

// StopTalkByChannel 按通道停止进行中的对讲。
func (s *GbVoiceService) StopTalkByChannel(ctx context.Context, channelID int64) error {
	return s.stopByChannel(ctx, channelID, model.VoiceSessionTalk)
}

// stopByChannel 找到该通道进行中的同类会话并停止。
func (s *GbVoiceService) stopByChannel(ctx context.Context, channelID int64, wantType string) error {
	list, err := s.sessionRepo.ListActiveByChannel(ctx, channelID)
	if err != nil {
		return err
	}
	for _, m := range list {
		if m.Type == wantType {
			return s.stop(ctx, m.SessionID, wantType)
		}
	}
	return ErrGbVoiceSessionNotFound
}

// stop 结束一路语音会话。
//
// 幂等：已结束的会话再次停止返回 nil（前端重复点击不报错）。
// 清理顺序：
//  1. 停 ZLM 的两路 startSendRtp（先停发包，避免 Pipecat 关闭后流消失导致停不掉）；
//  2. 关闭 Pipecat 会话（UDP 接收线程 + WebRTC push）；
//  3. 发 BYE、回收 SSRC、更新数据库。
//
// 信令服务未启动时只落库结束状态；Pipecat 清理独立于 SIP 可用性。
func (s *GbVoiceService) stop(ctx context.Context, sessionID, wantType string) error {
	sessionID = trim(sessionID)
	if sessionID == "" {
		return ErrGbVoiceSessionNotFound
	}

	m, err := s.sessionRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbVoiceSessionNotFound
		}
		return err
	}
	if m.Type != wantType {
		return ErrGbVoiceSessionTypeMismatch
	}
	if m.Status != voiceStatusActive {
		return nil
	}

	var errs []error

	// 1) ZLM：上行（设备流）与下行（Agent 流）分别停止。
	var ssrcNum uint32
	if v, perr := play.SSRCToUint32(m.SSRC); perr == nil {
		ssrcNum = v
	}
	// 用后台上下文解析节点：用户关页面/断网导致的 ctx 取消不应该
	// 让设备侧一直挂着一路 RTP。
	resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	client, _, cerr := play.ResolveMediaServer(resolveCtx, &model.Device{MediaServerID: m.MediaServerID})
	resolveCancel()
	if cerr == nil {
		deviceStream, _ := audio.DecodeSessionStream(m.Stream)
		if deviceStream == "" {
			deviceStream = m.Stream
		}
		s.stopUplinkRTP(client, m.App, deviceStream, ssrcNum)
		if boolValue(m.AgentEnabled) {
			agentStream := audio.AgentStreamName(deviceStream)
			s.stopUplinkRTP(client, m.App, agentStream, ssrcNum)
			cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if err := client.CloseStreams(cctx, m.App, agentStream); err != nil {
				utils.Warn("清理 Agent 下行流失败（可能已由 WebRTC 对端关闭）",
					zap.String("stream", agentStream), zap.Error(err))
			}
			cancel()
		}
	} else {
		utils.Warn("停止语音会话：无法解析媒体节点，跳过 ZLM 清理",
			zap.String("session_id", m.SessionID), zap.Error(cerr))
	}

	// 2) Pipecat：即使 SIP 服务不可用也要关，否则 UDP 端口与 provider 资源会泄漏。
	if boolValue(m.AgentEnabled) {
		if baseURL, token, ok := resolvePipecatConfig(ctx); ok {
			if pc, cerr := newPipecatClient(baseURL, token); cerr == nil {
				cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				if err := pc.stopSession(cctx, m.SessionID); err != nil {
					utils.Warn("关闭 Pipecat 会话失败", zap.String("session_id", m.SessionID), zap.Error(err))
				}
				cancel()
			}
		}
	}

	// 3) SIP BYE + SSRC 回收。
	if svc, serr := play.SIPService(); serr == nil {
		if dlg, ok := svc.Sessions().Get(m.SessionID); ok {
			cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := dlg.Bye(cctx); err != nil {
				errs = append(errs, fmt.Errorf("发送 BYE 失败：%w", err))
			}
			cancel()
		}
		svc.ReleaseSSRC(m.SSRC)
	}

	now := time.Now()
	fields := map[string]interface{}{
		"status":     voiceStatusStopped,
		"end_dt":     now,
		"end_reason": "用户停止",
	}
	if err := s.sessionRepo.UpdateFields(ctx, m.ID, fields); err != nil {
		errs = append(errs, fmt.Errorf("更新语音会话状态失败：%w", err))
	}
	return errors.Join(errs...)
}

// cleanupVoiceSession 回滚一个尚未落库的 SIP 会话。
func cleanupVoiceSession(dlg *session.Dialog, client *mediaserver.Client, svc *sipserver.Service, ssrc string) {
	if dlg != nil {
		cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = dlg.Bye(cctx)
		cancel()
	}
	if client != nil && dlg != nil {
		cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.CloseStreams(cctx, dlg.App, dlg.Stream)
		cancel()
	}
	if svc != nil {
		svc.ReleaseSSRC(ssrc)
	}
}

// ── 响应转换 ──────────────────────────────────────────────────

// toVoiceSessionResp 把会话模型转成契约响应。
func toVoiceSessionResp(m *model.VoiceSession) *response.GbVoiceSessionResp {
	if m == nil {
		return nil
	}
	start := ""
	if m.StartDT != nil {
		start = m.StartDT.Format("2006-01-02 15:04:05")
	}
	return &response.GbVoiceSessionResp{
		SessionID:    m.SessionID,
		Type:         m.Type,
		Mode:         voiceModeHalfDuplex,
		AgentEnabled: boolValue(m.AgentEnabled),
		SSRC:         m.SSRC,
		AudioCodec:   m.AudioCodec,
		StartDT:      start,
	}
}

// ════════════════════════════════════════════════════════════════
// Pipecat 控制面客户端
// ════════════════════════════════════════════════════════════════

// pipecatStartParams 是 POST /session/start 的请求体。
//
// 端口/SSRC/流名全部由控制面传入（设计 §H+：不在 Pipecat 代码里写死），
// 这样同一份 Pipecat 镜像可以服务多路会话与不同媒体节点。
type pipecatStartParams struct {
	SessionID      string `json:"session_id"`
	ChannelID      string `json:"channel_id"`
	Mode           string `json:"mode"`
	UplinkPort     int    `json:"uplink_port"`
	UplinkSSRC     uint32 `json:"uplink_ssrc"`
	DownlinkStream string `json:"downlink_stream"`
	DownlinkSSRC   uint32 `json:"downlink_ssrc"`
	ZLMBaseURL     string `json:"zlm_base_url"`
	ZLMSecret      string `json:"zlm_secret"`
	ZLMApp         string `json:"zlm_app"`
}

// pipecatStartResponse 是 POST /session/start 的响应。
type pipecatStartResponse struct {
	OK             bool   `json:"ok"`
	UplinkPort     int    `json:"uplink_port"`
	DownlinkStream string `json:"downlink_stream"`
	AudioBridge    string `json:"audio_bridge"`
	AgentReady     bool   `json:"agent_ready"`
	Detail         string `json:"detail"`
}

// pipecatClient 是薄 HTTP 客户端（不引入新依赖）。
type pipecatClient struct {
	baseURL string
	token   string
	udpHost string
	http    *http.Client
}

// newPipecatClient 校验地址并解析 ZLM 可用的 UDP 目标地址。
func newPipecatClient(baseURL, token string) (*pipecatClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("Pipecat 控制面地址为空")
	}
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("Pipecat 控制面地址非法：%s", baseURL)
	}
	return &pipecatClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		udpHost: strings.TrimSpace(os.Getenv(envPipecatUDPHost)),
		// /session/start 内部要等 WebRTC 连接（默认 5s）与管线启动（1.5s），
		// 6s 会误杀「其实正在建立」的会话；15s 留出余量。
		http: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// resolveUDPHost 返回 ZLM startSendRtp 可用的 IPv4/主机名。
//
// 为什么要把容器名解析成 IP：ZLM 的 startSendRtp 在部分版本里只接受
// IP 字面量；把 docker DNS 的解析工作放在 Go 侧更可控。
// 解析失败时退回原始主机名，让 ZLM 自己尝试，不直接失败。
func (c *pipecatClient) resolveUDPHost() string {
	if c == nil {
		return ""
	}
	if c.udpHost != "" {
		return c.udpHost
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		return host
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		utils.Warn("解析 Pipecat UDP 主机失败，交给 ZLM 再试",
			zap.String("host", host), zap.Error(err))
		return host
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
			c.udpHost = addr
			return addr
		}
	}
	c.udpHost = addrs[0]
	return addrs[0]
}

// health 探测控制面存活（不校验 token，/health 本身免鉴权）。
func (c *pipecatClient) health(ctx context.Context) bool {
	if c == nil {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// startSession 启动 Pipecat 会话并返回真实 UDP 接收端口。
func (c *pipecatClient) startSession(ctx context.Context, params pipecatStartParams) (*pipecatStartResponse, error) {
	if c == nil {
		return nil, errors.New("Pipecat 客户端为空")
	}
	if c.token == "" {
		return nil, errors.New("未配置 pipecat_token，无法调用控制面会话接口")
	}
	var out pipecatStartResponse
	if err := c.postJSON(ctx, "/session/start", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// stopSession 停止 Pipecat 会话；404 视为已停止（幂等）。
func (c *pipecatClient) stopSession(ctx context.Context, sessionID string) error {
	if c == nil || sessionID == "" {
		return nil
	}
	if c.token == "" {
		return errors.New("未配置 pipecat_token")
	}
	err := c.postJSON(ctx, "/session/stop", map[string]string{"session_id": sessionID}, nil)
	if err != nil && strings.Contains(err.Error(), "HTTP 404") {
		return nil
	}
	return err
}

// postJSON 发送 JSON 请求并把非 2xx 转成带响应体的错误。
func (c *pipecatClient) postJSON(ctx context.Context, path string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化 Pipecat 请求失败：%w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("构造 Pipecat 请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("连接 Pipecat 控制面失败：%w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Pipecat %s 返回 HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("解析 Pipecat 响应失败：%w（原文 %s）", err, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// resolvePipecatConfig 读取控制面地址与令牌。
//
// 优先 ac_thirdparty_config（后台「语音 Agent 配置」保存的值，设计 §8.2），
// 再用环境变量兜底：本地开发/容器编排可以不改库直接联调。
func resolvePipecatConfig(ctx context.Context) (baseURL, token string, ok bool) {
	if items, err := repository.NewSettingRepo().ListThirdPartyConfigs(ctx, pipecatConfigCategory); err == nil {
		for _, item := range items {
			switch strings.TrimSpace(item.Code) {
			case pipecatEndpointKey:
				baseURL = strings.TrimSpace(item.Value)
			case pipecatTokenKey:
				token = strings.TrimSpace(item.Value)
			}
		}
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envPipecatBaseURL))
	}
	if token == "" {
		token = strings.TrimSpace(os.Getenv(envPipecatToken))
	}
	if baseURL == "" {
		return "", "", false
	}
	return baseURL, token, true
}
