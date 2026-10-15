import { useEffect, useState } from 'react'

const NAV_LINKS = [
  { href: '#features', label: '平台能力' },
  { href: '#solution', label: '核心流程' },
  { href: '#scenario', label: '应用场景' },
  { href: '#stats', label: '平台亮点' },
  { href: '#contact', label: '联系我们' },
]

function BrandMark() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
      <path d="M3 8.5A2.5 2.5 0 0 1 5.5 6h1.7l1.1-1.8h7.4L16.8 6h1.7A2.5 2.5 0 0 1 21 8.5v8A2.5 2.5 0 0 1 18.5 19h-13A2.5 2.5 0 0 1 3 16.5z" />
      <circle cx="12" cy="12.5" r="3.4" />
    </svg>
  )
}

/** 固定顶部导航：毛玻璃背景 + 锚点链接 + 移动端汉堡菜单 */
export function Header() {
  const [scrolled, setScrolled] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 12)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  // 抽屉展开时锁定页面滚动
  useEffect(() => {
    document.body.style.overflow = menuOpen ? 'hidden' : ''
    return () => {
      document.body.style.overflow = ''
    }
  }, [menuOpen])

  const closeMenu = () => setMenuOpen(false)

  return (
    <header className={`site-header${scrolled ? ' is-scrolled' : ''}`}>
      <div className="container header-inner">
        <a className="brand" href="#top" onClick={closeMenu}>
          <span className="brand-mark">
            <BrandMark />
          </span>
          <span className="brand-text">
            <strong>AI Camera</strong>
            <em>AI 智能监控平台</em>
          </span>
        </a>

        <nav className="header-nav" aria-label="主导航">
          {NAV_LINKS.map((link) => (
            <a key={link.href} className="nav-link" href={link.href}>
              {link.label}
            </a>
          ))}
        </nav>

        <div className="header-actions">
          <a className="btn btn-primary btn-sm" href="/admin/">
            进入控制台
          </a>
          <button
            type="button"
            className={`menu-toggle${menuOpen ? ' is-open' : ''}`}
            aria-label={menuOpen ? '关闭导航菜单' : '打开导航菜单'}
            aria-expanded={menuOpen}
            aria-controls="mobile-drawer"
            onClick={() => setMenuOpen((open) => !open)}
          >
            <span />
            <span />
            <span />
          </button>
        </div>
      </div>

      <div id="mobile-drawer" className={`mobile-drawer${menuOpen ? ' is-open' : ''}`}>
        <nav className="drawer-nav" aria-label="移动端导航">
          {NAV_LINKS.map((link) => (
            <a key={link.href} className="drawer-link" href={link.href} onClick={closeMenu}>
              {link.label}
            </a>
          ))}
          <a className="btn btn-primary drawer-cta" href="/admin/" onClick={closeMenu}>
            进入控制台
          </a>
        </nav>
      </div>
    </header>
  )
}

export default Header
