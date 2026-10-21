// Package gb28181 实现国标视频（GB/T 28181）业务 CRUD 层。
//
// 分层约定与其他领域一致：Handler → Service → Repository → Model。
// 本包只依赖 repository 中已有的国标方法；个别缺失的查询（设备分页、信令日志、
// 订阅）直接使用 bootstrap.DB() 写轻量查询，避免为了本批次去改动 repository
// 的公共文件（repository 由其他 Agent 负责）。
package gb28181

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// ════════════════════════════════════════════════════════════════
// 业务错误：Handler 依据 errors.Is 分类映射 HTTP 状态码。
// ════════════════════════════════════════════════════════════════

var (
	// ErrSipServiceUnavailable SIP 信令服务未启动（control.Default()==nil）。
	//
	// ★ 必须显式失败：SIP 未启动时若静默返回成功，前端会以为「已同步/已订阅」，
	// 但设备侧什么也没发生，问题要到很久以后才暴露。
	ErrSipServiceUnavailable = errors.New("SIP 信令服务未启动")

	// ErrGbDeviceNotFound 国标设备不存在（或未配置国标接入）。
	ErrGbDeviceNotFound = errors.New("国标设备不存在")
	// ErrGbDeviceNotConfigured 设备尚未生成国标编码，不能执行信令操作。
	ErrGbDeviceNotConfigured = errors.New("设备尚未配置国标编码，请先生成激活凭证")
	// ErrGbDeviceIDExists 国标编码已被其他设备占用。
	ErrGbDeviceIDExists = errors.New("国标编码已被占用")
	// ErrGbSubscribeTypeInvalid 订阅类型非法。
	ErrGbSubscribeTypeInvalid = errors.New("订阅类型非法，仅支持 catalog/alarm/mobile_position")

	// ErrGbChannelNotFound 国标通道不存在。
	ErrGbChannelNotFound = errors.New("国标通道不存在")

	// ErrGbMediaServerNotFound 媒体节点不存在。
	ErrGbMediaServerNotFound = errors.New("媒体节点不存在")
	// ErrGbMediaServerIDExists 节点唯一标识已存在。
	ErrGbMediaServerIDExists = errors.New("节点唯一标识已存在")

	// ErrGbGroupNotFound 业务分组不存在。
	ErrGbGroupNotFound = errors.New("业务分组不存在")
	// ErrGbGroupHasChildren 分组仍有子分组，不能删除。
	ErrGbGroupHasChildren = errors.New("该分组下存在子分组，不能删除")
	// ErrGbGroupInUse 分组仍被通道引用，不能删除。
	ErrGbGroupInUse = errors.New("该分组已被通道引用，不能删除")
	// ErrGbGroupCycle 分组父子关系成环。
	ErrGbGroupCycle = errors.New("父分组不能是自身或其子分组")

	// ErrGbSubscribeNotFound 订阅记录不存在。
	ErrGbSubscribeNotFound = errors.New("订阅记录不存在")

	// ErrGbSipLogNotFound 信令日志不存在。
	ErrGbSipLogNotFound = errors.New("信令日志不存在")
)

// ── 通用小工具 ────────────────────────────────────────────────

// trim 去除首尾空白。
func trim(s string) string { return strings.TrimSpace(s) }

// boolOr 指针为空时取默认值。
func boolOr(p *bool, def bool) *bool {
	if p != nil {
		return p
	}
	v := def
	return &v
}

// boolValue 解引用布尔指针，nil 视为 false。
func boolValue(p *bool) bool { return p != nil && *p }

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// maskItemSecret 生成脱敏后的展示值。
func maskItemSecret(s string) string { return utils.MaskSecret(s) }

// ════════════════════════════════════════════════════════════════
// 国标接入配置读取
//
// 优先级：bootstrap.Config().GB28181.SIP → 环境变量 → 内置默认值。
// 测试环境下 bootstrap 未加载配置文件（globalCfg=nil），此时靠环境变量/默认值兜底，
// 保证 GenerateActivation 在单测里也能独立运行。
// ════════════════════════════════════════════════════════════════

// gbSipConfig SIP 服务对外参数。
type gbSipConfig struct {
	Domain    string // 10 位国标域
	ServerIP  string // SIP 服务地址
	Port      int    // SIP 端口
	Transport string // UDP / TCP
}

// loadGbSipConfig 读取 SIP 配置；缺失或非法时回落到默认值。
func loadGbSipConfig() gbSipConfig {
	var domain, ip, transport string
	port := 0

	if cfg := bootstrap.Config(); cfg != nil {
		sip := cfg.GB28181.SIP
		// advertise_ip 才是设备可达地址；listen_ip=0.0.0.0 不能写进激活凭证。
		domain = sip.Domain
		ip = firstNonEmpty(sip.AdvertiseIP, sip.ListenIP)
		port = sip.Port
		transport = sip.Transport
	}

	// 环境变量显式覆盖（bootstrap 未加载配置时也走这里）。
	if v := os.Getenv("GB28181_SIP_DOMAIN"); v != "" {
		domain = v
	}
	if v := firstNonEmpty(os.Getenv("GB28181_SIP_SERVER_IP"), os.Getenv("GB28181_SIP_ADVERTISE_IP")); v != "" {
		ip = v
	}
	if v := os.Getenv("GB28181_SIP_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 65535 {
			port = n
		}
	}
	if v := os.Getenv("GB28181_SIP_TRANSPORT"); v != "" {
		transport = v
	}

	domain = firstNonEmpty(domain, "3402000000")
	// 国标域必须是 10 位；配置写错时不能让生成的 20 位编码变成 19/21 位。
	if len(domain) != 10 {
		domain = "3402000000"
	}

	ip = firstNonEmpty(ip, "127.0.0.1")
	if ip == "0.0.0.0" || ip == "::" || ip == "[::]" {
		// 监听全部网卡不等于设备可达，回落到本机回环，至少不会给出一个必然失败的地址。
		ip = "127.0.0.1"
	}
	if port <= 0 || port > 65535 {
		port = 5060
	}

	// BOTH 是服务端监听模式；激活凭证只能让设备二选一，默认 UDP。
	transport = strings.ToUpper(firstNonEmpty(transport, model.GbSipTransportUDP))
	if transport != model.GbSipTransportTCP {
		transport = model.GbSipTransportUDP
	}

	return gbSipConfig{Domain: domain, ServerIP: ip, Port: port, Transport: transport}
}

// sipServerAddr 返回 host:port 形式的 SIP 服务地址。
func (c gbSipConfig) sipServerAddr() string {
	return net.JoinHostPort(c.ServerIP, strconv.Itoa(c.Port))
}

// ════════════════════════════════════════════════════════════════
// 国标编码与密码生成
// ════════════════════════════════════════════════════════════════

// gbDeviceTypeCode 设备类型编码（GB/T 28181 附录 D，132=摄像机/IPC）。
const gbDeviceTypeCode = "132"

// generateGbDeviceID 生成 20 位国标设备编码：
//
//	<domain 10 位><类型 3 位 132><序号 7 位>
//
// 序号来源优先级：
//  1. ac_sequence 中 code=gb_device_id / gb28181_device_id 的行（原子自增）；
//  2. 否则取同前缀已有最大序号 + 1。
//
// 无论哪种来源，最后都会再与库中最大序号比较并做唯一性校验，
// 避免「序列落后于实际数据」导致的主键式冲突。
func generateGbDeviceID(ctx context.Context, cfg gbSipConfig) (string, error) {
	db := bootstrap.DB()
	prefix := cfg.Domain + gbDeviceTypeCode

	var maxSeq int64
	// ★ 这里**不能**过滤 deleted_at：国标编码是设备的 SIP 身份，
	// 一旦分配给某台设备就应视为永久占用。若只看未删除行，删掉设备后
	// 新设备会拿到同一个 20 位编码（实测连续 5 台测试设备全部得到
	// 34020000001320000001），旧设备的残留注册/会话/信令日志可能串到新设备。
	err := db.WithContext(ctx).Raw(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(gb_device_id FROM 14 FOR 7) AS BIGINT)), 0)
		FROM ac_device
		WHERE gb_device_id LIKE ?
		  AND char_length(gb_device_id) = 20`, prefix+"%").Scan(&maxSeq).Error
	if err != nil {
		return "", fmt.Errorf("查询国标编码最大序号失败: %w", err)
	}

	seq := maxSeq + 1
	if next, ok := nextFromSequence(ctx, []string{"gb_device_id", "gb28181_device_id", "gb_device"}); ok {
		if next > seq {
			seq = next
		}
	}

	for i := 0; i < 100; i++ {
		candidate := fmt.Sprintf("%s%07d", prefix, seq)
		exists, err := deviceGbIDExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		seq++
	}
	return "", errors.New("生成国标编码失败：连续 100 个候选均已被占用")
}

// nextFromSequence 从 ac_sequence 原子取下一个值。
//
// 没有匹配行（例如种子数据尚未加入国标序列）时返回 ok=false，由调用方回落到 max+1。
func nextFromSequence(ctx context.Context, codes []string) (int64, bool) {
	db := bootstrap.DB()
	for _, code := range codes {
		var val int64
		err := db.WithContext(ctx).Raw(`
			UPDATE ac_sequence
			SET current_val = current_val + GREATEST(step, 1),
			    updated_dt = CURRENT_TIMESTAMP
			WHERE code = ? AND deleted_at IS NULL
			RETURNING current_val`, code).Scan(&val).Error
		if err == nil && val > 0 {
			return val, true
		}
	}
	return 0, false
}

// deviceGbIDExists 判断国标编码是否已被占用。
func deviceGbIDExists(ctx context.Context, gbDeviceID string) (bool, error) {
	var cnt int64
	// 同样必须 Unscoped：软删除设备的国标编码仍算「已占用」。
	err := bootstrap.DB().WithContext(ctx).Unscoped().Model(&model.Device{}).
		Where("gb_device_id = ?", gbDeviceID).Count(&cnt).Error
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// sipPasswordAlphabet 随机密码字符集。
//
// 有意排除 0/O、1/l/I 等易混淆字符：激活凭证常由人工抄写到设备，
// 混淆字符会直接造成「密码正确但注册失败」的排障黑洞。
const sipPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

// generateSipPassword 生成 16 位随机 SIP Digest 密码。
func generateSipPassword(length int) (string, error) {
	if length <= 0 {
		length = 16
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成 SIP 密码失败: %w", err)
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = sipPasswordAlphabet[int(b)%len(sipPasswordAlphabet)]
	}
	return string(out), nil
}

// buildActivationQrContent 构造二维码内容（JSON 字符串）。
//
// 前端拿到后直接交给二维码组件渲染，不再二次拼接字段，
// 避免前后端对「该放哪些字段」理解不一致。
func buildActivationQrContent(a response.GbActivationResp) string {
	payload := map[string]interface{}{
		"gb_device_id": a.GbDeviceID,
		"sip_password": a.SipPassword,
		"sip_server":   a.SipServer,
		"sip_port":     a.SipPort,
		"sip_domain":   a.SipDomain,
		"transport":    a.Transport,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		// map[string]interface{} + 基础类型不会失败；兜底返回空串而不是 panic。
		return ""
	}
	return string(b)
}

// ════════════════════════════════════════════════════════════════
// 模型 → 响应 DTO 转换
// ════════════════════════════════════════════════════════════════

// toGbDeviceItem 把 ac_device 转为列表项，并对 sip_password 脱敏。
func toGbDeviceItem(d model.Device) response.GbDeviceItem {
	regionName := ""
	if d.Region != nil {
		regionName = d.Region.Name
	}
	return response.GbDeviceItem{
		ID:             d.ID,
		Name:           d.Name,
		DeviceCode:     d.DeviceCode,
		RegionID:       d.RegionID,
		RegionName:     regionName,
		Location:       d.Location,
		Latitude:       d.Latitude,
		Longitude:      d.Longitude,
		Status:         d.Status,
		Active:         d.Active,
		GbDeviceID:     d.GbDeviceID,
		SipPassword:    maskItemSecret(d.SipPassword),
		SipRealm:       d.SipRealm,
		SipTransport:   d.SipTransport,
		StreamMode:     d.StreamMode,
		Charset:        d.Charset,
		RegisterStatus: d.RegisterStatus,
		RegisterDT:     d.RegisterDT,
		SipExpires:     d.SipExpires,
		MediaServerID:  d.MediaServerID,
		ChannelCount:   d.ChannelCount,
		LocalIP:        d.LocalIP,
		SDPIP:          d.SDPIP,
		Remark:         d.Remark,
		CreatedDT:      d.CreatedDT,
		UpdatedDT:      d.UpdatedDT,
	}
}

// toGbChannelItem 把 ac_gb_channel 转为列表项，关联名称从 Preload 结果读取。
func toGbChannelItem(c model.GbChannel) response.GbChannelItem {
	deviceName, deviceCode := "", c.DeviceCode
	if c.Device != nil {
		deviceName = c.Device.Name
		if deviceCode == "" {
			deviceCode = c.Device.DeviceCode
		}
	}
	regionName := ""
	if c.Region != nil {
		regionName = c.Region.Name
	}
	groupName := ""
	if c.Group != nil {
		groupName = c.Group.Name
	}
	planName := ""
	if c.RecordPlan != nil {
		planName = c.RecordPlan.Name
	}
	return response.GbChannelItem{
		ID:              c.ID,
		Active:          c.Active,
		DeviceID:        c.DeviceID,
		DeviceName:      deviceName,
		DeviceCode:      deviceCode,
		GbDeviceID:      c.GbDeviceID,
		RegionName:      regionName,
		GroupName:       groupName,
		GbChannelID:     c.GbChannelID,
		Name:            c.Name,
		Manufacturer:    c.Manufacturer,
		Model:           c.Model,
		Owner:           c.Owner,
		CivilCode:       c.CivilCode,
		Address:         c.Address,
		Parental:        c.Parental,
		ParentID:        c.ParentID,
		ChannelType:     c.ChannelType,
		PtzType:         c.PtzType,
		HasAudio:        c.HasAudio,
		Status:          c.Status,
		RegionID:        c.RegionID,
		GroupID:         c.GroupID,
		StreamID:        c.StreamID,
		RecordPlanID:    c.RecordPlanID,
		RecordPlanName:  planName,
		EnableBroadcast: c.EnableBroadcast,
		IsCustom:        c.IsCustom,
		SyncDT:          c.SyncDT,
		Remark:          c.Remark,
		CreatedDT:       c.CreatedDT,
		UpdatedDT:       c.UpdatedDT,
	}
}

// toGbMediaServerItem 把节点模型转为列表项，secret 脱敏。
func toGbMediaServerItem(m model.GbMediaServer) response.GbMediaServerItem {
	return response.GbMediaServerItem{
		ID:                m.ID,
		Active:            m.Active,
		Name:              m.Name,
		ServerID:          m.ServerID,
		Type:              m.Type,
		IP:                m.IP,
		HookIP:            m.HookIP,
		SDPIP:             m.SDPIP,
		StreamIP:          m.StreamIP,
		HTTPPort:          m.HTTPPort,
		HTTPSSLPort:       m.HTTPSSLPort,
		RTMPPort:          m.RTMPPort,
		RTMPSSLPort:       m.RTMPSSLPort,
		RTSPPort:          m.RTSPPort,
		RTSPSSLPort:       m.RTSPSSLPort,
		FLVPort:           m.FLVPort,
		FLVSSLPort:        m.FLVSSLPort,
		WSFLVPort:         m.WSFLVPort,
		WSFLVSSLPort:      m.WSFLVSSLPort,
		MP4Port:           m.MP4Port,
		MP4SSLPort:        m.MP4SSLPort,
		Secret:            maskItemSecret(m.Secret),
		RTPEnable:         m.RTPEnable,
		RTPPortRange:      m.RTPPortRange,
		RTPProxyPort:      m.RTPProxyPort,
		JTTProxyPort:      m.JTTProxyPort,
		SendRTPPortRange:  m.SendRTPPortRange,
		RecordAssistPort:  m.RecordAssistPort,
		RecordPath:        m.RecordPath,
		RecordDay:         m.RecordDay,
		TranscodeSuffix:   m.TranscodeSuffix,
		Online:            m.Online,
		LastKeepaliveDT:   m.LastKeepaliveDT,
		HookAliveInterval: m.HookAliveInterval,
		DefaultServer:     m.DefaultServer,
		AutoConfig:        m.AutoConfig,
		Remark:            m.Remark,
		CreatedDT:         m.CreatedDT,
		UpdatedDT:         m.UpdatedDT,
	}
}
