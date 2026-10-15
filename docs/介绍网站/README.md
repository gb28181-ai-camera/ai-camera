# AI Camera 介绍网站

AI Camera 智能监控平台的产品介绍页（官网）。深色科技风单页站，用于对外说明产品定位、
能力矩阵、核心流程、应用场景与联系方式。

> 版本: v0.1.0
> 最后更新: 2026-09-21
> 代码位置: `frontend/src/website/`（与前端管理后台同属一个 npm 工程）

---

## 概述

官网是 `frontend/` 工程的**第二个构建入口**，与管理后台共用同一份 `package.json`、
`node_modules` 与 `src/shared/` 工具代码，但拥有独立的：

- HTML 入口 `frontend/website.html`
- Vite 配置 `frontend/vite.website.config.ts`（`base: '/'`，产物 `dist-website/`，dev 端口 5174）
- 样式文件 `frontend/src/website/index.css`（纯 CSS 变量主题）

| 项 | 管理后台 | 官网介绍页 |
|:---|:---|:---|
| 入口 HTML | `index.html` | `website.html` |
| `base` | `/admin/` | `/` |
| 产物目录 | `dist/` | `dist-website/` |
| 开发端口 | 5173 | 5174 |
| 后端依赖 | 有（`/web`、`/app` 代理） | **无**（纯静态，不调任何 API） |
| 构建命令 | `npm run build:admin` | `npm run build:website` |

> **注意**：官网不依赖后端，因此 `vite.website.config.ts` **不配置 proxy**。
> 官网中的「登录」入口为跳转到管理后台域的绝对链接（`/admin/`）。

---

## 技术栈

| 项 | 选型 | 说明 |
|:---|:---|:---|
| 框架 | React 19 | 函数组件 + Hooks |
| 语言 | TypeScript 5.6 | 与工程统一（`strict` 模式） |
| 构建 | Vite 5.4 | 独立配置 `vite.website.config.ts` |
| 样式方案 | 纯 CSS（CSS 变量 + 原生嵌套） | **不引入 CSS-in-JS / Tailwind / Less** |
| 视觉实现 | CSS 渐变 + 内联 SVG | **不依赖任何位图资源**（无 `.png` / `.jpg` / 字体图标） |
| 图表 | ECharts 5 | 由 `echarts-for-react` 包装（与管理后台共用依赖） |
| 路由 | 无（单页锚点） | 导航为页内锚点跳转（`#capability`、`#solution`、`#scenario`、`#contact`） |
| 响应式 | 原生媒体查询 + 移动端抽屉导航 | 桌面 / 平板 / 手机三档断点 |
| 动效 | CSS `transition` + `IntersectionObserver` 滚动显现 | `.reveal` 类配合滚动进入视口触发 |

---

## 项目结构

```
frontend/src/website/
├── main.tsx                    # 挂载入口（ReactDOM.createRoot）
├── App.tsx                     # 组件装配（按区块顺序组装）
├── index.css                   # 官网全部样式（设计令牌 + 各区块样式）
├── pages/
│   └── Home.tsx                # 首页：按区块组织各 Section 组件
└── components/
    ├── Header.tsx              # 顶部导航（品牌 + 主导航 + 行动按钮 + 移动端抽屉）
    ├── HeroSection.tsx         # 首屏：主标题 + 副标题 + 行动按钮 + 监控台视觉 + 关键指标
    ├── CapabilitySection.tsx   # 平台能力：从识别到处置的完整能力矩阵
    ├── SolutionSection.tsx     # 核心流程：八步完成现场风险闭环
    ├── ScenarioSection.tsx     # 应用场景：适配需要持续监管的现场（痛点 / 方案）
    ├── StatsSection.tsx        # 平台亮点：数据留在平台，处置落在人
    ├── ContactSection.tsx      # 联系我们：预约平台演示表单
    └── Footer.tsx              # 页脚：产品 / 快速链接 / 联系方式 / 版权

frontend/
├── website.html                # 官网 HTML 入口（base /）
├── vite.website.config.ts      # 官网构建配置
├── Dockerfile.website          # 官网独立镜像
└── nginx.website.conf          # 官网独立 Nginx 配置
```

---

## 页面结构

```
┌───────────────────────────────────────────────────────────────────────────┐
│  Header（吸顶）                                                            │
│  ┌────────────┐  ┌──────────────────────────────┐  ┌──────────────────┐  │
│  │ AI Camera   │  │ 平台能力 · 解决方案 · 应用场景 │  │  登录  预约演示   │  │
│  │ 品牌标识    │  │ · 平台亮点 · 联系我们          │  │  （行动按钮）      │  │
│  └────────────┘  └──────────────────────────────┘  └──────────────────┘  │
│  移动端：折叠为抽屉导航（mobile-drawer）                                   │
├───────────────────────────────────────────────────────────────────────────┤
│  HeroSection（首屏）                                                       │
│  ┌──────────────────────────────┐  ┌──────────────────────────────────┐  │
│  │  AI 智能监控平台              │  │  监控台视觉（纯 CSS + SVG 绘制）   │  │
│  │  （主标题，渐变文字）          │  │  ┌────────────────────────────┐  │  │
│  │                              │  │  │ 实时监控 · 4 宫格     LIVE │  │  │
│  │  副标题：AI 摄像头 · 边缘识别  │  │  ├──────────┬─────────────────┤  │  │
│  │         · 云端管理            │  │  │ 画面占位  │ 画面占位        │  │  │
│  │                              │  │  ├──────────┼─────────────────┤  │  │
│  │  [预约演示]  [了解更多]        │  │  │ 画面占位  │ 画面占位        │  │  │
│  │                              │  │  └──────────┴─────────────────┘  │  │
│  │  ┌────────┬────────┬────────┐│  └──────────────────────────────────┘  │
│  │  │在线设备│待处理风险│今日巡检││                                        │
│  │  │ 指标   │ 指标    │ 任务  ││                                        │
│  │  └────────┴────────┴────────┘│                                        │
│  └──────────────────────────────┘                                        │
├───────────────────────────────────────────────────────────────────────────┤
│  CapabilitySection（平台能力）                                             │
│  标题：平台能力  ·  副标题：从识别到处置的完整能力矩阵                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                    │
│  │ SVG 图标  │ │ SVG 图标  │ │ SVG 图标  │ │ SVG 图标  │                    │
│  │ 能力卡片  │ │ 能力卡片  │ │ 能力卡片  │ │ 能力卡片  │                    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘                    │
├───────────────────────────────────────────────────────────────────────────┤
│  SolutionSection（核心流程）                                               │
│  标题：核心流程  ·  副标题：八步完成现场风险闭环                            │
│  ┌────┐   ┌────┐   ┌────┐   ┌────┐   ┌────┐   ┌────┐   ┌────┐   ┌────┐ │
│  │ 01 │ → │ 02 │ → │ 03 │ → │ 04 │ → │ 05 │ → │ 06 │ → │ 07 │ → │ 08 │ │
│  └────┘   └────┘   └────┘   └────┘   └────┘   └────┘   └────┘   └────┘ │
│  每步含 flow-index（序号）/ flow-title（标题）/ flow-desc（说明）          │
├───────────────────────────────────────────────────────────────────────────┤
│  ScenarioSection（应用场景）                                               │
│  标题：应用场景  ·  副标题：适配需要持续监管的现场                          │
│  ┌──────────────────────────┐  ┌──────────────────────────┐             │
│  │ 场景卡片                  │  │ 场景卡片                  │             │
│  │  · 痛点：……               │  │  · 痛点：……               │             │
│  │  · 方案：……               │  │  · 方案：……               │             │
│  └──────────────────────────┘  └──────────────────────────┘             │
├───────────────────────────────────────────────────────────────────────────┤
│  StatsSection（平台亮点）                                                  │
│  标题：平台亮点  ·  副标题：数据留在平台，处置落在人                        │
│  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐               │
│  │ 风险处置率       │ │ 指标            │ │ 指标            │               │
│  │（示例数据，动画） │ │                 │ │                 │               │
│  └────────────────┘ └────────────────┘ └────────────────┘               │
├───────────────────────────────────────────────────────────────────────────┤
│  ContactSection（联系我们）                                                │
│  标题：联系我们  ·  副标题：预约一次平台演示                                │
│  ┌───────────────────────────────────────────────────────────────────┐   │
│  │  表单：需求描述（textarea）+ 提交预约                               │   │
│  │  提交后切换为「预约申请已提交」，提供「再次填写」                      │   │
│  │  底部声明：提交即表示同意我们通过上述方式与您联系                       │   │
│  └───────────────────────────────────────────────────────────────────┘   │
├───────────────────────────────────────────────────────────────────────────┤
│  Footer（页脚）                                                            │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐                          │
│  │ 产品        │ │ 快速链接    │ │ 联系方式    │                          │
│  │ （锚点导航） │ │            │ │            │                          │
│  └────────────┘ └────────────┘ └────────────┘                          │
│  AI 摄像头 · 边缘识别 · 云端管理                                           │
│  © 2026 Boticz AI Camera. All rights reserved.                            │
└───────────────────────────────────────────────────────────────────────────┘
```

> **注意**：「平台亮点」区块中的**风险处置率等数字为示例数据**，仅用于官网展示，
> 不代表任何真实运营数据。正式对外发布前需替换为经确认的文案或删除该区块。

---

## 设计风格

深色科技风 + 科技蓝点缀。全部设计令牌定义在 `frontend/src/website/index.css` 的 `:root` 中。

| 属性 | 值 | 说明 |
|:---|:---|:---|
| 主题 | 深色（Dark） | 底色 `#0a0f1c`，与 AI 监控产品的行业调性一致 |
| 背景色 | `#0a0f1c` | 主背景（`--color-bg`） |
| 抬升背景 | `#111a2e` | 卡片 / 面板（`--color-bg-elevated`） |
| 玻璃面 | `rgba(255,255,255,0.04)` / `rgba(255,255,255,0.07)` | `--color-surface` / `--color-surface-strong` |
| **主色** | **`#2b7fff`** | `--color-primary`，品牌主色（科技蓝） |
| 主色亮阶 | `#5aa2ff` | `--color-primary-light` |
| 主色深阶 | `#1a5fd0` | `--color-primary-deep` |
| **点缀色** | **`#22d3ee`** | `--color-accent`，青蓝点缀（渐变末端、图标高亮、LIVE 标记） |
| 正文色 | `#e8eefc` | `--color-text` |
| 次级文字 | `#94a3b8` | `--color-text-secondary` |
| 弱化文字 | `#64748b` | `--color-text-muted` |
| 边框 | `rgba(255,255,255,0.1)` / `rgba(255,255,255,0.18)` | `--color-border` / `--color-border-strong` |
| 状态色 | `#34d399`（成功）/ `#fbbf24`（警告）/ `#f87171`（危险） | `--color-success` / `--color-warning` / `--color-danger` |
| 内容最大宽度 | `1200px` | `--max-width` |
| 圆角 | `8px` / `12px` / `20px` / 胶囊 `999px` | `--radius-sm|md|lg|pill` |
| 阴影 | 卡片 `0 4px 24px rgba(0,0,0,.35)`；悬停 `0 12px 40px rgba(0,0,0,.45)`；辉光 `0 8px 32px rgba(43,127,255,.28)` | `--shadow-card` / `--shadow-hover` / `--shadow-glow` |
| 顶栏高度 | `68px` | `--header-height`（吸顶导航） |
| 过渡曲线 | `cubic-bezier(0.4, 0, 0.2, 1)`，`0.3s ease` | `--ease` / `--transition` |

### 渐变体系

| 令牌 | 值 | 用途 |
|:---|:---|:---|
| `--gradient-primary` | `linear-gradient(135deg, #2b7fff 0%, #22d3ee 100%)` | 主行动按钮、品牌标识、强调元素 |
| `--gradient-heading` | `linear-gradient(120deg, #e8eefc 0%, #9dc4ff 55%, #22d3ee 100%)` | 大标题渐变文字（`background-clip: text`） |
| `--gradient-panel` | `linear-gradient(160deg, rgba(43,127,255,.14) 0%, rgba(255,255,255,.02) 45%, rgba(34,211,238,.08) 100%)` | 卡片 / 面板玻璃质感底色 |
| `--grid-line` | `rgba(43,127,255,0.055)` | 首屏背景网格线（科技感底纹） |

---

## 视觉实现方案

官网的**全部视觉效果均由 CSS 渐变与内联 SVG 实现，不依赖任何位图资源**。这带来三个好处：

| 收益 | 说明 |
|:---|:---|
| **零外部资源请求** | 无 `.png` / `.jpg` / `.webp` / 图标字体文件；首屏无图片加载瀑布，LCP 更优 |
| **天然高清** | SVG 与渐变在任意 DPR（含 4K 屏）下无锯齿，无需 @2x / @3x 多倍图 |
| **主题可编程** | 颜色全部走 CSS 变量，改一处令牌即可整体换色；暗色/亮色主题可加开关切换 |

具体实现手法：

| 视觉元素 | 实现方式 |
|:---|:---|
| **首屏背景网格** | `linear-gradient` 双层 + `background-size` 平铺，颜色取自 `--grid-line` |
| **首屏光晕 / 辉光** | `radial-gradient` 叠加，配合 `filter: blur()` |
| **监控台视觉** | 纯 HTML 结构 + CSS 绘制：窗口条（`monitor-bar`）、状态点（`monitor-dots`）、4 宫格分割线；「LIVE」标记为 CSS 动画闪烁红点 |
| **品牌标识** | 内联 `<svg>` 图形 + `--gradient-primary` 填充 |
| **区块图标** | 内联 `<svg>` 描边路径（`stroke="currentColor"`、`stroke-linejoin="round"`），随文字颜色自适应 |
| **大标题渐变文字** | `background: var(--gradient-heading)` + `background-clip: text` + `color: transparent` |
| **卡片玻璃质感** | `background: var(--gradient-panel)` + `border: 1px solid var(--color-border)` + `backdrop-filter: blur()` |
| **卡片悬停动效** | `transform: translateY(-4px)` + 阴影由 `--shadow-card` 切到 `--shadow-hover` |
| **滚动显现（reveal）** | `.reveal` 初始 `opacity: 0` + `translateY(24px)`；`IntersectionObserver` 进入视口后添加类名触发过渡 |
| **流程图连接线** | 伪元素 `::after` 绘制渐变横线（`--gradient-primary`） |
| **移动端导航抽屉** | `mobile-drawer` + `drawer-nav`，纯 CSS `transform` 位移 + `transition` 开合 |

> **注意**：新增视觉元素时**不得引入位图资源**（包括图标字体）。图标一律用内联 SVG，
> 装饰图形一律用 CSS 渐变 —— 这是官网的性能与可维护性底线。
> 若确有品牌位图需求（如客户 Logo），需与负责人确认后再引入，并显式设置宽高以避免布局抖动。

---

## 启动命令

```bash
cd frontend

# ── 安装依赖（与管理后台共用）──
npm install

# ── 官网开发模式（默认 http://localhost:5174，自动打开浏览器）──
npm run dev:website

# ── 官网构建（产物 dist-website/，base /）──
npm run build:website

# ── 同时构建管理后台 + 官网 ──
npm run build

# ── 预览构建产物 ──
npm run preview
```

`vite.website.config.ts` 关键配置：

```typescript
// frontend/vite.website.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// 官网介绍页（Web）构建：入口 website.html，base /，产物 dist-website/
// 官网不依赖后端，因此不配置 proxy。
export default defineConfig({
  base: '/',
  plugins: [react()],
  define: {
    __IS_ELECTRON__: 'false',
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '~': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist-website',
    emptyOutDir: true,
    rollupOptions: {
      input: path.resolve(__dirname, 'website.html'),
    },
  },
  server: {
    port: 5174,
    open: true,
  },
})
```

---

## 构建与部署

### 构建产物

```
frontend/dist-website/
├── index.html                 # 由 website.html 生成，资源路径以 / 为根
└── assets/
    ├── index-<hash>.js        # 打包后的 JS（含 React 运行时）
    └── index-<hash>.css       # 官网全部样式（单文件）
```

> **注意**：官网产物 `base` 为 `/`，管理后台产物 `base` 为 `/admin/`，**两者不可互换部署**。

### 部署方式一：独立官网镜像

```bash
cd frontend && docker build -f Dockerfile.website -t ai-camera-website .
cd frontend && docker run -p 8081:80 ai-camera-website
```

```nginx
# frontend/nginx.website.conf
server {
    listen 80;
    server_name www.ai-camera.com;

    root /usr/share/nginx/html;
    index index.html;

    # 官网为单页锚点站，回退到 index.html 即可
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 静态资源长缓存（Vite 产物带 hash，可安全长缓存）
    location /assets/ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }

    gzip on;
    gzip_types text/css application/javascript image/svg+xml;
}
```

### 部署方式二：单镜像双站点（推荐）

`frontend/Dockerfile` + `frontend/nginx.conf` 让一个 nginx 容器同时托管官网与管理后台：

```bash
cd frontend && docker build -t ai-camera-frontend .
cd frontend && docker run -p 8080:8080 ai-camera-frontend
```

| 路径 | 内容 |
|:---|:---|
| `/` | 官网介绍页（`dist-website/` → 站点根） |
| `/admin/` | 管理后台（`dist/` → `/admin/` 子目录） |

### Kubernetes 部署

官网在集群中为独立 Deployment（`ai-camera-website`），由 Traefik 按 Host 路由：

```yaml
# k8s-apply-gitops/ai-camera-prod/ingress/ai-camera-ingressroute.yaml（节选）
- match: Host(`www.ai-camera.com`)
  kind: Rule
  services:
    - name: ai-camera-website
      port: 8080
```

详见 [../部署文档/01-部署与运维.md](../部署文档/01-部署与运维.md) 第 3.3 与 4.3 节。

### 部署检查清单

| 检查项 | 期望 |
|:---|:---|
| 静态资源路径 | 页面内引用形如 `/assets/index-xxx.js`（**不带** `/admin/` 前缀） |
| 锚点导航 | `#capability` / `#solution` / `#scenario` / `#contact` 均可正常跳转 |
| 登录入口 | 指向管理后台域（`/admin/`）且可正常打开 |
| 移动端布局 | 手机宽度下导航折叠为抽屉，各区块单列排布 |
| 控制台无报错 | 无 404 资源、无 JS 运行时错误 |
| 长缓存生效 | `assets/` 下资源响应头含 `Cache-Control: public, immutable` |

---

## 相关文档

- [../README.md](../README.md) — 仓库根说明
- [../frontend/README.md](../frontend/README.md) — 前端工程说明（双入口构建、API 代码生成、测试）
- [../部署文档/01-部署与运维.md](../部署文档/01-部署与运维.md) — 官网 Dockerfile 与 Nginx 配置、K8s IngressRoute
- [../需求文档/00-需求总览.md](../需求文档/00-需求总览.md) — 产品定位与能力边界（官网文案依据）
