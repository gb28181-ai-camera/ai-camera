package iot

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 公共常量、错误、凭据生成与校验
// ════════════════════════════════════════════════════════════════

// 业务错误：Handler 依据 errors.Is 分类返回 400 / 401 / 404 / 409。
var (
	ErrProductNotFound        = errors.New("IOT产品不存在")
	ErrProductCodeExists      = errors.New("产品编码已存在")
	ErrServiceGroupNotFound   = errors.New("服务组不存在")
	ErrServiceGroupCodeExists = errors.New("服务组编码已存在")
	ErrServiceGroupInUse      = errors.New("服务组已被产品引用，无法删除")
	ErrDeviceNotFound         = errors.New("设备不存在")
	ErrAccountNotFound        = errors.New("设备账号不存在")
	ErrOwnerExists            = errors.New("设备主账号已存在")
	ErrAlreadyActivated       = errors.New("设备已激活，请勿重复激活")
	ErrOwnerDeleteForbidden   = errors.New("主账号不允许删除")

	// 设备二维码 / SIP 绑定相关。
	//
	// 二维码不含密钥，扫码即视为持有设备；SIP 侧则在 REGISTER 白名单未命中时
	// 用「产品编码 + 设备编码 + 国标编码」做一次显式绑定。两者的共同前提是
	// 产品与设备必须匹配，否则会把设备错挂到另一个产品下。
	ErrDeviceProductMismatch = errors.New("设备不属于该产品")
	ErrGbDeviceIDConflict    = errors.New("设备已绑定其他国标编码，禁止改绑")
	ErrGbDeviceIDRequired    = errors.New("国标编码不能为空")
	ErrBindQRInvalid         = errors.New("设备二维码无效：设备不存在或产品不匹配")

	// 设备端鉴权失败统一 401。刻意不区分「设备不存在」与「密钥错误」，
	// 避免攻击者用不同的错误文案枚举设备编号。
	ErrDeviceCredentialMissing = errors.New("缺少设备接入凭据")
	ErrDeviceSecretNotSet      = errors.New("设备未配置接入密钥，请先在平台重置")
	ErrDeviceSecretInvalid     = errors.New("设备接入密钥无效")
)

// credentialAlphabet 凭据字符集：排除易混淆的 0 O 1 l I。
//
// 这些用户名/密码需要人工抄录或扫码输入，混淆字符会显著提高支持成本。
const credentialAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// usernameAlphabet 用户名用小写字母 + 数字，便于日志与命令行输入。
const usernameAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// protocolDefaultPorts 服务项未填端口时的协议默认值。
var protocolDefaultPorts = map[string]int{
	model.IotProtocolSIP:       5060,
	model.IotProtocolRTSP:      554,
	model.IotProtocolWebRTC:    443,
	model.IotProtocolMQTT:      1883,
	model.IotProtocolWebSocket: 443,
}

// randomFrom 从字符集中等概率取 n 个字符。
//
// 使用 crypto/rand 而不是 math/rand：凭据一旦可预测，设备端鉴权就形同虚设。
func randomFrom(alphabet string, n int) (string, error) {
	max := big.NewInt(int64(len(alphabet)))
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[idx.Int64()])
	}
	return b.String(), nil
}

// GenerateServiceGroupPassword 生成 16 位服务组密码。
func GenerateServiceGroupPassword() (string, error) {
	return randomFrom(credentialAlphabet, 16)
}

// generateUsername 生成带前缀的随机用户名，并保证在服务组表中唯一。
func generateUsername(ctx context.Context, repo *repository.IotServiceGroupRepo, prefix string, excludeID int64) (string, error) {
	for i := 0; i < 10; i++ {
		suffix, err := randomFrom(usernameAlphabet, 10)
		if err != nil {
			return "", err
		}
		name := prefix + suffix
		exists, err := repo.ExistsByUsername(ctx, name, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return name, nil
		}
	}
	return "", errors.New("生成服务组用户名失败，请重试")
}

// generateAccountUsername 生成带前缀的随机设备账号登录名，并保证全局唯一。
func generateAccountUsername(ctx context.Context, repo *repository.IotDeviceRepo, prefix string) (string, error) {
	for i := 0; i < 10; i++ {
		suffix, err := randomFrom(usernameAlphabet, 10)
		if err != nil {
			return "", err
		}
		name := prefix + suffix
		exists, err := repo.ExistsAccountUsername(ctx, name, 0)
		if err != nil {
			return "", err
		}
		if !exists {
			return name, nil
		}
	}
	return "", errors.New("生成账号登录名失败，请重试")
}

// GenerateDeviceSecret 生成 48 位十六进制设备接入密钥（24 字节熵）。
func GenerateDeviceSecret() (string, error) {
	const hexAlphabet = "0123456789abcdef"
	return randomFrom(hexAlphabet, 48)
}

// validateGroupType 校验服务组类型。
func validateGroupType(t string) error {
	if _, ok := model.IotGroupAllowedProtocols[t]; !ok {
		return fmt.Errorf("服务组类型必须为 gb28181 / webrtc / aichat，当前为 %q", t)
	}
	return nil
}

// canonicalProtocol 把协议规范化为白名单中的标准写法，并校验是否属于该服务组类型。
func canonicalProtocol(groupType, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	for _, p := range model.IotGroupAllowedProtocols[groupType] {
		if strings.EqualFold(p, raw) {
			return p, nil
		}
	}
	return "", fmt.Errorf("协议 %q 不属于服务组类型 %s（允许：%s）",
		raw, groupType, strings.Join(model.IotGroupAllowedProtocols[groupType], "、"))
}

// validateNetConfigMode 校验产品配网方式。
func validateNetConfigMode(mode string) error {
	switch mode {
	case model.IotNetConfigBluetooth, model.IotNetConfigAP, model.IotNetConfigQRCode:
		return nil
	default:
		return fmt.Errorf("配网方式必须为 bluetooth / ap / qrcode，当前为 %q", mode)
	}
}

// validateAccountRole 校验设备账号角色。
func validateAccountRole(role string) error {
	switch role {
	case model.IotAccountRoleOwner, model.IotAccountRoleShared:
		return nil
	default:
		return fmt.Errorf("账号角色必须为 owner / shared，当前为 %q", role)
	}
}

// validateAccountStatus 校验设备账号状态。
func validateAccountStatus(status string) error {
	switch status {
	case model.IotAccountStatusActive, model.IotAccountStatusDisabled:
		return nil
	default:
		return fmt.Errorf("账号状态必须为 active / disabled，当前为 %q", status)
	}
}

// maskGroup 复制并脱敏服务组密码。
//
// 必须复制而不是原地修改：调用方可能仍需要明文（例如设备端 OTA），
// 原地改会把同一个实体在内存里的明文抹掉。
func maskGroup(g model.IotServiceGroup) model.IotServiceGroup {
	g.Password = utils.MaskSecret(g.Password)
	return g
}

// maskGroups 批量脱敏。
func maskGroups(items []model.IotServiceGroup) []model.IotServiceGroup {
	out := make([]model.IotServiceGroup, 0, len(items))
	for i := range items {
		out = append(out, maskGroup(items[i]))
	}
	return out
}

// maskDevice 复制并脱敏设备接入密钥。
//
// 同样必须复制：OTA 响应与鉴权逻辑仍要使用原值。
func maskDevice(d model.Device) model.Device {
	d.DeviceSecret = utils.MaskSecret(d.DeviceSecret)
	return d
}

// secureEqual 常量时间比较设备密钥，避免通过响应时间侧信道逐字节猜测。
func secureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// createOwnerAccount 创建设备主账号（Web 激活与设备端激活共用）。
//
// 这里的「激活」本质就是创建 role=owner 的账号：
// 设备首次联网后需要一个能登录平台/服务的身份，平台不预先分配，
// 而是在激活时生成并一次性返回明文密码，避免默认口令长期存在。
//
// event 决定写入的事件类型：Web 手动激活记为 activate，
// 通过账号接口创建时记为 account-create。
func createOwnerAccount(
	ctx context.Context,
	devices *repository.IotDeviceRepo,
	device *model.Device,
	nickname, phone, event, ip string,
) (*model.IotDeviceAccount, string, error) {
	username, err := generateAccountUsername(ctx, devices, "owner_")
	if err != nil {
		return nil, "", err
	}
	password, err := randomFrom(credentialAlphabet, 16)
	if err != nil {
		return nil, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", errors.New("密码加密失败")
	}
	if strings.TrimSpace(nickname) == "" {
		nickname = "主账号"
	}
	now := time.Now()
	account := &model.IotDeviceAccount{
		DeviceID:     device.ID,
		Username:     username,
		PasswordHash: string(hash),
		Role:         model.IotAccountRoleOwner,
		Nickname:     nickname,
		Phone:        strings.TrimSpace(phone),
		Status:       model.IotAccountStatusActive,
		ActivatedAt:  &now,
	}
	if err := devices.CreateAccount(ctx, account); err != nil {
		return nil, "", err
	}
	if err := devices.SetActivatedAt(ctx, device.ID, now); err != nil {
		return nil, "", err
	}
	_ = devices.CreateEvent(ctx, &model.IotDeviceEvent{
		DeviceID: device.ID,
		Event:    event,
		Detail:   fmt.Sprintf(`{"username":%q,"nickname":%q}`, username, nickname),
		IP:       ip,
	})
	return account, password, nil
}
