package voiceagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// EmbeddingDim 本项目默认向量维度（BGE-M3）。
const EmbeddingDim = 1024

// EmbeddingClient OpenAI-compatible /v1/embeddings 客户端。
//
// 未配置 endpoint 时 Embed 返回 ErrEmbeddingNotConfigured；
// 调用方据此把 embedding_status 置为 failed，但不阻塞记忆写入。
type EmbeddingClient struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

// ErrEmbeddingNotConfigured embedding 服务未配置。
var ErrEmbeddingNotConfigured = errors.New("未配置 Embedding 端点")

// NewEmbeddingClient 构造客户端。
func NewEmbeddingClient(endpoint, model, apiKey string) *EmbeddingClient {
	return &EmbeddingClient{
		endpoint: strings.TrimSpace(endpoint),
		model:    strings.TrimSpace(model),
		apiKey:   strings.TrimSpace(apiKey),
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Configured 是否已配置。
func (c *EmbeddingClient) Configured() bool { return c != nil && c.endpoint != "" }

// Embed 调用 /v1/embeddings。
func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	if !c.Configured() {
		return nil, ErrEmbeddingNotConfigured
	}
	url := c.endpoint
	if !strings.HasSuffix(url, "/embeddings") {
		if strings.HasSuffix(url, "/v1") {
			url += "/embeddings"
		} else {
			url = strings.TrimRight(url, "/") + "/v1/embeddings"
		}
	}
	model := c.model
	if model == "" {
		model = "bge-m3"
	}
	body, err := json.Marshal(map[string]interface{}{
		"model": model,
		"input": text,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding 服务返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析 embedding 响应失败: %w", err)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, errors.New("embedding 响应为空")
	}
	vec := out.Data[0].Embedding
	if len(vec) != EmbeddingDim {
		return nil, fmt.Errorf("embedding 维度 %d 与表定义 %d 不一致", len(vec), EmbeddingDim)
	}
	return vec, nil
}

// vectorLiteral 把向量转换为 pgvector 文本格式：[0.1,0.2,...]。
func vectorLiteral(vec []float32) string {
	if len(vec) == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(vec) * 10)
	b.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// vectorFromLiteral 解析 pgvector 文本（测试用）。
func vectorFromLiteral(s string) []float32 {
	s = strings.TrimSpace(strings.Trim(s, "[]"))
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil
		}
		out = append(out, float32(v))
	}
	return out
}
