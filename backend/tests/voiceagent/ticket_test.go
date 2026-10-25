package voiceagent_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
)

func TestTicketIssueVerifyOneTime(t *testing.T) {
	svc := voiceagent.NewTicketService("secret", 60*time.Second)
	token, claims, err := svc.Issue(voiceagent.TicketClaims{
		UserID: 42, UserName: "alice", RoleCode: "admin",
		DeptID: 7, DataPerm: "all", Kind: "web", ClientID: "c1",
	})
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if token == "" || claims.JTI == "" {
		t.Fatal("ticket 或 JTI 为空")
	}
	if !strings.Contains(token, ".") {
		t.Fatalf("ticket 格式异常: %q", token)
	}

	got, err := svc.Verify("web", token)
	if err != nil {
		t.Fatalf("首次校验失败: %v", err)
	}
	if got.UserID != 42 || got.DeptID != 7 || got.Kind != "web" || got.ClientID != "c1" {
		t.Errorf("claims 不一致: %+v", got)
	}
	if got.UserName != "alice" || got.DataPerm != "all" {
		t.Errorf("身份字段丢失: %+v", got)
	}

	// 重放
	if _, err := svc.Verify("web", token); !errors.Is(err, voiceagent.ErrTicketUsed) {
		t.Fatalf("重放应返回 ErrTicketUsed，实际 %v", err)
	}
	if svc.UsedCount() != 1 {
		t.Errorf("used 数量 = %d, 期望 1", svc.UsedCount())
	}
}

func TestTicketKindAndTamper(t *testing.T) {
	svc := voiceagent.NewTicketService("secret", 60*time.Second)
	token, _, err := svc.Issue(voiceagent.TicketClaims{UserID: 1, Kind: "web"})
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if _, err := svc.Verify("app", token); !errors.Is(err, voiceagent.ErrTicketKind) {
		t.Fatalf("kind 不匹配应失败，实际 %v", err)
	}
	// kind 校验发生在消费之前，原 kind 仍可用。
	if _, err := svc.Verify("web", token); err != nil {
		t.Fatalf("正确 kind 校验失败: %v", err)
	}

	svc2 := voiceagent.NewTicketService("secret", 60*time.Second)
	token2, _, _ := svc2.Issue(voiceagent.TicketClaims{UserID: 1, Kind: "web"})
	tampered := token2[:len(token2)-1] + "x"
	if tampered == token2 {
		tampered = token2[:len(token2)-2] + "yy"
	}
	if _, err := svc2.Verify("web", tampered); !errors.Is(err, voiceagent.ErrTicketInvalid) {
		t.Fatalf("篡改应返回 ErrTicketInvalid，实际 %v", err)
	}
	// 空密钥
	empty := voiceagent.NewTicketService("", 60*time.Second)
	if _, _, err := empty.Issue(voiceagent.TicketClaims{Kind: "web"}); err == nil {
		t.Fatal("空密钥签发应失败")
	}
	if _, err := empty.Verify("web", token2); !errors.Is(err, voiceagent.ErrTicketNotReady) {
		t.Fatalf("空密钥校验应返回 ErrTicketNotReady，实际 %v", err)
	}
}

func TestTicketExpiry(t *testing.T) {
	svc := voiceagent.NewTicketService("secret", 60*time.Second)
	// ticket 的 exp 为 Unix 秒，测试直接指定已过期时间，避免 sleep >=1s。
	token, _, err := svc.Issue(voiceagent.TicketClaims{
		UserID: 1, Kind: "web", ExpiresAt: time.Now().Add(-time.Second).Unix(),
	})
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if _, err := svc.Verify("web", token); !errors.Is(err, voiceagent.ErrTicketExpired) {
		t.Fatalf("过期应返回 ErrTicketExpired，实际 %v", err)
	}
}
