# h265web.js 第三方资源许可说明

本目录下的文件来自 **h265web.js**：

- 上游项目：<https://github.com/numberwolf/h265web.js>
- 作者：常炎隆（ChangYanlong / Numberwolf）
- 本目录文件：`h265web.js`、`h265web_wasm.js`、`h265web_wasm.wasm`、`extjs.js`、
  `extwasm.js`、`extwasm.wasm`
- 许可证：**CYL_Free-1.0**（Chang Yanlong Free License 1.0，自定义非标准许可证）

上游 README 与许可证文件为 `LICENSE-Free_CN.MD` / `LICENSE-Free_EN.MD`。该许可证要点
（以英文原文为准）：

- 允许使用项目全部功能与公开代码，包括商用与保留收益；
- 不允许把本项目**独立**出售（应集成到监控、直播平台等更高层业务中）；
- 不允许对本项目或其代码主张独立知识产权 / 专利 / 著作权；
- 禁止违法用途与恶意代码。

> 本仓库按上游分发形式**原样引用**这些静态资源，未做修改，也未将其打包进主 JS bundle；
> 通过 `src/admin/lib/h265webLoader.ts` 在用户切到 H265web 页签时按需动态加载。
>
> 若未来升级上游版本，请同步更新本文件并保留上游许可证原文。
