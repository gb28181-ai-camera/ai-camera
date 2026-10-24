package turn

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// TestSignRFCVector 用固定向量钉死 HMAC-SHA1 + 标准 Base64 的算法。
//
// 向量来自 openssl：
//
//	printf '%s' '1700000000:user1' | openssl dgst -sha1 -hmac 'test-secret' -binary | base64
//
// 这套算法一旦改成 SHA256 或 URLEncoding，客户端会稳定 401，
// 而 coturn 日志只有 Unauthorized，现场极难定位。
func TestSignRFCVector(t *testing.T) {
	got := sign("test-secret", "1700000000:user1")
	want := "Ld+toGHZwHpMqzdy3ekdai6SrQk="
	if got != want {
		t.Fatalf("HMAC 向量不匹配: got %q want %q", got, want)
	}
	if strings.ContainsAny(got, "-_") {
		t.Fatalf("必须是标准 Base64（不能是 URLEncoding）: %q", got)
	}
	if _, err := base64.StdEncoding.DecodeString(got); err != nil {
		t.Fatalf("不是合法 Base64: %v", err)
	}
}

// TestIssueFormat 凭据格式必须是 "<过期时间戳>:<subject>"。
func TestIssueFormat(t *testing.T) {
	t.Setenv(EnvSecret, "test-secret")
	t.Setenv(EnvRealm, "camera")
	t.Setenv(EnvURLs, "stun:turn:3478, turn:turn:3478?transport=udp")

	cred, err := New().Issue("device-001", 600)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	parts := strings.SplitN(cred.Username, ":", 2)
	if len(parts) != 2 || parts[1] != "device-001" {
		t.Fatalf("username 格式错误: %q", cred.Username)
	}
	if cred.Password != sign("test-secret", cred.Username) {
		t.Fatalf("密码与 username 不匹配")
	}
	if cred.Realm != "camera" {
		t.Fatalf("realm = %q", cred.Realm)
	}
	if len(cred.URLs) != 2 || cred.URLs[0] != "stun:turn:3478" {
		t.Fatalf("URL 列表解析错误: %+v", cred.URLs)
	}
	if cred.TTL != 600 {
		t.Fatalf("ttl = %d", cred.TTL)
	}
}

// TestIssueValidation 未配置密钥 / 非法 subject 必须明确报错。
func TestIssueValidation(t *testing.T) {
	t.Setenv(EnvSecret, "")
	if _, err := New().Issue("u", 60); err != ErrSecretNotConfigured {
		t.Fatalf("未配置密钥应返回 ErrSecretNotConfigured，实际 %v", err)
	}

	t.Setenv(EnvSecret, "s")
	for _, subject := range []string{"", "a:b", "a,b"} {
		if _, err := New().Issue(subject, 60); err != ErrSubjectInvalid {
			t.Fatalf("subject=%q 应返回 ErrSubjectInvalid，实际 %v", subject, err)
		}
	}
}

// TestIssueTTLClamp TTL 必须被限制在 60–86400。
func TestIssueTTLClamp(t *testing.T) {
	t.Setenv(EnvSecret, "s")
	cred, err := New().Issue("u", 1)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if cred.TTL != minTTL {
		t.Fatalf("过小 TTL 应夹到 %d，实际 %d", minTTL, cred.TTL)
	}
	cred, err = New().Issue("u", maxTTL+1000)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if cred.TTL != maxTTL {
		t.Fatalf("过大 TTL 应夹到 %d，实际 %d", maxTTL, cred.TTL)
	}
}

// TestConfigured 未配置密钥时 Configured 为 false。
func TestConfigured(t *testing.T) {
	t.Setenv(EnvSecret, "")
	if New().Configured() {
		t.Fatal("未设置密钥时 Configured 应为 false")
	}
	_ = os.Setenv(EnvSecret, "x")
	if !New().Configured() {
		t.Fatal("设置密钥后 Configured 应为 true")
	}
}
