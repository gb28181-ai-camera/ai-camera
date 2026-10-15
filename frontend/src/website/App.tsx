import { useEffect } from 'react'
import { Header } from './components/Header'
import { Footer } from './components/Footer'
import { Home } from './pages/Home'

/**
 * 官网根组件：固定顶部导航 + 单页内容 + 页脚。
 * 滚动渐显在挂载后统一注册 IntersectionObserver，组件内只负责挂 `.reveal` class。
 */
export function App() {
  useEffect(() => {
    const targets = Array.from(document.querySelectorAll<HTMLElement>('.reveal'))

    if (targets.length === 0) {
      return
    }

    // 老浏览器兜底：直接展示，不做动画
    if (typeof IntersectionObserver === 'undefined') {
      targets.forEach((el) => el.classList.add('visible'))
      return
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) {
            return
          }
          entry.target.classList.add('visible')
          observer.unobserve(entry.target)
        })
      },
      {
        threshold: 0.12,
        rootMargin: '0px 0px -60px 0px',
      },
    )

    targets.forEach((el) => observer.observe(el))

    return () => observer.disconnect()
  }, [])

  return (
    <div className="app-shell" id="top">
      <Header />
      <main>
        <Home />
      </main>
      <Footer />
    </div>
  )
}

export default App
