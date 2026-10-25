# Jessibuca 第三方资源许可说明

本目录下的文件来自 **Jessibuca**：

- 上游项目：<https://github.com/langhuihui/jessibuca>
- 作者：dexter langhuihui
- 本目录文件：`jessibuca.js`、`decoder.js`、`decoder.wasm`、`jessibuca.d.ts`
- 许可证：**GPL-3.0**（GNU General Public License v3.0）

## 合规策略

Jessibuca 采用 GPL-3.0。本项目对它的使用方式为：

1. **动态加载**：仅在用户手动切换到 Jessibuca 页签时，由
   `src/admin/lib/jessibucaLoader.ts` 注入 `<script>`，不静态打包进主 bundle；
2. **不作为默认引擎**：单通道默认 WebRTC，`codec=H265` 默认 H265web，视频墙默认 HTTP-FLV；
3. **原样引用**：直接分发上游构建产物（`jessibuca.js` / `decoder.js` / `decoder.wasm`），
   未修改源码；
4. 许可证原文随上游发布物提供。若需在闭源分发场景使用，请先由法务评估 GPL-3.0 义务，
   或替换为其他许可兼容的播放器实现。

> 若未来升级上游版本，请同步更新本文件并保留上游许可证原文。
