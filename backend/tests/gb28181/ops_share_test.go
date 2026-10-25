package gb28181_test

// ════════════════════════════════════════════════════════════════
// 运维增强：通道免登录分享令牌
//
// 覆盖：生成 / 列表 / 校验 / 吊销 / 过期 / 跨通道吊销拒绝，
// 以及「库中只存 hash、明文不可反查」这一安全不变量。
// ════════════════════════════════════════════════════════════════

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

func TestOpsShareTokenLifecycle(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := createDevice(t, ctx, "CAM_SHARE_001", "分享设备", "34020000001320009001")
		ch := createChannel(t, ctx, dev, "34020000001320009001", "分享通道", "secret")

		svc := gbsvc.NewGbShareService()
		created, err := svc.Create(ctx, ch.ID, &request.GbShareTokenCreateReq{Remark: "给访客"})
		if err != nil {
			t.Fatalf("创建分享令牌失败: %v", err)
		}
		if !strings.HasPrefix(created.Token, "gbs_") || len(created.Token) < 40 {
			t.Fatalf("令牌格式不正确: %q", created.Token)
		}
		if created.ExpiresDT.Before(time.Now().Add(50 * time.Minute)) {
			t.Fatalf("默认 TTL 应约 1 小时，实际过期时间 %v", created.ExpiresDT)
		}
		if created.TokenPrefix != created.Token[:12] {
			t.Fatalf("前缀应为明文前 12 位: %q vs %q", created.TokenPrefix, created.Token[:12])
		}

		// 库中只存 hash：明文不能出现在任何列中。
		var row model.GbShareToken
		if err := tx.First(&row, created.ID).Error; err != nil {
			t.Fatalf("读取令牌记录失败: %v", err)
		}
		if row.TokenHash == created.Token || strings.Contains(row.TokenHash, created.Token) {
			t.Fatalf("库中不应保存明文令牌")
		}
		if len(row.TokenHash) != 64 {
			t.Fatalf("SHA-256 hex 长度应为 64，实际 %d", len(row.TokenHash))
		}

		// 列表只返回未过期未吊销的令牌。
		items, err := svc.List(ctx, ch.ID, allScope())
		if err != nil {
			t.Fatalf("列出令牌失败: %v", err)
		}
		if len(items) != 1 || items[0].ID != created.ID {
			t.Fatalf("令牌列表不正确: %+v", items)
		}

		// 匿名信息：独立校验 hash，返回通道与设备名称。
		info, err := svc.Info(ctx, created.Token)
		if err != nil {
			t.Fatalf("校验分享令牌失败: %v", err)
		}
		if info.ChannelID != ch.ID || info.ChannelName != "分享通道" || info.DeviceName != "分享设备" {
			t.Fatalf("分享信息不正确: %+v", info)
		}
		if len(info.Protocols) != 3 {
			t.Fatalf("可用协议应包含 webrtc/flv/hls: %+v", info.Protocols)
		}
		if info.ExpiresDT.IsZero() {
			t.Fatalf("分享信息应返回过期时间")
		}

		// 伪造令牌不通过。
		if _, err := svc.Info(ctx, "gbs_this-is-not-a-real-token-1234567890"); err != gbsvc.ErrGbShareTokenInvalid {
			t.Fatalf("伪造令牌应返回 ErrGbShareTokenInvalid，实际 %v", err)
		}

		// 吊销后立即失效，且幂等。
		if err := svc.Revoke(ctx, ch.ID, created.ID); err != nil {
			t.Fatalf("吊销令牌失败: %v", err)
		}
		if _, err := svc.Info(ctx, created.Token); err != gbsvc.ErrGbShareTokenRevoked {
			t.Fatalf("吊销后应返回 ErrGbShareTokenRevoked，实际 %v", err)
		}
		if err := svc.Revoke(ctx, ch.ID, created.ID); err != nil {
			t.Fatalf("重复吊销应幂等成功: %v", err)
		}
		items, err = svc.List(ctx, ch.ID, allScope())
		if err != nil {
			t.Fatalf("吊销后列出令牌失败: %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("吊销后列表应为空，实际 %d", len(items))
		}
	})
}

func TestOpsShareTokenExpiryAndChannelIsolation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := createDevice(t, ctx, "CAM_SHARE_002", "分享设备2", "34020000001320009002")
		ch1 := createChannel(t, ctx, dev, "34020000001320009002", "通道一", "")
		ch2 := createChannel(t, ctx, dev, "34020000001320009003", "通道二", "")

		svc := gbsvc.NewGbShareService()
		created, err := svc.Create(ctx, ch1.ID, &request.GbShareTokenCreateReq{TTLSeconds: 60})
		if err != nil {
			t.Fatalf("创建短 TTL 令牌失败: %v", err)
		}

		// TTL 下限与上限：<60 拒绝，>86400 截断为 24h。
		if _, err := svc.Create(ctx, ch1.ID, &request.GbShareTokenCreateReq{TTLSeconds: 10}); err == nil {
			t.Fatalf("TTL 小于 60 秒应被拒绝")
		}
		long, err := svc.Create(ctx, ch1.ID, &request.GbShareTokenCreateReq{TTLSeconds: 999999})
		if err != nil {
			t.Fatalf("创建长 TTL 令牌失败: %v", err)
		}
		if long.ExpiresDT.After(time.Now().Add(24*time.Hour + time.Minute)) {
			t.Fatalf("TTL 应被截断为 24 小时，实际 %v", long.ExpiresDT)
		}

		// 跨通道吊销必须拒绝。
		if err := svc.Revoke(ctx, ch2.ID, created.ID); err != gbsvc.ErrGbShareTokenInvalid {
			t.Fatalf("跨通道吊销应返回 ErrGbShareTokenInvalid，实际 %v", err)
		}

		// 把过期时间改到过去，匿名校验应过期。
		past := time.Now().Add(-time.Minute)
		if err := tx.Model(&model.GbShareToken{}).Where("id = ?", created.ID).
			Update("expires_dt", past).Error; err != nil {
			t.Fatalf("修改过期时间失败: %v", err)
		}
		if _, err := svc.Info(ctx, created.Token); err != gbsvc.ErrGbShareTokenExpired {
			t.Fatalf("过期令牌应返回 ErrGbShareTokenExpired，实际 %v", err)
		}

		// 通道不存在。
		if _, err := svc.Create(ctx, 99999999, &request.GbShareTokenCreateReq{}); err != gbsvc.ErrGbShareChannelNotFound {
			t.Fatalf("不存在的通道应返回 ErrGbShareChannelNotFound，实际 %v", err)
		}
	})
}
