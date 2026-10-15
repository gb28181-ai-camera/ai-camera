import type { ReactNode } from 'react'

type Capability = {
  icon: ReactNode
  title: string
  desc: string
  tags: string[]
}

const S = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.7,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

const IconShield = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M12 3 5 6v5.5c0 4.2 2.9 7.6 7 9.5 4.1-1.9 7-5.3 7-9.5V6z" />
    <path d="M9.2 12.2 11 14l3.8-4" />
  </svg>
)

const IconVideo = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <rect x="3" y="6" width="12" height="12" rx="2" />
    <path d="M15 10.5 21 8v8l-6-2.5z" />
  </svg>
)

const IconDevice = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M12 21s6-4.6 6-9.6A6 6 0 0 0 6 11.4C6 16.4 12 21 12 21z" />
    <circle cx="12" cy="11" r="2.4" />
  </svg>
)

const IconCard = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <rect x="7" y="3" width="10" height="18" rx="2" />
    <path d="M10 7.5h4M10 12h4M10 16.5h2" />
  </svg>
)

const IconInspect = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M8 4h8a2 2 0 0 1 2 2v13a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2z" />
    <path d="M9.2 3h5.6v2.6H9.2z" />
    <path d="M9.4 12.4 11 14l3.4-3.6" />
  </svg>
)

const IconUpgrade = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M12 20V7" />
    <path d="M7.6 11.4 12 7l4.4 4.4" />
    <path d="M4.8 20h14.4" />
  </svg>
)

const CAPABILITIES: Capability[] = [
  {
    icon: IconShield,
    title: 'AI 风险识别',
    desc: '设备端边缘推理实时识别现场风险，识别结果连同风险图片、录像与置信度一并上报，风险图片与录像自动留证，无需人工截图。',
    tags: ['边缘推理', '自动留证', '风险分级'],
  },
  {
    icon: IconVideo,
    title: '实时视频监控',
    desc: '支持单屏、4 宫格、9 宫格自由切换，可按区域与摄像头筛选预览画面，风险发生时可直接调取对应通道的实时视频。',
    tags: ['1/4/9 宫格', '按区域筛选', '多通道预览'],
  },
  {
    icon: IconDevice,
    title: '设备管理',
    desc: '统一的设备台账，支持 GIS 地图定位查看设备分布，实时掌握电量、存储与在线状态，并可远程切换设备模式、同步设备时间。',
    tags: ['设备台账', 'GIS 定位', '模式切换'],
  },
  {
    icon: IconCard,
    title: '物联网卡管理',
    desc: '按设备维护物联网卡信息，监控流量总额、已用与剩余流量，临近耗尽自动预警；换卡操作留痕，换卡记录可完整追溯。',
    tags: ['流量监控', '到期预警', '换卡记录'],
  },
  {
    icon: IconInspect,
    title: '巡检中心',
    desc: '创建巡检任务并指派巡检人员，任务下发至移动端，现场完成巡检后回传处理结果与图片，系统自动归档生成巡检记录。',
    tags: ['任务派发', '移动端处理', '记录归档'],
  },
  {
    icon: IconUpgrade,
    title: '远程升级',
    desc: '固件与 AI 模型统一版本管理，支持现在升级或定时升级，可按区域批量下发，升级进度与结果全程可回溯。',
    tags: ['固件升级', '模型升级', '批量下发'],
  },
]

/** 平台能力概览：六张能力卡 */
export function CapabilitySection() {
  return (
    <section className="section section--alt" id="features">
      <div className="container">
        <div className="section-head reveal">
          <span className="section-eyebrow">平台能力</span>
          <h2 className="section-title">从识别到处置的完整能力矩阵</h2>
          <p className="section-subtitle">
            围绕 AI 摄像头这一采集终端，平台提供视频、风险、设备、巡检、升级五个方向的闭环能力，
            覆盖现场监管的日常操作流程。
          </p>
        </div>

        <div className="cap-grid">
          {CAPABILITIES.map((item) => (
            <article className="cap-card reveal" key={item.title}>
              <span className="cap-icon">{item.icon}</span>
              <h3 className="cap-title">{item.title}</h3>
              <p className="cap-desc">{item.desc}</p>
              <div className="cap-tags">
                {item.tags.map((tag) => (
                  <span className="cap-tag" key={tag}>
                    {tag}
                  </span>
                ))}
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  )
}

export default CapabilitySection
