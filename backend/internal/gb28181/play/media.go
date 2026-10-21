package play

import (
	"context"
	"fmt"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// HTTPBase 返回 ZLM 的 HTTP 基址（如 http://10.0.0.5:8082）。
//
// 前端契约 §15.1 要求同时返回整串播放地址与 http_base：
// 用整串最省事，而视频墙组件可能只想拿 http_base + app + stream 自己拼。
//
// 地址优先级与 BuildPlayURLs 保持一致：stream_ip（对外播放地址）优先于 ip（API 地址），
// 空了才回落到 127.0.0.1。端口同 BuildPlayURLs 的 HTTP 端口语义。
func HTTPBase(ms *model.GbMediaServer) string {
	if ms == nil {
		return ""
	}
	host := strings.TrimSpace(ms.StreamIP)
	if host == "" {
		host = strings.TrimSpace(ms.IP)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://"), "/")
	port := ms.HTTPPort
	if port <= 0 {
		port = 80
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

// ResolveMediaServer 选择本次点播使用的媒体节点。
//
// 选择顺序：
//  1. 设备显式绑定的节点（ac_device.media_server_id，且不是 "auto"）；
//  2. 显式绑定查不到时回落到默认节点。
//
// 为什么绑定失效要回落而不是直接报错：节点被下线/改名时，若直接失败，
// 整片设备会一起不可播；回落到默认节点至少能继续出图，排障时看日志即可。
func ResolveMediaServer(ctx context.Context, dev *model.Device) (*mediaserver.Client, *model.GbMediaServer, error) {
	if dev != nil {
		id := strings.TrimSpace(dev.MediaServerID)
		if id != "" && !strings.EqualFold(id, "auto") {
			client, ms, err := mediaserver.FromServerID(ctx, id)
			if err == nil && ms != nil {
				return client, ms, nil
			}
		}
	}

	client, ms, err := mediaserver.FromDefaultServer(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("%w：%v", ErrMediaServerUnavailable, err)
	}
	if ms == nil {
		return nil, nil, ErrMediaServerUnavailable
	}
	return client, ms, nil
}
