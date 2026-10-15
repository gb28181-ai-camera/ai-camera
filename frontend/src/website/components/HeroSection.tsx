const HERO_METRICS = [
  { value: '95%+', label: '风险识别准确率' },
  { value: '7×24', label: '小时实时监控' },
  { value: '1 / 4 / 9', label: '宫格视频预览' },
  { value: '远程', label: '固件与模型升级' },
]

const CAM_TILES = [
  { label: '1号门 · 安全帽识别', box: 'a', tag: '未戴安全帽' },
  { label: '2号塔吊 · 区域入侵', box: 'b', tag: '人员闯入' },
  { label: '库区 A · 通道占用', box: '', tag: '' },
  { label: '北围墙 · 明火检测', box: '', tag: '' },
]

/** 首屏：主标题 + 副标题 + 双 CTA + 关键数据 + 监控画面示意 */
export function HeroSection() {
  return (
    <section className="hero">
      <div className="hero-bg" />
      <div className="hero-grid" />

      <div className="container hero-inner">
        <div className="hero-copy">
          <span className="hero-pill">
            <span className="dot" />
            AI 摄像头 · 边缘识别 · 云端管理
          </span>

          <h1 className="hero-title">
            AI 智能监控平台
          </h1>

          <p className="hero-subtitle">
            AI 摄像头 + 边缘识别 + 云端管理，让风险看得见、管得住。
            设备端实时识别风险并自动留证，平台侧完成复核、处置与统计复盘，
            同时统一运维设备状态、物联网卡与固件版本。
          </p>

          <div className="hero-actions">
            <a className="btn btn-primary btn-lg" href="/admin/">
              进入控制台
            </a>
            <a className="btn btn-ghost btn-lg" href="#contact">
              预约演示
            </a>
          </div>

          <div className="hero-metrics">
            {HERO_METRICS.map((item) => (
              <div className="metric" key={item.label}>
                <div className="metric-value">{item.value}</div>
                <div className="metric-label">{item.label}</div>
              </div>
            ))}
          </div>
        </div>

        <div className="hero-visual" aria-hidden="true">
          <div className="monitor">
            <div className="monitor-bar">
              <span className="monitor-dots">
                <i />
                <i />
                <i />
              </span>
              <span className="monitor-title">实时监控 · 4 宫格</span>
              <span className="monitor-live">LIVE</span>
            </div>

            <div className="monitor-screen">
              {CAM_TILES.map((tile) => (
                <div className="cam-tile" key={tile.label}>
                  <span className="cam-scan" />
                  {tile.box ? (
                    <span className={`cam-box cam-box--${tile.box}`}>
                      <span className="cam-box-tag">{tile.tag}</span>
                    </span>
                  ) : null}
                  <span className="cam-tile-label">{tile.label}</span>
                </div>
              ))}
            </div>

            <div className="monitor-foot">
              <div className="monitor-stat">
                <strong>12</strong>
                <span>在线设备</span>
              </div>
              <div className="monitor-stat">
                <strong>3</strong>
                <span>待处理风险</span>
              </div>
              <div className="monitor-stat">
                <strong>7</strong>
                <span>今日巡检任务</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

export default HeroSection
