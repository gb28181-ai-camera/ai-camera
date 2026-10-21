package gb28181

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
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
// 一期形态：
//  1. 平台向通道发 INVITE（SDP s=Broadcast / Talk，m=audio PCMA/8000）；
//  2. 设备 200 OK 的 SDP 给出「往哪里推音频」；
//  3. ZLM startSendRtp 回推音频到设备喇叭；
//  4. ac_voice_session 落库（active → stopped）。
//
// ★ 默认半双工：全双工必然自激（设计 §8.1.1）—— 设备喇叭的声音会被
// 自己的麦克风收回，形成「agent 听到自己 → 判定被打断 → 再说话」的死循环。
// 因此响应里的 mode 恒为 half_duplex；agent_enabled 只表示控制面是否接上。
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
// agentEnabled 只是「希望接入 Pipecat」的意愿；控制面未配置/不可达时
// 自动降级为纯对讲，响应里的 agent_enabled=false，不把整个会话判失败。
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

	// 语音 Agent 的降级：Pipecat 控制面不可达时只保留纯对讲。
	agent := wantAgent && pipecatAvailable(ctx)

	now := time.Now()
	agentPtr := agent
	m := &model.VoiceSession{
		SessionID:     dlg.CallID,
		Type:          sessionType,
		DeviceID:      &dev.ID,
		ChannelID:     &ch.ID,
		GbChannelID:   ch.GbChannelID,
		MediaServerID: ms.ServerID,
		App:           "rtp",
		Stream:        dlg.Stream,
		SSRC:          ssrcStr,
		AudioCodec:    voiceAudioCodec,
		AgentEnabled:  &agentPtr,
		StartDT:       &now,
		Status:        voiceStatusActive,
		DeptID:        ch.DeptID,
	}
	if userID > 0 {
		uid := userID
		m.UserID = &uid
	}

	if err := s.sessionRepo.Create(ctx, m); err != nil {
		// 落库失败必须回滚 SIP 会话，否则设备侧会一直挂着一路无主音频。
		cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = dlg.Bye(cctx)
		_ = client.CloseStreams(cctx, "rtp", dlg.Stream)
		cancel()
		svc.ReleaseSSRC(ssrcStr)
		return nil, fmt.Errorf("保存语音会话失败：%w", err)
	}

	// 回推音频。本期没有音频源发布者（Web/Pipecat 推流链路尚未接入），
	// startSendRtp 可能因源流不存在而失败；此时保留已建立的语音通道并告警，
	// 不把整个会话判失败 —— 否则本接口在音频源接入前永远不可用。
	s.startSendRTP(ctx, client, dlg, ssrcStr)

	return toVoiceSessionResp(m), nil
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
// 信令服务未启动时只落库结束状态：此时没有可操作的 SIP 资源，
// 继续报 503 只会让「停止」按钮永远失败。
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
	if svc, serr := play.SIPService(); serr == nil {
		cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		if dlg, ok := svc.Sessions().Get(m.SessionID); ok {
			if err := dlg.Bye(cctx); err != nil {
				errs = append(errs, fmt.Errorf("发送 BYE 失败：%w", err))
			}
		}

		if client, _, cerr := play.ResolveMediaServer(ctx, &model.Device{MediaServerID: m.MediaServerID}); cerr == nil {
			var ssrcNum uint32
			if v, perr := play.SSRCToUint32(m.SSRC); perr == nil {
				ssrcNum = v
			}
			if err := client.StopSendRTP(cctx, m.App, m.Stream, ssrcNum); err != nil {
				errs = append(errs, fmt.Errorf("停止音频回推失败：%w", err))
			}
		}
		cancel()
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

// startSendRTP 让 ZLM 把音频回推给设备。
//
// dst_url/dst_port 必须来自设备 200 OK 的 SDP（m=audio），不能编平台自己的地址：
// 后者会把音频打进 ZLM 本机，表现为「接口成功、设备没声音」。
func (s *GbVoiceService) startSendRTP(ctx context.Context, client *mediaserver.Client, dlg *session.Dialog, ssrc string) {
	if client == nil || dlg == nil {
		return
	}
	dstIP, dstPort, ok := play.ParseAudioEndpoint(dlg.DeviceSDP)
	if !ok {
		utils.Warn("语音回推：设备 SDP 未给出音频接收地址，跳过 startSendRtp",
			zap.String("session_id", dlg.CallID))
		return
	}
	ssrcNum, err := play.SSRCToUint32(ssrc)
	if err != nil {
		utils.Warn("语音回推：SSRC 非法", zap.String("ssrc", ssrc), zap.Error(err))
		return
	}

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = client.StartSendRTP(cctx, mediaserver.SendRTPParam{
		App:       dlg.App,
		Stream:    dlg.Stream,
		SSRC:      ssrcNum,
		DstURL:    dstIP,
		DstPort:   dstPort,
		IsUDP:     true,
		PT:        play.AudioPayloadTypePCMA,
		OnlyAudio: true,
	})
	if err != nil {
		// 一期没有音频源发布者，源流不存在时 ZLM 会拒绝；保留会话并告警。
		utils.Warn("语音回推 startSendRtp 失败（音频源未就绪时属预期）",
			zap.String("session_id", dlg.CallID),
			zap.String("dst", fmt.Sprintf("%s:%d", dstIP, dstPort)),
			zap.Error(err))
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

// pipecatAvailable 判断 Pipecat 控制面是否可用。
//
// 本期没有把 Pipecat 配置并入 config.yaml（语音 Agent 配置走
// ac_thirdparty_config，控制面地址尚未定稿），因此先用环境变量
// PIPECAT_BASE_URL 做最小探测：未配置 → false；配置了但 /health
// 超时/非 2xx → false。返回 false 时上层降级为纯对讲，不报错。
func pipecatAvailable(ctx context.Context) bool {
	base := strings.TrimSpace(os.Getenv("PIPECAT_BASE_URL"))
	if base == "" {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(cctx, http.MethodGet, strings.TrimRight(base, "/")+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		utils.Warn("语音 Agent 控制面不可达，降级为纯对讲",
			zap.String("base_url", base), zap.Error(err))
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		utils.Warn("语音 Agent 控制面健康检查失败，降级为纯对讲",
			zap.String("base_url", base), zap.Int("status", resp.StatusCode))
		return false
	}
	return true
}
