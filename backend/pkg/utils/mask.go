package utils

import "strings"

// ════════════════════════════════════════════════════════════════
// 敏感字段脱敏（接口契约 §10.0-2）
//
// 国标模块有四个敏感字段：ac_device.sip_password、ac_gb_media_server.secret、
// ac_gb_platform.password、ac_gb_channel.password。
// 统一约定：
//   - 读接口返回 "****"，绝不回传明文；
//   - 写接口收到空值或含 "****" 的值时视为「不修改」，保留库中原值。
//
// 抽成工具函数而不是各 service 各写一遍，是因为「写回脱敏串」是极易漏掉的
// 一类缺陷：前端把读到的 **** 原样提交，若后端不识别就会把真实密码覆盖成星号。
// ════════════════════════════════════════════════════════════════

// MaskedValue 脱敏占位串。
const MaskedValue = "****"

// MaskSecret 对敏感值脱敏：空值仍为空，非空统一返回 ****。
//
// 不保留任何明文片段（与前 4 位 + **** 的算法不同），因为国标设备密码通常很短，
// 保留前缀会显著降低破解成本。
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	return MaskedValue
}

// IsMasked 判断值是否为脱敏占位串（或包含 ****）。
//
// 用 Contains 而非相等，是为了兼容前端可能拼接了多余字符的情况；
// 只要含 **** 就一律按「未修改」处理。
func IsMasked(s string) bool {
	return strings.Contains(s, MaskedValue)
}

// ApplyMaskedUpdate 计算敏感字段更新后的值。
//
// 返回规则：
//   - newVal 为空串，或含 ****（脱敏占位）→ 返回 oldVal（保持原值）；
//   - 否则返回 newVal（用户确实输入了新密码）。
//
// 注意：若业务上确实允许把密码清空，应显式传一个非空标记或单独的清空接口，
// 不要把「空串」当作清空 —— 否则前端未填密码就保存时会把设备密码抹掉。
func ApplyMaskedUpdate(newVal, oldVal string) string {
	if strings.TrimSpace(newVal) == "" || IsMasked(newVal) {
		return oldVal
	}
	return newVal
}
