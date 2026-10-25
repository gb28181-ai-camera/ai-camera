package voiceagent_test

import (
	"errors"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
)

func TestContextSignAndVerify(t *testing.T) {
	signer := voiceagent.NewContextSigner("ctx-secret")
	chat := request.ChatContext{
		SessionID: "s1", UserID: 9, DeptID: 3, DeviceID: 11,
		Kind: "web", Scopes: []string{"voice"},
	}
	value, err := signer.Sign(chat)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	got, err := signer.Verify(value)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if got.UserID != 9 || got.DeptID != 3 || got.DeviceID != 11 || got.SessionID != "s1" {
		t.Errorf("上下文不一致: %+v", got)
	}
	if got.ExpiresAt <= time.Now().Unix() {
		t.Errorf("Exp 未自动填充: %+v", got)
	}

	if _, err := signer.Verify(value + "x"); !errors.Is(err, voiceagent.ErrContextInvalid) {
		t.Fatalf("篡改应失败，实际 %v", err)
	}

	past, _ := signer.Sign(request.ChatContext{SessionID: "s2", ExpiresAt: time.Now().Add(-time.Minute).Unix()})
	if _, err := signer.Verify(past); !errors.Is(err, voiceagent.ErrContextExpired) {
		t.Fatalf("过期应失败，实际 %v", err)
	}

	empty := voiceagent.NewContextSigner("")
	if _, err := empty.Sign(chat); !errors.Is(err, voiceagent.ErrContextSecretMissing) {
		t.Fatalf("空密钥签名应失败，实际 %v", err)
	}
	if _, err := empty.Verify(value); !errors.Is(err, voiceagent.ErrContextSecretMissing) {
		t.Fatalf("空密钥校验应失败，实际 %v", err)
	}
}
