// Package push 通过个推（Getui）向 App 推送离线通知。
//
// 用途：设备呼叫 App 时，如果 App 没有在线 WebSocket（被系统回收/后台），
// 服务端发一条推送唤醒。推送只携带路由信息，不携带音频/视频。
//
// 配置来源：ac_thirdparty_config(Category='getui')，与「第三方平台 → 个推」页面一致。
// 未配置时返回 ErrNotConfigured，调用方只记录日志，不影响主流程。
package push

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

const (
	// CategoryGetui 配置分类（与 setting_service 保持一致）。
	CategoryGetui = "getui"
	// DefaultBaseURL 个推 REST API 地址。
	DefaultBaseURL = "https://restapi.getui.com/v2"
)

var (
	// ErrNotConfigured 未配置个推 AppID/AppKey/MasterSecret。
	ErrNotConfigured = errors.New("个推未配置")
	// ErrNoTokens 目标用户没有上报 CID。
	ErrNoTokens = errors.New("目标用户没有可用的推送 CID")
)

// GetuiConfig 个推配置。
type GetuiConfig struct {
	AppID        string
	AppKey       string
	AppSecret    string
	MasterSecret string
	BaseURL      string
}

// Configured 是否具备发起推送的最小配置。
func (c *GetuiConfig) Configured() bool {
	return c != nil && c.AppID != "" && c.AppKey != "" && c.MasterSecret != ""
}

// Service 个推推送服务。
type Service struct {
	repo   *repository.SettingRepo
	tokens *repository.PushTokenRepo
	client *http.Client
	now    func() time.Time

	mu        sync.Mutex
	cachedApp string
	cachedTok string
	cachedExp time.Time
}

// NewService 构造推送服务。
func NewService() *Service {
	return &Service{
		repo:   repository.NewSettingRepo(),
		tokens: repository.NewPushTokenRepo(),
		client: &http.Client{Timeout: 8 * time.Second},
		now:    time.Now,
	}
}

// LoadConfig 读取个推配置。
func (s *Service) LoadConfig(ctx context.Context) (*GetuiConfig, error) {
	rows, err := s.repo.ListThirdPartyConfigs(ctx, CategoryGetui)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Code] = row.Value
	}
	cfg := &GetuiConfig{
		AppID:        strings.TrimSpace(values[model.GetuiCfgAppID]),
		AppKey:       strings.TrimSpace(values[model.GetuiCfgAppKey]),
		AppSecret:    strings.TrimSpace(values[model.GetuiCfgAppSecret]),
		MasterSecret: strings.TrimSpace(values[model.GetuiCfgMasterSecret]),
		BaseURL:      DefaultBaseURL,
	}
	if !cfg.Configured() {
		return cfg, ErrNotConfigured
	}
	return cfg, nil
}

// PushToUsers 向一组用户推送通知；返回成功推送的 CID 数。
func (s *Service) PushToUsers(ctx context.Context, userIDs []int64, title, body string, payload map[string]interface{}) (int, error) {
	cfg, err := s.LoadConfig(ctx)
	if err != nil {
		return 0, err
	}
	tokens, err := s.tokens.ListByUserIDs(ctx, userIDs)
	if err != nil {
		return 0, err
	}
	if len(tokens) == 0 {
		return 0, ErrNoTokens
	}
	token, err := s.authToken(ctx, cfg)
	if err != nil {
		return 0, err
	}
	transmission := ""
	if len(payload) > 0 {
		if raw, err := json.Marshal(payload); err == nil {
			transmission = string(raw)
		}
	}
	pushed := 0
	var lastErr error
	for _, item := range tokens {
		if err := s.pushSingle(ctx, cfg, token, item.CID, title, body, transmission); err != nil {
			lastErr = err
			continue
		}
		pushed++
	}
	if pushed == 0 && lastErr != nil {
		return 0, lastErr
	}
	return pushed, nil
}

// authToken 获取并缓存个推 token。
func (s *Service) authToken(ctx context.Context, cfg *GetuiConfig) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.cachedTok != "" && s.cachedApp == cfg.AppID && now.Before(s.cachedExp) {
		return s.cachedTok, nil
	}
	ts := strconv.FormatInt(now.UnixMilli(), 10)
	sum := sha256.Sum256([]byte(cfg.AppKey + ts + cfg.MasterSecret))
	body, _ := json.Marshal(map[string]string{
		"sign": hex.EncodeToString(sum[:]), "timestamp": ts, "appkey": cfg.AppKey,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.BaseURL+"/"+cfg.AppID+"/auth", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("个推鉴权请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Token      string `json:"token"`
			ExpireTime int64  `json:"expire_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("个推鉴权响应解析失败: %w", err)
	}
	if out.Code != 0 || out.Data.Token == "" {
		return "", fmt.Errorf("个推鉴权失败 code=%d msg=%s", out.Code, out.Msg)
	}
	s.cachedApp = cfg.AppID
	s.cachedTok = out.Data.Token
	exp := time.UnixMilli(out.Data.ExpireTime)
	if exp.Before(now.Add(time.Minute)) {
		exp = now.Add(50 * time.Minute)
	}
	s.cachedExp = exp.Add(-time.Minute)
	return s.cachedTok, nil
}

func (s *Service) pushSingle(ctx context.Context, cfg *GetuiConfig, token, cid, title, body, transmission string) error {
	msg := map[string]interface{}{
		"request_id": newRequestID(),
		"audience":   map[string]interface{}{"cid": []string{cid}},
		"push_message": map[string]interface{}{
			"notification": map[string]interface{}{
				"title": title, "body": body, "click_type": "startapp",
			},
			"transmission": transmission,
		},
	}
	raw, _ := json.Marshal(msg)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.BaseURL+"/"+cfg.AppID+"/push/single/cid", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", token)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("个推推送请求失败: %w", err)
	}
	defer resp.Body.Close()
	respRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(respRaw, &out)
	if out.Code != 0 {
		return fmt.Errorf("个推推送失败 code=%d msg=%s", out.Code, out.Msg)
	}
	return nil
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(buf)
}
