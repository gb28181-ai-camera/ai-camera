const STEPS = [
  { title: '新建区域', desc: '按项目现场划分区域，区域是设备归属与数据权限的基本单位。' },
  { title: '新增设备', desc: '登记设备编号、名称与所属区域，绑定物联网卡并配置设备工作模式。' },
  { title: '配置风险类型', desc: '按现场场景自定义风险名称与等级，决定设备上报风险的归类方式。' },
  { title: '语音播报配置', desc: '为风险类型配置现场语音提示内容，识别到风险时设备端即时播报。' },
  { title: '实时监控', desc: '通过单屏 / 4 宫格 / 9 宫格查看实时画面，按区域与摄像头筛选。' },
  { title: '风险处置', desc: '查看风险图片与录像，复核检测准确性后填写处理结果与处理图片。' },
  { title: '巡检复核', desc: '派发巡检任务至移动端，现场核实并回传结果，形成人工补充闭环。' },
  { title: '统计复盘', desc: '按区域、类型、时间聚合风险与告警数据，输出分析结论与改进依据。' },
]

/** 解决方案：区域 → 设备 → 风险配置 → 实时监控 → 风险处置 → 巡检 → 复盘 */
export function SolutionSection() {
  return (
    <section className="section" id="solution">
      <div className="container">
        <div className="section-head reveal">
          <span className="section-eyebrow">核心流程</span>
          <h2 className="section-title">八步完成现场风险闭环</h2>
          <p className="section-subtitle">
            从区域建档到统计复盘，平台把设备端识别与人工处置串成一条可追溯的主线。
          </p>
        </div>

        <div className="solution-note reveal">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 8.2v4.4l2.8 1.7" strokeLinecap="round" />
          </svg>
          <span>
            风险上报后状态为「待处理」，安全员复核检测准确性：判为误报可一键归档；
            判为真实风险则填写处理结果与处理图片，状态转为「已处理」，处理人与处理时间自动记录。
          </span>
        </div>

        <ol className="flow">
          {STEPS.map((step, index) => (
            <li className="flow-step reveal" key={step.title}>
              <span className="flow-index">{index + 1}</span>
              <h3 className="flow-title">{step.title}</h3>
              <p className="flow-desc">{step.desc}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  )
}

export default SolutionSection
