const PRODUCT_LINKS = [
  { href: '#features', label: '平台能力' },
  { href: '#solution', label: '核心流程' },
  { href: '#scenario', label: '应用场景' },
  { href: '#stats', label: '平台亮点' },
]

const QUICK_LINKS = [
  { href: '/admin/', label: '进入控制台' },
  { href: '#contact', label: '预约演示' },
]

const CONTACT_LINES = ['sales@boticz.com', 'support@boticz.com', '周一至周五 09:00 - 18:00']

/** 页脚：产品导航、快速链接、联系方式、版权 */
export function Footer() {
  return (
    <footer className="site-footer">
      <div className="container">
        <div className="footer-inner">
          <div className="footer-col footer-col--brand">
            <div className="footer-brand">
              <span className="brand-mark">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
                  <path d="M3 8.5A2.5 2.5 0 0 1 5.5 6h1.7l1.1-1.8h7.4L16.8 6h1.7A2.5 2.5 0 0 1 21 8.5v8A2.5 2.5 0 0 1 18.5 19h-13A2.5 2.5 0 0 1 3 16.5z" />
                  <circle cx="12" cy="12.5" r="3.4" />
                </svg>
              </span>
              <span className="brand-text">
                <strong>AI Camera</strong>
                <em>AI 智能监控平台</em>
              </span>
            </div>
            <p className="footer-desc">
              以 AI 摄像头为采集终端，提供实时视频监控、风险识别与处置、设备与物联网卡管理、
              巡检中心、统计分析及远程升级能力。
            </p>
          </div>

          <div className="footer-col">
            <h4 className="footer-title">产品</h4>
            <ul className="footer-list">
              {PRODUCT_LINKS.map((link) => (
                <li key={link.href}>
                  <a className="footer-link" href={link.href}>
                    {link.label}
                  </a>
                </li>
              ))}
            </ul>
          </div>

          <div className="footer-col">
            <h4 className="footer-title">快速链接</h4>
            <ul className="footer-list">
              {QUICK_LINKS.map((link) => (
                <li key={link.href}>
                  <a
                    className={link.href.startsWith('/') ? 'footer-link footer-link--cta' : 'footer-link'}
                    href={link.href}
                  >
                    {link.label}
                    {link.href.startsWith('/') ? (
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                        <path d="M5 12h13M12.6 6.2 18.4 12l-5.8 5.8" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    ) : null}
                  </a>
                </li>
              ))}
            </ul>
          </div>

          <div className="footer-col footer-col--contact">
            <h4 className="footer-title">联系方式</h4>
            <ul className="footer-list">
              {CONTACT_LINES.map((line) => (
                <li key={line}>
                  <span className="footer-link">{line}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <div className="footer-bottom">
          <span className="footer-copy">© 2026 Boticz AI Camera. All rights reserved.</span>
          <span className="footer-meta">AI 摄像头 · 边缘识别 · 云端管理</span>
        </div>
      </div>
    </footer>
  )
}

export default Footer
