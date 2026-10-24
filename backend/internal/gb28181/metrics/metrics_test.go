package metrics

import (
	"strings"
	"testing"
)

// TestWriteMetric_TextFormat 校验 Prometheus 文本格式的骨架：
// HELP / TYPE / 标签 / 数值四部分齐全，且顺序稳定。
func TestWriteMetric_TextFormat(t *testing.T) {
	m := newMetric(
		"aicamera_gb_test_total",
		"测试计数器",
		kindCounter,
		[]string{"result"},
		[]string{"success"},
		[]string{"failed"},
	)
	m.inc("success")
	m.inc("success")
	m.inc("failed")

	var b strings.Builder
	writeMetric(&b, m)
	got := b.String()

	wantLines := []string{
		"# HELP aicamera_gb_test_total 测试计数器",
		"# TYPE aicamera_gb_test_total counter",
		`aicamera_gb_test_total{result="success"} 2`,
		`aicamera_gb_test_total{result="failed"} 1`,
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line+"\n") {
			t.Errorf("输出缺少行 %q，实际输出:\n%s", line, got)
		}
	}

	// 预置的顺序必须稳定：success 在 failed 之前，不能被 map 迭代打乱。
	if strings.Index(got, `result="success"`) > strings.Index(got, `result="failed"`) {
		t.Errorf("标签序列顺序不稳定，实际输出:\n%s", got)
	}
}

// TestWriteMetric_GaugeAndNoLabels 无标签 gauge 的输出不能带 `{}`。
func TestWriteMetric_GaugeAndNoLabels(t *testing.T) {
	m := newMetric("aicamera_gb_test_gauge", "测试仪表", kindGauge, nil)
	m.set(7)
	m.add(3)

	var b strings.Builder
	writeMetric(&b, m)
	got := b.String()

	if !strings.Contains(got, "# TYPE aicamera_gb_test_gauge gauge\n") {
		t.Errorf("TYPE 应为 gauge，实际:\n%s", got)
	}
	if !strings.Contains(got, "aicamera_gb_test_gauge 10\n") {
		t.Errorf("gauge 数值应为 10，实际:\n%s", got)
	}
	if strings.Contains(got, "aicamera_gb_test_gauge{") {
		t.Errorf("无标签指标不应输出花括号，实际:\n%s", got)
	}
}

// TestEscapeLabel 标签值里的反斜杠、引号、换行必须转义，否则 /metrics 无法解析。
func TestEscapeLabel(t *testing.T) {
	cases := map[string]string{
		`plain`:        `plain`,
		`a\b`:          `a\\b`,
		`a"b`:          `a\"b`,
		"a\nb":         `a\nb`,
		`a\b"c` + "\n": `a\\b\"c\n`,
	}
	for in, want := range cases {
		if got := escapeLabel(in); got != want {
			t.Errorf("escapeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAddNonNegative 活跃数仪表不允许出现负数：回调乱序时钳到 0。
func TestAddNonNegative(t *testing.T) {
	m := newMetric("aicamera_gb_test_streams", "测试活跃流", kindGauge, nil)
	m.addNonNegative(-1)
	if got := m.series().value.Load(); got != 0 {
		t.Fatalf("减到负数后应钳为 0，实际 %d", got)
	}
	m.addNonNegative(2)
	m.addNonNegative(-1)
	if got := m.series().value.Load(); got != 1 {
		t.Fatalf("期望 1，实际 %d", got)
	}
}

// TestWrongArityIgnored 标签数量不匹配时静默忽略，绝不能 panic。
func TestWrongArityIgnored(t *testing.T) {
	m := newMetric("aicamera_gb_test_arity", "测试参数校验", kindCounter, []string{"result"})
	m.inc()         // 少一个
	m.inc("a", "b") // 多一个
	if len(m.order) != 0 {
		t.Fatalf("错误参数不应创建序列，实际 %d 条", len(m.order))
	}
}

// TestRender_ContainsRequiredMetrics 全局快照至少包含任务要求的全部指标名。
func TestRender_ContainsRequiredMetrics(t *testing.T) {
	IncSipRegister("success")
	IncSipMessage("in")
	AddActiveStreams(1)
	IncHookCallback("server_keepalive")
	AddVoiceSessions(1)
	IncDiagnose("ok")
	AddSipLogDeleted(5)
	SetSipLogDropped(0)

	out := Render()
	required := []string{
		"aicamera_gb_sip_registers_total",
		"aicamera_gb_sip_messages_total",
		"aicamera_gb_active_streams",
		"aicamera_gb_hook_callbacks_total",
		"aicamera_gb_voice_sessions",
		"aicamera_gb_process_start_time_seconds",
		"aicamera_gb_diagnoses_total",
		"aicamera_gb_sip_log_deleted_total",
	}
	for _, name := range required {
		if !strings.Contains(out, "# TYPE "+name+" ") {
			t.Errorf("Render 缺少指标 %s，实际输出:\n%s", name, out)
		}
	}
}
