import type { ReactNode } from 'react'

type Scenario = {
  icon: ReactNode
  name: string
  pain: string
  fix: string
}

const S = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.7,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

const IconFactory = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M4 20V9.6l5-2.4v12.8" />
    <path d="M9 13.4 14.6 11v9H9" />
    <path d="M14.6 14.6 20 12.4V20h-5.4" />
    <path d="M3 20h18" />
  </svg>
)

const IconCrane = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M5 20V4h13" />
    <path d="M5 4 2.6 8h4.8z" />
    <path d="M12 4v5" />
    <path d="M12 9h3v3h-3z" />
    <path d="M3 20h18" />
  </svg>
)

const IconWarehouse = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M3 20V9.4L12 5l9 4.4V20z" />
    <path d="M8 20v-6h8v6" />
    <path d="M8 17h8" />
  </svg>
)

const IconHazard = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M12 3.4 21 19.6H3z" />
    <path d="M12 9.4v4.2M12 16.6v.2" />
  </svg>
)

const IconCity = (
  <svg viewBox="0 0 24 24" {...S} aria-hidden="true">
    <path d="M3 20h18" />
    <path d="M5 20V7l5-3v16" />
    <path d="M10 10h4l5 3v7h-9" />
    <path d="M7.6 9.4h.8M7.6 12.6h.8M7.6 15.8h.8" />
  </svg>
)

const SCENARIOS: Scenario[] = [
  {
    icon: IconFactory,
    name: '工业园区',
    pain: '厂区面积大、出入口多，安全帽佩戴与危险区域闯入难以靠人盯守。',
    fix: '重点区域部署 AI 摄像头，未戴安全帽、人员闯入自动识别并推送责任人处置。',
  },
  {
    icon: IconCrane,
    name: '建筑工地',
    pain: '塔吊与基坑作业面分散，违规操作与明火隐患发现滞后。',
    fix: '按作业面划分区域，明火与区域入侵实时告警，配合现场语音播报即时劝阻。',
  },
  {
    icon: IconWarehouse,
    name: '仓储物流',
    pain: '库区通道被占用、货物堆放越界，消防通道难以持续保持畅通。',
    fix: '通道占用与堆料越界识别自动留证，定时巡检复核，形成整改记录闭环。',
  },
  {
    icon: IconHazard,
    name: '危化场所',
    pain: '高危区域禁止无关人员进入，人工巡查存在安全风险且无法全时覆盖。',
    fix: '以设备替代人工巡查，闯入即识别并联动告警，录像与图片自动归档备查。',
  },
  {
    icon: IconCity,
    name: '城市管理',
    pain: '点位分散、事件类型繁杂，问题上报与处置进度难以统一跟踪。',
    fix: '按网格划分区域，自定义风险类型归类事件，处置进度与统计一并纳入平台。',
  },
]

/** 应用场景：五个典型行业场景，每个给出痛点与对应方案 */
export function ScenarioSection() {
  return (
    <section className="section section--alt" id="scenario">
      <div className="container">
        <div className="section-head reveal">
          <span className="section-eyebrow">应用场景</span>
          <h2 className="section-title">适配需要持续监管的现场</h2>
          <p className="section-subtitle">
            同一套平台能力，通过区域划分与风险类型自定义，适配不同行业的监管重点。
          </p>
        </div>

        <div className="scenario-grid">
          {SCENARIOS.map((item, index) => (
            <article className="scenario-card reveal" key={item.name}>
              <div className="scenario-head">
                <span className="scenario-icon">{item.icon}</span>
                <h3 className="scenario-name">{item.name}</h3>
                <span className="scenario-index">{String(index + 1).padStart(2, '0')}</span>
              </div>

              <div className="scenario-row">
                <span className="scenario-label scenario-label--pain">痛点</span>
                <span className="scenario-text">{item.pain}</span>
              </div>

              <div className="scenario-row">
                <span className="scenario-label scenario-label--fix">方案</span>
                <span className="scenario-text">{item.fix}</span>
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  )
}

export default ScenarioSection
