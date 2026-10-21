package ssrc

import (
	"strings"
	"testing"
)

func TestFormatAndAllocate(t *testing.T) {
	a := New("3402000000")

	first, err := a.Allocate(true)
	if err != nil {
		t.Fatalf("首次分配失败：%v", err)
	}
	if len(first) != SSRCWidth {
		t.Fatalf("SSRC %q 长度 = %d，期望 %d", first, len(first), SSRCWidth)
	}
	if first[0] != '0' {
		t.Fatalf("实时点播首位应为 0：%q", first)
	}
	if !strings.HasPrefix(first, "020000") {
		t.Fatalf("domain 前缀错误：%q（期望 0 + 20000 起）", first)
	}
	if first != "0200000000" {
		t.Fatalf("首个 SSRC = %q，期望 0200000000", first)
	}

	second, _ := a.Allocate(true)
	if second == first {
		t.Fatalf("连续两次分配不应相同：%q", second)
	}

	playback, _ := a.Allocate(false)
	if playback[0] != '1' {
		t.Fatalf("回放首位应为 1：%q", playback)
	}
	if playback == first || playback == second {
		t.Fatalf("回放与实时点播的 SSRC 不应冲突")
	}
}

func TestReleaseReusesSlot(t *testing.T) {
	a := New("3402000000")
	s1, _ := a.Allocate(true)
	s2, _ := a.Allocate(true)
	if !a.InUse(s1) || !a.InUse(s2) {
		t.Fatalf("刚分配的 SSRC 应处于占用状态")
	}

	a.Release(s1)
	if a.InUse(s1) {
		t.Fatalf("释放后不应仍占用")
	}
	// 轮转指针已指向 s2 之后，但其后还有大量空槽，这里只断言「不再占用」。
	if err := Validate(s1); err != nil {
		t.Fatalf("格式校验失败：%v", err)
	}
}

func TestPoolExhausted(t *testing.T) {
	a := New("3402000000")
	seen := map[string]bool{}
	for i := 0; i < SlotCount; i++ {
		s, err := a.Allocate(true)
		if err != nil {
			t.Fatalf("第 %d 次分配失败：%v", i+1, err)
		}
		if seen[s] {
			t.Fatalf("分配出重复 SSRC：%s", s)
		}
		seen[s] = true
	}
	if _, err := a.Allocate(true); err == nil {
		t.Fatalf("槽位耗尽后应返回错误")
	}
}

func TestHexStreamConversion(t *testing.T) {
	// 1000000001（十进制）→ 3B9ACA01（十六进制），与设计 §2.2 的实测值一致。
	hexName, err := ToHexStream("1000000001")
	if err != nil {
		t.Fatalf("ToHexStream 失败：%v", err)
	}
	if hexName != "3B9ACA01" {
		t.Fatalf("流名 = %q，期望 3B9ACA01", hexName)
	}

	back, err := FromHexStream(hexName)
	if err != nil {
		t.Fatalf("FromHexStream 失败：%v", err)
	}
	if back != "1000000001" {
		t.Fatalf("往返结果 = %q，期望 1000000001", back)
	}

	if _, err := FromHexStream("not-hex"); err == nil {
		t.Fatalf("非法流名应报错")
	}
	if _, err := ToHexStream("abc"); err == nil {
		t.Fatalf("非法 SSRC 应报错")
	}
}

func TestRebuildFromStreams(t *testing.T) {
	a := New("3402000000")

	// 外部已知一条流（例如平台重启前 ZLM 里已有的流）。
	hexName, _ := ToHexStream("0200000005")
	a.Rebuild([]string{hexName, "garbage"})

	if !a.InUse("0200000005") {
		t.Fatalf("Rebuild 后应标记外部流为占用")
	}

	// 分配时不能撞上已占用的槽位。
	for i := 0; i < 10; i++ {
		s, err := a.Allocate(true)
		if err != nil {
			t.Fatalf("分配失败：%v", err)
		}
		if s == "0200000005" {
			t.Fatalf("分配到了已被 Rebuild 标记的 SSRC")
		}
	}
}

func TestRebuildPreservesActive(t *testing.T) {
	a := New("3402000000")
	s, _ := a.Allocate(true)

	// 周期性 Rebuild 时 ZLM 可能还没列出刚建的流，本进程的分配不能被清掉。
	a.Rebuild(nil)
	if !a.InUse(s) {
		t.Fatalf("Rebuild 不应清掉本进程仍持有的分配：%s", s)
	}
}

func TestDomainPartFallback(t *testing.T) {
	a := New("123")
	s, err := a.Allocate(true)
	if err != nil {
		t.Fatalf("短域名分配失败：%v", err)
	}
	if len(s) != SSRCWidth {
		t.Fatalf("SSRC 长度应为 %d，实际 %d（%s）", SSRCWidth, len(s), s)
	}
}

func TestValidate(t *testing.T) {
	if err := Validate("0200000001"); err != nil {
		t.Fatalf("合法 SSRC 校验失败：%v", err)
	}
	for _, bad := range []string{"", "123456789", "12345678901", "abcdefghij"} {
		if err := Validate(bad); err == nil {
			t.Fatalf("非法 SSRC %q 应校验失败", bad)
		}
	}
}
