package version

import "testing"

// 任意写法都必须归一，未知值安全回落到 2016。
func Test2022VersionNormalize(t *testing.T) {
	cases := map[string]Version{
		"":                V2016,
		"   ":             V2016,
		"2016":            V2016,
		"2016版":           V2016,
		"GB/T 28181-2016": V2016,
		"2022":            V2022,
		"v2022":           V2022,
		"GB/T 28181-2022": V2022,
		"GB28181—2022":    V2022,
		"28181-2022":      V2022,
		"2.0":             V2016, // 不认识 → 安全默认
		"unknown":         V2016,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// 能力表：Catalog Event 两个版本都有；2022 新能力不能对 2016 放行。
func Test2022VersionSupports(t *testing.T) {
	if !Supports(V2016, FeatureCatalogEvent) {
		t.Fatalf("Catalog Event 是 2016 已有能力")
	}
	if !Supports(V2022, FeatureCatalogEvent) {
		t.Fatalf("Catalog Event 在 2022 必须继续支持")
	}

	features2022 := []Feature{
		FeaturePTZPosition, FeaturePTZPositionNotify,
		FeatureCruiseTrackList, FeatureCruiseTrack,
		FeatureHomePosition, FeaturePTZPreciseCtrl, FeatureSDCardStatus,
		FeatureVideoH265, FeatureAudioG7221, FeatureAudioAAC,
	}
	for _, f := range features2022 {
		if Supports(V2016, f) {
			t.Errorf("2016 不应支持 %q", f)
		}
		if !Supports(V2022, f) {
			t.Errorf("2022 应支持 %q", f)
		}
		if !Is2022Only(f) {
			t.Errorf("%q 应判定为 2022 独有", f)
		}
	}

	// 未知能力按白名单语义返回 false。
	if Supports(V2022, Feature("no_such_feature")) {
		t.Fatalf("未知能力应为 false")
	}
	// 未知版本按 2016 处理。
	if Supports(Version("9.9"), FeaturePTZPosition) {
		t.Fatalf("未知版本应回落 2016，不应支持 2022 能力")
	}
}

// 收到 2022 独有命令才升级；2016 命令/未知命令保持原值。
func Test2022VersionUpgradeOnCommand(t *testing.T) {
	cases := []struct {
		current Version
		cmd     string
		want    Version
	}{
		{V2016, CommandPTZPosition, V2022},
		{V2016, CommandCruiseTrackList, V2022},
		{V2016, CommandCruiseTrack, V2022},
		{V2016, CommandHomePosition, V2022},
		{V2016, CommandSDCardStatus, V2022},
		{V2016, CommandStorageCardStatus, V2022}, // 非标准别名也识别
		{V2016, CommandPTZPreciseCtrl, V2022},    // Control 子元素
		{V2016, "Catalog", V2016},
		{V2016, "DeviceControl", V2016},
		{V2016, "NoSuchCommand", V2016},
		{V2022, "Catalog", V2022}, // 已是 2022 不降级
	}
	for _, c := range cases {
		if got := UpgradeOnCommand(c.current, c.cmd); got != c.want {
			t.Errorf("UpgradeOnCommand(%q, %q) = %q，期望 %q", c.current, c.cmd, got, c.want)
		}
	}
}

// 命令名映射必须覆盖任务书列出的全部 2022 命令。
func Test2022VersionFeatureOfCommand(t *testing.T) {
	must := map[string]Feature{
		CommandPTZPosition:     FeaturePTZPosition,
		CommandCruiseTrackList: FeatureCruiseTrackList,
		CommandCruiseTrack:     FeatureCruiseTrack,
		CommandHomePosition:    FeatureHomePosition,
		CommandSDCardStatus:    FeatureSDCardStatus,
		CommandPTZPreciseCtrl:  FeaturePTZPreciseCtrl,
	}
	for name, want := range must {
		got, ok := FeatureOfCommand(name)
		if !ok || got != want {
			t.Errorf("FeatureOfCommand(%q) = (%q, %v)，期望 (%q, true)", name, got, ok, want)
		}
	}
}
