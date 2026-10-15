const RADIUS = 54
const STROKE_WIDTH = 14
const CIRCUMFERENCE = 2 * Math.PI * RADIUS
const VIEW_BOX = 140
const CENTER = VIEW_BOX / 2

/** 示例数据：写死，仅用于官网展示平台统计能力；tone 对应 index.css 中的配色类 */
const SEGMENTS = [
  { label: '已处置风险', value: 92.6, tone: 'primary' },
  { label: '待处理风险', value: 5.1, tone: 'accent' },
  { label: '复核误报归档', value: 2.3, tone: 'muted' },
]

const HIGHLIGHTS = [
  {
    index: '01',
    title: '风险类型可自定义',
    desc: '按现场场景自定义风险名称与等级，无需等待平台版本更新即可适配新的监管要求。',
  },
  {
    index: '02',
    title: '支持批量远程升级',
    desc: '固件与 AI 模型按区域批量下发，可选现在升级或定时升级，逐个设备回溯升级结果。',
  },
  {
    index: '03',
    title: '可视化大屏',
    desc: '风险与设备告警数据聚合展示，适配大屏分辨率投放，用于值班室与指挥中心。',
  },
  {
    index: '04',
    title: '移动端协同',
    desc: '巡检任务派发到移动端，现场处理结果与图片即时回传，与 Web 端共用同一套数据。',
  },
]

function buildSegments() {
  let consumed = 0

  return SEGMENTS.map((segment) => {
    const length = (segment.value / 100) * CIRCUMFERENCE
    const dash = `${length} ${CIRCUMFERENCE - length}`
    const offset = -(consumed / 100) * CIRCUMFERENCE
    consumed += segment.value
    return { ...segment, dash, offset }
  })
}

/** 平台亮点：风险处置率环形图（SVG 纯手绘，无图表库）+ 四项能力亮点 */
export function StatsSection() {
  const segments = buildSegments()
  const handledRate = SEGMENTS[0].value

  return (
    <section className="section" id="stats">
      <div className="container">
        <div className="section-head reveal">
          <span className="section-eyebrow">平台亮点</span>
          <h2 className="section-title">数据留在平台，处置落在人</h2>
          <p className="section-subtitle">
            风险从上报到归档的每个环节都记录责任人与时间，统计口径统一、结果可复核。
          </p>
        </div>

        <div className="stats-layout">
          <div className="donut-card reveal">
            <div className="donut-head">风险处置率（示例数据）</div>

            <div className="donut-wrap">
              <svg
                className="donut-svg"
                viewBox={`0 0 ${VIEW_BOX} ${VIEW_BOX}`}
                role="img"
                aria-label={`风险处置率 ${handledRate}%`}
              >
                <g transform={`rotate(-90 ${CENTER} ${CENTER})`}>
                  <circle className="donut-track" cx={CENTER} cy={CENTER} r={RADIUS} strokeWidth={STROKE_WIDTH} />
                  {segments.map((segment) => (
                    <circle
                      key={segment.label}
                      className={`donut-seg donut-seg--${segment.tone}`}
                      cx={CENTER}
                      cy={CENTER}
                      r={RADIUS}
                      strokeWidth={STROKE_WIDTH}
                      strokeDasharray={segment.dash}
                      strokeDashoffset={segment.offset}
                    />
                  ))}
                </g>
              </svg>

              <div className="donut-center">
                <div className="donut-value">{handledRate}%</div>
                <div className="donut-label">风险处置率</div>
              </div>
            </div>

            <div className="donut-legend">
              {SEGMENTS.map((segment) => (
                <div className="legend-item" key={segment.label}>
                  <span className={`legend-dot legend-dot--${segment.tone}`} />
                  <span>{segment.label}</span>
                  <span className="legend-value">{segment.value}%</span>
                </div>
              ))}
            </div>
          </div>

          <div className="stats-caps">
            {HIGHLIGHTS.map((item) => (
              <article className="stats-item reveal" key={item.index}>
                <span className="stats-item-index">{item.index}</span>
                <h3 className="stats-item-title">{item.title}</h3>
                <p className="stats-item-desc">{item.desc}</p>
              </article>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}

export default StatsSection
