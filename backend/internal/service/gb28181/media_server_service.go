package gb28181

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 流媒体节点（ZLMediaKit）
// ════════════════════════════════════════════════════════════════

// GbMediaServerService 流媒体节点业务逻辑。
type GbMediaServerService struct {
	repo *repository.GbMediaServerRepo
}

// NewGbMediaServerService 构造媒体节点服务。
func NewGbMediaServerService() *GbMediaServerService {
	return &GbMediaServerService{repo: repository.NewGbMediaServerRepo()}
}

// List 媒体节点简单列表（节点数量天然很少，不分页）。
//
// ds 为可变参数：契约签名是 List(ctx)，但接口契约 §10.0 又要求列表施加数据权限。
// 用变参同时满足两种调用方式，Handler 会传入 DataScope。
func (s *GbMediaServerService) List(ctx context.Context, ds ...*utils.DataScope) ([]response.GbMediaServerItem, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.GbMediaServer{})
	if len(ds) > 0 && ds[0] != nil {
		q = ds[0].ApplyToQuery(q, "ac_gb_media_server.dept_id")
	}

	var nodes []model.GbMediaServer
	if err := q.Order("default_server DESC, id ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	out := make([]response.GbMediaServerItem, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, toGbMediaServerItem(n))
	}
	return out, nil
}

// Get 媒体节点详情。
func (s *GbMediaServerService) Get(ctx context.Context, id int64) (*response.GbMediaServerDetailResp, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbMediaServerNotFound
		}
		return nil, err
	}
	return &response.GbMediaServerDetailResp{GbMediaServerItem: toGbMediaServerItem(*m)}, nil
}

// Create 新增媒体节点。
func (s *GbMediaServerService) Create(ctx context.Context, req *request.CreateGbMediaServerReq) (*model.GbMediaServer, error) {
	name := trim(req.Name)
	if name == "" {
		return nil, errors.New("节点名称不能为空")
	}
	serverID := trim(req.ServerID)
	if serverID == "" {
		return nil, errors.New("节点唯一标识不能为空")
	}

	if _, err := s.repo.GetByServerID(ctx, serverID); err == nil {
		return nil, ErrGbMediaServerIDExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	m := &model.GbMediaServer{
		Name:              name,
		ServerID:          serverID,
		Type:              firstNonEmpty(req.Type, "zlm"),
		IP:                trim(req.IP),
		HookIP:            trim(req.HookIP),
		SDPIP:             trim(req.SDPIP),
		StreamIP:          trim(req.StreamIP),
		HTTPPort:          req.HTTPPort,
		HTTPSSLPort:       req.HTTPSSLPort,
		RTMPPort:          req.RTMPPort,
		RTMPSSLPort:       req.RTMPSSLPort,
		RTSPPort:          req.RTSPPort,
		RTSPSSLPort:       req.RTSPSSLPort,
		FLVPort:           req.FLVPort,
		FLVSSLPort:        req.FLVSSLPort,
		WSFLVPort:         req.WSFLVPort,
		WSFLVSSLPort:      req.WSFLVSSLPort,
		MP4Port:           req.MP4Port,
		MP4SSLPort:        req.MP4SSLPort,
		Secret:            applyServerSecret(req.Secret, ""),
		RTPEnable:         boolOr(req.RTPEnable, true),
		RTPPortRange:      firstNonEmpty(req.RTPPortRange, "30000-30500"),
		RTPProxyPort:      req.RTPProxyPort,
		JTTProxyPort:      req.JTTProxyPort,
		SendRTPPortRange:  trim(req.SendRTPPortRange),
		RecordAssistPort:  req.RecordAssistPort,
		RecordPath:        trim(req.RecordPath),
		RecordDay:         req.RecordDay,
		TranscodeSuffix:   trim(req.TranscodeSuffix),
		HookAliveInterval: req.HookAliveInterval,
		DefaultServer:     boolOr(req.DefaultServer, false),
		AutoConfig:        boolOr(req.AutoConfig, false),
		Remark:            trim(req.Remark),
		Active:            boolOr(req.Active, true),
		Online:            boolOr(nil, false),
	}
	if m.RecordDay <= 0 {
		m.RecordDay = 7
	}
	if m.HookAliveInterval <= 0 {
		m.HookAliveInterval = 15
	}

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	if boolValue(m.DefaultServer) {
		if err := s.repo.ClearDefaultExcept(ctx, m.ID); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Update 编辑媒体节点。
//
// 全部字段增量更新；secret 为空 / **** 时保留原值。
func (s *GbMediaServerService) Update(ctx context.Context, id int64, req *request.UpdateGbMediaServerReq) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbMediaServerNotFound
		}
		return err
	}

	fields := map[string]interface{}{}

	if req.Name != nil {
		if v := trim(*req.Name); v == "" {
			return errors.New("节点名称不能为空")
		} else {
			fields["name"] = v
		}
	}
	if req.ServerID != nil {
		v := trim(*req.ServerID)
		if v == "" {
			return errors.New("节点唯一标识不能为空")
		}
		if v != m.ServerID {
			if _, err := s.repo.GetByServerID(ctx, v); err == nil {
				return ErrGbMediaServerIDExists
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			fields["server_id"] = v
		}
	}

	setString := func(key string, v *string) {
		if v != nil {
			fields[key] = trim(*v)
		}
	}
	setInt := func(key string, v *int64) {
		if v != nil {
			fields[key] = *v
		}
	}
	setBool := func(key string, v *bool) {
		if v != nil {
			fields[key] = *v
		}
	}

	setString("type", req.Type)
	setString("ip", req.IP)
	setString("hook_ip", req.HookIP)
	setString("sdp_ip", req.SDPIP)
	setString("stream_ip", req.StreamIP)
	setInt("http_port", req.HTTPPort)
	setInt("http_ssl_port", req.HTTPSSLPort)
	setInt("rtmp_port", req.RTMPPort)
	setInt("rtmp_ssl_port", req.RTMPSSLPort)
	setInt("rtsp_port", req.RTSPPort)
	setInt("rtsp_ssl_port", req.RTSPSSLPort)
	setInt("flv_port", req.FLVPort)
	setInt("flv_ssl_port", req.FLVSSLPort)
	setInt("ws_flv_port", req.WSFLVPort)
	setInt("ws_flv_ssl_port", req.WSFLVSSLPort)
	setInt("mp4_port", req.MP4Port)
	setInt("mp4_ssl_port", req.MP4SSLPort)
	setBool("rtp_enable", req.RTPEnable)
	setString("rtp_port_range", req.RTPPortRange)
	setInt("rtp_proxy_port", req.RTPProxyPort)
	setInt("jtt_proxy_port", req.JTTProxyPort)
	setString("send_rtp_port_range", req.SendRTPPortRange)
	setInt("record_assist_port", req.RecordAssistPort)
	setString("record_path", req.RecordPath)
	setInt("record_day", req.RecordDay)
	setString("transcode_suffix", req.TranscodeSuffix)
	setInt("hook_alive_interval", req.HookAliveInterval)
	setBool("default_server", req.DefaultServer)
	setBool("auto_config", req.AutoConfig)
	setString("remark", req.Remark)
	setBool("active", req.Active)

	if req.Secret != nil {
		fields["secret"] = applyServerSecret(*req.Secret, m.Secret)
	}

	if err := s.repo.UpdateFields(ctx, id, fields); err != nil {
		return err
	}
	if req.DefaultServer != nil && *req.DefaultServer {
		return s.repo.ClearDefaultExcept(ctx, id)
	}
	return nil
}

// Delete 软删除媒体节点。
func (s *GbMediaServerService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbMediaServerNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Check 检查节点连通性。
//
// 调用 ZLM 的 /index/api/getServerConfig。故意不依赖 internal/gb28181/mediaserver
// （该包由其他 Agent 实现）：HTTP 直连足以完成健康检查，且本批次编译不被阻塞。
func (s *GbMediaServerService) Check(ctx context.Context, id int64) (*response.GbMediaServerCheckResp, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbMediaServerNotFound
		}
		return nil, err
	}

	start := time.Now()
	resp := &response.GbMediaServerCheckResp{
		HTTPPort:      m.HTTPPort,
		MediaServerID: m.ServerID,
	}
	body, code, msg, err := zlmGet(ctx, *m, "getServerConfig")
	resp.ResponseElapsed = time.Since(start).Milliseconds()
	if err != nil {
		resp.Message = fmt.Sprintf("连接失败: %v", err)
		return resp, nil
	}
	if code != 0 {
		resp.Message = firstNonEmpty(msg, "ZLM 返回非 0 状态码，请检查 secret")
		return resp, nil
	}

	resp.Online = true
	resp.SecretMatched = true
	cfg := firstObject(body)
	resp.Version = pickString(cfg, "version", "general.version", "api.version")
	resp.RTPPort = pickInt64(cfg, "rtp_proxy.port")
	if resp.RTPPort > 0 {
		// ZLM 的 RTP 收流端口为偶数端口 + 下一端口作为 RTCP。
		resp.RTCPPort = resp.RTPPort + 1
	}
	if v := pickString(cfg, "general.mediaServerId", "mediaServerId"); v != "" {
		resp.MediaServerID = v
	}
	if p := pickInt64(cfg, "http.port"); p > 0 {
		resp.HTTPPort = p
	}
	resp.Message = "节点在线"
	return resp, nil
}

// ServerInfo 汇总节点运行信息：活跃流数与线程负载。
func (s *GbMediaServerService) ServerInfo(ctx context.Context, id int64) (*response.GbMediaServerInfoResp, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbMediaServerNotFound
		}
		return nil, err
	}

	resp := &response.GbMediaServerInfoResp{Loads: []response.GbMediaServerThreadLoad{}}

	mediaBody, code, msg, err := zlmGet(ctx, *m, "getMediaList")
	if err != nil {
		resp.Message = fmt.Sprintf("连接失败: %v", err)
		return resp, nil
	}
	if code != 0 {
		resp.Message = firstNonEmpty(msg, "getMediaList 返回非 0 状态码")
		return resp, nil
	}
	resp.Online = true
	resp.ActiveStreams = objectArrayLen(mediaBody)

	loadBody, code, msg, err := zlmGet(ctx, *m, "getThreadsLoad")
	if err != nil {
		resp.Message = fmt.Sprintf("读取线程负载失败: %v", err)
		return resp, nil
	}
	if code != 0 {
		resp.Message = firstNonEmpty(msg, "getThreadsLoad 返回非 0 状态码")
		return resp, nil
	}

	loads := firstObjectArray(loadBody)
	resp.ThreadCount = len(loads)
	var sum float64
	for i, item := range loads {
		load := pickFloat64(item, "load")
		name := pickString(item, "name", "thread_name", "key")
		if name == "" {
			name = "thread-" + strconv.Itoa(i)
		}
		resp.Loads = append(resp.Loads, response.GbMediaServerThreadLoad{ThreadName: name, Load: load})
		sum += load
		if load > resp.MaxLoad {
			resp.MaxLoad = load
		}
	}
	if resp.ThreadCount > 0 {
		resp.AvgLoad = sum / float64(resp.ThreadCount)
	}
	resp.Message = "节点在线"
	return resp, nil
}

// ── ZLM HTTP 客户端 ───────────────────────────────────────────

// zlmAPIResp ZLM 通用响应。
type zlmAPIResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// zlmGet 调用 ZLM HTTP API，返回 data 原始 JSON。
func zlmGet(ctx context.Context, m model.GbMediaServer, api string) ([]byte, int, string, error) {
	base, err := zlmBaseURL(m)
	if err != nil {
		return nil, 0, "", err
	}

	u, err := url.Parse(base + "/index/api/" + api)
	if err != nil {
		return nil, 0, "", fmt.Errorf("节点地址非法: %w", err)
	}
	if m.Secret != "" {
		q := u.Query()
		q.Set("secret", m.Secret)
		u.RawQuery = q.Encode()
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, "", err
	}
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, 0, "", err
	}
	var parsed zlmAPIResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, 0, "", fmt.Errorf("解析 ZLM 响应失败(HTTP %d): %w", res.StatusCode, err)
	}
	return parsed.Data, parsed.Code, parsed.Msg, nil
}

// zlmBaseURL 由节点配置拼出 API 基址。
func zlmBaseURL(m model.GbMediaServer) (string, error) {
	ip := trim(m.IP)
	if ip == "" {
		return "", errors.New("节点 IP 未配置")
	}
	base := ip
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if u.Port() == "" && m.HTTPPort > 0 {
		u.Host = net.JoinHostPort(u.Hostname(), strconv.FormatInt(m.HTTPPort, 10))
	}
	return u.String(), nil
}

// firstObject 解析形如 [{"key":"value"}] 的 data，返回第一个对象。
func firstObject(data []byte) map[string]interface{} {
	list := firstObjectArray(data)
	if len(list) == 0 {
		return map[string]interface{}{}
	}
	return list[0]
}

// firstObjectArray 解析 data 为对象数组。
func firstObjectArray(data []byte) []map[string]interface{} {
	var list []map[string]interface{}
	if err := json.Unmarshal(data, &list); err != nil {
		// 某些接口 data 直接是对象而非数组，兼容之。
		var single map[string]interface{}
		if err2 := json.Unmarshal(data, &single); err2 == nil {
			return []map[string]interface{}{single}
		}
		return nil
	}
	return list
}

// objectArrayLen 返回对象数组长度。
func objectArrayLen(data []byte) int { return len(firstObjectArray(data)) }

// pickString 从 map 中取第一个非空字符串值。
func pickString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			}
		}
	}
	return ""
}

// pickInt64 从 map 中取整数（ZLM 的值大多是字符串）。
func pickInt64(m map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			if n, err := strconv.ParseInt(t, 10, 64); err == nil {
				return n
			}
		case float64:
			return int64(t)
		}
	}
	return 0
}

// pickFloat64 从 map 中取浮点值。
func pickFloat64(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			if f, err := strconv.ParseFloat(t, 64); err == nil {
				return f
			}
		case float64:
			return t
		}
	}
	return 0
}

// applyServerSecret 敏感字段更新：空值 / **** 保留原值。
func applyServerSecret(newVal, oldVal string) string {
	return utils.ApplyMaskedUpdate(newVal, oldVal)
}
